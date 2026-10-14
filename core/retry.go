package core

import (
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// RetryPolicy governs automatic retries of idempotent requests.
type RetryPolicy struct {
	MaxRetries  int
	BaseDelayMS int
	MaxDelayMS  int
}

// DefaultRetry is the default policy: 2 retries, 250 ms base, 4000 ms cap.
var DefaultRetry = RetryPolicy{MaxRetries: 2, BaseDelayMS: 250, MaxDelayMS: 4000}

// IsRetryableStatus reports whether an HTTP status is worth retrying.
func IsRetryableStatus(status int) bool {
	switch status {
	case 429, 502, 503, 504:
		return true
	}
	return false
}

// BackoffDelay computes the delay (ms) before the next attempt (0-indexed).
//
// A positive retryAfterMS (server hint) wins, capped at p.MaxDelayMS. A hint of
// 0 is treated as no hint: honoring it exactly would make simultaneous clients
// hammer the server in lockstep, so full jitter is used instead. Without a hint,
// the delay is full jitter over an exponentially growing window.
func BackoffDelay(attempt int, p RetryPolicy, retryAfterMS *int) int {
	if retryAfterMS != nil && *retryAfterMS > 0 {
		d := *retryAfterMS
		if d > p.MaxDelayMS {
			d = p.MaxDelayMS
		}
		return d
	}
	window := p.BaseDelayMS << uint(attempt)
	if window > p.MaxDelayMS || attempt > 20 {
		window = p.MaxDelayMS
	}
	if window <= 0 {
		return 0
	}
	return rand.Intn(window + 1) // uniform over [0, window]
}

// ParseRetryAfter parses a Retry-After header value (delta-seconds or an
// HTTP-date) into milliseconds. It returns nil when no usable hint is present.
func ParseRetryAfter(value string) *int {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if secs, err := strconv.ParseFloat(value, 64); err == nil {
		ms := int(secs * 1000)
		if ms < 0 {
			ms = 0
		}
		return &ms
	}
	if t, err := time.Parse(time.RFC1123, value); err == nil {
		ms := int(time.Until(t).Round(time.Millisecond).Milliseconds())
		if ms < 0 {
			ms = 0
		}
		return &ms
	}
	return nil
}
