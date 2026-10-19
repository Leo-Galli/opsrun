// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Leo-Galli/opsrun/internal/inventory"
)

func TestFakeTransportDefaultHandler(t *testing.T) {
	tr := NewFakeTransport(nil)
	sess, err := tr.Open(context.Background(), inventory.Host{Name: "h1"})
	require.NoError(t, err)
	result, err := sess.Run(context.Background(), "anything", RunOptions{})
	require.NoError(t, err)
	require.Equal(t, 0, result.ExitCode)
	require.NoError(t, sess.Close())
	require.NoError(t, tr.Close())
}

func TestFakeTransportScriptedHandler(t *testing.T) {
	tr := NewFakeTransport(func(host inventory.Host, command string, _ RunOptions) (*Result, error) {
		if host.Name == "bad-host" {
			return &Result{ExitCode: 1}, nil
		}
		return &Result{ExitCode: 0, Stdout: command}, nil
	})

	sess, err := tr.Open(context.Background(), inventory.Host{Name: "good-host"})
	require.NoError(t, err)
	result, err := sess.Run(context.Background(), "echo hi", RunOptions{})
	require.NoError(t, err)
	require.Equal(t, "echo hi", result.Stdout)

	sess2, err := tr.Open(context.Background(), inventory.Host{Name: "bad-host"})
	require.NoError(t, err)
	result2, err := sess2.Run(context.Background(), "echo hi", RunOptions{})
	require.NoError(t, err)
	require.Equal(t, 1, result2.ExitCode)

	require.Equal(t, []inventory.Host{{Name: "good-host"}, {Name: "bad-host"}}, tr.Opens())
}

func TestFakeTransportContextCanceled(t *testing.T) {
	tr := NewFakeTransport(nil)
	sess, err := tr.Open(context.Background(), inventory.Host{Name: "h1"})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = sess.Run(ctx, "anything", RunOptions{})
	require.Error(t, err)
	var te *TransportError
	require.ErrorAs(t, err, &te)
	require.Equal(t, ErrorClassCanceled, te.Class)
}
