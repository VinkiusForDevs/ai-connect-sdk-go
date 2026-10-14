package core

import "testing"

func TestRedactHeaders(t *testing.T) {
	in := map[string]string{
		"Authorization":   "Bearer vk_app_sk_secret",
		"Idempotency-Key": "k-1",
		"Cookie":          "c=1",
		"x-request-id":    "rid-1",
	}
	out := RedactHeaders(in)
	if out["Authorization"] != Redacted {
		t.Fatalf("authorization not redacted: %q", out["Authorization"])
	}
	if out["Idempotency-Key"] != Redacted {
		t.Fatalf("idempotency-key not redacted: %q", out["Idempotency-Key"])
	}
	if out["Cookie"] != Redacted {
		t.Fatalf("cookie not redacted: %q", out["Cookie"])
	}
	if out["x-request-id"] != "rid-1" {
		t.Fatalf("non-secret header should pass through: %q", out["x-request-id"])
	}
}

func TestRedactURLMasksLiveToken(t *testing.T) {
	in := "https://runtime/vk_live_abc123/mcp"
	out := RedactURL(in)
	if out != "https://runtime/"+Redacted+"/mcp" {
		t.Fatalf("url redaction: %q", out)
	}
	if got := RedactURL("https://api.vinkius.com/apps/a/users/u/mcps/c1"); got != "https://api.vinkius.com/apps/a/users/u/mcps/c1" {
		t.Fatalf("non-live url should be unchanged: %q", got)
	}
}

func TestRedactBodyRedactsSecretKeys(t *testing.T) {
	body := map[string]any{
		"data": map[string]any{
			"token":     "sek",
			"mcp_url":   "https://runtime/vk_live_x/mcp",
			"public":    "kept",
			"nested":    map[string]any{"access_token": "a"},
			"list":      []any{map[string]any{"password": "p"}},
		},
	}
	out, ok := RedactBody(body).(map[string]any)
	if !ok {
		t.Fatalf("redacted body not a map: %T", out)
	}
	data := out["data"].(map[string]any)
	if data["token"] != Redacted {
		t.Fatalf("token not redacted: %v", data["token"])
	}
	if data["mcp_url"] != Redacted {
		t.Fatalf("mcp_url not redacted: %v", data["mcp_url"])
	}
	if data["public"] != "kept" {
		t.Fatalf("public value should survive: %v", data["public"])
	}
	if nested := data["nested"].(map[string]any); nested["access_token"] != Redacted {
		t.Fatalf("nested secret not redacted: %v", nested)
	}
	list := data["list"].([]any)
	if m := list[0].(map[string]any); m["password"] != Redacted {
		t.Fatalf("list secret not redacted: %v", m)
	}
}

func TestRedactBodyIsCycleSafeAndPure(t *testing.T) {
	cycle := map[string]any{"a": 1}
	cycle["self"] = cycle
	out, ok := RedactBody(cycle).(map[string]any)
	if !ok {
		t.Fatalf("cycle redact not a map: %T", out)
	}
	if out["a"] != 1 {
		t.Fatalf("cycle redact lost a value: %v", out)
	}
	// The input must not be mutated.
	if v, _ := cycle["a"].(int); v != 1 {
		t.Fatalf("input mutated: %v", cycle["a"])
	}
}
