package vinkius

// Connector status values (derived state of a connector for a user).
const (
	StatusNotConnected     = "not_connected"
	StatusNeedsCredentials = "needs_credentials"
	StatusReady            = "ready"
	StatusDisabled         = "disabled"
)

// DeriveStatus maps a Connection map to its derived connector status. The
// backend owns readiness (connection["ready"]); the SDK only maps it.
// not_connected is handled by the caller when no connection matches.
func DeriveStatus(connection map[string]any) string {
	if asString(connection["status"]) != "active" {
		return StatusDisabled
	}
	if boolPtr(connection["ready"], false) {
		return StatusReady
	}
	return StatusNeedsCredentials
}

// Summarize produces a connector summary: { slug, status, connection_id? }.
func Summarize(connection map[string]any) map[string]any {
	slug := asString(connection["slug"])
	if slug == "" {
		slug = asString(connection["name"])
	}
	summary := map[string]any{
		"slug":   slug,
		"status": DeriveStatus(connection),
	}
	if id := asString(connection["id"]); id != "" {
		summary["connection_id"] = id
	}
	return summary
}

// summarize is the internal alias used by the fluent layer.
func summarize(connection map[string]any) map[string]any { return Summarize(connection) }
