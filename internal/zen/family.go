package zen

// Per-family wires (WP-162): each API family encodes the family-neutral
// conversation into its endpoint's request shape and decodes its streamed
// response back into the neutral assistant message — so the tool loop
// (zen.go) stays single and family-blind. Families route from the
// catalog's model metadata.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// familyWire is the internal seam: one adapter per API family. Tests cross
// it through the client with fake SSE servers, never directly.
type familyWire interface {
	endpoint() string // path appended to the base URL
	// buildBody encodes the conversation + tools into the request payload.
	// It guarantees the free-tier shape: stream=true and a non-empty tools
	// array (the placeholder injected when the caller passed none).
	buildBody(model string, msgs []message, maxTokens int, tools []toolDef) (map[string]any, error)
	// decodeStream assembles the streamed response into msg (content +
	// tool calls). Returns the stream's finish reason.
	decodeStream(r io.Reader, msg *message) (string, *Error)
}

// familyWireFor resolves the wire for a family; unknown families default
// to chat-completions.
func familyWireFor(api string) familyWire {
	switch api {
	case apiOpenAIResponses:
		return responsesWire{}
	case apiAnthropicMessages:
		return anthropicWire{}
	default:
		return chatWire{}
	}
}

// ---- chat-completions ---------------------------------------------------------

// chatWire is the /v1/chat/completions family: messages carry tool_calls /
// tool role turns inline; tools nest under "function".
type chatWire struct{}

func (chatWire) endpoint() string { return "/v1/chat/completions" }

func (chatWire) buildBody(model string, msgs []message, maxTokens int, tools []toolDef) (map[string]any, error) {
	body := map[string]any{
		"model":      model,
		"messages":   msgs,
		"max_tokens": maxTokens,
		"stream":     true,
		// no tool_choice — mimo rejects it (WP-127)
	}
	wrapped := make([]any, len(tools))
	for i, t := range tools {
		wrapped[i] = wireTool(t, apiChatCompletions)
	}
	body["tools"] = wrapped
	ensureFreeTierShape(body, apiChatCompletions)
	return body, nil
}

func (chatWire) decodeStream(r io.Reader, msg *message) (string, *Error) {
	return decodeSSEChat(r, msg)
}

// ---- openai-responses ---------------------------------------------------------

// responsesWire is the /v1/responses family: input items replace messages,
// tools are flat, and function calls flow as output items with call_ids.
type responsesWire struct{}

func (responsesWire) endpoint() string { return "/v1/responses" }

func (responsesWire) buildBody(model string, msgs []message, maxTokens int, tools []toolDef) (map[string]any, error) {
	input := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case "system":
			input = append(input, map[string]any{
				"role":    "system",
				"content": []map[string]any{{"type": "input_text", "text": m.Content}},
			})
		case "user":
			input = append(input, map[string]any{
				"role":    "user",
				"content": []map[string]any{{"type": "input_text", "text": m.Content}},
			})
		case "assistant":
			if m.Content != "" {
				input = append(input, map[string]any{
					"role":    "assistant",
					"content": []map[string]any{{"type": "output_text", "text": m.Content}},
				})
			}
			for _, tc := range m.ToolCalls {
				input = append(input, map[string]any{
					"type": "function_call", "call_id": tc.ID,
					"name": tc.Function.Name, "arguments": tc.Function.Arguments,
				})
			}
		case "tool":
			input = append(input, map[string]any{
				"type": "function_call_output", "call_id": m.ToolCallID, "output": m.Content,
			})
		}
	}
	body := map[string]any{
		"model":             model,
		"input":             input,
		"max_output_tokens": maxTokens,
		"stream":            true,
	}
	wrapped := make([]any, len(tools))
	for i, t := range tools {
		wrapped[i] = wireTool(t, apiOpenAIResponses)
	}
	body["tools"] = wrapped
	ensureFreeTierShape(body, apiOpenAIResponses)
	return body, nil
}

