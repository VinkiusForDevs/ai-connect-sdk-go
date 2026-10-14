package core

import (
	"strings"
	"testing"
)

func TestAssertExternalIDAcceptsSafeValues(t *testing.T) {
	for _, id := range []string{"alice_123", "user-456", "a.b", "x"} {
		if err := AssertExternalID(id); err != nil {
			t.Fatalf("%q should be accepted: %v", id, err)
		}
	}
}

func TestAssertExternalIDRejectsUnsafeValues(t *testing.T) {
	cases := map[string]string{
		"":                        "empty",
		"bad id":                  "whitespace",
		"a/b":                     "slash",
		"a\\b":                    "backslash",
		strings.Repeat("a", 256):  "too long",
	}
	for id, why := range cases {
		err := AssertExternalID(id)
		if err == nil {
			t.Fatalf("%q (%s) should be rejected", id, why)
		}
		if !IsConfigError(err) {
			t.Fatalf("%q (%s) should be a config error: %v", id, why, err)
		}
	}
}

func TestAssertExternalIDBoundaryLength(t *testing.T) {
	if err := AssertExternalID(strings.Repeat("a", 255)); err != nil {
		t.Fatalf("255 chars should be accepted: %v", err)
	}
}
