package adapters

import (
	"context"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/vinkius"
)

// ToJSONSchemaTools converts Vinkius capabilities into neutral JSON-Schema tool
// definitions: { name, description, parameters }. Use it with any
// OpenAI-compatible provider not listed here, or as the bridge for a custom
// agent loop.
func ToJSONSchemaTools(set *vinkius.CapabilitySet) []map[string]any {
	out := make([]map[string]any, 0, set.Len())
	for _, c := range set.Items() {
		out = append(out, map[string]any{
			"name":        c.Name,
			"description": c.Description,
			"parameters":  NormalizeParams(c.InputSchema),
		})
	}
	return out
}

// ExecuteByName runs a capability by its (namespaced or raw) name, a generic
// dispatch helper for a custom agent loop.
func ExecuteByName(ctx context.Context, set *vinkius.CapabilitySet, name string, args map[string]any, opts *vinkius.ExecuteOptions) (*vinkius.CapabilityResult, error) {
	c, err := FindCapability(set, name)
	if err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}
	return c.Execute(ctx, args, opts)
}
