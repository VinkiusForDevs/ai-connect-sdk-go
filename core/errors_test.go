package core

import (
	"errors"
	"testing"
)

func TestMapHTTPErrorAuth(t *testing.T) {
	for _, s := range []int{401, 403} {
		err := MapHTTPError(s, map[string]any{"message": "nope"}, "rid", nil)
		if !IsAuthError(err) {
			t.Fatalf("status %d should map to auth_error: %v", s, err)
		}
		if SDKError(err).Message != "nope" {
			t.Fatalf("message not picked from body: %q", SDKError(err).Message)
		}
	}
}

func TestMapHTTPErrorNotFound(t *testing.T) {
	if err := MapHTTPError(404, map[string]any{"message": "missing"}, "rid", nil); !IsNotFoundError(err) {
		t.Fatalf("404 should map to not_found: %v", err)
	}
}

func TestMapHTTPErrorValidation(t *testing.T) {
	body := map[string]any{
		"message": "invalid",
		"errors":  map[string]any{"name": []any{"required"}},
	}
	err := MapHTTPError(422, body, "rid", nil)
	if !IsValidationError(err) {
		t.Fatalf("422 should map to validation_error: %v", err)
	}
	if e := SDKError(err); e.ValidationErrors["name"][0] != "required" {
		t.Fatalf("field errors not surfaced: %v", e.ValidationErrors)
	}
}

func TestMapHTTPErrorOverage(t *testing.T) {
	err := MapHTTPError(402, map[string]any{"upgrade_url": "https://plan"}, "rid", nil)
	if !IsOverageError(err) {
		t.Fatalf("402 should map to overage_blocked: %v", err)
	}
	if e := SDKError(err); e.UpgradeURL != "https://plan" {
		t.Fatalf("upgrade url not surfaced: %q", e.UpgradeURL)
	}
}

func TestMapHTTPErrorRateLimit(t *testing.T) {
	ms := 500
	err := MapHTTPError(429, map[string]any{"message": "slow"}, "rid", &ms)
	if !IsRateLimitError(err) {
		t.Fatalf("plain 429 should map to rate_limit: %v", err)
	}
	if e := SDKError(err); e.RetryAfterMS != 500 {
		t.Fatalf("retry-after not surfaced: %d", e.RetryAfterMS)
	}
}

func TestMapHTTPErrorQuota(t *testing.T) {
	body := map[string]any{"isError": true, "upgrade_url": "https://plan"}
	err := MapHTTPError(429, body, "rid", nil)
	if !IsQuotaError(err) {
		t.Fatalf("mcp-shaped 429 should map to quota_exceeded: %v", err)
	}
	if e := SDKError(err); e.UpgradeURL != "https://plan" {
		t.Fatalf("upgrade url not surfaced: %q", e.UpgradeURL)
	}
}

func TestMapHTTPErrorGenericAPI(t *testing.T) {
	err := MapHTTPError(500, map[string]any{"message": "boom"}, "rid", nil)
	if CodeOf(err) != CodeAPIError {
		t.Fatalf("500 should map to api_error: %v", err)
	}
}

func TestPredicatesMatchMappedErrors(t *testing.T) {
	cases := []struct {
		status int
		body   map[string]any
		is     func(error) bool
	}{
		{401, map[string]any{"message": "x"}, IsAuthError},
		{404, map[string]any{"message": "x"}, IsNotFoundError},
		{422, map[string]any{"message": "x"}, IsValidationError},
		{402, map[string]any{"message": "x"}, IsOverageError},
		{429, map[string]any{"message": "x"}, IsRateLimitError},
		{500, map[string]any{"message": "x"}, func(e error) bool { return CodeOf(e) == CodeAPIError }},
	}
	for _, c := range cases {
		if !c.is(MapHTTPError(c.status, c.body, "rid", nil)) {
			t.Fatalf("status %d predicate failed", c.status)
		}
	}
}

func TestCodeOfAndSDKErrorOnPlainErrors(t *testing.T) {
	plain := errors.New("boom")
	if CodeOf(plain) != "" {
		t.Fatalf("plain error should have empty code: %v", CodeOf(plain))
	}
	if IsVinkiusError(plain) {
		t.Fatalf("plain error is not a vinkius error")
	}
	if !IsVinkiusError(NewConfigError("x")) {
		t.Fatalf("config error is a vinkius error")
	}
}
