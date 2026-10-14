package core

import (
	"context"
	"testing"
	"sync/atomic"
)

type ctxKey string

func TestPooledMapPreservesOrder(t *testing.T) {
	in := []int{1, 2, 3, 4, 5}
	out := PooledMap(context.Background(), in, 2, func(_ context.Context, v int, _ int) int {
		return v * 10
	})
	want := []int{10, 20, 30, 40, 50}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("order/transform mismatch at %d: got %v", i, out)
		}
	}
}

func TestPooledMapPropagatesContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKey("mark"), "v")
	out := PooledMap(ctx, []string{"a"}, 1, func(ctx context.Context, _ string, _ int) string {
		if got := ctx.Value(ctxKey("mark")); got != "v" {
			return "lost"
		}
		return "kept"
	})
	if out[0] != "kept" {
		t.Fatalf("context value should propagate to the item function: %v", out)
	}
}

func TestPooledMapLimitsConcurrency(t *testing.T) {
	const lim = 3
	var current, peak int64
	in := make([]int, 50)
	_ = PooledMap(context.Background(), in, lim, func(_ context.Context, _ int, _ int) int {
		now := atomic.AddInt64(&current, 1)
		if now > peak {
			peak = now
		}
		defer atomic.AddInt64(&current, -1)
		return 0
	})
	if peak > int64(lim) {
		t.Fatalf("concurrency peaked at %d, limit was %d", peak, lim)
	}
}

func TestPooledMapEmptyInput(t *testing.T) {
	out := PooledMap(context.Background(), []int{}, 4, func(_ context.Context, v int, _ int) int { return v })
	if len(out) != 0 {
		t.Fatalf("empty input should give an empty result: %v", out)
	}
}
