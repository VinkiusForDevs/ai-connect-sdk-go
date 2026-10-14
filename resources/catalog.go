package resources

import (
	"context"
	"net/url"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
)

// CatalogClient discovers public connectors.
//
//	GET /catalog/mcps                   -> paginated CatalogConnector
//	GET /catalog/mcps/{slug}           -> CatalogConnectorDetail (+ credential_schema)
//	GET /marketplace/search?q=...&page= -> marketplace-ranked search results
//
// Search uses the public marketplace endpoint so consumers receive the same
// ranking and results as the Vinkius website.
type CatalogClient struct {
	http *core.HttpClient
}

// NewCatalogClient builds a catalog client.
func NewCatalogClient(http *core.HttpClient) *CatalogClient {
	return &CatalogClient{http: http}
}

// List lists public connectors (page-based).
func (c *CatalogClient) List(ctx context.Context, opts *ListOptions) (map[string]any, error) {
	copts := coList(opts)
	copts.Query = pageQuery(opts)
	body, err := c.http.Get(ctx, "/catalog/mcps", copts)
	if err != nil {
		return nil, err
	}
	return core.NormalizePaginated(body)
}

// Iterate walks all catalog connectors across pages, returning each item in
// order. It seeds with the explicitly-requested page (if given), so a caller can
// resume from a known page; otherwise it starts at the first page.
func (c *CatalogClient) Iterate(ctx context.Context, opts *ListOptions) ([]map[string]any, error) {
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

// Get fetches a single connector by slug (or uuid), including its credential
// schema.
func (c *CatalogClient) Get(ctx context.Context, slug string, opts *core.RequestOptions) (map[string]any, error) {
	body, err := c.http.Get(ctx, "/catalog/mcps/"+url.PathEscape(slug), co(opts))
	if err != nil {
		return nil, err
	}
	return core.UnwrapItemMap(body), nil
}

// Search uses the same endpoint and ranking as the Vinkius marketplace.
func (c *CatalogClient) Search(ctx context.Context, query string, opts *ListOptions) ([]map[string]any, error) {
	copts := coList(opts)
	page := 1
	if opts != nil && opts.Page != 0 {
		page = opts.Page
	}
	copts.Query = map[string]any{"q": query, "page": page}
	body, err := c.http.Get(ctx, "/marketplace/search", copts)
	if err != nil {
		return nil, err
	}
	return normalizeMarketplaceSearch(body), nil
}

// normalizeMarketplaceSearch coerces a marketplace search body into a list of
// normalized connectors, falling back listing_type <- server_type and
// requires_buyer_auth <- requires_auth.
func normalizeMarketplaceSearch(body any) []map[string]any {
	m, ok := body.(map[string]any)
	if !ok {
		return []map[string]any{}
	}
	results, ok := m["results"].([]any)
	if !ok {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(results))
	for _, raw := range results {
		conn, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		listingType := ""
		if v, ok := conn["listing_type"]; ok {
			if s, isStr := v.(string); isStr {
				listingType = s
			}
		} else if v, ok := conn["server_type"]; ok {
			if s, isStr := v.(string); isStr {
				listingType = s
			}
		}
		var requiresBuyerAuth any = false
		if v, ok := conn["requires_buyer_auth"]; ok {
			requiresBuyerAuth = v
		} else if v, ok := conn["requires_auth"]; ok {
			requiresBuyerAuth = v
		}
		out = append(out, map[string]any{
			"id":                  conn["id"],
			"slug":                conn["slug"],
			"title":               conn["title"],
			"short_description":   conn["short_description"],
			"publisher_type":      conn["publisher_type"],
			"listing_type":        listingType,
			"requires_buyer_auth": requiresBuyerAuth,
			"server_type":         conn["server_type"],
			"tools_count":         conn["tools_count"],
		})
	}
	return out
}
