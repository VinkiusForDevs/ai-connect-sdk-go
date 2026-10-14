package core

import (
	"reflect"
	"regexp"
	"strings"
)

// Redacted is the placeholder substituted for any secret value.
const Redacted = "[REDACTED]"

const redactMaxDepth = 6

var secretHeaders = map[string]bool{
	"authorization":   true,
	"idempotency-key": true,
	"cookie":          true,
	"set-cookie":      true,
}

// secretKeys are object keys (case-insensitive) whose values are never surfaced.
var secretKeys = map[string]bool{
	"authorization":  true,
	"apikey":         true,
	"api_key":        true,
	"token":          true,
	"access_token":   true,
	"refresh_token":  true,
	"mcp_url":        true,
	"credentials":    true,
	"password":       true,
	"secret":         true,
	"client_secret":  true,
}

var liveTokenRE = regexp.MustCompile(`vk_live_[A-Za-z0-9_-]+`)

// RedactHeaders returns a copy of headers with secret values redacted.
func RedactHeaders(headers map[string]string) map[string]string {
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		if secretHeaders[strings.ToLower(k)] {
			out[k] = Redacted
		} else {
			out[k] = v
		}
	}
	return out
}

// RedactURL masks any vk_live_* path segment in a URL, so the data-plane bearer
// token embedded in the runtime endpoint never reaches an observability hook.
func RedactURL(url string) string {
	return liveTokenRE.ReplaceAllString(url, Redacted)
}

// RedactBody deep-clones a value, redacting any property whose key looks like a
// secret. The clone is depth-limited and cycle-safe, and the input is never
// mutated.
func RedactBody(value any) any {
	return cloneRedact(value, 0, map[uintptr]bool{})
}

func cloneRedact(value any, depth int, seen map[uintptr]bool) any {
	switch v := value.(type) {
	case []any:
		if depth >= redactMaxDepth {
			return "[TRUNCATED]"
		}
		ptr := reflect.ValueOf(v).Pointer()
		if seen[ptr] {
			return "[CIRCULAR]"
		}
		seen[ptr] = true
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = cloneRedact(item, depth+1, seen)
		}
		delete(seen, ptr)
		return out
	case map[string]any:
		if depth >= redactMaxDepth {
			return "[TRUNCATED]"
		}
		ptr := reflect.ValueOf(v).Pointer()
		if seen[ptr] {
			return "[CIRCULAR]"
		}
		seen[ptr] = true
		out := make(map[string]any, len(v))
		for k, item := range v {
			if secretKeys[strings.ToLower(k)] {
				out[k] = Redacted
			} else {
				out[k] = cloneRedact(item, depth+1, seen)
			}
		}
		delete(seen, ptr)
		return out
	default:
		return value // nil and scalars pass through
	}
}
