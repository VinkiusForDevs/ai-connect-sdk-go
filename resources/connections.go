package resources

import (
	"context"
	"net/url"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
)

// ConnectionsClient manages a user's connections. A connection is an
// established connector for a user; creating one connects the connector
// (get-or-create user + connection; idempotent per user + connector).
type ConnectionsClient struct {
	http       *core.HttpClient
	appID      string
	externalID string
}

// NewConnectionsClient builds a connections client for a user, validating the
// external id up front.
func NewConnectionsClient(http *core.HttpClient, appID, externalID string) (*ConnectionsClient, error) {
	if err := core.AssertExternalID(externalID); err != nil {
		return nil, err
	}
	return newConnectionsClient(http, appID, externalID), nil
}

// newConnectionsClient is the unchecked internal constructor, used where the
// external id is already known-safe.
func newConnectionsClient(http *core.HttpClient, appID, externalID string) *ConnectionsClient {
	return &ConnectionsClient{http: http, appID: appID, externalID: externalID}
}

func (c *ConnectionsClient) base() string {
	return "/apps/" + url.PathEscape(c.appID) +
		"/users/" + url.PathEscape(c.externalID) +
		"/mcps"
}

// List lists the user's connections (not paginated; empty for an unknown user).
func (c *ConnectionsClient) List(ctx context.Context, opts *core.RequestOptions) ([]map[string]any, error) {
	body, err := c.http.Get(ctx, c.base(), co(opts))
	if err != nil {
		return nil, err
	}
	return core.UnwrapListMaps(body)
}

// Create connects a catalog connector for the user (get-or-create; idempotent).
// The input carries connector (a catalog slug or id).
func (c *ConnectionsClient) Create(ctx context.Context, in map[string]any, opts *core.RequestOptions) (map[string]any, error) {
	copts := co(opts)
	copts.Idempotent = core.PtrBool(true)
	body, err := c.http.Post(ctx, c.base(), map[string]any{"catalog_mcp_id": in["connector"]}, copts)
	if err != nil {
		return nil, err
	}
	return core.UnwrapItemMap(body), nil
}

// Get fetches one connection by id.
func (c *ConnectionsClient) Get(ctx context.Context, connectionID string, opts *core.RequestOptions) (map[string]any, error) {
	body, err := c.http.Get(ctx, c.base()+"/"+url.PathEscape(connectionID), co(opts))
	if err != nil {
		return nil, err
	}
	return core.UnwrapItemMap(body), nil
}

// Delete removes a connection.
func (c *ConnectionsClient) Delete(ctx context.Context, connectionID string, opts *core.RequestOptions) error {
	_, err := c.http.Delete(ctx, c.base()+"/"+url.PathEscape(connectionID), co(opts))
	return err
}

// Credentials returns credential management for this connection (the external id
// is already validated on this client).
func (c *ConnectionsClient) Credentials(connectionID string) *CredentialsClient {
	return newCredentialsClient(c.http, c.appID, c.externalID, connectionID)
}

// Tokens returns data-plane token management for this connection.
func (c *ConnectionsClient) Tokens(connectionID string) *ConnectionTokensClient {
	return newConnectionTokensClient(c.http, c.appID, c.externalID, connectionID)
}
