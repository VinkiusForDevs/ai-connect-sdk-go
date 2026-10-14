package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
	"github.com/VinkiusForDevs/ai-connect-sdk-go/internal/fake"
)

// newClient wires a control-plane HTTP client over the fake doer. The retry
// policy zeroes its delays so tests that exercise retries do not actually sleep.
func newClient(d *fake.FakeDoer) *core.HttpClient {
	return core.NewHttpClient(core.HttpClientConfig{
		BaseURL:   "https://api.vinkius.com",
		APIKey:    "vk_app_sk_test",
		AppID:     "vk_app_test",
		Retry:     core.RetryPolicy{MaxRetries: 2, BaseDelayMS: 0, MaxDelayMS: 0},
		Doer:      d,
		UserAgent: "vinkius-connect/0.1.0",
	})
}

func TestGetSendsAuthAndAppIDHeaders(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, map[string]any{"data": map[string]any{"ok": true}}))
	c := newClient(d)

	out, err := c.Get(context.Background(), "/apps/vk_app_test/users/alice_123", core.CallOptions{})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if m := out.(map[string]any)["data"].(map[string]any)["ok"]; m != true {
		t.Fatalf("unwrapped body: %v", out)
	}
	last := d.Last()
	if last.Method != "GET" {
		t.Fatalf("method %q", last.Method)
	}
	if !strings.HasSuffix(last.URL, "/apps/vk_app_test/users/alice_123") {
		t.Fatalf("url %q", last.URL)
	}
	if got := last.Headers["authorization"]; got != "Bearer vk_app_sk_test" {
		t.Fatalf("authorization %q", got)
	}
	if got := last.Headers[core.AppIDHeader]; got != "vk_app_test" {
		t.Fatalf("app-id %q", got)
	}
	if !strings.Contains(last.Headers["user-agent"], "vinkius-connect/") {
		t.Fatalf("user-agent %q", last.Headers["user-agent"])
	}
}

func TestPostSerializesBodyAndSetsContentType(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, map[string]any{"data": map[string]any{"id": "c1"}}))
	c := newClient(d)

	_, err := c.Post(context.Background(), "/apps/vk_app_test/users/alice_123/mcps",
		map[string]any{"catalog_mcp_id": "github"}, core.CallOptions{})
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	last := d.Last()
	if last.Method != "POST" {
		t.Fatalf("method %q", last.Method)
	}
	if got := last.Headers["content-type"]; got != "application/json" {
		t.Fatalf("content-type %q", got)
	}
	body, _ := last.BodyJSON()
	if m, ok := body.(map[string]any); !ok || m["catalog_mcp_id"] != "github" {
		t.Fatalf("body %v", body)
	}
}

func TestQueryParamsAreEncoded(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, map[string]any{"data": []any{}}))
	c := newClient(d)

	_, err := c.Get(context.Background(), "/apps/vk_app_test/users",
		core.CallOptions{Query: map[string]any{"status": "active", "page": 2}})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	last := d.Last()
	for _, want := range []string{"status=active", "page=2"} {
		if !strings.Contains(last.URL, want) {
			t.Fatalf("url %q should contain %q", last.URL, want)
		}
	}
	if !strings.HasSuffix(strings.SplitN(last.URL, "?", 2)[0], "/users") {
		t.Fatalf("base path should end at /users: %q", last.URL)
	}
}

func TestIdempotencyKeyHeader(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, map[string]any{"data": map[string]any{}}))
	c := newClient(d)

	_, err := c.Get(context.Background(), "/apps/vk_app_test/users/alice_123",
		core.CallOptions{IdempotencyKey: "idem-1"})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := d.Last().Headers["idempotency-key"]; got != "idem-1" {
		t.Fatalf("idempotency-key %q", got)
	}
}

func Test404MapsToNotFound(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(404, map[string]any{"message": "nope"}))
	c := newClient(d)

	_, err := c.Get(context.Background(), "/apps/vk_app_test/users/ghost", core.CallOptions{})
	if !core.IsNotFoundError(err) {
		t.Fatalf("404 should map to not_found: %v", err)
	}
}

func TestNonIdempotentPostIsNotRetried(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(503, map[string]any{"message": "down"}))
	c := newClient(d)

	o := core.CallOptions{Idempotent: core.PtrBool(false)}
	_, err := c.Post(context.Background(), "/apps/vk_app_test/users/alice_123/mcps/c1/tokens", map[string]any{}, o)
	// Not retry-safe: a final 503 maps to the generic api_error.
	if !core.IsVinkiusError(err) {
		t.Fatalf("expected a vinkius error, got %v", err)
	}
	if code := core.CodeOf(err); code != core.CodeAPIError {
		t.Fatalf("code %q, want api_error", code)
	}
	if d.Len() != 1 {
		t.Fatalf("non-idempotent POST should make one attempt, made %d", d.Len())
	}
}

