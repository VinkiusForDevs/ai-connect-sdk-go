// Package adapters converts Vinkius capabilities into the tool/function shapes
// of common agent platforms.
//
// The reference SDK ships framework-specific adapters for the Python ecosystem
// (LangChain, LlamaIndex, OpenAI Agents, CrewAI). Those have no natural Go
// equivalents and are intentionally not ported. Instead, the neutral
// JSON-Schema adapter (jsonschema below) is the bridge for any OpenAI-compatible
// provider or a custom agent loop: convert capabilities to plain { name,
// description, parameters } definitions and dispatch by name, then wrap the
// result in whatever tooling the host framework provides.
package adapters

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
	"github.com/VinkiusForDevs/ai-connect-sdk-go/vinkius"
)

// NormalizeParams returns a usable JSON Schema object for function/tool
// parameters, ensuring the common empty case.
func NormalizeParams(schema map[string]any) map[string]any {
	if len(schema) > 0 {
		return schema
	}
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

// ParseArgs parses a JSON arguments string into a map, tolerating malformed
// input. The platform validates tool input, so an unparsable payload executes
// as an empty argument set and surfaces as an IsError tool result the model can
// recover from, rather than failing mid agent-loop.
func ParseArgs(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return map[string]any{}
	}
	m, _ := parsed.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}

// FindCapability returns the capability with the given display or raw name, or a
// not-found error when absent.
func FindCapability(set *vinkius.CapabilitySet, name string) (*vinkius.Capability, error) {
	if c := set.FindCapability(name); c != nil {
		return c, nil
	}
	return nil, core.NewNotFoundError(fmt.Sprintf("Unknown capability: %s", name), 0, nil, "")
}

// ValidateToolName guards against tool names a platform would reject
// (length/charset), raising a clear config error up front instead of a cryptic
// platform 400 at call time.
func ValidateToolName(name, platform string, pattern *regexp.Regexp, maxLength int) error {
	if len(name) > maxLength || !pattern.MatchString(name) {
		return core.NewConfigError(fmt.Sprintf(
			"Capability %q cannot be exposed as a %s tool name: it must be %d char(s) or fewer and match %q (got %d char(s)). "+
				"Override the namespace capability to produce platform-compatible names.",
			name, platform, maxLength, pattern.String(), len(name)))
	}
	return nil
}
