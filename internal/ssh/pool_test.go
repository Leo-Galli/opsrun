// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	xssh "golang.org/x/crypto/ssh"

	"github.com/Leo-Galli/opsrun/internal/inventory"
)

func testTransport(t testing.TB, ts *testServer, cfgOverride func(*PoolConfig)) *SSHTransport {
	t.Helper()
	signer := clientKeySigner(t)
	cfg := PoolConfig{
		GlobalMaxConnections: 10,
		PerHostMaxSessions:   2,
		ConnectTimeout:       2 * time.Second,
		HostKeyCallback:      xssh.FixedHostKey(ts.HostKey()),
		AuthForHost: func(_ context.Context, _ inventory.Host) ([]xssh.AuthMethod, error) {
			return []xssh.AuthMethod{xssh.PublicKeys(signer)}, nil
		},
	}
	if cfgOverride != nil {
		cfgOverride(&cfg)
	}
	tr, err := NewSSHTransport(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { tr.Close() }) //nolint:errcheck
	return tr
}

func testHost(t testing.TB, ts *testServer) inventory.Host {
	t.Helper()
	addr, portStr, err := net.SplitHostPort(ts.Addr())
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)
	return inventory.Host{Name: "test-host", Address: addr, Port: port, User: "tester"}
}

func TestSSHTransportRunSuccess(t *testing.T) {
	ts := newTestServer(t)
	tr := testTransport(t, ts, nil)
	host := testHost(t, ts)

	sess, err := tr.Open(context.Background(), host)
	require.NoError(t, err)
	defer sess.Close() //nolint:errcheck

	result, err := sess.Run(context.Background(), "echo:hello", RunOptions{})
	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
	require.Equal(t, "hello\n", result.Stdout)
}

func TestSSHTransportNonZeroExit(t *testing.T) {
	ts := newTestServer(t)
	tr := testTransport(t, ts, nil)
	host := testHost(t, ts)

	sess, err := tr.Open(context.Background(), host)
	require.NoError(t, err)
	defer sess.Close() //nolint:errcheck

	result, err := sess.Run(context.Background(), "exit:7", RunOptions{})
	require.NoError(t, err, "non-zero exit must not be reported as a transport error")
	require.Equal(t, 7, result.ExitCode)
}

func TestSSHTransportStderr(t *testing.T) {
	ts := newTestServer(t)
	tr := testTransport(t, ts, nil)
	host := testHost(t, ts)

	sess, err := tr.Open(context.Background(), host)
	require.NoError(t, err)
	defer sess.Close() //nolint:errcheck

	result, err := sess.Run(context.Background(), "stderr:oops", RunOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, result.ExitCode)
	require.Equal(t, "oops\n", result.Stderr)
}

func TestSSHTransportStreaming(t *testing.T) {
	ts := newTestServer(t)
	tr := testTransport(t, ts, nil)
	host := testHost(t, ts)

	sess, err := tr.Open(context.Background(), host)
	require.NoError(t, err)
	defer sess.Close() //nolint:errcheck

	var out bytes.Buffer
	_, err = sess.Run(context.Background(), "echo:streamed", RunOptions{Stdout: &out})
	require.NoError(t, err)
	require.Contains(t, out.String(), "[test-host] streamed")
}

func TestSSHTransportCommandTimeout(t *testing.T) {
	ts := newTestServer(t)
	tr := testTransport(t, ts, nil)
	host := testHost(t, ts)

	sess, err := tr.Open(context.Background(), host)
	require.NoError(t, err)
	defer sess.Close() //nolint:errcheck

	_, err = sess.Run(context.Background(), "sleep:5s", RunOptions{CommandTimeout: 100 * time.Millisecond})
	require.Error(t, err)
	var te *TransportError
	require.ErrorAs(t, err, &te)
	require.Equal(t, ErrorClassTimeout, te.Class)
	require.True(t, te.Retryable())
}

func TestSSHTransportContextCancel(t *testing.T) {
	ts := newTestServer(t)
	tr := testTransport(t, ts, nil)
	host := testHost(t, ts)

	sess, err := tr.Open(context.Background(), host)
	require.NoError(t, err)
	defer sess.Close() //nolint:errcheck

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	_, err = sess.Run(ctx, "sleep:5s", RunOptions{})
	require.Error(t, err)
	var te *TransportError
	require.ErrorAs(t, err, &te)
	require.Equal(t, ErrorClassCanceled, te.Class)
}

func TestSSHTransportDisconnect(t *testing.T) {
	ts := newTestServer(t)
	tr := testTransport(t, ts, nil)
	host := testHost(t, ts)

	sess, err := tr.Open(context.Background(), host)
	require.NoError(t, err)
	defer sess.Close() //nolint:errcheck

	_, err = sess.Run(context.Background(), "disconnect:", RunOptions{})
	require.Error(t, err)
}

func TestSSHTransportAuthFailure(t *testing.T) {
	ts := newTestServer(t)
	ts.rejectAll = true
	tr := testTransport(t, ts, nil)
	host := testHost(t, ts)

	_, err := tr.Open(context.Background(), host)
	require.Error(t, err)
	var te *TransportError
	require.ErrorAs(t, err, &te)
	require.Equal(t, ErrorClassAuthn, te.Class)
	require.False(t, te.Retryable())
}

