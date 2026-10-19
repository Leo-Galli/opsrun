// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Leo-Galli/opsrun/internal/inventory"
)

// requireNoGoroutineLeak gives background goroutines (keepalive loops, session readers)
// a short grace period to exit, then fails the test if the goroutine count grew.
func requireNoGoroutineLeak(t *testing.T, before int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		runtime.GC()
		after := runtime.NumGoroutine()
		if after <= before {
			return
		}
		if time.Now().After(deadline) {
			require.LessOrEqual(t, after, before, "goroutine count grew and did not settle")
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSSHTransportNoGoroutineLeak(t *testing.T) {
	ts := newTestServer(t)
	tr := testTransport(t, ts, nil)
	host := testHost(t, ts)

	// Baseline is captured after the test server's own permanent Accept loop exists,
	// so it does not get misattributed as a leak caused by the transport under test.
	before := runtime.NumGoroutine()

	for i := 0; i < 20; i++ {
		sess, err := tr.Open(context.Background(), host)
		require.NoError(t, err)
		_, err = sess.Run(context.Background(), "echo:x", RunOptions{})
		require.NoError(t, err)
		require.NoError(t, sess.Close())
	}
	require.NoError(t, tr.Close())

	requireNoGoroutineLeak(t, before)
}

func TestSSHTransportNoGoroutineLeakOnTimeout(t *testing.T) {
	ts := newTestServer(t)
	tr := testTransport(t, ts, nil)
	host := testHost(t, ts)

	before := runtime.NumGoroutine()

	for i := 0; i < 5; i++ {
		sess, err := tr.Open(context.Background(), host)
		require.NoError(t, err)
		_, err = sess.Run(context.Background(), "sleep:2s", RunOptions{CommandTimeout: 50 * time.Millisecond})
		require.Error(t, err)
		require.NoError(t, sess.Close())
	}
	require.NoError(t, tr.Close())

	requireNoGoroutineLeak(t, before)
}

func TestLocalTransportNoGoroutineLeak(t *testing.T) {
	before := runtime.NumGoroutine()

	tr := NewLocalTransport()
	for i := 0; i < 10; i++ {
		sess, err := tr.Open(context.Background(), inventory.Host{Name: "local"})
		require.NoError(t, err)
		_, err = sess.Run(context.Background(), "echo hi", RunOptions{})
		require.NoError(t, err)
		require.NoError(t, sess.Close())
	}

	requireNoGoroutineLeak(t, before)
}
