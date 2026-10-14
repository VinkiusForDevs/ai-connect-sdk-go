package core

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// DoRequest is a single HTTP request handed to a Doer. Body is nil when the
// request carries no payload. The Body is a byte slice (not an io.Reader) so a
// retried request can replay the identical payload.
type DoRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    []byte
}

// DoResponse is the raw response returned by a Doer.
type DoResponse struct {
	Status int
	Body   []byte
	// Header carries response headers with lower-cased keys so lookups are
	// case-insensitive.
	Header map[string]string
}

// GetHeader returns the named response header, matched case-insensitively.
func (r *DoResponse) GetHeader(name string) string {
	if r.Header == nil {
		return ""
	}
	return r.Header[strings.ToLower(name)]
}

// Doer is the transport seam between the SDK and the network. The SDK's default
// implementation (HTTPDoer) is built on net/http; tests and custom runtimes
// inject their own.
type Doer interface {
	Do(req *DoRequest, ctx context.Context) (*DoResponse, error)
}

// HTTPDoer is the default Doer, backed by net/http.
type HTTPDoer struct {
	client *http.Client
}

// NewHTTPDoer builds the default transport with a dedicated client and a 30s
// wall-clock request timeout. The wall-clock timeout is a backstop; callers
// prefer per-request deadlines via context.
func NewHTTPDoer() *HTTPDoer {
	return &HTTPDoer{client: &http.Client{Timeout: 30 * time.Second}}
}

// Do performs one HTTP request, following redirects and filling DoResponse with
// the parsed status, body, and lower-cased headers.
func (d *HTTPDoer) Do(req *DoRequest, ctx context.Context) (*DoResponse, error) {
	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
	if err != nil {
		return nil, err
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	resp, err := d.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	header := make(map[string]string, len(resp.Header))
	for k := range resp.Header {
		header[strings.ToLower(k)] = resp.Header.Get(k)
	}
	return &DoResponse{Status: resp.StatusCode, Body: raw, Header: header}, nil
}

// Close releases idle connections on the underlying client.
func (d *HTTPDoer) Close() {
	if d != nil && d.client != nil {
		d.client.CloseIdleConnections()
	}
}

// isTimeoutErr reports whether err is a deadline/timeout failure.
func isTimeoutErr(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}
