// Package core is the shared infrastructure of the Vinkius Connect Go SDK:
// transport, retry, redaction, caching, pagination, error mapping, and the
// JSON-RPC runtime client. The fluent client and the low-level resource
// clients build on it.
package core

import "fmt"

// Stable, machine-readable error codes.
const (
	CodeConfigError          = "config_error"
	CodeAuthError            = "auth_error"
	CodeNotFound             = "not_found"
	CodeValidationError      = "validation_error"
	CodeRateLimit            = "rate_limit"
	CodeQuotaExceeded        = "quota_exceeded"
	CodeOverageBlocked       = "overage_blocked"
	CodeConnectorNotConn     = "connector_not_connected"
	CodeNotImplemented       = "not_implemented"
	CodeConnectionError      = "connection_error"
	CodeProtocolError        = "protocol_error"
	CodeAPIError             = "api_error"
)

// Error is the single type behind all failures the SDK surfaces. It carries
// Status (0 for client-side and transport errors), a stable Code, and the
// server RequestID when available. Details only ever holds the parsed API
// body, which by contract never echoes credentials, so no secret is placed on
// an error object.
type Error struct {
	Message   string
	Code      string
	Status    int
	RequestID string
	Details   any
	Cause     error

	// Fields set by the specific constructors:
	ValidationErrors map[string][]string // CodeValidationError: field -> messages
	RetryAfterMS     int                 // CodeRateLimit: server-advised wait
	UpgradeURL       string              // CodeQuotaExceeded / CodeOverageBlocked
}

// Error implements the error interface.
func (e *Error) Error() string { return e.Message }

// Unwrap returns the underlying transport cause, if any.
func (e *Error) Unwrap() error { return e.Cause }

// NewConfigError builds a client-side configuration error.
func NewConfigError(msg string) *Error {
	return &Error{Message: msg, Code: CodeConfigError}
}

// NewProtocolError builds an error for a response that violated the wire contract.
func NewProtocolError(msg string, details any) *Error {
	return &Error{Message: msg, Code: CodeProtocolError, Details: details}
}

// NewConnectorNotConnectedError builds a client-side error for an unconnected slug.
func NewConnectorNotConnectedError(msg string) *Error {
	return &Error{Message: msg, Code: CodeConnectorNotConn}
}

// NewNotImplementedError builds an error for a feature awaiting an endpoint.
func NewNotImplementedError(msg string) *Error {
	return &Error{Message: msg, Code: CodeNotImplemented}
}

// NewConnectionError builds a transport-level error (network, timeout, aborted).
func NewConnectionError(msg string, cause error) *Error {
	return &Error{Message: msg, Code: CodeConnectionError, Cause: cause}
}

// NewAuthError builds a 401/403 error.
func NewAuthError(msg string, status int, details any, requestID string) *Error {
	return &Error{Message: msg, Code: CodeAuthError, Status: status, Details: details, RequestID: requestID}
}

// NewNotFoundError builds a 404 error.
func NewNotFoundError(msg string, status int, details any, requestID string) *Error {
	return &Error{Message: msg, Code: CodeNotFound, Status: status, Details: details, RequestID: requestID}
}

// NewValidationError builds a 422 error carrying the per-field messages.
func NewValidationError(msg string, status int, details any, requestID string, fieldErrors map[string][]string) *Error {
	if fieldErrors == nil {
		fieldErrors = map[string][]string{}
	}
	return &Error{
		Message: msg, Code: CodeValidationError, Status: status,
		Details: details, RequestID: requestID, ValidationErrors: fieldErrors,
	}
}

// NewRateLimitError builds a control-plane 429 error.
func NewRateLimitError(msg string, status int, details any, requestID string, retryAfterMS int) *Error {
	return &Error{
		Message: msg, Code: CodeRateLimit, Status: status,
		Details: details, RequestID: requestID, RetryAfterMS: retryAfterMS,
	}
}

// NewQuotaError builds a data-plane 429 error (plan quota exhausted).
func NewQuotaError(msg string, status int, details any, requestID string, upgradeURL string) *Error {
	return &Error{
		Message: msg, Code: CodeQuotaExceeded, Status: status,
		Details: details, RequestID: requestID, UpgradeURL: upgradeURL,
	}
}

// NewOverageError builds a 402 error.
func NewOverageError(msg string, status int, details any, requestID string, upgradeURL string) *Error {
	return &Error{
		Message: msg, Code: CodeOverageBlocked, Status: status,
		Details: details, RequestID: requestID, UpgradeURL: upgradeURL,
	}
}

