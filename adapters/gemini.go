package adapters

import (
	"context"
	"regexp"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/vinkius"
)

// Gemini function names: a letter or underscore followed by letters, numbers, and
// underscores, max 64 chars. Hyphens are not allowed, so connector slugs like
// google-calendar would be rejected; they are validated up front.
var geminiNameRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
const geminiNameMax = 64

// GeminiFunctionCall is a Gemini function call.
type GeminiFunctionCall struct {
	// Name is the function name.
	Name string
	// Args is the parsed argument map (may be nil for no args).
	Args map[string]any
}

// ValidateGeminiFunctionName guards against a Gemini-incompatible name up front.
func ValidateGeminiFunctionName(name string) error {
	return ValidateToolName(name, "Gemini", geminiNameRE, geminiNameMax)
}

// ToGeminiTools converts Vinkius capabilities into Gemini function declarations.
func ToGeminiTools(set *vinkius.CapabilitySet) ([]map[string]any, error) {
	out := make([]map[string]any, 0, set.Len())
	for _, c := range set.Items() {
		if err := ValidateGeminiFunctionName(c.Name); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"name":        c.Name,
			"description": c.Description,
			"parameters":  NormalizeParams(c.InputSchema),
		})
	}
	return out, nil
}

// RunGeminiFunctionCall executes the Vinkius capability named by a Gemini
// function call.
func RunGeminiFunctionCall(ctx context.Context, set *vinkius.CapabilitySet, call GeminiFunctionCall, opts *vinkius.ExecuteOptions) (*vinkius.CapabilityResult, error) {
	c, err := FindCapability(set, call.Name)
	if err != nil {
		return nil, err
	}
	args := call.Args
	if args == nil {
		args = map[string]any{}
	}
	return c.Execute(ctx, args, opts)
}
