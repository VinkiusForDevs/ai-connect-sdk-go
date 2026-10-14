package resources

import (
	"context"
	"net/url"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
)

// AppUsersClient manages the application's end users.
//
//	GET/POST         /apps/{app}/users
//	GET/PATCH/DELETE /apps/{app}/users/{externalId}
//
// Users are addressed by their client-supplied external_id. create is
// idempotent: the API upserts by (application_id, external_id).
//
// Tool listing and execution run through the runtime per connection, not here.
type AppUsersClient struct {
	http  *core.HttpClient
	appID string
}

// NewAppUsersClient builds a users client scoped to one application.
func NewAppUsersClient(http *core.HttpClient, appID string) *AppUsersClient {
	return &AppUsersClient{http: http, appID: appID}
}

func (c *AppUsersClient) base() string {
	return "/apps/" + url.PathEscape(c.appID) + "/users"
}

func (c *AppUsersClient) userPath(externalID string) (string, error) {
	if err := core.AssertExternalID(externalID); err != nil {
		return "", err
	}
	return c.base() + "/" + url.PathEscape(externalID), nil
}

// Create get-or-creates a user by external id (idempotent upsert). The input map
// carries external_id (required), and optional status and metadata.
func (c *AppUsersClient) Create(ctx context.Context, in map[string]any, opts *core.RequestOptions) (map[string]any, error) {
	extID, _ := in["external_id"].(string)
	if err := core.AssertExternalID(extID); err != nil {
		return nil, err
	}
	copts := co(opts)
	copts.Idempotent = core.PtrBool(true)
	body, err := c.http.Post(ctx, c.base(), in, copts)
	if err != nil {
		return nil, err
	}
	return core.UnwrapItemMap(body), nil
}

// Get fetches the underlying user resource.
func (c *AppUsersClient) Get(ctx context.Context, externalID string, opts *core.RequestOptions) (map[string]any, error) {
	path, err := c.userPath(externalID)
	if err != nil {
		return nil, err
	}
	body, err := c.http.Get(ctx, path, co(opts))
	if err != nil {
		return nil, err
	}
	return core.UnwrapItemMap(body), nil
}

// Update patches a user (status, metadata).
func (c *AppUsersClient) Update(ctx context.Context, externalID string, patch map[string]any, opts *core.RequestOptions) (map[string]any, error) {
	path, err := c.userPath(externalID)
	if err != nil {
		return nil, err
	}
	body, err := c.http.Patch(ctx, path, patch, co(opts))
	if err != nil {
		return nil, err
	}
	return core.UnwrapItemMap(body), nil
}

// Delete removes a user.
func (c *AppUsersClient) Delete(ctx context.Context, externalID string, opts *core.RequestOptions) error {
	path, err := c.userPath(externalID)
	if err != nil {
		return err
	}
	_, err = c.http.Delete(ctx, path, co(opts))
	return err
}

// List lists users (page-based; supports status and page).
func (c *AppUsersClient) List(ctx context.Context, opts *ListOptions) (map[string]any, error) {
	copts := coList(opts)
	copts.Query = userQuery(opts)
	body, err := c.http.Get(ctx, c.base(), copts)
	if err != nil {
		return nil, err
	}
	return core.NormalizePaginated(body)
}

// Iterate walks all users across all pages, returning each item in order.
func (c *AppUsersClient) Iterate(ctx context.Context, opts *ListOptions) ([]map[string]any, error) {
	first, err := c.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	items, err := core.PageItems(ctx, first, func(ctx context.Context, page int) (map[string]any, error) {
		clone := *opts
		clone.Page = page
		return c.List(ctx, &clone)
	})
	if err != nil {
		return nil, err
	}
	return core.UnwrapListMaps(items)
}

// Connections returns the scoped connections client for a specific user. The
// external id is validated up front.
func (c *AppUsersClient) Connections(externalID string) (*ConnectionsClient, error) {
	return NewConnectionsClient(c.http, c.appID, externalID)
}
