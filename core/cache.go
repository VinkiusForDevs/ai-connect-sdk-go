package core

import (
	"context"
	"sync"
	"time"
)

// DefaultCacheTTL is the default time-to-live for cached, non-secret resolution
// data.
const DefaultCacheTTL = 5 * time.Minute

// ResolverCache is an in-memory TTL cache for stable, non-secret resolution data
// (for example, catalog credential schemas).
//
// Golden rule: the cache is an optimization, never a source of authorization.
// Only non-secret, stable values belong here. Credentials, Authorization
// headers, and any vk_live_* value must never be cached, and never even reach
// the SDK.
type ResolverCache[T any] struct {
	mu       sync.Mutex
	ttl      time.Duration
	store    map[string]cacheEntry[T]
	createdAt time.Time
}

type cacheEntry[T any] struct {
	value     T
	expiresAt time.Time
}

// NewResolverCache builds a cache with the given TTL; a non-positive ttl uses the
// 5-minute default.
func NewResolverCache[T any](ttl time.Duration) *ResolverCache[T] {
	if ttl <= 0 {
		ttl = DefaultCacheTTL
	}
	return &ResolverCache[T]{ttl: ttl, store: map[string]cacheEntry[T]{}, createdAt: time.Now()}
}

// Get returns the cached value for key and whether it was present and fresh.
func (c *ResolverCache[T]) Get(key string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.store[key]
	if !ok {
		var zero T
		return zero, false
	}
	if !time.Now().Before(entry.expiresAt) {
		delete(c.store, key)
		var zero T
		return zero, false
	}
	return entry.value, true
}

// Set stores value for key, expiring after the cache TTL.
func (c *ResolverCache[T]) Set(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = cacheEntry[T]{value: value, expiresAt: time.Now().Add(c.ttl)}
}

// Delete removes key from the cache.
func (c *ResolverCache[T]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
}

// Clear empties the cache.
func (c *ResolverCache[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = map[string]cacheEntry[T]{}
}

// Resolve returns the cached value for key, or computes, caches, and returns it
// on a miss. compute runs only on a cache miss.
func (c *ResolverCache[T]) Resolve(ctx context.Context, key string, compute func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	if cached, ok := c.Get(key); ok {
		return cached, nil
	}
	value, err := compute(ctx)
	if err != nil {
		return zero, err
	}
	c.Set(key, value)
	return value, nil
}