func TestIdempotencyKeyMakesPostRetryable(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(
		fake.NewJSON(503, map[string]any{"message": "down"}),
		fake.NewJSON(200, map[string]any{"data": map[string]any{"id": "c1"}}),
	)
	c := newClient(d)

	o := core.CallOptions{Idempotent: core.PtrBool(false), IdempotencyKey: "k-1"}
	_, err := c.Post(context.Background(), "/apps/vk_app_test/users/alice_123/mcps", map[string]any{"catalog_mcp_id": "github"}, o)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	// A POST with an idempotency key is retry-safe and may be retried.
	if d.Len() != 2 {
		t.Fatalf("idempotent-keyed POST should retry, made %d", d.Len())
	}
}

func TestTransient503IsRetriedForIdempotentGet(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(
		fake.NewJSON(503, map[string]any{"message": "down"}),
		fake.NewJSON(200, map[string]any{"data": map[string]any{"ok": true}}),
	)
	c := newClient(d)

	if _, err := c.Get(context.Background(), "/apps/vk_app_test/users/alice_123", core.CallOptions{}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if d.Len() != 2 {
		t.Fatalf("transient 503 should retry once, made %d", d.Len())
	}
}

func TestRetryAfterIsHonored(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"message": "slow"})
	d := fake.NewFakeDoer().Enqueue(
		&fake.Response{Status: 429, Body: body, Header: map[string]string{"retry-after": "0"}},
		fake.NewJSON(200, map[string]any{"data": map[string]any{"ok": 1}}),
	)
	c := newClient(d)

	out, err := c.Get(context.Background(), "/apps/vk_app_test/users/alice_123", core.CallOptions{})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if m := out.(map[string]any)["data"].(map[string]any)["ok"]; m != 1.0 {
		t.Fatalf("body: %v", out)
	}
	if d.Len() != 2 {
		t.Fatalf("retry-after should trigger a retry, made %d", d.Len())
	}
}

func TestNetworkErrorWrappedAsConnectionError(t *testing.T) {
	d := fake.NewFakeDoer().
		EnqueueError(errors.New("boom")).
		EnqueueError(errors.New("boom")).
		EnqueueError(errors.New("boom"))
	c := newClient(d)

	_, err := c.Get(context.Background(), "/apps/vk_app_test/users/alice_123", core.CallOptions{})
	if !core.IsConnectionError(err) {
		t.Fatalf("network error should map to connection_error: %v", err)
	}
	if d.Len() != 3 {
		t.Fatalf("retryable GET should exhaust retries, made %d", d.Len())
	}
}

func TestDefaultEndpointIsFixed(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, map[string]any{"data": map[string]any{}}))
	c := newClient(d)

	if _, err := c.Get(context.Background(), "/apps/vk_app_test/users/alice_123", core.CallOptions{}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if !strings.HasPrefix(d.Last().URL, "https://api.vinkius.com/apps/") {
		t.Fatalf("endpoint should be fixed to the Vinkius API: %q", d.Last().URL)
	}
}

type captureHooks struct {
	mu     sync.Mutex
	reqs   []map[string]any
	resps  []map[string]any
}

func (h *captureHooks) OnRequest(method, url string, headers map[string]string, attempt int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.reqs = append(h.reqs, map[string]any{"method": method, "url": url, "headers": headers, "attempt": attempt})
}

func (h *captureHooks) OnResponse(status int, url, requestID string, body any, attempt int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.resps = append(h.resps, map[string]any{"status": status, "url": url, "request_id": requestID, "body": body, "attempt": attempt})
}

func TestHooksReceiveRedactedData(t *testing.T) {
	d := fake.NewFakeDoer().Enqueue(fake.NewJSON(200, map[string]any{"data": map[string]any{"token": "sek"}}))
	hooks := &captureHooks{}
	c := core.NewHttpClient(core.HttpClientConfig{
		BaseURL:   "https://api.vinkius.com",
		APIKey:    "vk_app_sk_test",
		AppID:     "vk_app_test",
		Retry:     core.RetryPolicy{MaxRetries: 2, BaseDelayMS: 0, MaxDelayMS: 0},
		Doer:      d,
		UserAgent: "vinkius-connect/0.1.0",
		Hooks:     hooks,
	})

	if _, err := c.Get(context.Background(), "/apps/vk_app_test/users/alice_123", core.CallOptions{}); err != nil {
		t.Fatalf("get: %v", err)
	}
	hooks.mu.Lock()
	defer hooks.mu.Unlock()

	reqHeaders := hooks.reqs[0]["headers"].(map[string]string)
	if reqHeaders["authorization"] != "[REDACTED]" {
		t.Fatalf("request authorization should be redacted: %q", reqHeaders["authorization"])
	}
	if strings.Contains(fmt.Sprint(reqHeaders), "vk_app_sk_test") {
		t.Fatalf("secret key leaked into request hook: %v", reqHeaders)
	}
	resBody := hooks.resps[0]["body"].(map[string]any)
	if resBody["data"].(map[string]any)["token"] != "[REDACTED]" {
		t.Fatalf("response body should be redacted: %v", resBody)
	}
	if hooks.resps[0]["status"] != 200 {
		t.Fatalf("response status: %v", hooks.resps[0]["status"])
	}
}
