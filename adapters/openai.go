package adapters

import (
	"context"
	"regexp"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/vinkius"
)

// OpenAI tool-function names must be 1-64 chars of [a-zA-Z0-9_-]. The SDK
// namespaces capabilities as connector__name, which can violate this (too long,
// or containing dots/spaces from a connector or capability name).
var openaiNameRE = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
const openaiNameMax = 64

// OpenAIFunction is the function payload of an OpenAI chat-completions tool call.
type OpenAIFunction struct {
	// Name is the tool function name.
	Name string
	// Arguments is the JSON-encoded argument string (may be empty).
	Arguments string
}

// OpenAIToolCall is an OpenAI chat-completions tool call.
type OpenAIToolCall struct {
	Function OpenAIFunction
}

// ValidateOpenAIFunctionName guards against an OpenAI-incompatible name up front.
func ValidateOpenAIFunctionName(name string) error {
	return ValidateToolName(name, "OpenAI", openaiNameRE, openaiNameMax)
}

// ToOpenAITools converts Vinkius capabilities into OpenAI chat-completions tools.
// It returns a config error up front when a namespaced capability cannot be
// exposed as an OpenAI tool function name.
func ToOpenAITools(set *vinkius.CapabilitySet) ([]map[string]any, error) {
	out := make([]map[string]any, 0, set.Len())
	for _, c := range set.Items() {
		if err := ValidateOpenAIFunctionName(c.Name); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        c.Name,
				"description": c.Description,
				"parameters":  NormalizeParams(c.InputSchema),
			},
		})
	}
	return out, nil
}

// RunOpenAIToolCall executes the Vinkius capability named by an OpenAI tool call.
func RunOpenAIToolCall(ctx context.Context, set *vinkius.CapabilitySet, call OpenAIToolCall, opts *vinkius.ExecuteOptions) (*vinkius.CapabilityResult, error) {
	c, err := FindCapability(set, call.Function.Name)
	if err != nil {
		return nil, err
	}
	return c.Execute(ctx, ParseArgs(call.Function.Arguments), opts)
}
