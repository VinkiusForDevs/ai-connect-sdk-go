package vinkius_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/internal/fake"
	"github.com/VinkiusForDevs/ai-connect-sdk-go/vinkius"
)

func makeVinkius(t *testing.T, d *fake.FakeDoer, extra ...vinkius.Option) *vinkius.Vinkius {
	opts := append([]vinkius.Option{vinkius.WithDoer(d)}, extra...)
	v, err := vinkius.New("vk_app_test", "vk_app_sk_test", opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return v
}

func TestUserGet(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, map[string]any{"data": map[string]any{"external_id": "alice_123", "ok": true}}))
	v := makeVinkius(t, d)

	user, err := v.User("alice_123")
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	out, err := user.Get(context.Background(), nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if out["external_id"] != "alice_123" {
		t.Fatalf("out: %v", out)
	}
	if !strings.HasSuffix(d.Last().URL, "/apps/vk_app_test/users/alice_123") {
		t.Fatalf("url: %q", d.Last().URL)
	}
}

func TestUserGetSendsIdempotencyKey(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, map[string]any{"data": map[string]any{}}))
	v := makeVinkius(t, d)

	user, _ := v.User("alice_123")
	if _, err := user.Get(context.Background(), &vinkius.RequestOptions{IdempotencyKey: "idem-1"}); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := d.Last().Headers["idempotency-key"]; got != "idem-1" {
		t.Fatalf("idempotency-key: %q", got)
	}
}

func TestConnectionsCreate(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, map[string]any{"data": map[string]any{"id": "c1"}}))
	v := makeVinkius(t, d)

	conns, err := v.Users.Connections("alice_123")
	if err != nil {
		t.Fatalf("Connections: %v", err)
	}
	out, err := conns.Create(context.Background(), map[string]any{"connector": "github"}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if out["id"] != "c1" {
		t.Fatalf("out: %v", out)
	}
	last := d.Last()
	if last.Method != "POST" {
		t.Fatalf("method: %s", last.Method)
	}
	body, _ := last.BodyJSON()
	if body.(map[string]any)["catalog_mcp_id"] != "github" {
		t.Fatalf("body: %v", body)
	}
}

func TestTokensIssueNotRetried(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(503, map[string]any{"message": "down"}))
	v := makeVinkius(t, d)

	conns, _ := v.Users.Connections("alice_123")
	_, err := conns.Tokens("c1").Issue(context.Background(), nil, nil)
	// The token mint is a non-retry-safe POST: a final 503 is one attempt.
	if !vinkius.IsVinkiusError(err) {
		t.Fatalf("expected a vinkius error: %v", err)
	}
	if code := vinkius.CodeOf(err); code != "api_error" {
		t.Fatalf("code: %s", code)
	}
	if d.Len() != 1 {
		t.Fatalf("issue should make one attempt, made %d", d.Len())
	}
}

func TestTokensIssueBodyAndOneTimeURL(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, map[string]any{"data": map[string]any{"token": "vk_live_c1", "mcp_url": "https://runtime/vk_live_c1/mcp"}}))
	v := makeVinkius(t, d)

	conns, _ := v.Users.Connections("alice_123")
	out, err := conns.Tokens("c1").Issue(context.Background(), map[string]any{"name": "prod"}, nil)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	body, _ := d.Last().BodyJSON()
	if body.(map[string]any)["name"] != "prod" {
		t.Fatalf("body: %v", body)
	}
	if out["mcp_url"] == "" {
		t.Fatalf("the one-time mcp_url should be returned: %v", out)
	}
}

func TestCatalogSearchNormalizes(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, map[string]any{
		"results": []any{
			map[string]any{"id": "1", "slug": "github", "title": "GitHub", "server_type": "remote", "requires_auth": true},
		},
	}))
	v := makeVinkius(t, d)

	out, err := v.Catalog.Search(context.Background(), "github", nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("out: %v", out)
	}
	// listing_type falls back to server_type; requires_buyer_auth to requires_auth.
	if out[0]["listing_type"] != "remote" {
		t.Fatalf("listing_type fallback: %v", out[0])
	}
	if out[0]["requires_buyer_auth"] != true {
		t.Fatalf("requires_buyer_auth fallback: %v", out[0])
	}
	if !strings.Contains(d.Last().URL, "q=github") {
		t.Fatalf("url: %q", d.Last().URL)
	}
}

func TestCatalogGetIncludesCredentialSchema(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, map[string]any{
		"data": map[string]any{"slug": "github", "credential_schema": map[string]any{"properties": map[string]any{"token": map[string]any{"type": "string"}}}},
	}))
	v := makeVinkius(t, d)

	out, err := v.Catalog.Get(context.Background(), "github", nil)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if out["slug"] != "github" {
		t.Fatalf("out: %v", out)
	}
	if out["credential_schema"] == nil {
		t.Fatalf("credential_schema missing: %v", out)
	}
}

