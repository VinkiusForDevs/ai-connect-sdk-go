package core

import (
	"strings"
	"time"
)

// ContentPart is one part of a capability result payload.
type ContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CapabilityResult is the outcome of a capability execution. IsError is a
// result, not an error: a failing capability comes back as IsError=true so an
// agent loop can feed the failure back to the model and recover.
type CapabilityResult struct {
	// Content holds the (optionally multi-part) textual result.
	Content []ContentPart
	// IsError is set when the tool execution reported a failure.
	IsError bool
	// StructuredContent carries the parsed output when the connector provides
	// one; it is surfaced verbatim, never parsed or transformed by the SDK.
	StructuredContent any
}

// Text joins the textual content parts with newlines (the common case).
func (r *CapabilityResult) Text() string {
	parts := make([]string, 0, len(r.Content))
	for _, p := range r.Content {
		parts = append(parts, p.Text)
	}
	return strings.Join(parts, "\n")
}

// RequestOptions are the optional per-request controls accepted by all SDK calls.
type RequestOptions struct {
	// IdempotencyKey is forwarded as the Idempotency-Key header. Declaring it
	// makes a request retry-safe even when its method is not idempotent.
	IdempotencyKey string
	// Timeout is a per-request deadline that overrides the client-level default.
	// A zero value uses the client default.
	Timeout time.Duration
}

// ExecuteOptions are the per-request controls for capability execution.
type ExecuteOptions struct {
	RequestOptions
}
