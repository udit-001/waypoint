package discovery

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/config"
)

func stubProbe(t *testing.T, jobs int, err error) {
	t.Helper()
	orig := ProbeBoard
	ProbeBoard = func(_ context.Context, _ BoardLink) (int, error) { return jobs, err }
	t.Cleanup(func() { ProbeBoard = orig })
}

func ghLink(url string) BoardLink { return BoardLink{Provider: "greenhouse", URL: url} }

// TestPromote_writesVerifiedBoards: the happy path passes each link
// through the verify gate and lands it as an enabled board.toml entry,
// named deterministically from the company name.
func TestPromote_writesVerifiedBoards(t *testing.T) {
	stubProbe(t, 4, nil)
	bf := &config.BoardsFile{}

	out, err := Promote(context.Background(), "Acme Corp", []BoardLink{ghLink("https://boards.greenhouse.io/acme")}, bf)
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if out.Added != 1 || out.Fetched != 4 || out.Provider != "greenhouse" {
		t.Errorf("outcome = %+v, want 1 added / 4 fetched / greenhouse", out)
	}
	e := bf.Find("acme-corp")
	if e == nil {
		t.Fatalf("entry acme-corp missing; boards = %+v", bf.Boards)
	}
	if !e.Enabled || e.Provider != "greenhouse" || e.Company != "Acme Corp" {
		t.Errorf("entry = %+v, want enabled greenhouse entry for Acme Corp", e)
	}
}

// TestPromote_skipsAlreadyWatched: a URL already in boards.toml counts
// as already-watched and is not re-added nor errored.
func TestPromote_skipsAlreadyWatched(t *testing.T) {
	stubProbe(t, 2, nil)
	bf := &config.BoardsFile{}
	bf.Upsert(config.BoardEntry{Name: "acme-manual", Company: "Acme", URL: "https://boards.greenhouse.io/acme", Enabled: true})

	out, err := Promote(context.Background(), "Acme Corp", []BoardLink{ghLink("https://boards.greenhouse.io/acme")}, bf)
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if out.AlreadyWatched != 1 || out.Added != 0 {
		t.Errorf("outcome = %+v, want 1 already-watched / 0 added", out)
	}
}

// TestPromote_nameConflictFails: a same-name board pointing elsewhere
// must fail loudly instead of silently replacing the user's entry.
func TestPromote_nameConflictFails(t *testing.T) {
	stubProbe(t, 4, nil)
	bf := &config.BoardsFile{}
	bf.Upsert(config.BoardEntry{Name: "acme-corp", Company: "Acme", URL: "https://boards.greenhouse.io/different", Enabled: true})

	_, err := Promote(context.Background(), "Acme Corp", []BoardLink{ghLink("https://boards.greenhouse.io/acme")}, bf)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want same-name conflict", err)
	}
	if got := bf.Find("acme-corp"); got == nil || got.URL != "https://boards.greenhouse.io/different" {
		t.Errorf("existing entry mutated: %+v", got)
	}
}

// TestPromote_verifyFailureAborts: a board that fails the verify gate
// aborts promotion with the URL called out; nothing lands in bf.
func TestPromote_verifyFailureAborts(t *testing.T) {
	stubProbe(t, 0, errors.New("connection refused"))
	bf := &config.BoardsFile{}

	_, err := Promote(context.Background(), "Acme Corp", []BoardLink{ghLink("https://boards.greenhouse.io/acme")}, bf)
	if err == nil || !strings.Contains(err.Error(), "verification failed") || !strings.Contains(err.Error(), "acme") {
		t.Fatalf("err = %v, want verification failure naming the URL", err)
	}
	if len(bf.Boards) != 0 {
		t.Errorf("failed verify left entries behind: %+v", bf.Boards)
	}
}

// TestPromote_requiresBoards: promoting a candidate with no links is a
// caller bug and fails rather than flipping status on thin air.
func TestPromote_requiresBoards(t *testing.T) {
	_, err := Promote(context.Background(), "Acme Corp", nil, &config.BoardsFile{})
	if err == nil || !strings.Contains(err.Error(), "no boards") {
		t.Fatalf("err = %v, want no-boards failure", err)
	}
}
