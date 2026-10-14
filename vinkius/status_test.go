package vinkius_test

import (
	"testing"

	"github.com/VinkiusForDevs/ai-connect-sdk-go/vinkius"
)

func TestDeriveStatus(t *testing.T) {
	cases := []struct {
		name string
		conn map[string]any
		want string
	}{
		{"active and ready", map[string]any{"status": "active", "ready": true}, vinkius.StatusReady},
		{"active, not ready", map[string]any{"status": "active", "ready": false}, vinkius.StatusNeedsCredentials},
		{"inactive", map[string]any{"status": "suspended"}, vinkius.StatusDisabled},
	}
	for _, c := range cases {
		if got := vinkius.DeriveStatus(c.conn); got != c.want {
			t.Fatalf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestSummarize(t *testing.T) {
	s := vinkius.Summarize(map[string]any{"id": "c1", "slug": "github", "status": "active", "ready": true})
	if s["slug"] != "github" {
		t.Fatalf("slug: %v", s)
	}
	if s["status"] != vinkius.StatusReady {
		t.Fatalf("status: %v", s)
	}
	if s["connection_id"] != "c1" {
		t.Fatalf("connection_id: %v", s)
	}

	// Falls back to name when slug is absent, and drops the id when there is none.
	s2 := vinkius.Summarize(map[string]any{"name": "GitHub", "status": "suspended"})
	if s2["slug"] != "GitHub" {
		t.Fatalf("name fallback: %v", s2)
	}
	if _, present := s2["connection_id"]; present {
		t.Fatalf("connection_id should be dropped when absent: %v", s2)
	}
}
