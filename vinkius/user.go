package vinkius

import (
	"context"
	"sync"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/core"
	"github.com/VinkiusForDevs/ai-connect-sdk-go/resources"
)

// maxFanoutConcurrency bounds how many connector runtimes may list tools in
// parallel during a capabilities aggregation.
const maxFanoutConcurrency = 8

// UserContext is a lazy handle to one end-user's connectors and capabilities.
// Constructing it performs zero network calls and stores nothing but the client
// external id. All operations address the API by that external id directly; the
// SDK never resolves or holds an internal user id.
type UserContext struct {
	ctx        *sdkContext
	externalID string
}

// ExternalID returns the user's client-supplied external id.
func (u *UserContext) ExternalID() string { return u.externalID }

// Ensure explicitly upserts the user (for example to attach metadata up front).
// It is optional. Returns the AppUser map.
func (u *UserContext) Ensure(ctx context.Context, metadata map[string]any) (map[string]any, error) {
	in := map[string]any{"external_id": u.externalID}
	if len(metadata) > 0 {
		in["metadata"] = metadata
	}
	return u.users().Create(ctx, in, nil)
}

// Get fetches the underlying user resource.
func (u *UserContext) Get(ctx context.Context, opts *core.RequestOptions) (map[string]any, error) {
	return u.users().Get(ctx, u.externalID, opts)
}

// Connector returns a lazy handle to one connector.
func (u *UserContext) Connector(slug string) *Connector {
	conn := &Connector{ctx: u.ctx, user: u, slug: slug}
	conn.Credentials = &CredentialsHandle{ctx: u.ctx, user: u, connector: conn}
	return conn
}

// Connectors lists the user's connected connectors with derived status.
func (u *UserContext) Connectors(ctx context.Context, opts *core.RequestOptions) ([]map[string]any, error) {
	clients, err := u.connectionsClient()
	if err != nil {
		return nil, err
	}
	connections, err := clients.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(connections))
	for _, c := range connections {
		out = append(out, summarize(c))
	}
	return out, nil
}

// Capabilities aggregates the executable capabilities across the user's ready
// connectors. Aggregation is a fan-out over the connected connectors: each
// resolves its own runtime and lists its tools. Only ready connectors are
// included; the rest cannot list tools until they are connected and
// credentialed.
//
// A single flaky runtime does not sink the whole aggregation (an all-failed
// batch still raises). For a single connector, prefer
// user.Connector(slug).Capabilities: it avoids listing all connections.
func (u *UserContext) Capabilities(ctx context.Context, q *CapabilityQuery) (*CapabilitySet, error) {
	if q == nil {
		q = &CapabilityQuery{}
	}
	include := toSet(q.Include)
	exclude := toSet(q.Exclude)

	reqOpts := &core.RequestOptions{IdempotencyKey: q.IdempotencyKey, Timeout: q.Timeout}
	summaries, err := u.Connectors(ctx, reqOpts)
	if err != nil {
		return nil, err
	}

	targets := []map[string]any{}
	for _, s := range summaries {
		slug := asString(s["slug"])
		if asString(s["status"]) != "ready" {
			continue
		}
		if include != nil && !include[slug] {
			continue
		}
		if exclude != nil && exclude[slug] {
			continue
		}
		targets = append(targets, s)
	}

	var firstError error
	var firstErrMu sync.Mutex
	one := func(ctx context.Context, target map[string]any, _ int) *CapabilitySet {
		id := asString(target["connection_id"])
		conn := u.Connector(asString(target["slug"]))
		var set *CapabilitySet
		var err error
		if id != "" {
			set, err = conn.CapabilitiesForConnection(ctx, id, reqOpts)
		} else {
			set, err = conn.Capabilities(ctx, reqOpts)
		}
		if err != nil {
			// Isolated failure: it never sinks the batch.
			firstErrMu.Lock()
			if firstError == nil {
				firstError = err
			}
			firstErrMu.Unlock()
			if q.OnConnectorError != nil {
				q.OnConnectorError(asString(target["slug"]), err)
			}
			return nil
		}
		return set
	}

	settled := core.PooledMap(ctx, targets, maxFanoutConcurrency, one)

	failures := 0
	for _, s := range settled {
		if s == nil {
			failures++
		}
	}
	if failures > 0 && failures == len(targets) {
		return nil, firstError
	}

	all := &CapabilitySet{}
	for _, s := range settled {
		if s != nil {
			*all = append(*all, *s...)
		}
	}
	return all, nil
}

// users returns the application's users client.
func (u *UserContext) users() *resources.AppUsersClient {
	return resources.NewAppUsersClient(u.ctx.http, u.ctx.appID)
}

// connectionsClient returns a connections client for this user.
func (u *UserContext) connectionsClient() (*resources.ConnectionsClient, error) {
	return resources.NewConnectionsClient(u.ctx.http, u.ctx.appID, u.externalID)
}

// toSet builds a lookup set from a slice, or nil when the slice is empty.
func toSet(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]bool, len(items))
	for _, s := range items {
		set[s] = true
	}
	return set
}
