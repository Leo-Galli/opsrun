// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"crypto/rand"
	"math"
	"math/big"
	"time"
)

// retryConfig bounds exponential backoff with jitter for retryable transport errors.
type retryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

func defaultRetryConfig() retryConfig {
	return retryConfig{MaxAttempts: 4, BaseDelay: 200 * time.Millisecond, MaxDelay: 5 * time.Second}
}

// withRetry calls op until it succeeds, op returns a non-retryable error, ctx is done, or
// the attempt budget is exhausted. It never retries a non-zero command exit: callers must
// only pass operations whose failures are connection-level (dialing, authentication).
func withRetry(ctx context.Context, cfg retryConfig, op func() error) error {
	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := backoffDelay(cfg, attempt)
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		err := op()
		if err == nil {
			return nil
		}
		lastErr = err
		var te *TransportError
		if !isRetryable(err, &te) {
			return err
		}
	}
	return lastErr
}

func isRetryable(err error, out **TransportError) bool {
	te, ok := err.(*TransportError)
	if !ok {
		return false
	}
	*out = te
	return te.Retryable()
}

// backoffDelay returns an exponentially growing delay, capped at MaxDelay, with up to
// +/-25% jitter so many concurrent retries do not all fire at the same instant.
func backoffDelay(cfg retryConfig, attempt int) time.Duration {
	base := float64(cfg.BaseDelay) * math.Pow(2, float64(attempt-1))
	if base > float64(cfg.MaxDelay) {
		base = float64(cfg.MaxDelay)
	}
	jitterRange := base * 0.5
	n, err := rand.Int(rand.Reader, big.NewInt(int64(jitterRange)+1))
	jitter := float64(0)
	if err == nil {
		jitter = float64(n.Int64()) - jitterRange/2
	}
	d := time.Duration(base + jitter)
	if d < 0 {
		d = 0
	}
	return d
}
