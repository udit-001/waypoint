package zen

import (
	"context"
	"fmt"
)

// The agent loop (WP-180): a tool-calling conversation the model ends by
// calling one designated tool — the tool-call-as-structured-output pattern
// proven by the WP-131 curation prototype and validated for discovery by
// the WP-172 prototype. zen owns the protocol: turns, batched tool-call
// answers, streaming, the free-tier gate. The caller owns the tools, what
// the replies say, and when a call is terminal.
//
// The curation loop (toolLoop) keeps its own shape: its verdict validation
// and normalization are curate-domain, and its tool schema is locked
// (WP-131). Unifying the two loops is a later refactor, deliberately not
// smuggled into this one.

// AgentTool is one tool the loop exposes to the model. Terminal marks the
// call that ends the loop — the loop nudges with its name when the model
// answers in prose instead.
type AgentTool struct {
	Name        string
	Description string
	Parameters  map[string]any
	Terminal    bool
}

// AgentExec answers one tool call. reply is the text the model sees as the
// tool response; done=true ends the loop — the caller judged this call
// terminal and valid, and the loop returns its raw arguments.
type AgentExec func(name, argsJSON string) (reply string, done bool)

// AgentLoop runs system + user through the model, answering every tool
// call via exec, until exec reports done or maxTurns. Returns the terminal
// call's raw arguments. Budgets and validation live in the executor — the
// loop enforces only the turn cap and the call-a-tool rule.
func (c *Client) AgentLoop(ctx context.Context, system, user string, tools []AgentTool, exec AgentExec, maxTurns int) (args string, turns int, err error) {
	real := make([]toolDef, 0, len(tools))
	// withGateDecoys carries the gate composition (decoys first); the
	// loop's tools follow.
	defs := withGateDecoys(real)
	terminal := ""
	for _, t := range tools {
		defs = append(defs, toolDef{Name: t.Name, Description: t.Description, Parameters: t.Parameters})
		if t.Terminal {
			terminal = t.Name
		}
	}

	wire, path := c.wireFor(c.cfg.Model)
	msgs := []message{{Role: "system", Content: system}, {Role: "user", Content: user}}
	for turn := 1; turn <= maxTurns; turn++ {
		turns = turn
		if cerr := ctx.Err(); cerr != nil {
			return "", turn, fmt.Errorf("context done: %w", cerr)
		}

		res, _, aerr := c.decodeTurn(ctx, wire, path, c.cfg.Model, msgs, defs)
		if aerr != nil {
			return "", turn, aerr
		}

		if len(res.ToolCalls) == 0 {
			// Model answered in prose: name the terminal tool and let the
			// turn budget catch a model that never complies.
			msgs = append(msgs, res, message{Role: "user", Content: "Call " + terminal + " with what you have. Never answer in prose."})
			continue
		}

		// Record the assistant message verbatim, then one tool response
		// per call — protocol-valid for batched calls, same rule toolLoop
		// runs (models batch parallel calls; an unanswered id 400s the
		// next request).
		ext := make([]message, 0, len(res.ToolCalls)+1)
		ext = append(ext, res)
		doneArgs, done := "", false
		for _, tc := range res.ToolCalls {
			reply, d := exec(tc.Function.Name, tc.Function.Arguments)
			ext = append(ext, message{Role: "tool", Content: reply, ToolCallID: tc.ID})
			if d {
				doneArgs, done = tc.Function.Arguments, true
			}
		}
		msgs = append(msgs, ext...)
		if done {
			return doneArgs, turn, nil
		}
	}
	return "", maxTurns, fmt.Errorf("exceeded %d turns without calling %s", maxTurns, terminal)
}
