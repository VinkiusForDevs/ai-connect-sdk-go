package vinkius

import (
	"context"
	"fmt"
	"sync"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
	"github.com/VinkiusForDevs/ai-connect-sdk-go/resources"
)

// Connector is a lazy handle to one connector for a user. Constructing it
// performs no network call. The connection is addressed by the user's external id
// directly; the connection id is resolved on demand and memoized for the handle.
type Connector struct {
	ctx    *sdkContext
	user   *UserContext
	slug   string
	// Credentials manages this connector's credentials (operates on an existing
	// connection; it never connects implicitly).
	Credentials *CredentialsHandle

	mu             sync.Mutex
	connectionID   string
	runtimeClient  *core.RuntimeClient
	provisionedFor string
	provisionedURL string
	provisioning   *provisionState
}

// Slug returns the connector's catalog slug.
func (c *Connector) Slug() string { return c.slug }

// provisionState is the shared handle for an in-flight (or settled) token
// provisioning, so concurrent callers never race into a double mint.
type provisionState struct {
	done chan struct{}
	url  string
	err  error
}

// Connect get-or-creates the connection (idempotent), then provisions the
// connection's vk_live_* data-plane token. The returned connection carries
// runtime_url, the only surface for listing and executing this connection's
// tools; callers should persist it, since the URL and its embedded token are
// returned only here.
func (c *Connector) Connect(ctx context.Context, opts *core.RequestOptions) (map[string]any, error) {
	clients, err := c.connectionsClient()
	if err != nil {
		return nil, err
	}
	connection, err := clients.Create(ctx, map[string]any{"connector": c.slug}, opts)
	if err != nil {
		return nil, err
	}
	id := asString(connection["id"])
	if id == "" {
		return nil, core.NewProtocolError("Connection create did not return an id.", connection)
	}
	c.mu.Lock()
	c.connectionID = id
	c.mu.Unlock()

	mcpURL, err := c.provisionRuntime(ctx, id, opts)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any, len(connection)+1)
	for k, v := range connection {
		out[k] = v
	}
	out["runtime_url"] = mcpURL
	return out, nil
}

// Disconnect removes this connector's connection for the user and clears the
// memoized runtime state.
func (c *Connector) Disconnect(ctx context.Context, opts *core.RequestOptions) error {
	connectionID, err := c.ResolveConnectionID(ctx, opts)
	if err != nil {
		return err
	}
	clients, err := c.connectionsClient()
	if err != nil {
		return err
	}
	if err := clients.Delete(ctx, connectionID, opts); err != nil {
		return err
	}
	c.mu.Lock()
	c.connectionID = ""
	c.runtimeClient = nil
	c.provisionedFor = ""
	c.provisionedURL = ""
	c.provisioning = nil
	c.mu.Unlock()
	return nil
}

// Status returns the current readiness state.
func (c *Connector) Status(ctx context.Context, opts *core.RequestOptions) (string, error) {
	connection, err := c.findConnection(ctx, opts)
	if err != nil {
		return "", err
	}
	if connection == nil {
		return "not_connected", nil
	}
	return DeriveStatus(connection), nil
}

// Capabilities lists the capabilities this connector exposes, listed directly
// from the runtime.
func (c *Connector) Capabilities(ctx context.Context, opts *core.RequestOptions) (*CapabilitySet, error) {
	runtime, connectionID, err := c.resolveRuntime(ctx, opts)
	if err != nil {
		return nil, err
	}
	return c.listCapabilities(ctx, runtime, connectionID, opts)
}

// CapabilitiesForConnection lists capabilities for an already-known connection
// id, skipping the connection lookup entirely (used by the user.Capabilities
// fan-out).
func (c *Connector) CapabilitiesForConnection(ctx context.Context, connectionID string, opts *core.RequestOptions) (*CapabilitySet, error) {
	c.mu.Lock()
	c.connectionID = connectionID
	c.mu.Unlock()
	runtime, _, err := c.resolveRuntime(ctx, opts)
	if err != nil {
		return nil, err
	}
	return c.listCapabilities(ctx, runtime, connectionID, opts)
}

// ResolveConnectionID resolves (and memoizes) this connector's connection id, or
// raises connector_not_connected when the connector is not connected.
func (c *Connector) ResolveConnectionID(ctx context.Context, opts *core.RequestOptions) (string, error) {
	c.mu.Lock()
	if c.connectionID != "" {
		id := c.connectionID
		c.mu.Unlock()
		return id, nil
	}
	c.mu.Unlock()

	connection, err := c.findConnection(ctx, opts)
	if err != nil {
		return "", err
	}
	if connection == nil {
		return "", core.NewConnectorNotConnectedError(
			fmt.Sprintf("Connector %q is not connected for this user. Call Connect first.", c.slug))
	}
	return asString(connection["id"]), nil
}

