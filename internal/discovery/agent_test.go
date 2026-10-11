package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/db/dbtest"
	"github.com/udit-001/waypoint/internal/exa"
	"github.com/udit-001/waypoint/internal/zen"
)

// loopCaptor replays a script and captures the prompts it was handed.
type loopCaptor struct {
	fakeLoop
	onTurn func(system, user string)
}

func (f *loopCaptor) AgentLoop(ctx context.Context, system, user string, tools []zen.AgentTool, exec zen.AgentExec, maxTurns int) (string, int, error) {
	f.onTurn(system, user)
	return f.fakeLoop.AgentLoop(ctx, system, user, tools, exec, maxTurns)
}

// ---- test doubles ----------------------------------------------------------

// fakeLoop replays a script of tool calls, honoring the executor's done
// signal; a prose turn (toolName "") sends no tool call.
type fakeLoop struct {
	script []fakeTurn
	turns  int
}

type fakeTurn struct {
	tool string
	args string
}

func (f *fakeLoop) AgentLoop(_ context.Context, system, user string, tools []zen.AgentTool, exec zen.AgentExec, _ int) (string, int, error) {
	for _, turn := range f.script {
		f.turns++
		if turn.tool == "" {
			continue // prose turn: the real loop would nudge; script moves on
		}
		_, done := exec(turn.tool, turn.args)
		if done {
			return turn.args, f.turns, nil
		}
	}
	return "", f.turns, errors.New("exceeded turns without terminal call")
}

func fakeSearcher(calls *[]string, per map[string]string) *fakeSearch {
	return &fakeSearch{calls: calls, per: per}
}

type fakeSearch struct {
	calls *[]string
	per   map[string]string
}

func (f *fakeSearch) Search(_ context.Context, query string, _ int) ([]exa.CompanyHit, error) {
	if f.calls != nil {
		*f.calls = append(*f.calls, query)
	}
	if f.per != nil {
		if raw, ok := f.per[query]; ok {
			if raw == "" {
				return nil, errors.New("search down")
			}
			var hits []exa.CompanyHit
			_ = json.Unmarshal([]byte(raw), &hits)
			return hits, nil
		}
	}
	return []exa.CompanyHit{{Name: "Generic Co", URL: "https://generic.com"}}, nil
}

const submitValid = `{"companies":[{"name":"MongoDB","domain":"MongoDB.com","why":"data platform"},{"name":"","domain":"empty.com"},{"name":"Ghost","domain":""},{"name":"Ramp","domain":"https://www.ramp.com/about","why":"fintech"},{"name":"MongoDB again","domain":"mongodb.com"}],"clusters":["devtools","fintech"],"notes":["thin: fintech"]}`

