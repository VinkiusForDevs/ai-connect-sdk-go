package vinkius

import (
	"context"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
	"github.com/VinkiusForDevs/ai-connect-sdk-go/resources"
)

// CredentialsHandle manages one connection's credentials at the high level. It
// operates on an existing connection (it never connects implicitly). The flat
// Set map is wrapped into the { credentials: values } HTTP envelope.
type CredentialsHandle struct {
	ctx       *sdkContext
	user      *UserContext
	connector *Connector
}

// Schema returns the credential schema this connector requires, from the
// catalog. It is cached (non-secret, stable data: per the cache golden rule, no
// credential values ever pass through here, only field descriptors).
func (h *CredentialsHandle) Schema(ctx context.Context, opts *core.RequestOptions) (map[string]any, error) {
	key := "credential-schema:" + h.connector.Slug()
	return h.ctx.cache.Resolve(ctx, key, func(ctx context.Context) (map[string]any, error) {
		detail, err := resources.NewCatalogClient(h.ctx.http).Get(ctx, h.connector.Slug(), opts)
		if err != nil {
			return nil, err
		}
		schema := asMap(detail["credential_schema"])
		if schema == nil {
			schema = map[string]any{}
		}
		return schema, nil
	})
}

// Status reports which credential keys are configured (never the values).
func (h *CredentialsHandle) Status(ctx context.Context, opts *core.RequestOptions) (map[string]any, error) {
	connectionID, err := h.connector.ResolveConnectionID(ctx, opts)
	if err != nil {
		return nil, err
	}
	client, err := h.client(connectionID)
	if err != nil {
		return nil, err
	}
	return client.Status(ctx, opts)
}

// Set stores credentials (validated server-side against the schema).
func (h *CredentialsHandle) Set(ctx context.Context, values map[string]any, opts *core.RequestOptions) (map[string]any, error) {
	connectionID, err := h.connector.ResolveConnectionID(ctx, opts)
	if err != nil {
		return nil, err
	}
	client, err := h.client(connectionID)
	if err != nil {
		return nil, err
	}
	return client.Set(ctx, map[string]any{"credentials": values}, opts)
}

// client builds the low-level credentials client for a connection id.
func (h *CredentialsHandle) client(connectionID string) (*resources.CredentialsClient, error) {
	clients, err := resources.NewConnectionsClient(h.ctx.http, h.ctx.appID, h.user.externalID)
	if err != nil {
		return nil, err
	}
	return clients.Credentials(connectionID), nil
}
