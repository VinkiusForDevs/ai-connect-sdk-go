package core

import (
	"context"
)

// NormalizePaginated coerces a list response into a { data, meta?, links? } map.
// A valid envelope has a "data" array; meta and links are kept only when
// present. A malformed shape is a protocol_error.
func NormalizePaginated(body any) (map[string]any, error) {
	envelope, err := listEnvelope(body)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"data": envelope["data"]}
	if meta, ok := envelope["meta"]; ok && meta != nil {
		out["meta"] = meta
	}
	if links, ok := envelope["links"]; ok && links != nil {
		out["links"] = links
	}
	return out, nil
}

// UnwrapList unwraps a { data: [...] } (or bare array) response into a plain
// list.
func UnwrapList(body any) ([]any, error) {
	if list, ok := body.([]any); ok {
		return list, nil
	}
	envelope, err := listEnvelope(body)
	if err != nil {
		return nil, err
	}
	list, _ := envelope["data"].([]any)
	return list, nil
}

// UnwrapItem unwraps a single-resource { data: {...} } response, falling back to
// the body itself when there is no "data" key (for example an empty DELETE).
func UnwrapItem(body any) any {
	if m, ok := body.(map[string]any); ok {
		if v, present := m["data"]; present && v != nil {
			return v
		}
	}
	return body
}

// UnwrapItemMap is UnwrapItem with the result cast to a map, or nil when the
// body is not a map.
func UnwrapItemMap(body any) map[string]any {
	m, _ := UnwrapItem(body).(map[string]any)
	return m
}

// UnwrapListMaps is UnwrapList with each element cast to a map, dropping
// non-map elements.
func UnwrapListMaps(body any) ([]map[string]any, error) {
	raw, err := UnwrapList(body)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// PageItems walks all pages of a page-based list and returns each item in
// order. It starts from first (already fetched) and advances meta.current_page
// until meta.last_page is reached. fetchPage must fetch the given 1-based page
// number and return a normalized envelope. A single-page response (no meta)
// yields exactly its own items.
func PageItems(ctx context.Context, first map[string]any, fetchPage func(ctx context.Context, page int) (map[string]any, error)) ([]any, error) {
	current := first
	meta := mapFromAny(current["meta"])
	page := intFromAny(meta["current_page"])
	if page == 0 {
		page = 1
	}

	out := []any{}
	for {
		if data, ok := current["data"].([]any); ok {
			out = append(out, data...)
		}
		lastPage := intFromAny(meta["last_page"])
		if lastPage == 0 {
			lastPage = page
		}
		if page >= lastPage {
			return out, nil
		}
		next, err := fetchPage(ctx, page+1)
		if err != nil {
			return nil, err
		}
		current = next
		meta = mapFromAny(current["meta"])
		page++
	}
}

// listEnvelope validates that body is a list envelope with a "data" array.
func listEnvelope(body any) (map[string]any, error) {
	m, ok := body.(map[string]any)
	if !ok {
		return nil, NewProtocolError(
			"Expected a list response shaped as an array or { data: [...] }.", body)
	}
	if _, ok := m["data"].([]any); !ok {
		return nil, NewProtocolError(
			"Expected a list response shaped as an array or { data: [...] }.", body)
	}
	return m, nil
}

func mapFromAny(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func intFromAny(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}
