package adapters

import (
	"context"
	"regexp"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/vinkius"
)

// Anthropic tool names must be 1-128 chars of [a-zA-Z0-9_-].
var anthropicNameRE = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
const anthropicNameMax = 128

// AnthropicToolUse is an Anthropic Messages tool_use block.
type AnthropicToolUse struct {
	// Name is the tool name.
	Name string
	// Input is the parsed tool input (may be nil for no input).
	Input map[string]any
}

// ValidateAnthropicToolName guards against an Anthropic-incompatible name up front.
func ValidateAnthropicToolName(name string) error {
	return ValidateToolName(name, "Anthropic", anthropicNameRE, anthropicNameMax)
}

// ToAnthropicTools converts Vinkius capabilities into Anthropic Messages tools.
func ToAnthropicTools(set *vinkius.CapabilitySet) ([]map[string]any, error) {
	out := make([]map[string]any, 0, set.Len())
	for _, c := range set.Items() {
		if err := ValidateAnthropicToolName(c.Name); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"name":         c.Name,
			"description":  c.Description,
			"input_schema": NormalizeParams(c.InputSchema),
		})
	}
	return out, nil
}

// RunAnthropicToolUse executes the Vinkius capability named by an Anthropic
// tool_use block.
func RunAnthropicToolUse(ctx context.Context, set *vinkius.CapabilitySet, use AnthropicToolUse, opts *vinkius.ExecuteOptions) (*vinkius.CapabilityResult, error) {
	c, err := FindCapability(set, use.Name)
	if err != nil {
		return nil, err
	}
	input := use.Input
	if input == nil {
		input = map[string]any{}
	}
	return c.Execute(ctx, input, opts)
}
