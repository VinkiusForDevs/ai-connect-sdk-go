package vinkius

import (
	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
)

// Type aliases surface the shared contracts at the package root.
type (
	// Error is the single error type behind all SDK failures.
	Error = core.Error
	// RequestOptions are the optional per-request controls.
	RequestOptions = core.RequestOptions
	// ExecuteOptions are the per-request controls for capability execution.
	ExecuteOptions = core.ExecuteOptions
	// CapabilityResult is the outcome of a capability execution.
	CapabilityResult = core.CapabilityResult
	// ContentPart is one part of a capability result payload.
	ContentPart = core.ContentPart
	// Hooks are observability callbacks that receive only redacted data.
	Hooks = core.Hooks
	// Doer is the transport seam (inject a fake in tests).
	Doer = core.Doer
)

// CapabilityQuery filters a CapabilitySet aggregation. Include and Exclude are
// client-side slug filters applied before any call. OnConnectorError, when set,
// observes a single connector's capability-list failure; by default such
// failures are swallowed so the rest still resolves.
type CapabilityQuery struct {
	core.RequestOptions
	Include          []string
	Exclude          []string
	OnConnectorError func(slug string, err error)
}

// Predicate aliases mirror the core error predicates at the package root, so
// callers can branch on the typed error without importing core directly.

func IsAuthError(err error) bool                    { return core.IsAuthError(err) }
func IsNotFoundError(err error) bool                { return core.IsNotFoundError(err) }
func IsValidationError(err error) bool              { return core.IsValidationError(err) }
func IsRateLimitError(err error) bool               { return core.IsRateLimitError(err) }
func IsQuotaError(err error) bool                   { return core.IsQuotaError(err) }
func IsOverageError(err error) bool                  { return core.IsOverageError(err) }
func IsConnectorNotConnectedError(err error) bool    { return core.IsConnectorNotConnectedError(err) }
func IsNotImplementedError(err error) bool           { return core.IsNotImplementedError(err) }
func IsConnectionError(err error) bool               { return core.IsConnectionError(err) }
func IsProtocolError(err error) bool                 { return core.IsProtocolError(err) }
func IsConfigError(err error) bool                   { return core.IsConfigError(err) }
func IsVinkiusError(err error) bool                  { return core.IsVinkiusError(err) }
func CodeOf(err error) string                         { return core.CodeOf(err) }
func SDKError(err error) *core.Error                  { return core.SDKError(err) }

// Error-constructor aliases for client-side errors the fluent layer raises.
var (
	NewConfigError               = core.NewConfigError
	NewProtocolError             = core.NewProtocolError
	NewConnectorNotConnectedError = core.NewConnectorNotConnectedError
	NewNotImplementedError       = core.NewNotImplementedError
	NewConnectionError           = core.NewConnectionError
)
