package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MCPProtocolVersion is the JSON-RPC protocol version advertised to the runtime.
const MCPProtocolVersion = "2026-07-28"

// RuntimeClientConfig binds the data-plane runtime client settings.
type RuntimeClientConfig struct {
	Timeout   time.Duration
	Retry     RetryPolicy
	Doer      Doer
	UserAgent string
	Hooks     Hooks
}

// RuntimeClient is a stateless JSON-RPC 2.0 client over Streamable HTTP, bound to
// one runtime endpoint (mcp_url). The mcp_url already embeds the vk_live_*
// data-plane token, so no auth headers are added here.
//
// tools/list and tools/call are single POSTs. tools/list is idempotent and is
// always retried on transient failures; tools/call retries only when an
// idempotency key is supplied. Runtime errors map to AuthError (401/403),
// NotFoundError (404), and connection_error otherwise; JSON-RPC errors map to
// protocol_error. Tool-level failures come back as IsError results.
type RuntimeClient struct {
	mcpURL string
	cfg    RuntimeClientConfig
	t      *Transport

	mu     sync.Mutex
	nextID int
}

// NewRuntimeClient builds a runtime client for one mcp_url.
func NewRuntimeClient(mcpURL string, cfg RuntimeClientConfig) *RuntimeClient {
	return &RuntimeClient{
		mcpURL: mcpURL,
		cfg:    cfg,
		t: NewTransport(TransportConfig{
			Timeout:   cfg.Timeout,
			Retry:     cfg.Retry,
			Doer:      cfg.Doer,
			UserAgent: cfg.UserAgent,
			Hooks:     cfg.Hooks,
		}),
		nextID: 1,
	}
}

// ListTools lists the tools this connection exposes.
func (c *RuntimeClient) ListTools(ctx context.Context, o RequestOptions) ([]map[string]any, error) {
	result, err := c.rpc(ctx, "tools/list", map[string]any{}, o, true)
	if err != nil {
		return nil, err
	}
	tools, ok := toolList(result)
	if !ok {
		return nil, NewProtocolError("Runtime tools/list returned no tools array.", result)
	}
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		norm, err := NormalizeToolDefinition(tool)
		if err != nil {
			return nil, err
		}
		out = append(out, norm)
	}
	return out, nil
}

// CallTool executes a tool.
func (c *RuntimeClient) CallTool(ctx context.Context, name string, args map[string]any, o RequestOptions) (*CapabilityResult, error) {
	params := map[string]any{"name": name, "arguments": map[string]any{}}
	if args != nil {
		params["arguments"] = args
	}
	result, err := c.rpc(ctx, "tools/call", params, o, o.IdempotencyKey != "")
	if err != nil {
		return nil, err
	}
	return NormalizeCallResult(result), nil
}

// rpc sends one JSON-RPC request and returns the parsed result, mapping
// transport failures and non-2xx responses to typed errors.
func (c *RuntimeClient) rpc(ctx context.Context, method string, params map[string]any, o RequestOptions, retryable bool) (any, error) {
	id := c.nextRPCID()
	envelope := map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
	body, _ := json.Marshal(envelope)

	result, err := c.t.Send(ctx, &TransportRequest{
		URL:       c.mcpURL,
		Method:    "POST",
		Headers:   c.headers(o.IdempotencyKey),
		Body:      body,
		Retryable: retryable,
		Label:     "Runtime request",
		Timeout:   o.Timeout,
	})
	if err != nil {
		return nil, err
	}
	if !(result.Status >= 200 && result.Status < 300) {
		return nil, c.mapRuntimeHTTPError(result.Status, result.Body)
	}
	return c.unwrapEnvelope(result.Body)
}

func (c *RuntimeClient) nextRPCID() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.nextID
	c.nextID = id + 1
	return id
}

// headers assembles the data-plane request headers. No auth: the mcp_url embeds
// the vk_live_* token.
func (c *RuntimeClient) headers(idempotencyKey string) map[string]string {
	headers := map[string]string{
		"content-type":         "application/json",
		"accept":               "application/json, text/event-stream",
		"mcp-protocol-version": MCPProtocolVersion,
		"user-agent":           c.cfg.UserAgent,
	}
	if idempotencyKey != "" {
		headers["idempotency-key"] = idempotencyKey
	}
	return headers
}

