package core_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
	"github.com/VinkiusForDevs/ai-connect-sdk-go/internal/fake"
)

func newRuntime(d *fake.FakeDoer) *core.RuntimeClient {
	return core.NewRuntimeClient("https://runtime/vk_live_t/mcp", core.RuntimeClientConfig{
		Timeout:   time.Second,
		Retry:     core.RetryPolicy{MaxRetries: 2, BaseDelayMS: 0, MaxDelayMS: 0},
		Doer:      d,
		UserAgent: "vinkius-connect/test",
	})
}

func TestListToolsParsesJSONRPCAndNormalizes(t *testing.T) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]any{
			"tools": []any{
				map[string]any{"name": "create_issue", "title": "Create", "description": "d", "inputSchema": map[string]any{"type": "object"}},
			},
		},
	}
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, payload))

	tools, err := newRuntime(d).ListTools(context.Background(), core.RequestOptions{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("tools: %v", tools)
	}
	if tools[0]["name"] != "create_issue" {
		t.Fatalf("tool name: %v", tools[0]["name"])
	}
	if tools[0]["input_schema"].(map[string]any)["type"] != "object" {
		t.Fatalf("input_schema: %v", tools[0]["input_schema"])
	}
	if _, has := tools[0]["inputSchema"]; has {
		t.Fatalf("camelCase inputSchema should be coerced to input_schema: %v", tools[0])
	}
}

func TestListToolsRejectsMissingToolsArray(t *testing.T) {
	payload := map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}}
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, payload))

	_, err := newRuntime(d).ListTools(context.Background(), core.RequestOptions{})
	if !core.IsProtocolError(err) {
		t.Fatalf("missing tools array should be a protocol error: %v", err)
	}
}

func TestCallToolReturnsCapabilityResult(t *testing.T) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]any{
			"content":           []any{map[string]any{"type": "text", "text": "done"}},
			"isError":           false,
			"structuredContent": map[string]any{"ok": true},
		},
	}
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, payload))

	res, err := newRuntime(d).CallTool(context.Background(), "create_issue", map[string]any{"title": "x"}, core.RequestOptions{})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if len(res.Content) != 1 || res.Content[0].Text != "done" {
		t.Fatalf("content: %v", res.Content)
	}
	if res.IsError {
		t.Fatalf("is_error should be false: %v", res)
	}
	if res.StructuredContent.(map[string]any)["ok"] != true {
		t.Fatalf("structured_content: %v", res.StructuredContent)
	}
	body, _ := d.BodyOf()
	bm := body.(map[string]any)
	if bm["method"] != "tools/call" {
		t.Fatalf("method: %v", body)
	}
	params := bm["params"].(map[string]any)
	if params["name"] != "create_issue" {
		t.Fatalf("params: %v", params)
	}
	if a := params["arguments"].(map[string]any); a["title"] != "x" {
		t.Fatalf("arguments: %v", a)
	}
}

func TestCallToolIsErrorIsAResultNotAnError(t *testing.T) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"result": map[string]any{
			"content": []any{map[string]any{"type": "text", "text": "oops"}},
			"isError": true,
		},
	}
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, payload))

	res, err := newRuntime(d).CallTool(context.Background(), "x", nil, core.RequestOptions{})
	if err != nil {
		t.Fatalf("is_error must not surface as an error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("is_error should be true: %v", res)
	}
}

func TestRuntimeMapsJSONRPCErrorToProtocolError(t *testing.T) {
	payload := map[string]any{"jsonrpc": "2.0", "id": 1, "error": map[string]any{"code": -32000, "message": "bad"}}
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, payload))

	_, err := newRuntime(d).CallTool(context.Background(), "x", map[string]any{}, core.RequestOptions{})
	if !core.IsProtocolError(err) {
		t.Fatalf("JSON-RPC error should map to protocol_error: %v", err)
	}
	if e := core.SDKError(err); e.Message != "bad" {
		t.Fatalf("message: %q", e.Message)
	}
}

func TestRuntimeMapsHTTP403ToAuthError(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(403, map[string]any{"error": "revoked"}))

	_, err := newRuntime(d).ListTools(context.Background(), core.RequestOptions{})
	if !core.IsAuthError(err) {
		t.Fatalf("403 should map to auth_error: %v", err)
	}
	if e := core.SDKError(err); e.Message != "revoked" {
		t.Fatalf("message: %q", e.Message)
	}
}

func TestRuntimeMapsHTTP404ToNotFoundError(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(404, map[string]any{"error": "unknown"}))

	_, err := newRuntime(d).ListTools(context.Background(), core.RequestOptions{})
	if !core.IsNotFoundError(err) {
		t.Fatalf("404 should map to not_found: %v", err)
	}
}

func TestRuntimeMaps5xxToConnectionError(t *testing.T) {
	// tools/list is idempotent, so it exhausts the default 2 retries (3 attempts).
	d := fake.NewFakeDoer().Enqueue(
		fake.NewJSON(503, map[string]any{"error": "x"}),
		fake.NewJSON(503, map[string]any{"error": "x"}),
		fake.NewJSON(503, map[string]any{"error": "x"}),
	)

	_, err := newRuntime(d).ListTools(context.Background(), core.RequestOptions{})
	if !core.IsConnectionError(err) {
		t.Fatalf("5xx should map to connection_error: %v", err)
	}
	if code := core.CodeOf(err); code != core.CodeConnectionError {
		t.Fatalf("code %q, want connection_error", code)
	}
	if d.Len() != 3 {
		t.Fatalf("tools/list should retry, made %d attempts", d.Len())
	}
}

func TestSSEStreamLastFrameWins(t *testing.T) {
	last, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1,
		"result": map[string]any{"tools": []any{map[string]any{"name": "t1"}}},
	})
	sse := "data: {\"jsonrpc\": \"2.0\", \"id\": 1, \"result\": {\"tools\": []}}\n\ndata: " + string(last)
	d := fake.NewFakeDoer().Enqueue(fake.NewText(200, sse))

	tools, err := newRuntime(d).ListTools(context.Background(), core.RequestOptions{})
	if err != nil {
		t.Fatalf("list tools from SSE: %v", err)
	}
	if len(tools) != 1 || tools[0]["name"] != "t1" {
		t.Fatalf("last frame should win, got %v", tools)
	}
	if schema, ok := tools[0]["input_schema"].(map[string]any); !ok || len(schema) != 0 {
		t.Fatalf("missing schema should default to an empty map, got %v", tools[0])
	}
}

func TestExtractEnvelopeIgnoresNonJSONFrames(t *testing.T) {
	sse := ": comment\ndata: not-json\ndata: {\"a\": 1}\n"
	out, ok := core.ExtractEnvelope(sse).(map[string]any)
	if !ok || out["a"] != 1.0 {
		t.Fatalf("non-JSON frames should be ignored: %v", out)
	}
}

func TestExtractEnvelopePassesThroughJSON(t *testing.T) {
	out := core.ExtractEnvelope(map[string]any{"a": 1})
	if m, ok := out.(map[string]any); !ok || m["a"] != 1 {
		t.Fatalf("JSON should pass through: %v", out)
	}
}
