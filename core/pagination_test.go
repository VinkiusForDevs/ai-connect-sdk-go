package core

import (
	"context"
	"errors"
	"testing"
)

var errSentinel = errors.New("sentinel")

func TestNormalizePaginatedKeepsMetaAndLinks(t *testing.T) {
	body := map[string]any{
		"data":  []any{"a", "b"},
		"meta":  map[string]any{"current_page": 1.0, "last_page": 3.0},
		"links": map[string]any{"next": "https://x/2"},
	}
	out, err := NormalizePaginated(body)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if len(out["data"].([]any)) != 2 {
		t.Fatalf("data lost: %v", out["data"])
	}
	if out["meta"] == nil || out["links"] == nil {
		t.Fatalf("meta/links should be kept: %v", out)
	}
}

func TestNormalizePaginatedDropsAbsentMeta(t *testing.T) {
	out, err := NormalizePaginated(map[string]any{"data": []any{}})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if _, present := out["meta"]; present {
		t.Fatalf("absent meta should not be added: %v", out)
	}
}

func TestNormalizePaginatedRejectsMalformed(t *testing.T) {
	if _, err := NormalizePaginated(map[string]any{"data": map[string]any{}}); !IsProtocolError(err) {
		t.Fatalf("non-array data should be a protocol error: %v", err)
	}
	if _, err := NormalizePaginated("not a map"); !IsProtocolError(err) {
		t.Fatalf("non-map body should be a protocol error: %v", err)
	}
}

func TestUnwrapList(t *testing.T) {
	if l, err := UnwrapList([]any{1.0, 2.0}); err != nil || len(l) != 2 {
		t.Fatalf("bare array: (%v, %v)", l, err)
	}
	if l, err := UnwrapList(map[string]any{"data": []any{"x"}}); err != nil || len(l) != 1 {
		t.Fatalf("envelope array: (%v, %v)", l, err)
	}
	if _, err := UnwrapList(map[string]any{"data": "x"}); !IsProtocolError(err) {
		t.Fatalf("envelope non-array should error: %v", err)
	}
}

func TestUnwrapItem(t *testing.T) {
	if UnwrapItem(map[string]any{"data": map[string]any{"id": "1"}}) == nil {
		t.Fatalf("item should unwrap data")
	}
	// No data key: the body itself is returned (empty DELETE).
	if UnwrapItem(map[string]any{}) == nil {
		t.Fatalf("body without data should fall back to itself")
	}
}

func TestUnwrapListMaps(t *testing.T) {
	in := map[string]any{"data": []any{map[string]any{"id": "1"}, "skip", map[string]any{"id": "2"}}}
	out, err := UnwrapListMaps(in)
	if err != nil {
		t.Fatalf("unwrap: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("non-map elements should be dropped, got %v", out)
	}
}

func TestPageItemsWalksAllPages(t *testing.T) {
	pages := map[int]map[string]any{
		1: pageEnv([]any{"a"}, 1, 3),
		2: pageEnv([]any{"b"}, 2, 3),
		3: pageEnv([]any{"c"}, 3, 3),
	}
	fetched := []int{}
	items, err := PageItems(context.Background(), pages[1], func(_ context.Context, p int) (map[string]any, error) {
		fetched = append(fetched, p)
		return pages[p], nil
	})
	if err != nil {
		t.Fatalf("page walk: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items across pages, got %v", items)
	}
	if fetched[0] != 2 || fetched[1] != 3 {
		t.Fatalf("fetched pages out of order: %v", fetched)
	}
}

func TestPageItemsSinglePageStops(t *testing.T) {
	fetches := 0
	items, err := PageItems(context.Background(), pageEnv([]any{"only"}, 1, 1), func(_ context.Context, p int) (map[string]any, error) {
		fetches++
		return nil, nil
	})
	if err != nil {
		t.Fatalf("single page: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %v", items)
	}
	if fetches != 0 {
		t.Fatalf("single page should not fetch more, fetched %d", fetches)
	}
}

func TestPageItemsPropagatesFetchError(t *testing.T) {
	boom := errSentinel
	_, err := PageItems(context.Background(), pageEnv([]any{"a"}, 1, 3), func(_ context.Context, p int) (map[string]any, error) {
		return nil, boom
	})
	if err != boom {
		t.Fatalf("fetch error should propagate, got %v", err)
	}
}

func pageEnv(data []any, current, last int) map[string]any {
	return map[string]any{
		"data": data,
		"meta": map[string]any{"current_page": current, "last_page": last},
	}
}
