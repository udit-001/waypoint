// Package mcp is a minimal MCP Streamable HTTP client. It speaks JSON-RPC
// 2.0 over POST to an MCP endpoint. Responses come back as either a single
// JSON object (Content-Type: application/json) or an SSE stream
// (Content-Type: text/event-stream) that concludes with the JSON-RPC
// response. This client handles both.
//
// The client is stateless — each method call takes a headers map carrying
// auth (e.g. {"Authorization": "Bearer token"} or {"Mcp-Session-Id":
// "session-id"}). The caller owns auth state; this module owns transport,
// encoding, and SSE parsing.
//
// Three methods: Initialize (negotiate protocol + get session ID), ListTools
// (discover tools), CallTool (invoke a tool, return text content).
//
// This is the same pattern used by the income-tracker project
// (internal/mcp/client.go) — Waypoint's Exa LinkedIn integration is the
// first consumer; more MCP servers can hang off it the same way.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a minimal MCP Streamable HTTP client.
type Client struct {
	endpoint        string
	protocolVersion string
	httpClient      *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithProtocolVersion sets the MCP protocol version for the initialize
// request. Defaults to "2025-06-18" (modern).
func WithProtocolVersion(v string) Option { return func(c *Client) { c.protocolVersion = v } }

// WithTimeout sets the HTTP client timeout. Defaults to 30s.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// InitResult holds the result of an initialize call.
type InitResult struct {
	ProtocolVersion string
	SessionID       string
}

// Tool is a single entry in the tools/list response.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ErrUnauthorized is returned when the MCP server responds with 401.
var ErrUnauthorized = fmt.Errorf("unauthorized")

// New creates an MCP client for the given endpoint URL.
func New(endpoint string, opts ...Option) *Client {
	c := &Client{
		endpoint:        endpoint,
		protocolVersion: "2025-06-18",
		httpClient:      &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Initialize sends the MCP initialize request and returns the negotiated
// protocol version and session ID (if the server provides one).
func (c *Client) Initialize(ctx context.Context, headers map[string]string) (*InitResult, error) {
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": c.protocolVersion,
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "waypoint",
				"version": "0.1.0",
			},
		},
	}

	resp, err := c.send(ctx, headers, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("initialize returned %d: %s", resp.StatusCode, string(raw))
	}

	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := c.decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	pv := result.ProtocolVersion
	if pv == "" {
		pv = c.protocolVersion
	}
	return &InitResult{
		ProtocolVersion: pv,
		SessionID:       resp.Header.Get("Mcp-Session-Id"),
	}, nil
}

// ListTools calls tools/list and returns the available tools.
func (c *Client) ListTools(ctx context.Context, headers map[string]string) ([]Tool, error) {
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}

	resp, err := c.send(ctx, headers, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tools/list returned %d: %s", resp.StatusCode, string(raw))
	}

	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := c.decodeResponse(resp, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// CallTool invokes a tool by name with the given arguments and returns the
// text content of the first content block.
func (c *Client) CallTool(ctx context.Context, headers map[string]string, name string, args map[string]interface{}) (string, error) {
	if args == nil {
		args = map[string]interface{}{}
	}
	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      name,
			"arguments": args,
		},
	}

	resp, err := c.send(ctx, headers, req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("tools/call returned %d: %s", resp.StatusCode, string(raw))
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := c.decodeResponse(resp, &result); err != nil {
		return "", err
	}
	if result.IsError {
		if len(result.Content) > 0 {
			return "", fmt.Errorf("tool error: %s", result.Content[0].Text)
		}
		return "", fmt.Errorf("tool returned isError with no content")
	}
	if len(result.Content) == 0 {
		return "", fmt.Errorf("tool returned no content")
	}
	return result.Content[0].Text, nil
}

// FindTool searches a tool list for one whose name contains the given
// substring (case-insensitive). Returns empty string if not found.
func FindTool(tools []Tool, substr string) string {
	for _, t := range tools {
		if strings.Contains(strings.ToLower(t.Name), strings.ToLower(substr)) {
			return t.Name
		}
	}
	return ""
}

// --- internal ---

type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *jsonrpcError) Error() string {
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}

// send POSTs a JSON-RPC request and returns the raw HTTP response.
func (c *Client) send(ctx context.Context, headers map[string]string, req jsonrpcRequest) (*http.Response, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	// Caller-provided headers (auth, session ID) take precedence.
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	return c.httpClient.Do(httpReq)
}

// decodeResponse reads the HTTP response body (single JSON object or SSE
// stream) and unmarshals the JSON-RPC result into target.
func (c *Client) decodeResponse(resp *http.Response, target interface{}) error {
	ct := resp.Header.Get("Content-Type")

	if strings.Contains(ct, "text/event-stream") {
		return c.decodeSSEResponse(resp.Body, target)
	}

	var rpc jsonrpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		return fmt.Errorf("decode json response: %w", err)
	}
	if rpc.Error != nil {
		return rpc.Error
	}
	if len(rpc.Result) == 0 {
		return fmt.Errorf("empty result in jsonrpc response")
	}
	return json.Unmarshal(rpc.Result, target)
}

// decodeSSEResponse reads an SSE stream, extracts the JSON-RPC response
// from the data events, and unmarshals its result into target.
func (c *Client) decodeSSEResponse(body io.Reader, target interface{}) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "" || data == "[DONE]" {
			continue
		}

		var rpc jsonrpcResponse
		if err := json.Unmarshal([]byte(data), &rpc); err != nil {
			continue
		}
		if rpc.Error != nil {
			return rpc.Error
		}
		if len(rpc.Result) == 0 {
			continue
		}
		return json.Unmarshal(rpc.Result, target)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read sse stream: %w", err)
	}
	return fmt.Errorf("sse stream ended without a jsonrpc response")
}