// decodeResponses assembles the streamed response. Function calls come
// from the authoritative `response.completed` output array (arguments are
// complete there); output-text deltas accumulate content. A `response.failed`
// or `error` event classifies into the client's error ladder.
func (responsesWire) decodeStream(r io.Reader, msg *message) (string, *Error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1<<20)

	msg.Role = "assistant" // the assembled message is the assistant turn
	finish := ""
	argDeltas := map[string]*strings.Builder{} // item_id → arguments (fallback path)
	var order []string

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var ev struct {
			Type     string `json:"type"`
			Delta    string `json:"delta"`
			ItemID   string `json:"item_id"`
			Response *struct {
				Status string `json:"status"`
				Error  *struct {
					Message string `json:"message"`
				} `json:"error"`
				Output []struct {
					Type      string `json:"type"`
					ID        string `json:"id"`
					CallID    string `json:"call_id"`
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"output"`
			} `json:"response"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return finish, &Error{Msg: "decode stream event: " + err.Error()}
		}
		switch ev.Type {
		case "response.output_text.delta":
			msg.Content += ev.Delta
		case "response.function_call_arguments_delta":
			b, ok := argDeltas[ev.ItemID]
			if !ok {
				b = &strings.Builder{}
				argDeltas[ev.ItemID] = b
				order = append(order, ev.ItemID)
			}
			b.WriteString(ev.Delta)
		case "response.completed", "response.incomplete":
			if ev.Response == nil {
				continue
			}
			if ev.Response.Error != nil && ev.Response.Error.Message != "" {
				return finish, streamError("", ev.Response.Error.Message)
			}
			// Authoritative tool calls from the completed output; content
			// stays the delta accumulation (already complete).
			msg.ToolCalls = nil
			for _, item := range ev.Response.Output {
				if item.Type == "function_call" {
					msg.ToolCalls = append(msg.ToolCalls, toolCall{
						ID:   item.CallID,
						Type: "function",
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{Name: item.Name, Arguments: item.Arguments},
					})
				}
			}
		case "response.failed":
			m := "response failed"
			if ev.Response != nil && ev.Response.Error != nil && ev.Response.Error.Message != "" {
				m = ev.Response.Error.Message
			}
			return finish, streamError("", m)
		case "error":
			m := "stream error"
			if ev.Error != nil && ev.Error.Message != "" {
				m = ev.Error.Message
			}
			return finish, streamError("", m)
		}
	}
	if err := sc.Err(); err != nil {
		return finish, &Error{Msg: "read stream: " + err.Error()}
	}
	// No completed event: fall back to the delta-accumulated calls.
	if len(msg.ToolCalls) == 0 && len(order) > 0 {
		for _, id := range order {
			msg.ToolCalls = append(msg.ToolCalls, toolCall{
				ID:   id,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{Arguments: argDeltas[id].String()},
			})
		}
	}
	return finish, nil
}

// ---- anthropic-messages ---------------------------------------------------------

// anthropicWire is the /v1/messages family: system is top-level, assistant
// tool calls are tool_use content blocks, tool results are tool_result
// blocks in a user turn, and tools carry input_schema.
type anthropicWire struct{}

func (anthropicWire) endpoint() string { return "/v1/messages" }