func TestConnectorConnectProvisionsRuntime(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(
		fake.NewJSON(200, map[string]any{"data": map[string]any{"id": "c1", "slug": "github", "status": "active", "ready": true}}),
		fake.NewJSON(200, map[string]any{"data": map[string]any{"mcp_url": "https://runtime/vk_live_c1/mcp"}}),
	)
	v := makeVinkius(t, d)

	conn := userConnector(t, v, "alice_123", "github")
	out, err := conn.Connect(context.Background(), nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if out["runtime_url"] != "https://runtime/vk_live_c1/mcp" {
		t.Fatalf("runtime_url: %v", out)
	}
	if out["id"] != "c1" {
		t.Fatalf("id: %v", out)
	}
	if d.Len() != 2 {
		t.Fatalf("connect should make create + issue calls, made %d", d.Len())
	}
}

func TestConnectorStatusNotConnected(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, map[string]any{"data": []any{}}))
	v := makeVinkius(t, d)

	status, err := userConnector(t, v, "alice_123", "github").Status(context.Background(), nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status != vinkius.StatusNotConnected {
		t.Fatalf("status: %s", status)
	}
}

func TestResolveConnectionIDNotConnected(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, map[string]any{"data": []any{}}))
	v := makeVinkius(t, d)

	_, err := userConnector(t, v, "alice_123", "github").ResolveConnectionID(context.Background(), nil)
	if !vinkius.IsConnectorNotConnectedError(err) {
		t.Fatalf("expected connector_not_connected: %v", err)
	}
}

func TestCapabilitiesFanOut(t *testing.T) {
	d := fake.NewFakeDoer().Route(func(rec fake.Recorded) any {
		switch {
		case rec.Method == "GET" && strings.HasSuffix(rec.URL, "/alice_123/mcps"):
			return fake.NewJSON(200, map[string]any{"data": []any{
				map[string]any{"id": "c1", "slug": "github", "status": "active", "ready": true},
				map[string]any{"id": "c2", "slug": "gitlab", "status": "active", "ready": true},
			}})
		case rec.Method == "POST" && strings.HasSuffix(rec.URL, "/tokens"):
			id := rec.URL[:strings.LastIndex(rec.URL, "/")+1]
			return fake.NewJSON(200, map[string]any{"data": map[string]any{"mcp_url": "https://runtime/vk_live_" + id + "/mcp"}})
		case strings.HasPrefix(rec.URL, "https://runtime/"):
			var env map[string]any
			_ = json.Unmarshal(rec.Body, &env)
			if env["method"] == "tools/call" {
				return fake.NewJSON(200, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"content": []any{map[string]any{"type": "text", "text": "done"}}}})
			}
			return fake.NewJSON(200, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"tools": []any{
				map[string]any{"name": "do_thing", "description": "d", "inputSchema": map[string]any{"type": "object"}},
			}}})
		default:
			t.Errorf("unexpected request %s %s", rec.Method, rec.URL)
			return fake.NewJSON(200, map[string]any{})
		}
	})
	v := makeVinkius(t, d)

	user, _ := v.User("alice_123")
	set, err := user.Capabilities(context.Background(), nil)
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if set.Len() != 2 {
		t.Fatalf("expected 2 capabilities, got %d", set.Len())
	}
	names := map[string]bool{}
	for _, c := range set.Items() {
		names[c.Name] = true
	}
	if !names["github__do_thing"] || !names["gitlab__do_thing"] {
		t.Fatalf("expected namespaced capabilities, got %v", names)
	}
}

func TestCredentialsSet(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(
		fake.NewJSON(200, map[string]any{"data": []any{map[string]any{"id": "c1", "slug": "github", "status": "active", "ready": true}}}),
		fake.NewJSON(200, map[string]any{"data": map[string]any{"set": []any{"token"}}}),
	)
	v := makeVinkius(t, d)

	conn := userConnector(t, v, "alice_123", "github")
	out, err := conn.Credentials.Set(context.Background(), map[string]any{"token": "sek"}, nil)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	// The low-level client returns the data envelope, mirroring the reference.
	if out["data"] == nil {
		t.Fatalf("out: %v", out)
	}
	last := d.Last()
	if last.Method != "PUT" {
		t.Fatalf("method: %s", last.Method)
	}
	body, _ := last.BodyJSON()
	if body.(map[string]any)["credentials"].(map[string]any)["token"] != "sek" {
		t.Fatalf("body should wrap the flat map: %v", body)
	}
}

func TestCredentialsSchemaIsCached(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, map[string]any{
		"data": map[string]any{"slug": "github", "credential_schema": map[string]any{"properties": map[string]any{"token": map[string]any{"type": "string"}}}},
	}))
	v := makeVinkius(t, d)

	conn := userConnector(t, v, "alice_123", "github")
	schema, err := conn.Credentials.Schema(context.Background(), nil)
	if err != nil {
		t.Fatalf("Schema: %v", err)
	}
	if schema["properties"] == nil {
		t.Fatalf("schema: %v", schema)
	}
	if d.Len() != 1 {
		t.Fatalf("the catalog lookup should fetch once, made %d", d.Len())
	}
}

// userConnector resolves a user and one of its connectors, failing the test on
// any error.
func userConnector(t *testing.T, v *vinkius.Vinkius, externalID, slug string) *vinkius.Connector {
	user, err := v.User(externalID)
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	return user.Connector(slug)
}
