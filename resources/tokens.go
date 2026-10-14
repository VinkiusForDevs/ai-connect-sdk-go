package resources

import (
	"context"
	"net/url"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
)

// ConnectionTokensClient manages a connection's data-plane tokens.
//
//	POST /apps/{app}/users/{externalId}/mcps/{connection}/tokens -> IssuedConnectionToken
//
// A token is the vk_live_* data-plane credential bound to one connection. The
// API returns its plaintext and its runtime mcp_url only on this create call;
// they are never re-fetchable. Each POST mints a distinct token, so the request
// is not retry-safe and is never auto-retried.
type ConnectionTokensClient struct {
	http         *core.HttpClient
	appID        string
	externalID   string
	connectionID string
}

// NewConnectionTokensClient builds a tokens client, validating the external id
// up front.
func NewConnectionTokensClient(http *core.HttpClient, appID, externalID, connectionID string) (*ConnectionTokensClient, error) {
	if err := core.AssertExternalID(externalID); err != nil {
		return nil, err
	}
	return newConnectionTokensClient(http, appID, externalID, connectionID), nil
}

func newConnectionTokensClient(http *core.HttpClient, appID, externalID, connectionID string) *ConnectionTokensClient {
	return &ConnectionTokensClient{http: http, appID: appID, externalID: externalID, connectionID: connectionID}
}

func (c *ConnectionTokensClient) base() string {
	return "/apps/" + url.PathEscape(c.appID) +
		"/users/" + url.PathEscape(c.externalID) +
		"/mcps/" + url.PathEscape(c.connectionID) +
		"/tokens"
}

// Issue mints a data-plane token; the response carries the one-time plaintext and
// its runtime mcp_url. The input carries an optional name label. The request is
// intentionally non-retryable: each POST mints a distinct token.
func (c *ConnectionTokensClient) Issue(ctx context.Context, in map[string]any, opts *core.RequestOptions) (map[string]any, error) {
	body := map[string]any{}
	if in != nil {
		if name, present := in["name"]; present && name != nil {
			body = map[string]any{"name": name}
		}
	}
	copts := co(opts)
	copts.Idempotent = core.PtrBool(false)
	raw, err := c.http.Post(ctx, c.base(), body, copts)
	if err != nil {
		return nil, err
	}
	return core.UnwrapItemMap(raw), nil
}
