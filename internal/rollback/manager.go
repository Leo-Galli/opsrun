// SPDX-License-Identifier: Apache-2.0

package rollback

import (
	"context"
	"fmt"
	"time"

	"github.com/Leo-Galli/opsrun/internal/ssh"
)

// DefaultManager rolls back completed steps by opening a Transport session per step/host
// and running its resolved rollback command, enforcing its own timeout independent of
// the run that triggered the rollback.
type DefaultManager struct {
	Transport ssh.Transport
	Timeout   time.Duration // zero means no per-rollback timeout beyond ctx
}

// Rollback implements Manager. It never retries a failed rollback: a failure is recorded
// in the returned Outcome and rollback continues with the next entry.
func (m *DefaultManager) Rollback(ctx context.Context, completed []CompletedStep) []Outcome {
	outcomes := make([]Outcome, 0, len(completed))
	for _, c := range completed {
		outcomes = append(outcomes, m.rollbackOne(ctx, c))
	}
	return outcomes
}

func (m *DefaultManager) rollbackOne(ctx context.Context, c CompletedStep) Outcome {
	if c.Step.Rollback == nil {
		return Outcome{CompletedStep: c}
	}

	rctx := ctx
	if m.Timeout > 0 {
		var cancel context.CancelFunc
		rctx, cancel = context.WithTimeout(ctx, m.Timeout)
		defer cancel()
	}

	sess, err := m.Transport.Open(rctx, c.Host)
	if err != nil {
		return Outcome{CompletedStep: c, Err: fmt.Errorf("open session for rollback: %w", err)}
	}
	defer sess.Close() //nolint:errcheck

	result, err := sess.Run(rctx, c.ResolvedRollbackCommand, ssh.RunOptions{CommandTimeout: m.Timeout})
	if err != nil {
		return Outcome{CompletedStep: c, Err: fmt.Errorf("run rollback: %w", err)}
	}
	if result.ExitCode != 0 {
		return Outcome{CompletedStep: c, Err: fmt.Errorf("rollback exited with code %d: %s", result.ExitCode, result.Stderr)}
	}
	return Outcome{CompletedStep: c}
}
