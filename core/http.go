package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// AppIDHeader carries the public app id; the API authenticates it against the key.
const AppIDHeader = "x-vinkius-app-id"

// CallOptions are the optional per-request controls for a control-plane call.
//
// Idempotent is a tri-state: nil means "use the method default" (idempotent for
// GET/PUT/DELETE, otherwise not). Declaring IdempotencyKey is itself a
// retry-safety declaration: the server deduplicates replays, so such a request
// is retried even when its method is not idempotent.
type CallOptions struct {
	Query          map[string]any
	Headers        map[string]string
	Idempotent     *bool
	IdempotencyKey string
	Timeout        time.Duration
}

// PtrBool returns a pointer to b, for the tri-state CallOptions.Idempotent.
func PtrBool(b bool) *bool { return &b }

// HttpClientConfig binds the control-plane HTTP client settings.
type HttpClientConfig struct {
	BaseURL   string
	APIKey    string
	AppID     string
	Timeout   time.Duration
	Retry     RetryPolicy
	Doer      Doer
	UserAgent string
	Hooks     Hooks
}

// HttpClient is the control-plane transport: Bearer auth with the vk_app_sk_*
// key, automatic retries (idempotent requests only), consistent error mapping,
// and redacted observability hooks.
type HttpClient struct {
	cfg HttpClientConfig
	t   *Transport
}

// NewHttpClient builds a control-plane HTTP client.
func NewHttpClient(cfg HttpClientConfig) *HttpClient {
	return &HttpClient{
		cfg: cfg,
		t: NewTransport(TransportConfig{
			Timeout:   cfg.Timeout,
			Retry:     cfg.Retry,
			Doer:      cfg.Doer,
			UserAgent: cfg.UserAgent,
			Hooks:     cfg.Hooks,
		}),
	}
}

// Get issues a GET and returns the parsed JSON body.
func (c *HttpClient) Get(ctx context.Context, path string, o CallOptions) (any, error) {
	return c.Request(ctx, "GET", path, nil, o)
}

// Post issues a POST and returns the parsed JSON body.
func (c *HttpClient) Post(ctx context.Context, path string, body any, o CallOptions) (any, error) {
	return c.Request(ctx, "POST", path, body, o)
}

// Put issues a PUT and returns the parsed JSON body.
func (c *HttpClient) Put(ctx context.Context, path string, body any, o CallOptions) (any, error) {
	return c.Request(ctx, "PUT", path, body, o)
}

// Patch issues a PATCH and returns the parsed JSON body.
func (c *HttpClient) Patch(ctx context.Context, path string, body any, o CallOptions) (any, error) {
	return c.Request(ctx, "PATCH", path, body, o)
}

// Delete issues a DELETE.
func (c *HttpClient) Delete(ctx context.Context, path string, o CallOptions) (any, error) {
	return c.Request(ctx, "DELETE", path, nil, o)
}

// Request issues one logical HTTP request and returns the parsed JSON body.
// Non-2xx responses are mapped to a typed *Error.
func (c *HttpClient) Request(ctx context.Context, method, path string, body any, o CallOptions) (any, error) {
	fullURL := c.buildURL(path, o.Query)
	headers := c.buildHeaders(o.Headers, body != nil, o.IdempotencyKey)

	idempotent := false
	switch {
	case o.Idempotent != nil:
		idempotent = *o.Idempotent
	default:
		idempotent = method == "GET" || method == "PUT" || method == "DELETE"
	}
	retryable := idempotent || o.IdempotencyKey != ""

	var bodyBytes []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyBytes = encoded
	}

	result, err := c.t.Send(ctx, &TransportRequest{
		URL:       fullURL,
		Method:    method,
		Headers:   headers,
		Body:      bodyBytes,
		Retryable: retryable,
		Label:     "Request",
		Timeout:   o.Timeout,
	})
	if err != nil {
		return nil, err
	}
	if result.Status >= 200 && result.Status < 300 {
		return result.Body, nil
	}
	return nil, MapHTTPError(result.Status, result.Body, result.RequestID, result.RetryAfterMS)
}

// buildURL joins the fixed base URL, the resource path, and an encoded query,
// skipping nil query values.
func (c *HttpClient) buildURL(path string, query map[string]any) string {
	base := strings.TrimRight(c.cfg.BaseURL, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	full := base + path
	if len(query) > 0 {
		vals := url.Values{}
		for k, v := range query {
			if v == nil {
				continue
			}
			vals.Set(k, fmt.Sprint(v))
		}
		if q := vals.Encode(); q != "" {
			if strings.Contains(full, "?") {
				full += "&" + q
			} else {
				full += "?" + q
			}
		}
	}
	return full
}

// buildHeaders assembles the request headers: Bearer auth, the app-id binding,
// JSON accept, the user agent, optional extras, content type, and the
// idempotency key.
func (c *HttpClient) buildHeaders(extra map[string]string, hasBody bool, idempotencyKey string) map[string]string {
	headers := map[string]string{
		"authorization": "Bearer " + c.cfg.APIKey,
		AppIDHeader:     c.cfg.AppID,
		"accept":        "application/json",
		"user-agent":    c.cfg.UserAgent,
	}
	for k, v := range extra {
		headers[k] = v
	}
	if hasBody {
		headers["content-type"] = "application/json"
	}
	if idempotencyKey != "" {
		headers["idempotency-key"] = idempotencyKey
	}
	return headers
}
