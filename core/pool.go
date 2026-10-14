package core

import (
	"context"
	"sync"
)

// PooledMap maps over items with at most limit in-flight goroutines, preserving
// the input order in the result. A limit below 1 is treated as 1. Per-item
// failures are not swallowed here; the caller handles them (typically by
// representing a failed item as a nil/zero result).
func PooledMap[T any, R any](ctx context.Context, items []T, limit int, fn func(ctx context.Context, item T, index int) R) []R {
	results := make([]R, len(items))
	if limit < 1 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i, item := range items {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, item T) {
			defer wg.Done()
			results[i] = fn(ctx, item, i)
			<-sem
		}(i, item)
	}
	wg.Wait()
	return results
}
