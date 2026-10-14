package vinkius

import (
	"time"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
)

// sdkContext is the shared runtime context threaded through the fluent layer.
type sdkContext struct {
	// http is the control-plane HTTP client (Bearer + app-id binding).
	http *core.HttpClient
	// appID is the public application id (vk_app_*).
	appID string
	// cache is a shared, process-scoped TTL cache for NON-SECRET, stable
	// resolution data (for example, catalog credential schemas). Per the cache
	// golden rule, secrets and anything vk_live_* must never enter it.
	cache *core.ResolverCache[map[string]any]
	// namespace is how a capability's display name is namespaced across
	// connectors.
	namespace func(connector, name string) string
	// runtime builds a client for the MCP runtime given a connection's mcp_url.
	// It is the ONLY surface for tool listing and execution.
	runtime func(mcpURL string) *core.RuntimeClient
}

// asString returns v as a string, or "" when it is not one.
func asString(v any) string {
	s, _ := v.(string)
	return s
}

// asMap returns v as a map, or nil when it is not one.
func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// boolPtr converts a value to a bool, defaulting to false when it is not one.
func boolPtr(v any, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

// reqOpts returns the per-request controls, tolerating a nil receiver.
func reqOpts(o *core.RequestOptions) core.RequestOptions {
	if o == nil {
		return core.RequestOptions{}
	}
	return *o
}

// reqOptsTimeout returns the timeout from a possibly-nil per-request control.
func reqOptsTimeout(o *core.RequestOptions) time.Duration {
	if o == nil {
		return 0
	}
	return o.Timeout
}
