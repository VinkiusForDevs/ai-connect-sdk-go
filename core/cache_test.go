package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCacheSetGet(t *testing.T) {
	c := NewResolverCache[string](time.Minute)
	if _, ok := c.Get("k"); ok {
		t.Fatalf("miss should report absent")
	}
	c.Set("k", "v")
	if v, ok := c.Get("k"); !ok || v != "v" {
		t.Fatalf("got (%q,%v)", v, ok)
	}
	c.Delete("k")
	if _, ok := c.Get("k"); ok {
		t.Fatalf("deleted key should be absent")
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	c := NewResolverCache[int](50 * time.Millisecond)
	c.Set("k", 1)
	if _, ok := c.Get("k"); !ok {
		t.Fatalf("value should be fresh immediately")
	}
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatalf("value should be expired after the TTL")
	}
}

func TestCacheDefaultTTLOnNonPositive(t *testing.T) {
	c := NewResolverCache[struct{}](0)
	if c.ttl != DefaultCacheTTL {
		t.Fatalf("non-positive ttl should fall back to the default, got %v", c.ttl)
	}
}

func TestCacheResolveComputesOnce(t *testing.T) {
	c := NewResolverCache[int](time.Minute)
	calls := 0
	compute := func(context.Context) (int, error) { calls++; return 42, nil }
	v1, err := c.Resolve(context.Background(), "k", compute)
	if err != nil || v1 != 42 {
		t.Fatalf("resolve miss: (%d, %v)", v1, err)
	}
	v2, err := c.Resolve(context.Background(), "k", compute)
	if err != nil || v2 != 42 {
		t.Fatalf("resolve hit: (%d, %v)", v2, err)
	}
	if calls != 1 {
		t.Fatalf("compute should run once, ran %d", calls)
	}
}

func TestCacheResolvePropagatesErrorWithoutCaching(t *testing.T) {
	c := NewResolverCache[string](time.Minute)
	boom := errors.New("boom")
	compute := func(context.Context) (string, error) { return "", boom }
	if _, err := c.Resolve(context.Background(), "k", compute); !errors.Is(err, boom) {
		t.Fatalf("error should propagate: %v", err)
	}
	// A failed resolve must not poison the cache with a zero value.
	if v, ok := c.Get("k"); ok {
		t.Fatalf("failed resolve should not cache, got %q", v)
	}
}
