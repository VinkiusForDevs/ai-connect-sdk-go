package vinkius

import (
	"context"
	"strings"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
)

// CapabilityExecutor executes a capability at the runtime. The runtime target (a
// connection's mcp_url, which embeds its vk_live_* token) is bound into the
// executor when the capability is built, so the signature carries no routing
// coordinates.
type CapabilityExecutor func(ctx context.Context, rawName string, args map[string]any, opts *core.ExecuteOptions) (*core.CapabilityResult, error)

// Capability is an executable, framework-neutral unit of a user's connectors.
type Capability struct {
	executor CapabilityExecutor

	// Connector is the slug of the connector this capability belongs to.
	Connector string
	// ConnectionID is the connection id used to route execution.
	ConnectionID string
	// Name is the display name, namespaced across connectors (for example
	// github__create_issue).
	Name string
	// RawName is the raw name as the connector exposes it (for example
	// create_issue).
	RawName string
	// Title is an optional display title.
	Title string
	// Description is the capability's description.
	Description string
	// InputSchema is the JSON schema of the capability's input.
	InputSchema map[string]any
}

// Execute runs this capability at the runtime. IsError results are returned,
// not thrown, so an agent loop can feed the failure back to the model and
// recover.
func (c *Capability) Execute(ctx context.Context, args map[string]any, opts *core.ExecuteOptions) (*core.CapabilityResult, error) {
	return c.executor(ctx, c.RawName, args, opts)
}

// CapabilitySet is an ordered collection of capabilities with lookup helpers.
type CapabilitySet []*Capability

// NewCapabilitySet builds a CapabilitySet from the given capabilities.
func NewCapabilitySet(items ...*Capability) *CapabilitySet {
	s := make(CapabilitySet, len(items))
	copy(s, items)
	return &s
}

// ForConnector returns the capabilities belonging to a specific connector slug.
func (s *CapabilitySet) ForConnector(slug string) *CapabilitySet {
	out := CapabilitySet{}
	for _, c := range *s {
		if c != nil && c.Connector == slug {
			out = append(out, c)
		}
	}
	return &out
}

// FindCapability returns the capability with the given display name or raw name,
// or nil when absent.
func (s *CapabilitySet) FindCapability(name string) *Capability {
	for _, c := range *s {
		if c != nil && (c.Name == name || c.RawName == name) {
			return c
		}
	}
	return nil
}

// Len returns the number of capabilities in the set.
func (s *CapabilitySet) Len() int { return len(*s) }

// Items returns the underlying slice of capabilities.
func (s *CapabilitySet) Items() []*Capability { return *s }

// capabilityContext carries the routing metadata for building a capability.
type capabilityContext struct {
	Connector    string
	ConnectionID string
	Namespace    func(connector, name string) string
	Executor     CapabilityExecutor
}

// BuildCapability builds a Capability from a normalized runtime tool
// definition (snake_case) plus its routing context.
func BuildCapability(data map[string]any, c capabilityContext) (*Capability, error) {
	rawName := asString(data["name"])
	if strings.TrimSpace(rawName) == "" {
		return nil, core.NewProtocolError("Capability payload is missing a non-empty name.", data)
	}
	if c.Connector == "" || c.ConnectionID == "" {
		return nil, core.NewProtocolError("Capability payload is missing connector routing metadata.",
			map[string]any{"connector": c.Connector, "connection_id": c.ConnectionID})
	}
	schema := asMap(data["input_schema"])
	if schema == nil {
		schema = map[string]any{}
	}
	return &Capability{
		executor:     c.Executor,
		Connector:    c.Connector,
		ConnectionID: c.ConnectionID,
		Name:         c.Namespace(c.Connector, rawName),
		RawName:      rawName,
		Title:        asString(data["title"]),
		Description:  asString(data["description"]),
		InputSchema:  schema,
	}, nil
}

// MakeExecutor returns a CapabilityExecutor bound to one connection's runtime.
// Execution runs against the runtime client; the connection's vk_live_* token is
// embedded in its mcp_url, so no API auth headers are added here.
func MakeExecutor(runtime *core.RuntimeClient) CapabilityExecutor {
	return func(ctx context.Context, rawName string, args map[string]any, opts *core.ExecuteOptions) (*core.CapabilityResult, error) {
		ro := core.RequestOptions{}
		if opts != nil {
			ro.IdempotencyKey = opts.IdempotencyKey
			ro.Timeout = opts.Timeout
		}
		return runtime.CallTool(ctx, rawName, args, ro)
	}
}
