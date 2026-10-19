// SPDX-License-Identifier: Apache-2.0

package rollback

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Leo-Galli/opsrun/internal/inventory"
	"github.com/Leo-Galli/opsrun/internal/runbook"
	"github.com/Leo-Galli/opsrun/internal/ssh"
)

func TestDefaultManagerRollbackSuccess(t *testing.T) {
	var gotCommands []string
	tr := ssh.NewFakeTransport(func(host inventory.Host, command string, _ ssh.RunOptions) (*ssh.Result, error) {
		gotCommands = append(gotCommands, command)
		return &ssh.Result{ExitCode: 0}, nil
	})
	m := &DefaultManager{Transport: tr}

	completed := []CompletedStep{
		{Step: &runbook.Step{ID: "a", Rollback: &runbook.Action{Command: "undo-a"}}, Host: inventory.Host{Name: "h1"}, ResolvedRollbackCommand: "undo-a"},
		{Step: &runbook.Step{ID: "b", Rollback: &runbook.Action{Command: "undo-b"}}, Host: inventory.Host{Name: "h1"}, ResolvedRollbackCommand: "undo-b"},
	}
	outcomes := m.Rollback(context.Background(), completed)
	require.Len(t, outcomes, 2)
	for _, o := range outcomes {
		require.NoError(t, o.Err)
	}
	require.Equal(t, []string{"undo-a", "undo-b"}, gotCommands)
}

func TestDefaultManagerRollbackNoRollbackAction(t *testing.T) {
	tr := ssh.NewFakeTransport(nil)
	m := &DefaultManager{Transport: tr}

	completed := []CompletedStep{
		{Step: &runbook.Step{ID: "a"}, Host: inventory.Host{Name: "h1"}},
	}
	outcomes := m.Rollback(context.Background(), completed)
	require.Len(t, outcomes, 1)
	require.NoError(t, outcomes[0].Err)
	require.Empty(t, tr.Opens())
}

func TestDefaultManagerRollbackCommandFails(t *testing.T) {
	tr := ssh.NewFakeTransport(func(host inventory.Host, command string, _ ssh.RunOptions) (*ssh.Result, error) {
		return &ssh.Result{ExitCode: 1, Stderr: "boom"}, nil
	})
	m := &DefaultManager{Transport: tr}

	completed := []CompletedStep{
		{Step: &runbook.Step{ID: "a", Rollback: &runbook.Action{Command: "undo-a"}}, Host: inventory.Host{Name: "h1"}, ResolvedRollbackCommand: "undo-a"},
	}
	outcomes := m.Rollback(context.Background(), completed)
	require.Len(t, outcomes, 1)
	require.Error(t, outcomes[0].Err)
}

func TestDefaultManagerRollbackContinuesAfterFailure(t *testing.T) {
	tr := ssh.NewFakeTransport(func(host inventory.Host, command string, _ ssh.RunOptions) (*ssh.Result, error) {
		if command == "undo-a" {
			return &ssh.Result{ExitCode: 1}, nil
		}
		return &ssh.Result{ExitCode: 0}, nil
	})
	m := &DefaultManager{Transport: tr}

	completed := []CompletedStep{
		{Step: &runbook.Step{ID: "a", Rollback: &runbook.Action{Command: "undo-a"}}, Host: inventory.Host{Name: "h1"}, ResolvedRollbackCommand: "undo-a"},
		{Step: &runbook.Step{ID: "b", Rollback: &runbook.Action{Command: "undo-b"}}, Host: inventory.Host{Name: "h1"}, ResolvedRollbackCommand: "undo-b"},
	}
	outcomes := m.Rollback(context.Background(), completed)
	require.Len(t, outcomes, 2)
	require.Error(t, outcomes[0].Err)
	require.NoError(t, outcomes[1].Err, "a failing rollback must not stop the rest from running")
}

func TestDefaultManagerRollbackOpenSessionFails(t *testing.T) {
	m := &DefaultManager{Transport: failingOpenTransport{}}
	completed := []CompletedStep{
		{Step: &runbook.Step{ID: "a", Rollback: &runbook.Action{Command: "undo-a"}}, Host: inventory.Host{Name: "h1"}, ResolvedRollbackCommand: "undo-a"},
	}
	outcomes := m.Rollback(context.Background(), completed)
	require.Len(t, outcomes, 1)
	require.Error(t, outcomes[0].Err)
}

func TestDefaultManagerRollbackTimeoutIsEnforced(t *testing.T) {
	m := &DefaultManager{Transport: blockingTransport{}, Timeout: 20 * time.Millisecond}
	completed := []CompletedStep{
		{Step: &runbook.Step{ID: "a", Rollback: &runbook.Action{Command: "undo-a"}}, Host: inventory.Host{Name: "h1"}, ResolvedRollbackCommand: "undo-a"},
	}
	outcomes := m.Rollback(context.Background(), completed)
	require.Len(t, outcomes, 1)
	require.Error(t, outcomes[0].Err)
}

// blockingTransport's Session.Run blocks until ctx is done, to exercise
// DefaultManager.Timeout actually cutting a hung rollback command short.
type blockingTransport struct{}

func (blockingTransport) Open(context.Context, inventory.Host) (ssh.Session, error) {
	return blockingSession{}, nil
}
func (blockingTransport) Close() error { return nil }

type blockingSession struct{}

func (blockingSession) Run(ctx context.Context, _ string, _ ssh.RunOptions) (*ssh.Result, error) {
	<-ctx.Done()
	return nil, &ssh.TransportError{Class: ssh.ErrorClassTimeout, Err: ctx.Err()}
}
func (blockingSession) Close() error { return nil }

type failingOpenTransport struct{}

func (failingOpenTransport) Open(context.Context, inventory.Host) (ssh.Session, error) {
	return nil, &ssh.TransportError{Class: ssh.ErrorClassConnection, Err: errors.New("no route to host")}
}
func (failingOpenTransport) Close() error { return nil }