// unwrapEnvelope extracts the JSON-RPC result, or maps a JSON-RPC error object
// to a protocol_error.
func (c *RuntimeClient) unwrapEnvelope(body any) (any, error) {
	envelope := ExtractEnvelope(body)
	m, ok := envelope.(map[string]any)
	if !ok {
		return nil, NewProtocolError("Runtime returned a non-JSON-RPC response.", body)
	}
	if rpcErr, present := m["error"]; present && rpcErr != nil {
		message := "Runtime returned a JSON-RPC error."
		if em, ok := rpcErr.(map[string]any); ok {
			if s, ok := em["message"].(string); ok && s != "" {
				message = s
			}
		}
		return nil, &Error{Message: message, Code: CodeProtocolError, Details: rpcErr}
	}
	if result, present := m["result"]; present {
		return result, nil
	}
	return nil, NewProtocolError("Runtime JSON-RPC response has no result.", body)
}

func (c *RuntimeClient) mapRuntimeHTTPError(status int, body any) error {
	message := pickRuntimeMessage(body, fmt.Sprintf("Runtime request failed with status %d", status))
	switch {
	case status == 401 || status == 403:
		return &Error{Message: message, Code: CodeAuthError, Status: status, Details: body}
	case status == 404:
		return &Error{Message: message, Code: CodeNotFound, Status: status, Details: body}
	default:
		return &Error{Message: message, Code: CodeConnectionError, Status: status, Details: body}
	}
}

// toolList returns the "tools" array of a tools/list result, reporting whether it
// is a usable list.
func toolList(result any) ([]any, bool) {
	m, ok := result.(map[string]any)
	if !ok {
		return nil, false
	}
	tools, ok := m["tools"].([]any)
	return tools, ok
}

// NormalizeToolDefinition maps a raw runtime tool definition (camelCase) to the
// SDK's snake_case capability shape.
func NormalizeToolDefinition(tool any) (map[string]any, error) {
	t, _ := tool.(map[string]any)
	name, _ := t["name"].(string)
	if strings.TrimSpace(name) == "" {
		return nil, NewProtocolError("Runtime tool definition is missing a name.", tool)
	}
	out := map[string]any{
		"name": name,
		"title":       optionalString(t, "title"),
		"description": optionalString(t, "description"),
	}
	// The runtime uses JSON-Schema inputSchema (camelCase); the SDK contract is snake_case.
	if v, ok := t["inputSchema"]; ok {
		out["input_schema"] = v
	} else if v, ok := t["input_schema"]; ok {
		out["input_schema"] = v
	} else {
		out["input_schema"] = map[string]any{}
	}
	if a, ok := t["annotations"]; ok {
		out["annotations"] = a
	}
	return out, nil
}

// optionalString returns the string value at key, or nil when absent or not a
// string.
func optionalString(m map[string]any, key string) any {
	if s, ok := m[key].(string); ok {
		return s
	}
	return nil
}

// NormalizeCallResult maps a runtime tools/call JSON-RPC result to a
// CapabilityResult. structuredContent is surfaced verbatim; the SDK never
// parses or transforms it.
func NormalizeCallResult(result any) *CapabilityResult {
	r, _ := result.(map[string]any)
	out := &CapabilityResult{}
	if rawContent, ok := r["content"].([]any); ok {
		for _, entry := range rawContent {
			e, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			ctype := "text"
			if s, ok := e["type"].(string); ok {
				ctype = s
			}
			text := ""
			if s, ok := e["text"].(string); ok {
				text = s
			} else if b, err := json.Marshal(e); err == nil {
				text = string(b)
			}
			out.Content = append(out.Content, ContentPart{Type: ctype, Text: text})
		}
	}
	out.IsError = r["isError"] == true
	if sc, present := r["structuredContent"]; present {
		out.StructuredContent = sc
	}
	return out
}

// pickRuntimeMessage extracts a human-readable message from a runtime error body.
func pickRuntimeMessage(body any, fallback string) string {
	if m, ok := body.(map[string]any); ok {
		if s, ok := m["error"].(string); ok && s != "" {
			return s
		}
		if em, ok := m["error"].(map[string]any); ok {
			if s, ok := em["message"].(string); ok {
				return s
			}
		}
		if s, ok := m["message"].(string); ok && s != "" {
			return s
		}
	}
	if s, ok := body.(string); ok && s != "" {
		return s
	}
	return fallback
}

// ExtractEnvelope accepts either a parsed JSON object or an SSE stream string and
// returns the authoritative envelope. For an SSE stream, the last JSON data
// frame wins; non-JSON frames (comments, keep-alives) are ignored.
func ExtractEnvelope(body any) any {
	s, isStr := body.(string)
	if !isStr {
		return body
	}
	var last any
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "data:") {
			continue
		}
		payloadText := strings.TrimSpace(trimmed[len("data:"):])
		var payload any
		if err := json.Unmarshal([]byte(payloadText), &payload); err != nil {
			continue // ignore non-JSON SSE frames
		}
		if _, ok := payload.(map[string]any); ok {
			last = payload
		}
	}
	if last == nil {
		return body
	}
	return last
}