func (anthropicWire) buildBody(model string, msgs []message, maxTokens int, tools []toolDef) (map[string]any, error) {
	system := ""
	messages := []map[string]any{}
	for _, m := range msgs {
		switch m.Role {
		case "system":
			system = m.Content
		case "user":
			messages = append(messages, map[string]any{
				"role":    "user",
				"content": []map[string]any{{"type": "text", "text": m.Content}},
			})
		case "assistant":
			content := []map[string]any{}
			if m.Content != "" {
				content = append(content, map[string]any{"type": "text", "text": m.Content})
			}
			for _, tc := range m.ToolCalls {
				var input any
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
					input = map[string]any{}
				}
				content = append(content, map[string]any{
					"type": "tool_use", "id": tc.ID, "name": tc.Function.Name, "input": input,
				})
			}
			messages = append(messages, map[string]any{"role": "assistant", "content": content})
		case "tool":
			messages = append(messages, map[string]any{
				"role": "user",
				"content": []map[string]any{{
					"type": "tool_result", "tool_use_id": m.ToolCallID,
					"content": []map[string]any{{"type": "text", "text": m.Content}},
				}},
			})
		}
	}
	body := map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"stream":     true,
		"messages":   messages,
	}
	if system != "" {
		body["system"] = system
	}
	wrapped := make([]any, len(tools))
	for i, t := range tools {
		wrapped[i] = wireTool(t, apiAnthropicMessages)
	}
	body["tools"] = wrapped
	ensureFreeTierShape(body, apiAnthropicMessages)
	return body, nil
}

// decodeAnthropic assembles content blocks: tool_use blocks start with
// content_block_start (id, name), their arguments arrive as
// input_json_delta partials, text via text_delta; message_delta carries
// the stop reason.
func (anthropicWire) decodeStream(r io.Reader, msg *message) (string, *Error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1<<20)

	msg.Role = "assistant" // the assembled message is the assistant turn
	finish := ""
	var blocks []struct {
		id, name, args string
	}

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var ev struct {
			Type         string `json:"type"`
			Index        *int   `json:"index"`
			ContentBlock *struct {
				Type string `json:"type"`
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"content_block"`
			Delta *struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
				StopReason  string `json:"stop_reason"`
			} `json:"delta"`
			Error *struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return finish, &Error{Msg: "decode stream event: " + err.Error()}
		}
		switch ev.Type {
		case "content_block_start":
			if ev.ContentBlock != nil && ev.ContentBlock.Type == "tool_use" {
				idx := 0
				if ev.Index != nil {
					idx = *ev.Index
				}
				for len(blocks) <= idx {
					blocks = append(blocks, struct {
						id, name, args string
					}{})
				}
				blocks[idx].id = ev.ContentBlock.ID
				blocks[idx].name = ev.ContentBlock.Name
			}
		case "content_block_delta":
			if ev.Delta == nil {
				continue
			}
			switch ev.Delta.Type {
			case "text_delta":
				msg.Content += ev.Delta.Text
			case "input_json_delta":
				idx := 0
				if ev.Index != nil {
					idx = *ev.Index
				}
				for len(blocks) <= idx {
					blocks = append(blocks, struct {
						id, name, args string
					}{})
				}
				blocks[idx].args += ev.Delta.PartialJSON
			}
		case "message_delta":
			if ev.Delta != nil && ev.Delta.StopReason != "" {
				finish = ev.Delta.StopReason
			}
		case "error":
			t, m := "", "stream error"
			if ev.Error != nil {
				t = ev.Error.Type
				if ev.Error.Message != "" {
					m = ev.Error.Message
				}
			}
			return finish, streamError(t, m)
		}
	}
	if err := sc.Err(); err != nil {
		return finish, &Error{Msg: "read stream: " + err.Error()}
	}
	for _, b := range blocks {
		if b.name == "" {
			continue
		}
		msg.ToolCalls = append(msg.ToolCalls, toolCall{
			ID:   b.id,
			Type: "tool_use",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: b.name, Arguments: b.args},
		})
	}
	return finish, nil
}

// streamError classifies an in-stream error: FreeTierError is fatal (the
// gate rejected the shape/identity — retrying can't help), everything else
// is recoverable.
func streamError(errType, message string) *Error {
	fatal := errType == "FreeTierError"
	status := 0
	if fatal {
		status = 403
	}
	if message == "" {
		message = "stream error"
	}
	return &Error{Status: status, Fatal: fatal, Msg: fmt.Sprintf("%s", message)}
}
