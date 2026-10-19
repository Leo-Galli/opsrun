// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWithRetrySucceedsFirstTry(t *testing.T) {
	calls := 0
	err := withRetry(context.Background(), defaultRetryConfig(), func() error {
		calls++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

func TestWithRetryRetriesRetryableError(t *testing.T) {
	cfg := retryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}
	calls := 0
	err := withRetry(context.Background(), cfg, func() error {
		calls++
		if calls < 3 {
			return &TransportError{Class: ErrorClassConnection, Err: errors.New("boom")}
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 3, calls)
}

func TestWithRetryDoesNotRetryNonRetryable(t *testing.T) {
	cfg := retryConfig{MaxAttempts: 5, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}
	calls := 0
	err := withRetry(context.Background(), cfg, func() error {
		calls++
		return &TransportError{Class: ErrorClassAuthn, Err: errors.New("denied")}
	})
	require.Error(t, err)
	require.Equal(t, 1, calls)
}

func TestWithRetryDoesNotRetryPlainError(t *testing.T) {
	calls := 0
	err := withRetry(context.Background(), defaultRetryConfig(), func() error {
		calls++
		return errors.New("not a transport error")
	})
	require.Error(t, err)
	require.Equal(t, 1, calls)
}

func TestWithRetryExhaustsAttempts(t *testing.T) {
	cfg := retryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond}
	calls := 0
	err := withRetry(context.Background(), cfg, func() error {
		calls++
		return &TransportError{Class: ErrorClassTimeout, Err: errors.New("still timing out")}
	})
	require.Error(t, err)
	require.Equal(t, 3, calls)
}

func TestWithRetryContextCanceled(t *testing.T) {
	cfg := retryConfig{MaxAttempts: 5, BaseDelay: time.Second, MaxDelay: time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	err := withRetry(ctx, cfg, func() error {
		calls++
		return &TransportError{Class: ErrorClassConnection, Err: errors.New("boom")}
	})
	require.Error(t, err)
	require.Equal(t, 1, calls)
}

func TestBackoffDelayCapsAtMaxDelay(t *testing.T) {
	cfg := retryConfig{BaseDelay: time.Second, MaxDelay: 2 * time.Second}
	for attempt := 1; attempt < 10; attempt++ {
		d := backoffDelay(cfg, attempt)
		require.LessOrEqual(t, d, cfg.MaxDelay+cfg.MaxDelay/2)
		require.GreaterOrEqual(t, d, time.Duration(0))
	}
}
