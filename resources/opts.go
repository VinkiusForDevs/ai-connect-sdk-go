// Package resources is the low-level REST surface of the control plane: the
// application's end users, their connections, connection credentials, data-plane
// tokens, and public catalog discovery. The fluent client in the root package
// builds on these.
package resources

import "github.com/VinkiusForDevs/ai-connect-sdk-go/core"

// ListOptions are the query controls for list/search/iterate methods. It embeds
// the per-request controls and adds the endpoint-specific query fields. Status
// and Page drive user listing; Search takes its query string as a method
// argument and uses Page.
type ListOptions struct {
	core.RequestOptions
	Status string
	Page   int
}

// co builds CallOptions from a per-request control, tolerating a nil receiver.
func co(o *core.RequestOptions) core.CallOptions {
	if o == nil {
		return core.CallOptions{}
	}
	return core.CallOptions{IdempotencyKey: o.IdempotencyKey, Timeout: o.Timeout}
}

// coList builds CallOptions from ListOptions, tolerating a nil receiver.
func coList(o *ListOptions) core.CallOptions {
	if o == nil {
		return core.CallOptions{}
	}
	return co(&o.RequestOptions)
}

// userQuery builds the status/page query map for user listing, skipping empty
// values so they are dropped from the URL.
func userQuery(o *ListOptions) map[string]any {
	q := map[string]any{}
	if o == nil {
		return q
	}
	if o.Status != "" {
		q["status"] = o.Status
	}
	if o.Page != 0 {
		q["page"] = o.Page
	}
	return q
}

// pageQuery builds the page query map, dropping a zero page.
func pageQuery(o *ListOptions) map[string]any {
	q := map[string]any{}
	if o != nil && o.Page != 0 {
		q["page"] = o.Page
	}
	return q
}
