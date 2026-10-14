// Package fake provides an in-memory core.Doer for tests: it records each
// request and replays queued responses or errors. It stands in for the network
// seam so the transport, HTTP, runtime, and fluent layers can be tested without
// touching the wire.
package fake

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
)

// Recorded is a captured request. Headers and Body are copies, safe to inspect
// after the request completes.
type Recorded struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    []byte
}

// BodyJSON returns the parsed JSON body of the recorded request, or the raw
// string when the body is not JSON.
func (r *Recorded) BodyJSON() (any, error) {
	if len(r.Body) == 0 {
		return nil, nil
	}
	var out any
	if err := json.Unmarshal(r.Body, &out); err != nil {
		return string(r.Body), nil
	}
	return out, nil
}

// Response is a queued fake response. Body is the raw response byte string; the
// transport JSON-decodes it when possible and otherwise keeps it as a string.
// Header keys are stored lower-cased so lookups are case-insensitive.
type Response struct {
	Status int
	Body   []byte
	Header map[string]string
}

// NewJSON builds a response with a JSON-serialized body.
func NewJSON(status int, body any) *Response {
	encoded, err := json.Marshal(body)
	if err != nil {
		encoded = []byte("{}")
	}
	return &Response{Status: status, Body: encoded}
}

// NewText builds a response with a raw text body (for example an SSE stream).
func NewText(status int, text string) *Response {
	return &Response{Status: status, Body: []byte(text)}
}

// FakeDoer implements core.Doer. It records each request and returns queued
// responses, queued errors, or a URL-keyed routed response.
type FakeDoer struct {
	mu       sync.Mutex
	Requests []Recorded
	queue    []any
	handler  func(Recorded) any
}

// NewFakeDoer builds an empty fake doer.
func NewFakeDoer() *FakeDoer { return &FakeDoer{} }

// Enqueue queues responses or errors to be returned in order. A nil item yields
// a 200 empty-object response.
func (f *FakeDoer) Enqueue(items ...any) *FakeDoer {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queue = append(f.queue, items...)
	return f
}

// EnqueueError queues a transport error, simulating a network failure.
func (f *FakeDoer) EnqueueError(err error) *FakeDoer {
	return f.Enqueue(err)
}

// Route installs a URL-keyed handler that takes priority over the queue. Use it
// for deterministic, order-independent responses (needed by concurrent fan-out).
func (f *FakeDoer) Route(handler func(Recorded) any) *FakeDoer {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handler = handler
	return f
}

// Last returns the most recently recorded request, or nil when none.
func (f *FakeDoer) Last() *Recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Requests) == 0 {
		return nil
	}
	rec := f.Requests[len(f.Requests)-1]
	return &rec
}

// BodyOf returns the parsed JSON body of the last recorded request.
func (f *FakeDoer) BodyOf() (any, error) {
	last := f.Last()
	if last == nil {
		return nil, nil
	}
	return last.BodyJSON()
}

// Len returns the number of recorded requests.
func (f *FakeDoer) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.Requests)
}

// Do implements core.Doer.
func (f *FakeDoer) Do(req *core.DoRequest, _ context.Context) (*core.DoResponse, error) {
	headers := make(map[string]string, len(req.Headers))
	for k, v := range req.Headers {
		headers[k] = v
	}
	body := make([]byte, len(req.Body))
	copy(body, req.Body)
	rec := Recorded{Method: req.Method, URL: req.URL, Headers: headers, Body: body}

	f.mu.Lock()
	f.Requests = append(f.Requests, rec)
	handler := f.handler
	var item any
	switch {
	case handler != nil:
		item = handler(rec)
	case len(f.queue) > 0:
		item = f.queue[0]
		f.queue = f.queue[1:]
	default:
		item = NewJSON(200, map[string]any{})
	}
	f.mu.Unlock()

	if err, ok := item.(error); ok {
		return nil, err
	}
	if resp, ok := item.(*Response); ok && resp != nil {
		return &core.DoResponse{Status: resp.Status, Body: resp.Body, Header: resp.Header}, nil
	}
	return &core.DoResponse{Status: 200, Body: []byte("{}")}, nil
}
