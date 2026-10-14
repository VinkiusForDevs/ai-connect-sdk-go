package resources

import (
	"context"
	"net/url"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
)

// CredentialsClient manages a connection's credentials.
//
//	GET /apps/{app}/users/{externalId}/mcps/{connection}/credentials -> CredentialStatus
//	PUT  (same)  body { credentials: { KEY: value } }               -> CredentialStatus
//
// Values are write-only: the API never returns them, only which keys are set.
// The low-level Set takes the raw HTTP envelope { credentials: {...} }; the
// fluent CredentialsHandle accepts the flat map instead.
type CredentialsClient struct {
	http         *core.HttpClient
	appID        string
	externalID   string
	connectionID string
}

// NewCredentialsClient builds a credentials client, validating the external id
// up front.
func NewCredentialsClient(http *core.HttpClient, appID, externalID, connectionID string) (*CredentialsClient, error) {
	if err := core.AssertExternalID(externalID); err != nil {
		return nil, err
	}
	return newCredentialsClient(http, appID, externalID, connectionID), nil
}

func newCredentialsClient(http *core.HttpClient, appID, externalID, connectionID string) *CredentialsClient {
	return &CredentialsClient{http: http, appID: appID, externalID: externalID, connectionID: connectionID}
}

func (c *CredentialsClient) base() string {
	return "/apps/" + url.PathEscape(c.appID) +
		"/users/" + url.PathEscape(c.externalID) +
		"/mcps/" + url.PathEscape(c.connectionID) +
		"/credentials"
}

// Status reports which credential keys are configured (never the values).
func (c *CredentialsClient) Status(ctx context.Context, opts *core.RequestOptions) (map[string]any, error) {
	body, err := c.http.Get(ctx, c.base(), co(opts))
	if err != nil {
		return nil, err
	}
	m, _ := body.(map[string]any)
	return m, nil
}

// Set stores credentials, validated server-side against the connector schema. The
// input is the raw envelope: { credentials: { KEY: value } }.
func (c *CredentialsClient) Set(ctx context.Context, in map[string]any, opts *core.RequestOptions) (map[string]any, error) {
	body, err := c.http.Put(ctx, c.base(), in, co(opts))
	if err != nil {
		return nil, err
	}
	m, _ := body.(map[string]any)
	return m, nil
}