// connectionsClient returns a connections client for this connector's user.
func (c *Connector) connectionsClient() (*resources.ConnectionsClient, error) {
	return resources.NewConnectionsClient(c.ctx.http, c.ctx.appID, c.user.externalID)
}

func (c *Connector) tokensClient(connectionID string) (*resources.ConnectionTokensClient, error) {
	clients, err := c.connectionsClient()
	if err != nil {
		return nil, err
	}
	return clients.Tokens(connectionID), nil
}

// resolveRuntime returns the runtime client and connection id, provisioning the
// data-plane token when the runtime is not yet memoized.
func (c *Connector) resolveRuntime(ctx context.Context, opts *core.RequestOptions) (*core.RuntimeClient, string, error) {
	connectionID, err := c.ResolveConnectionID(ctx, opts)
	if err != nil {
		return nil, "", err
	}
	c.mu.Lock()
	if c.runtimeClient != nil {
		rc := c.runtimeClient
		c.mu.Unlock()
		return rc, connectionID, nil
	}
	c.mu.Unlock()

	if _, err := c.provisionRuntime(ctx, connectionID, opts); err != nil {
		return nil, "", err
	}
	c.mu.Lock()
	rc := c.runtimeClient
	c.mu.Unlock()
	return rc, connectionID, nil
}

// provisionRuntime mints the connection's data-plane token and memoizes a runtime
// client from its mcp_url. Memo hits are keyed by connection id: a new
// connection (for example after a delete + reconnect) must never inherit the
// previous token's mcp_url. The in-flight provisioning is shared, so parallel
// callers never race into a double mint.
func (c *Connector) provisionRuntime(ctx context.Context, connectionID string, opts *core.RequestOptions) (string, error) {
	c.mu.Lock()
	if c.provisionedFor == connectionID && c.provisionedURL != "" {
		url := c.provisionedURL
		c.mu.Unlock()
		return url, nil
	}
	if c.provisioning != nil {
		ps := c.provisioning
		c.mu.Unlock()
		<-ps.done
		return ps.url, ps.err
	}
	ps := &provisionState{done: make(chan struct{})}
	c.provisioning = ps
	c.mu.Unlock()

	go func() {
		url, err := c.provision(ctx, connectionID, opts)
		c.mu.Lock()
		if err == nil {
			ps.url = url
		} else {
			ps.err = err
		}
		close(ps.done)
		if c.provisioning == ps {
			c.provisioning = nil
		}
		c.mu.Unlock()
	}()

	<-ps.done
	return ps.url, ps.err
}

// provision mints the token and updates the memoized runtime under the lock.
func (c *Connector) provision(ctx context.Context, connectionID string, opts *core.RequestOptions) (string, error) {
	clients, err := c.tokensClient(connectionID)
	if err != nil {
		return "", err
	}
	issued, err := clients.Issue(ctx, nil, opts)
	if err != nil {
		return "", err
	}
	mcpURL := asString(issued["mcp_url"])
	if mcpURL == "" {
		return "", core.NewProtocolError(
			"Token provisioning did not return a runtime URL (mcp_url).",
			map[string]any{"connection_id": connectionID})
	}
	c.mu.Lock()
	c.runtimeClient = c.ctx.runtime(mcpURL)
	c.provisionedFor = connectionID
	c.provisionedURL = mcpURL
	c.mu.Unlock()
	return mcpURL, nil
}

// findConnection lists the user's connections and matches this slug (or a
// connection id equal to the slug), memoizing the connection id.
func (c *Connector) findConnection(ctx context.Context, opts *core.RequestOptions) (map[string]any, error) {
	clients, err := c.connectionsClient()
	if err != nil {
		return nil, err
	}
	connections, err := clients.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	var match map[string]any
	for _, conn := range connections {
		if asString(conn["slug"]) == c.slug || asString(conn["id"]) == c.slug {
			match = conn
			break
		}
	}
	c.mu.Lock()
	if match != nil {
		c.connectionID = asString(match["id"])
	} else {
		c.connectionID = ""
	}
	c.mu.Unlock()
	return match, nil
}

// listCapabilities lists the tools for a connection's runtime and builds the
// executable capabilities.
func (c *Connector) listCapabilities(ctx context.Context, runtime *core.RuntimeClient, connectionID string, opts *core.RequestOptions) (*CapabilitySet, error) {
	raw, err := runtime.ListTools(ctx, reqOpts(opts))
	if err != nil {
		return nil, err
	}
	executor := MakeExecutor(runtime)
	cctx := capabilityContext{
		Connector:    c.slug,
		ConnectionID: connectionID,
		Namespace:    c.ctx.namespace,
		Executor:     executor,
	}
	set := &CapabilitySet{}
	for _, data := range raw {
		cap, err := BuildCapability(data, cctx)
		if err != nil {
			return nil, err
		}
		*set = append(*set, cap)
	}
	return set, nil
}
