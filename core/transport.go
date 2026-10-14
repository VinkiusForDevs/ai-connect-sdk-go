package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Hooks are optional observability callbacks. They receive ONLY redacted data:
// secret headers are masked and any vk_live_* path segment in the URL is
// replaced, so no secret ever reaches a hook.
//
// Attempt is the 0-based retry attempt that produced the event.
type Hooks interface {
	OnRequest(method, url string, headers map[string]string, attempt int)
	OnResponse(status int, url, requestID string, body any, attempt int)
}

// TransportRequest is a single logical request driven through one fetch loop.
type TransportRequest struct {
	URL     string
	Method  string
	Headers map[string]string
	Body    []byte
	// Retryable controls whether transport-level failures and transient status
	// codes may be retried.
	Retryable bool
	// Label is a noun used in error messages, e.g. "Request" or "Runtime request".
	Label string
	// Timeout is a per-request deadline; zero falls back to the transport default.
	Timeout time.Duration
}

// TransportResult is the final response of one logical request. The caller maps
// non-2xx statuses to typed errors, since control-plane and data-plane
// contracts differ.
type TransportResult struct {
	Status       int
	Body         any
	RequestID    string
	RetryAfterMS *int
}

// TransportConfig binds the shared transport settings.
type TransportConfig struct {
	Timeout   time.Duration
	Retry     RetryPolicy
	Doer      Doer
	UserAgent string
	Hooks     Hooks
}

// Transport shares the fetch loop between the control plane (HttpClient) and the
// data plane (RuntimeClient) so timeout, retry, and hook semantics stay
// consistent.
type Transport struct {
	cfg TransportConfig
}

// NewTransport builds a Transport from its config.
func NewTransport(cfg TransportConfig) *Transport {
	return &Transport{cfg: cfg}
}

// Send performs one logical request, retrying transient failures when the
// request is retryable, and returns the final response. It returns a *Error only
// for transport-level failures (network errors, cancellation); a non-2xx
// response is returned as a TransportResult for the caller to map.
func (t *Transport) Send(ctx context.Context, req *TransportRequest) (*TransportResult, error) {
	maxAttempts := 1
	if req.Retryable {
		maxAttempts = t.cfg.Retry.MaxRetries + 1
	}
	timeout := req.Timeout
	if timeout == 0 {
		timeout = t.cfg.Timeout
	}

	doReq := &DoRequest{Method: req.Method, URL: req.URL, Headers: req.Headers, Body: req.Body}
	attempt := 0
	for {
		t.emitRequest(req, attempt)

		cctx := ctx
		var cancel context.CancelFunc
		if timeout > 0 {
			cctx, cancel = context.WithTimeout(ctx, timeout)
		}
		resp, err := t.cfg.Doer.Do(doReq, cctx)
		if cancel != nil {
			cancel()
		}

		if err != nil {
			if req.Retryable && attempt < maxAttempts-1 {
				if !t.pause(ctx, attempt, nil) {
					return nil, NewConnectionError(fmt.Sprintf("%s cancelled", req.Label), ctx.Err())
				}
				attempt++
				continue
			}
			if isTimeoutErr(err) {
				return nil, NewConnectionError(fmt.Sprintf("%s timed out after %dms", req.Label, int(timeout.Milliseconds())), err)
			}
			return nil, NewConnectionError(fmt.Sprintf("%s failed", req.Label), err)
		}

		requestID := extractRequestID(resp)
		body := parseBody(resp.Body)
		retryAfterMS := ParseRetryAfter(resp.GetHeader("retry-after"))

		t.emitResponse(req, resp.Status, requestID, body, attempt)

		if resp.Status >= 200 && resp.Status < 300 {
			return &TransportResult{Status: resp.Status, Body: body, RequestID: requestID, RetryAfterMS: retryAfterMS}, nil
		}
		if req.Retryable && IsRetryableStatus(resp.Status) && attempt < maxAttempts-1 {
			if !t.pause(ctx, attempt, retryAfterMS) {
				return nil, NewConnectionError(fmt.Sprintf("%s cancelled", req.Label), ctx.Err())
			}
			attempt++
			continue
		}
		return &TransportResult{Status: resp.Status, Body: body, RequestID: requestID, RetryAfterMS: retryAfterMS}, nil
	}
}

// pause sleeps for the backoff delay, honoring context cancellation. It reports
// whether the sleep completed (true) or the context finished first (false).
func (t *Transport) pause(ctx context.Context, attempt int, retryAfterMS *int) bool {
	delay := BackoffDelay(attempt, t.cfg.Retry, retryAfterMS)
	if delay <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(time.Duration(delay) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// extractRequestID pulls the server request id from response headers.
func extractRequestID(resp *DoResponse) string {
	for _, name := range []string{"x-request-id", "x-vinkius-request-id"} {
		if v := resp.GetHeader(name); v != "" {
			return v
		}
	}
	return ""
}

// parseBody decodes a JSON body when possible, otherwise keeps the raw text.
// An empty body is nil.
func parseBody(raw []byte) any {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	var out any
	if err := json.Unmarshal(trimmed, &out); err == nil {
		return out
	}
	return string(raw)
}

func (t *Transport) emitRequest(req *TransportRequest, attempt int) {
	if hooks := t.cfg.Hooks; hooks != nil {
		hooks.OnRequest(req.Method, RedactURL(req.URL), RedactHeaders(req.Headers), attempt)
	}
}

func (t *Transport) emitResponse(req *TransportRequest, status int, requestID string, body any, attempt int) {
	if hooks := t.cfg.Hooks; hooks != nil {
		hooks.OnResponse(status, RedactURL(req.URL), requestID, RedactBody(body), attempt)
	}
}
