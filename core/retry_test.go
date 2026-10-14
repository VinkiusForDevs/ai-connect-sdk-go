package core

import "testing"

func TestIsRetryableStatus(t *testing.T) {
	for _, s := range []int{429, 502, 503, 504} {
		if !IsRetryableStatus(s) {
			t.Errorf("status %d should be retryable", s)
		}
	}
	for _, s := range []int{200, 400, 401, 404, 500} {
		if IsRetryableStatus(s) {
			t.Errorf("status %d should not be retryable", s)
		}
	}
}

func TestBackoffDelayHonorsPositiveRetryAfter(t *testing.T) {
	ms := 300
	if got := BackoffDelay(0, DefaultRetry, &ms); got != 300 {
		t.Fatalf("retry-after 300 -> got %d", got)
	}
	big := 99999
	if got := BackoffDelay(0, DefaultRetry, &big); got != DefaultRetry.MaxDelayMS {
		t.Fatalf("retry-after capped at max delay, got %d", got)
	}
}

func TestBackoffDelayZeroHintFallsBackToJitter(t *testing.T) {
	// A zero hint must NOT sleep zero deterministically for lockstep avoidance:
	// with a positive window it draws full jitter in [0, window].
	window := 250
	for i := 0; i < 50; i++ {
		d := BackoffDelay(0, RetryPolicy{BaseDelayMS: window, MaxDelayMS: window}, nil)
		if d < 0 || d > window {
			t.Fatalf("jitter %d out of [0, %d]", d, window)
		}
	}
}

func TestBackoffDelayZeroWindowIsZero(t *testing.T) {
	p := RetryPolicy{MaxRetries: 2, BaseDelayMS: 0, MaxDelayMS: 0}
	if d := BackoffDelay(0, p, nil); d != 0 {
		t.Fatalf("zero window should give zero delay, got %d", d)
	}
}

func TestParseRetryAfter(t *testing.T) {
	if got := ParseRetryAfter(""); got != nil {
		t.Fatalf("empty -> nil, got %v", got)
	}
	if got := ParseRetryAfter("0"); got == nil || *got != 0 {
		t.Fatalf("'0' -> 0, got %v", got)
	}
	if got := ParseRetryAfter("2"); got == nil || *got != 2000 {
		t.Fatalf("'2' -> 2000, got %v", got)
	}
	if got := ParseRetryAfter("1.5"); got == nil || *got != 1500 {
		t.Fatalf("'1.5' -> 1500, got %v", got)
	}
	if got := ParseRetryAfter("garbage"); got != nil {
		t.Fatalf("garbage -> nil, got %v", got)
	}
}