// MapHTTPError maps an HTTP error response to the appropriate *Error.
func MapHTTPError(status int, body any, requestID string, retryAfterMS *int) *Error {
	message := pickMessage(body, fmt.Sprintf("Request failed with status %d", status))
	switch {
	case status == 401 || status == 403:
		return NewAuthError(message, status, body, requestID)
	case status == 404:
		return NewNotFoundError(message, status, body, requestID)
	case status == 422:
		return NewValidationError(message, status, body, requestID, pickFieldErrors(body))
	case status == 402:
		return NewOverageError(message, status, body, requestID, pickUpgradeURL(body))
	case status == 429:
		if isMCPShaped(body) {
			return NewQuotaError(message, status, body, requestID, pickUpgradeURL(body))
		}
		retry := 0
		if retryAfterMS != nil {
			retry = *retryAfterMS
		}
		return NewRateLimitError(message, status, body, requestID, retry)
	default:
		return &Error{Message: message, Code: CodeAPIError, Status: status, Details: body, RequestID: requestID}
	}
}

// CodeOf unwraps err to the underlying *Error and returns its Code, or "" if
// err is not a SDK error.
func CodeOf(err error) string {
	for err != nil {
		if e, ok := err.(*Error); ok {
			return e.Code
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return ""
		}
		err = u.Unwrap()
	}
	return ""
}

// SDKError unwraps err to the underlying *Error, or nil if it is not one.
func SDKError(err error) *Error {
	for err != nil {
		if e, ok := err.(*Error); ok {
			return e
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return nil
		}
		err = u.Unwrap()
	}
	return nil
}

// IsVinkiusError reports whether err is any SDK error.
func IsVinkiusError(err error) bool { return SDKError(err) != nil }

func isCode(err error, code string) bool { return CodeOf(err) == code }

// IsAuthError reports whether err is an authentication/authorization failure.
func IsAuthError(err error) bool { return isCode(err, CodeAuthError) }

// IsNotFoundError reports whether err is a 404.
func IsNotFoundError(err error) bool { return isCode(err, CodeNotFound) }

// IsValidationError reports whether err is a 422.
func IsValidationError(err error) bool { return isCode(err, CodeValidationError) }

// IsRateLimitError reports whether err is a control-plane 429.
func IsRateLimitError(err error) bool { return isCode(err, CodeRateLimit) }

// IsQuotaError reports whether err is a data-plane 429 (quota).
func IsQuotaError(err error) bool { return isCode(err, CodeQuotaExceeded) }

// IsOverageError reports whether err is a 402.
func IsOverageError(err error) bool { return isCode(err, CodeOverageBlocked) }

// IsConnectorNotConnectedError reports a client-side unconnected-connector error.
func IsConnectorNotConnectedError(err error) bool { return isCode(err, CodeConnectorNotConn) }

// IsNotImplementedError reports a feature awaiting an endpoint.
func IsNotImplementedError(err error) bool { return isCode(err, CodeNotImplemented) }

// IsConnectionError reports a network/timeout/aborted transport error.
func IsConnectionError(err error) bool { return isCode(err, CodeConnectionError) }

// IsProtocolError reports a response that violated the wire contract.
func IsProtocolError(err error) bool { return isCode(err, CodeProtocolError) }

// IsConfigError reports a client-side configuration error.
func IsConfigError(err error) bool { return isCode(err, CodeConfigError) }

// --- body helpers --------------------------------------------------------

func asRecord(body any) map[string]any {
	m, _ := body.(map[string]any)
	return m
}

func pickMessage(body any, fallback string) string {
	if m := asRecord(body); m != nil {
		if s, ok := m["message"].(string); ok && s != "" {
			return s
		}
		if s, ok := m["error"].(string); ok && s != "" {
			return s
		}
		if c, ok := m["content"].([]any); ok {
			for _, entry := range c {
				if e, ok := entry.(map[string]any); ok {
					if t, ok := e["text"].(string); ok && t != "" {
						return t
					}
				}
			}
		}
	}
	if s, ok := body.(string); ok && s != "" {
		return s
	}
	return fallback
}

// isMCPShaped reports whether a 429 body carries the data-plane quota markers.
func isMCPShaped(body any) bool {
	m := asRecord(body)
	if m == nil {
		return false
	}
	if b, ok := m["isError"].(bool); ok {
		return b
	}
	if s, ok := m["upgrade_url"].(string); ok {
		return s != ""
	}
	return false
}

func pickUpgradeURL(body any) string {
	if m := asRecord(body); m != nil {
		if s, ok := m["upgrade_url"].(string); ok {
			return s
		}
	}
	return ""
}

func pickFieldErrors(body any) map[string][]string {
	m := asRecord(body)
	if m == nil {
		return map[string][]string{}
	}
	raw, ok := m["errors"].(map[string]any)
	if !ok {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(raw))
	for k, v := range raw {
		list := []string{}
		if sl, ok := v.([]any); ok {
			for _, item := range sl {
				if s, ok := item.(string); ok {
					list = append(list, s)
				}
			}
		}
		out[k] = list
	}
	return out
}