func TestSSHTransportHostKeyMismatch(t *testing.T) {
	ts := newTestServer(t)
	otherSigner := clientKeySigner(t)
	tr := testTransport(t, ts, func(cfg *PoolConfig) {
		cfg.HostKeyCallback = xssh.FixedHostKey(otherSigner.PublicKey())
	})
	host := testHost(t, ts)

	_, err := tr.Open(context.Background(), host)
	require.Error(t, err)
	var te *TransportError
	require.ErrorAs(t, err, &te)
	require.False(t, te.Retryable())
}

func TestSSHTransportConnectionPooling(t *testing.T) {
	ts := newTestServer(t)
	tr := testTransport(t, ts, nil)
	host := testHost(t, ts)

	for i := 0; i < 5; i++ {
		sess, err := tr.Open(context.Background(), host)
		require.NoError(t, err)
		_, err = sess.Run(context.Background(), "echo:x", RunOptions{})
		require.NoError(t, err)
		require.NoError(t, sess.Close())
	}

	require.Equal(t, 1, ts.Connections(), "expected a single pooled connection to be reused")
	require.Equal(t, 5, ts.Sessions())
}

func TestSSHTransportPerHostSessionLimit(t *testing.T) {
	ts := newTestServer(t)
	tr := testTransport(t, ts, func(cfg *PoolConfig) { cfg.PerHostMaxSessions = 2 })
	host := testHost(t, ts)

	s1, err := tr.Open(context.Background(), host)
	require.NoError(t, err)
	s2, err := tr.Open(context.Background(), host)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = tr.Open(ctx, host)
	require.Error(t, err, "a third concurrent session should block until the context times out")

	require.NoError(t, s1.Close())
	s3, err := tr.Open(context.Background(), host)
	require.NoError(t, err, "closing a session must free its slot")
	require.NoError(t, s2.Close())
	require.NoError(t, s3.Close())
}

func TestSSHTransportConcurrentHosts(t *testing.T) {
	const numHosts = 8
	servers := make([]*testServer, numHosts)
	hosts := make([]inventory.Host, numHosts)
	for i := range servers {
		servers[i] = newTestServer(t)
	}

	signer := clientKeySigner(t)
	hostKeyByName := make(map[string]xssh.PublicKey, numHosts)
	for i, ts := range servers {
		h := testHost(t, ts)
		h.Name = ts.Addr()
		hosts[i] = h
		hostKeyByName[h.Name] = ts.HostKey()
	}

	cfg := PoolConfig{
		GlobalMaxConnections: numHosts,
		PerHostMaxSessions:   4,
		ConnectTimeout:       2 * time.Second,
		HostKeyCallback: func(hostname string, remote net.Addr, key xssh.PublicKey) error {
			want, ok := hostKeyByName[hostname]
			if !ok || string(want.Marshal()) != string(key.Marshal()) {
				return &TransportError{Class: ErrorClassHostKey, Err: context.DeadlineExceeded}
			}
			return nil
		},
		AuthForHost: func(_ context.Context, _ inventory.Host) ([]xssh.AuthMethod, error) {
			return []xssh.AuthMethod{xssh.PublicKeys(signer)}, nil
		},
	}
	tr, err := NewSSHTransport(cfg)
	require.NoError(t, err)
	defer tr.Close() //nolint:errcheck

	results := make(chan error, numHosts)
	for _, h := range hosts {
		go func(h inventory.Host) {
			sess, err := tr.Open(context.Background(), h)
			if err != nil {
				results <- err
				return
			}
			defer sess.Close() //nolint:errcheck
			res, err := sess.Run(context.Background(), "echo:ok", RunOptions{})
			if err != nil {
				results <- err
				return
			}
			if res.Stdout != "ok\n" {
				results <- context.DeadlineExceeded
				return
			}
			results <- nil
		}(h)
	}
	for i := 0; i < numHosts; i++ {
		require.NoError(t, <-results)
	}
}

// TestSSHTransportConcurrentOpenSameHostDialsOnce guards against a race where many
// callers opening the same host at once each observe "no pooled connection yet" and all
// start their own dial, leaking connections and exhausting the global semaphore.
func TestSSHTransportConcurrentOpenSameHostDialsOnce(t *testing.T) {
	ts := newTestServer(t)
	tr := testTransport(t, ts, func(cfg *PoolConfig) {
		cfg.GlobalMaxConnections = 1
		cfg.PerHostMaxSessions = 100
	})
	host := testHost(t, ts)

	const concurrency = 50
	results := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			sess, err := tr.Open(ctx, host)
			if err != nil {
				results <- err
				return
			}
			defer sess.Close() //nolint:errcheck
			_, err = sess.Run(ctx, "echo:ok", RunOptions{})
			results <- err
		}()
	}
	for i := 0; i < concurrency; i++ {
		require.NoError(t, <-results)
	}

	require.Equal(t, 1, ts.Connections(), "concurrent opens of the same host must share a single dial")
}
