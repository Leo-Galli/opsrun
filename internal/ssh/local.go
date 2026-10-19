// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/Leo-Galli/opsrun/internal/inventory"
)

// LocalTransport runs commands on the control node itself, via /bin/sh. It implements
// the same Transport interface as the SSH transport, so the executor can run read-only
// or control-node steps without a network round trip, and tests can exercise the
// executor without any SSH server at all.
type LocalTransport struct{}

// NewLocalTransport returns a Transport that executes commands locally.
func NewLocalTransport() *LocalTransport {
	return &LocalTransport{}
}

// Open implements Transport. The host argument is accepted for interface compatibility
// but otherwise ignored: every session runs on the control node.
func (t *LocalTransport) Open(_ context.Context, _ inventory.Host) (Session, error) {
	return &localSession{}, nil
}

// Close implements Transport.
func (t *LocalTransport) Close() error { return nil }

type localSession struct {
	mu     sync.Mutex
	closed bool
}

// Run implements Session.
func (s *localSession) Run(ctx context.Context, command string, opts RunOptions) (*Result, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, &TransportError{Class: ErrorClassCanceled, Err: errors.New("session closed")}
	}
	s.mu.Unlock()

	if opts.Sudo {
		command = "sudo -n -- " + command
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if opts.CommandTimeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, opts.CommandTimeout)
		defer cancel()
	}

	cmd := exec.Command("sh", "-c", command) //nolint:gosec // executing the (parsed, validated, operator-reviewed) runbook command is this package's entire purpose
	setProcessGroup(cmd)

	limit := opts.MaxOutputBytes
	if limit <= 0 {
		limit = defaultMaxOutputBytes
	}

	stdoutCapture := newCapturingWriter(opts.Stdout, limit)
	stderrCapture := newCapturingWriter(opts.Stderr, limit)
	cmd.Stdout = stdoutCapture
	cmd.Stderr = stderrCapture

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, &TransportError{Class: ErrorClassConnection, Err: err}
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var err error
	select {
	case err = <-done:
	case <-runCtx.Done():
		killProcessGroup(cmd)
		<-done //nolint:errcheck // draining cmd.Wait(); its error is moot once we have already decided to report a timeout
		result := &Result{
			Duration:  time.Since(start),
			Stdout:    stdoutCapture.String(),
			Stderr:    stderrCapture.String(),
			Truncated: stdoutCapture.truncated || stderrCapture.truncated,
		}
		class := ErrorClassTimeout
		if ctx.Err() != nil {
			class = ErrorClassCanceled
		}
		return result, &TransportError{Class: class, Err: runCtx.Err()}
	}

	result := &Result{
		Duration:  time.Since(start),
		Stdout:    stdoutCapture.String(),
		Stderr:    stderrCapture.String(),
		Truncated: stdoutCapture.truncated || stderrCapture.truncated,
	}

	var exitErr *exec.ExitError
	switch {
	case err == nil:
		result.ExitCode = 0
		return result, nil
	case errors.As(err, &exitErr):
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	default:
		return result, &TransportError{Class: ErrorClassConnection, Err: err}
	}
}

// Close implements Session.
func (s *localSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

const defaultMaxOutputBytes = 1 << 20 // 1 MiB

// capturingWriter tees written bytes to an optional downstream writer (for streaming)
// while also buffering them, up to a limit, for the final Result.
type capturingWriter struct {
	mu         sync.Mutex
	downstream io.Writer
	buf        []byte
	limit      int64
	truncated  bool
}

func newCapturingWriter(downstream io.Writer, limit int64) *capturingWriter {
	return &capturingWriter{downstream: downstream, limit: limit}
}

func (w *capturingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if int64(len(w.buf)) < w.limit {
		remaining := w.limit - int64(len(w.buf))
		if int64(len(p)) <= remaining {
			w.buf = append(w.buf, p...)
		} else {
			w.buf = append(w.buf, p[:remaining]...)
			w.truncated = true
		}
	} else if len(p) > 0 {
		w.truncated = true
	}
	if w.downstream != nil {
		return w.downstream.Write(p)
	}
	return len(p), nil
}

func (w *capturingWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return string(w.buf)
}

// lineWriter wraps an io.Writer so that each write is prefixed with a host label and
// flushed line by line, matching the "stream output prefixed with host" requirement.
type lineWriter struct {
	mu     sync.Mutex
	prefix string
	out    io.Writer
	buf    []byte
}

func newLineWriter(out io.Writer, prefix string) *lineWriter {
	return &lineWriter{out: out, prefix: prefix}
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := w.buf[:i]
		w.buf = w.buf[i+1:]
		if _, err := io.WriteString(w.out, w.prefix+string(line)+"\n"); err != nil {
			return len(p), err
		}
	}
	return len(p), nil
}

// Flush writes any buffered partial line, without a trailing newline.
func (w *lineWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) > 0 {
		io.WriteString(w.out, w.prefix+string(w.buf)) //nolint:errcheck
		w.buf = nil
	}
}
