package zen

// SSE decoding for the streamed chat-completions wire (WP-162): assistant
// content and tool-call arguments arrive as deltas across chunks; the
// [DONE] sentinel ends the stream; mid-stream errors surface as the
// client's recoverable/fatal classification — never a hang, never a
// silent empty result.

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
)

// sseChunk is one chat-completions stream chunk: the fields we act on.
type sseChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Name     string `json:"name"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// decodeSSEChat consumes an SSE stream and assembles the assistant
// message: content concatenates, tool calls reassemble by index (id/name
// from the first fragment, arguments concatenated across chunks). Returns
// the last finish_reason. An in-stream error object aborts with the
// client's error classification: FreeTierError is fatal (the free-tier
// gate rejected the shape/identity — retrying can't help), anything else
// is recoverable.
func decodeSSEChat(r io.Reader, msg *message) (string, *Error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1<<20) // argument deltas can be long

	msg.Role = "assistant" // the assembled message is the assistant turn
	finish := ""
	var order []int
	calls := map[int]*toolCall{}
	seen := map[int]bool{}

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue // event:/id:/comment/blank lines
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk sseChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return finish, &Error{Msg: "decode stream chunk: " + err.Error()}
		}
		if chunk.Error != nil {
			e := chunk.Error
			// The gateway's mid-stream error mirrors its HTTP-body shape:
			// {type, error:{type, message}} or a bare {error:{message}}.
			fatal := e.Type == "FreeTierError"
			status := 0
			if fatal {
				status = 403
			}
			m := e.Message
			if m == "" {
				m = "stream error"
			}
			return finish, &Error{Status: status, Fatal: fatal, Msg: m}
		}
		if len(chunk.Choices) == 0 {
			continue // usage-only or keep-alive chunk
		}
		choice := chunk.Choices[0]
		msg.Content += choice.Delta.Content
		for _, tc := range choice.Delta.ToolCalls {
			call, ok := calls[tc.Index]
			if !ok {
				call = &toolCall{}
				calls[tc.Index] = call
				order = append(order, tc.Index)
				seen[tc.Index] = true
			}
			if tc.ID != "" {
				call.ID = tc.ID
			}
			if tc.Type != "" {
				call.Type = tc.Type
			}
			name := tc.Function.Name
			if name == "" {
				name = tc.Name
			}
			if name != "" {
				call.Function.Name = name
			}
			call.Function.Arguments += tc.Function.Arguments
		}
		if choice.FinishReason != nil {
			finish = *choice.FinishReason
		}
	}
	if err := sc.Err(); err != nil {
		return finish, &Error{Msg: "read stream: " + err.Error()}
	}

	for _, i := range order {
		msg.ToolCalls = append(msg.ToolCalls, *calls[i])
	}
	return finish, nil
}
