// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	xssh "golang.org/x/crypto/ssh"
)

// sshSession implements Session over a single *xssh.Session (one SSH "exec" channel).
type sshSession struct {
	session *xssh.Session
	host    string
	release func()

	closeOnce sync.Once
}

// Run implements Session.
func (s *sshSession) Run(ctx context.Context, command string, opts RunOptions) (*Result, error) {
	if opts.Sudo {
		command = "sudo -n -- " + command
	}
	if opts.PTY {
		if err := s.session.RequestPty("xterm", 80, 40, xssh.TerminalModes{}); err != nil {
			return nil, &TransportError{Class: ErrorClassConnection, Host: s.host, Err: err}
		}
	}

	limit := opts.MaxOutputBytes
	if limit <= 0 {
		limit = defaultMaxOutputBytes
	}

	outLW := newHostWriter(opts.Stdout, s.host)
	errLW := newHostWriter(opts.Stderr, s.host)
	stdoutCap := newCapturingWriter(asWriter(outLW), limit)
	stderrCap := newCapturingWriter(asWriter(errLW), limit)
	s.session.Stdout = stdoutCap
	s.session.Stderr = stderrCap
	defer func() {
		if outLW != nil {
			outLW.Flush()
		}
		if errLW != nil {
			errLW.Flush()
		}
	}()

	runCtx := ctx
	var cancel context.CancelFunc
	if opts.CommandTimeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, opts.CommandTimeout)
		defer cancel()
	}

	start := time.Now()
	if err := s.session.Start(command); err != nil {
		return nil, &TransportError{Class: ErrorClassConnection, Host: s.host, Err: err}
	}

	done := make(chan error, 1)
	go func() { done <- s.session.Wait() }()

	select {
	case waitErr := <-done:
		return s.finish(start, stdoutCap, stderrCap, waitErr)
	case <-runCtx.Done():
		_ = s.session.Signal(xssh.SIGKILL) // best effort; many servers ignore signal requests
		_ = s.session.Close()
		<-done //nolint:errcheck // draining Wait(); its error is moot once we have already decided to report a timeout/cancellation
		class := ErrorClassTimeout
		if ctx.Err() != nil {
			class = ErrorClassCanceled
		}
		result := &Result{
			Duration:  time.Since(start),
			Stdout:    stdoutCap.String(),
			Stderr:    stderrCap.String(),
			Truncated: stdoutCap.truncated || stderrCap.truncated,
		}
		return result, &TransportError{Class: class, Host: s.host, Err: runCtx.Err()}
	}
}

func (s *sshSession) finish(start time.Time, stdoutCap, stderrCap *capturingWriter, waitErr error) (*Result, error) {
	result := &Result{
		Duration:  time.Since(start),
		Stdout:    stdoutCap.String(),
		Stderr:    stderrCap.String(),
		Truncated: stdoutCap.truncated || stderrCap.truncated,
	}
	if waitErr == nil {
		result.ExitCode = 0
		return result, nil
	}
	var exitErr *xssh.ExitError
	if errors.As(waitErr, &exitErr) {
		result.ExitCode = exitErr.ExitStatus()
		return result, nil
	}
	return result, &TransportError{Class: ErrorClassConnection, Host: s.host, Err: waitErr}
}

// Close implements Session.
func (s *sshSession) Close() error {
	s.closeOnce.Do(s.release)
	if err := s.session.Close(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func newHostWriter(w io.Writer, host string) *lineWriter {
	if w == nil {
		return nil
	}
	return newLineWriter(w, "["+host+"] ")
}

// asWriter returns w as an io.Writer, or a true nil interface if w is a nil *lineWriter,
// avoiding the typed-nil-interface pitfall.
func asWriter(w *lineWriter) io.Writer {
	if w == nil {
		return nil
	}
	return w
}