// TestAgentUniverse_validatesSubmission: empty names and domains drop,
// domains dedup case-insensitively with www/URL stripping, and the
// surviving companies come back as one facet.
func TestAgentUniverse_validatesSubmission(t *testing.T) {
	var calls []string
	loop := &fakeLoop{script: []fakeTurn{
		{tool: "exa_search", args: `{"query":"devtools"}`},
		{tool: "submit_companies", args: submitValid},
	}}
	uni, err := AgentUniverse(context.Background(), "title: Backend Engineer\nskills: Go", loop, fakeSearcher(&calls, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(uni.Facets) != 1 {
		t.Fatalf("facets = %d, want 1", len(uni.Facets))
	}
	companies := uni.Facets[0].Companies
	if len(companies) != 2 {
		fatal := fmt.Sprintf("companies = %+v, want MongoDB + Ramp (empty dropped, dup domain merged)", companies)
		t.Fatal(fatal)
	}
	if companies[0].Name != "MongoDB" || companies[0].Domain != "mongodb.com" {
		t.Errorf("first = %+v, want MongoDB/mongodb.com normalized", companies[0])
	}
	if companies[1].Name != "Ramp" || companies[1].Domain != "ramp.com" {
		t.Errorf("second = %+v, want Ramp/ramp.com (URL-shaped domain reduced to host)", companies[1])
	}
	if len(calls) != 1 {
		t.Errorf("searches = %v, want 1", calls)
	}
}

// TestAgentUniverse_rejectsEmptyList: a submit with no usable companies
// is refused — the loop continues, and only exhaustion ends it.
func TestAgentUniverse_rejectsEmptyList(t *testing.T) {
	var calls []string
	loop := &fakeLoop{script: []fakeTurn{
		{tool: "submit_companies", args: `{"companies":[]}`},
		{tool: "submit_companies", args: `{"companies":[{"name":"X","domain":"x.com"}]}`},
	}}
	uni, err := AgentUniverse(context.Background(), "brief", loop, fakeSearcher(&calls, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(uni.Facets[0].Companies) != 1 {
		t.Errorf("companies = %+v, want the later valid submit", uni.Facets[0].Companies)
	}
}

// TestAgentUniverse_allDroppedIsError: a submit whose every company fails
// validation leaves nothing to save — an error, not an empty facet.
func TestAgentUniverse_allDroppedIsError(t *testing.T) {
	var calls []string
	loop := &fakeLoop{script: []fakeTurn{
		{tool: "submit_companies", args: `{"companies":[{"name":"","domain":"a.com"}]}`},
	}}
	if _, err := AgentUniverse(context.Background(), "brief", loop, fakeSearcher(&calls, nil)); err == nil {
		t.Fatal("want error for a fully-dropped submission")
	}
}

// TestAgentUniverse_searchBudget: past MaxAgentSearches the tool replies
// with the exhaustion line and the loop still reaches submit.
func TestAgentUniverse_searchBudget(t *testing.T) {
	var calls []string
	turns := []fakeTurn{}
	for i := 0; i < MaxAgentSearches+1; i++ {
		turns = append(turns, fakeTurn{tool: "exa_search", args: fmt.Sprintf(`{"query":"q%d"}`, i)})
	}
	turns = append(turns, fakeTurn{tool: "submit_companies", args: `{"companies":[{"name":"X","domain":"x.com"}]}`})
	loop := &fakeLoop{script: turns}

	uni, err := AgentUniverse(context.Background(), "brief", loop, fakeSearcher(&calls, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(uni.Facets[0].Companies) != 1 {
		t.Errorf("companies = %+v, want the submit under exhaustion", uni.Facets[0].Companies)
	}
	if len(calls) != MaxAgentSearches {
		t.Errorf("live searches = %d, want %d", len(calls), MaxAgentSearches)
	}
}

// TestAgentUniverse_searchFailureNotFatal: one failed search is answered
// as an error reply; the loop continues to submit.
func TestAgentUniverse_searchFailureNotFatal(t *testing.T) {
	var calls []string
	loop := &fakeLoop{script: []fakeTurn{
		{tool: "exa_search", args: `{"query":"broken"}`},
		{tool: "submit_companies", args: `{"companies":[{"name":"X","domain":"x.com"}]}`},
	}}
	uni, err := AgentUniverse(context.Background(), "brief", loop, fakeSearcher(&calls, map[string]string{"broken": ""}))
	if err != nil {
		t.Fatal(err)
	}
	if len(uni.Facets[0].Companies) != 1 {
		t.Errorf("companies = %+v", uni.Facets[0].Companies)
	}
}

// TestAgentUniverse_gateDecoysAnswered: the gate decoy tools get their
// refusal reply instead of a search.
func TestAgentUniverse_gateDecoysAnswered(t *testing.T) {
	var calls []string
	loop := &fakeLoop{script: []fakeTurn{
		{tool: "read", args: "{}"},
		{tool: "submit_companies", args: `{"companies":[{"name":"X","domain":"x.com"}]}`},
	}}
	if _, err := AgentUniverse(context.Background(), "brief", loop, fakeSearcher(&calls, nil)); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Errorf("searches = %v, want none (decoy answered, not searched)", calls)
	}
}

// TestAgentUniverse_loopError: a loop that never submits surfaces its
// error — the caller's fallback path.
func TestAgentUniverse_loopError(t *testing.T) {
	var calls []string
	loop := &fakeLoop{script: []fakeTurn{{tool: "exa_search", args: `{"query":"q"}`}}}
	if _, err := AgentUniverse(context.Background(), "brief", loop, fakeSearcher(&calls, nil)); err == nil {
		t.Fatal("want the loop's exhaustion error")
	}
}

// TestParseSubmission_dedupPreservesFirst: the first occurrence of a
// domain wins, order preserved — deterministic output for the probe stage.
func TestParseSubmission_dedupPreservesFirst(t *testing.T) {
	got, err := parseSubmission(`{"companies":[
		{"name":"A","domain":"a.com"},
		{"name":"B","domain":"A.com"},
		{"name":"C","domain":"https://www.a.com/x"},
		{"name":"D","domain":"d.io"}],
		"clusters":["devtools"],"notes":["thin: fintech"]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Companies) != 2 || got.Companies[0].Name != "A" || got.Companies[1].Name != "D" {
		t.Errorf("companies = %+v, want A then D", got.Companies)
	}
	if got.Clusters[0] != "devtools" || got.Notes[0] != "thin: fintech" {
		t.Errorf("clusters/notes round-trip = %+v", got)
	}
}

// TestAgentUniverse_InputIsPIIFree: the discovery agent loop is an
// LLM-input builder — its only personal input is the rendered brief, so the
// never-leaves set cannot reach the wire. A future builder that reaches for
// raw profile rows instead of the renderer fails here.
func TestAgentUniverse_InputIsPIIFree(t *testing.T) {
	f := db.NewFakeStore()
	f.Profile = dbtest.PIIProfile()

	brief, err := f.GetBrief()
	if err != nil {
		t.Fatalf("GetBrief: %v", err)
	}

	var sent string
	loop := &loopCaptor{onTurn: func(system, user string) { sent = system + "\n" + user }}
	loop.script = []fakeTurn{{tool: "submit_companies", args: `{"companies":[{"name":"X","domain":"x.com"}]}`}}
	if _, err := AgentUniverse(context.Background(), brief.LLMInput().Text(), loop, fakeSearcher(nil, nil)); err != nil {
		t.Fatalf("AgentUniverse: %v", err)
	}

	for _, leak := range dbtest.NeverLeaves() {
		if strings.Contains(sent, leak.Value) {
			t.Errorf("%s reached the wire: %q in:\n%s", leak.What, leak.Value, sent)
		}
	}
	if !strings.Contains(sent, "Backend Engineer") {
		t.Errorf("allowlisted title missing from the discovery input:\n%s", sent)
	}
}
