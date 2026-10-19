// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTransportErrorErrorAndUnwrap(t *testing.T) {
	inner := errors.New("boom")
	e := &TransportError{Class: ErrorClassConnection, Host: "web-1", Err: inner}
	require.Equal(t, "connection: web-1: boom", e.Error())
	require.Equal(t, inner, errors.Unwrap(e))

	e2 := &TransportError{Class: ErrorClassConnection, Err: inner}
	require.Equal(t, "connection: boom", e2.Error())
}

func TestPooledClientIsStaleDisabled(t *testing.T) {
	pc := &pooledClient{lastUsed: time.Now().Add(-time.Hour)}
	require.False(t, pc.isStale(0))
}

func TestSSHTransportKeepaliveAndIdleExpiry(t *testing.T) {
	ts := newTestServer(t)
	tr := testTransport(t, ts, func(cfg *PoolConfig) {
		cfg.KeepAliveInterval = 20 * time.Millisecond
		cfg.IdleTimeout = 50 * time.Millisecond
	})
	host := testHost(t, ts)

	sess, err := tr.Open(context.Background(), host)
	require.NoError(t, err)
	_, err = sess.Run(context.Background(), "echo:x", RunOptions{})
	require.NoError(t, err)
	require.NoError(t, sess.Close())

	time.Sleep(150 * time.Millisecond)

	sess2, err := tr.Open(context.Background(), host)
	require.NoError(t, err)
	defer sess2.Close()
	_, err = sess2.Run(context.Background(), "echo:y", RunOptions{})
	require.NoError(t, err)

	require.GreaterOrEqual(t, ts.Connections(), 2, "idle connection should have been replaced")
}

func TestSSHTransportDropsBrokenPooledClient(t *testing.T) {
	ts := newTestServer(t)
	tr := testTransport(t, ts, nil)
	host := testHost(t, ts)

	sess, err := tr.Open(context.Background(), host)
	require.NoError(t, err)
	require.NoError(t, sess.Close())

	tr.mu.Lock()
	pc := tr.clients[host.Name]
	tr.mu.Unlock()
	require.NotNil(t, pc)
	tr.dropClient(host.Name, pc)

	tr.mu.Lock()
	_, ok := tr.clients[host.Name]
	tr.mu.Unlock()
	require.False(t, ok)
}

func TestLoadSSHConfigExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	require.NoError(t, os.WriteFile(path, []byte("Host foo\n\tUser bar\n"), 0o600))

	cfg, err := LoadSSHConfig(path)
	require.NoError(t, err)
	require.Equal(t, "bar", cfg.Lookup("foo").User)
}

func TestSplitDirectiveEqualsForm(t *testing.T) {
	key, value, ok := splitDirective("User=someone")
	require.True(t, ok)
	require.Equal(t, "User", key)
	require.Equal(t, "someone", value)

	_, _, ok = splitDirective("garbage")
	require.False(t, ok)
}
