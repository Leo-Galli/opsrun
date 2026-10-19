// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Leo-Galli/opsrun/internal/inventory"
)

func TestLocalTransportRunSuccess(t *testing.T) {
	tr := NewLocalTransport()
	sess, err := tr.Open(context.Background(), inventory.Host{Name: "local"})
	require.NoError(t, err)
	defer sess.Close()

	var out bytes.Buffer
	result, err := sess.Run(context.Background(), "echo hello", RunOptions{Stdout: &out})
	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
	require.Contains(t, result.Stdout, "hello")
	require.Contains(t, out.String(), "hello")
}

func TestLocalTransportNonZeroExit(t *testing.T) {
	tr := NewLocalTransport()
	sess, err := tr.Open(context.Background(), inventory.Host{Name: "local"})
	require.NoError(t, err)
	defer sess.Close()

	result, err := sess.Run(context.Background(), "exit 3", RunOptions{})
	require.NoError(t, err)
	require.Equal(t, 3, result.ExitCode)
}

func TestLocalTransportStderr(t *testing.T) {
	tr := NewLocalTransport()
	sess, err := tr.Open(context.Background(), inventory.Host{Name: "local"})
	require.NoError(t, err)
	defer sess.Close()

	result, err := sess.Run(context.Background(), "echo oops 1>&2", RunOptions{})
	require.NoError(t, err)
	require.Contains(t, result.Stderr, "oops")
}

func TestLocalTransportTimeout(t *testing.T) {
	tr := NewLocalTransport()
	sess, err := tr.Open(context.Background(), inventory.Host{Name: "local"})
	require.NoError(t, err)
	defer sess.Close()

	_, err = sess.Run(context.Background(), "sleep 5", RunOptions{CommandTimeout: 100 * time.Millisecond})
	require.Error(t, err)
	var te *TransportError
	require.ErrorAs(t, err, &te)
	require.Equal(t, ErrorClassTimeout, te.Class)
}

func TestLocalTransportClosedSession(t *testing.T) {
	tr := NewLocalTransport()
	sess, err := tr.Open(context.Background(), inventory.Host{Name: "local"})
	require.NoError(t, err)
	require.NoError(t, sess.Close())

	_, err = sess.Run(context.Background(), "echo hi", RunOptions{})
	require.Error(t, err)
}

func TestLocalTransportOutputTruncation(t *testing.T) {
	tr := NewLocalTransport()
	sess, err := tr.Open(context.Background(), inventory.Host{Name: "local"})
	require.NoError(t, err)
	defer sess.Close()

	result, err := sess.Run(context.Background(), "printf 'abcdefghij'", RunOptions{MaxOutputBytes: 4})
	require.NoError(t, err)
	require.Equal(t, "abcd", result.Stdout)
	require.True(t, result.Truncated)
}

func TestLocalTransportClose(t *testing.T) {
	tr := NewLocalTransport()
	require.NoError(t, tr.Close())
}
