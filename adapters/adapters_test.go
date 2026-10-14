package adapters_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/adapters"
	"github.com/VinkiusForDevs/ai-connect-sdk-go/internal/fake"
	"github.com/VinkiusForDevs/ai-connect-sdk-go/vinkius"
)

// capSet builds a two-capability set: one with a full schema and one with an
// empty schema that must normalize to an empty object schema.
func capSet() *vinkius.CapabilitySet {
	caps := []*vinkius.Capability{
		{Connector: "github", ConnectionID: "c1", Name: "github__create_issue", RawName: "create_issue", Description: "Open an issue", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"title": map[string]any{"type": "string"}}}},
		{Connector: "github", ConnectionID: "c1", Name: "github__list_repos", RawName: "list_repos", Description: "", InputSchema: nil},
	}
	return vinkius.NewCapabilitySet(caps...)
}

func TestToJSONSchemaTools(t *testing.T) {
	out := adapters.ToJSONSchemaTools(capSet())
	if len(out) != 2 {
		t.Fatalf("len: %d", len(out))
	}
	if out[0]["name"] != "github__create_issue" {
		t.Fatalf("name: %v", out[0])
	}
	if p := out[1]["parameters"].(map[string]any); p["type"] != "object" || p["properties"] == nil {
		t.Fatalf("empty schema should normalize: %v", p)
	}
}

func TestToOpenAITools(t *testing.T) {
	out, err := adapters.ToOpenAITools(capSet())
	if err != nil {
		t.Fatalf("openai tools: %v", err)
	}
	if out[0]["type"] != "function" {
		t.Fatalf("type: %v", out[0])
	}
	if fn := out[0]["function"].(map[string]any); fn["name"] != "github__create_issue" {
		t.Fatalf("function name: %v", fn)
	}
}

func TestToAnthropicTools(t *testing.T) {
	out, err := adapters.ToAnthropicTools(capSet())
	if err != nil {
		t.Fatalf("anthropic tools: %v", err)
	}
	if out[0]["name"] != "github__create_issue" {
		t.Fatalf("name: %v", out[0])
	}
	if out[0]["input_schema"] == nil {
		t.Fatalf("input_schema missing: %v", out[0])
	}
}

func TestToGeminiToolsValid(t *testing.T) {
	decls, err := adapters.ToGeminiTools(capSet())
	if err != nil {
		t.Fatalf("gemini tools: %v", err)
	}
	if decls[0]["name"] != "github__create_issue" {
		t.Fatalf("name: %v", decls[0])
	}
}

func TestToGeminiToolsRejectsHyphenatedConnector(t *testing.T) {
	bad := vinkius.NewCapabilitySet(&vinkius.Capability{Name: "google-calendar__do", RawName: "do", Description: "d"})
	if _, err := adapters.ToGeminiTools(bad); err == nil {
		t.Fatalf("a hyphenated name should be rejected for Gemini")
	}
}

func TestOpenAIRejectsOverlongName(t *testing.T) {
	long := strings.Repeat("a", 80)
	bad := vinkius.NewCapabilitySet(&vinkius.Capability{Name: long, RawName: "x", Description: "d"})
	if _, err := adapters.ToOpenAITools(bad); err == nil {
		t.Fatalf("an 80-char name should exceed OpenAI's 64-char cap")
	}
}

func TestParseArgsToleratesMalformed(t *testing.T) {
	if got := adapters.ParseArgs(""); len(got) != 0 {
		t.Fatalf("empty: %v", got)
	}
	if got := adapters.ParseArgs("not json"); len(got) != 0 {
		t.Fatalf("malformed: %v", got)
	}
	if got := adapters.ParseArgs("[1,2,3]"); len(got) != 0 {
		t.Fatalf("non-object: %v", got)
	}
	if got := adapters.ParseArgs(`{"a": 1}`); got["a"] != float64(1) {
		t.Fatalf("object: %v", got)
	}
}

func TestFindCapability(t *testing.T) {
	c, err := adapters.FindCapability(capSet(), "create_issue")
	if err != nil {
		t.Fatalf("by raw name: %v", err)
	}
	if c.RawName != "create_issue" {
		t.Fatalf("cap: %v", c)
	}
	c2, err := adapters.FindCapability(capSet(), "github__list_repos")
	if err != nil {
		t.Fatalf("by display name: %v", err)
	}
	if c2.RawName != "list_repos" {
		t.Fatalf("cap: %v", c2)
	}
	if _, err := adapters.FindCapability(capSet(), "nope"); err == nil {
		t.Fatalf("an unknown capability should error")
	}
}

func TestNormalizeParams(t *testing.T) {
	if got := adapters.NormalizeParams(nil); got["type"] != "object" || got["properties"] == nil {
		t.Fatalf("empty schema: %v", got)
	}
	full := map[string]any{"type": "object"}
	if got := adapters.NormalizeParams(full); got["type"] != "object" {
		t.Fatalf("a non-empty schema should pass through: %v", got)
	}
}

// TestExecuteByNameThroughRuntime drives a real JSON-RPC tools/call round trip
// against a fake runtime, exercising the executor wired into the capability.
func TestExecuteByNameThroughRuntime(t *testing.T) {
	d := fake.NewFakeDoer().Route(func(rec fake.Recorded) any {
		switch {
		case rec.Method == "GET" && strings.HasSuffix(rec.URL, "/alice_123/mcps"):
			return fake.NewJSON(200, map[string]any{"data": []any{
				map[string]any{"id": "c1", "slug": "github", "status": "active", "ready": true},
			}})
		case rec.Method == "POST" && strings.HasSuffix(rec.URL, "/tokens"):
			return fake.NewJSON(200, map[string]any{"data": map[string]any{"mcp_url": "https://runtime/vk_live_c1/mcp"}})
		case strings.HasPrefix(rec.URL, "https://runtime/"):
			var env map[string]any
			_ = json.Unmarshal(rec.Body, &env)
			if env["method"] == "tools/call" {
				return fake.NewJSON(200, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{
					"content": []any{map[string]any{"type": "text", "text": "done"}},
				}})
			}
			return fake.NewJSON(200, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"tools": []any{
				map[string]any{"name": "create_issue", "description": "d", "inputSchema": map[string]any{"type": "object"}},
			}}})
		default:
			t.Errorf("unexpected request %s %s", rec.Method, rec.URL)
			return fake.NewJSON(200, map[string]any{})
		}
	})
	v, err := vinkius.New("vk_app_test", "vk_app_sk_test", vinkius.WithDoer(d))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	user, _ := v.User("alice_123")
	set, err := user.Capabilities(context.Background(), nil)
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}

	res, err := adapters.ExecuteByName(context.Background(), set, "github__create_issue", map[string]any{"title": "x"}, nil)
	if err != nil {
		t.Fatalf("ExecuteByName: %v", err)
	}
	if res.Text() != "done" {
		t.Fatalf("text: %q", res.Text())
	}
	// The runtime received a JSON-RPC tools/call with the parsed arguments.
	body, _ := d.Last().BodyJSON()
	bm := body.(map[string]any)
	if bm["method"] != "tools/call" {
		t.Fatalf("method: %v", body)
	}
	if args := bm["params"].(map[string]any)["arguments"].(map[string]any); args["title"] != "x" {
		t.Fatalf("arguments: %v", args)
	}
}
