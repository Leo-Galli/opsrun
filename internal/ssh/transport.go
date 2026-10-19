// SPDX-License-Identifier: Apache-2.0

// Package ssh implements remote command execution over SSH, behind a Transport/Session
// abstraction that also admits a local transport for the control node and for tests.
package ssh

import (
	"context"
	"io"
	"time"

	"github.com/Leo-Galli/opsrun/internal/inventory"
)

// ErrorClass classifies why a transport operation failed, so callers can decide whether
// a retry makes sense.
type ErrorClass string

// Supported error classes.
const (
	ErrorClassConnection  ErrorClass = "connection"
	ErrorClassAuthn       ErrorClass = "authentication"
	ErrorClassHostKey     ErrorClass = "host_key"
	ErrorClassTimeout     ErrorClass = "timeout"
	ErrorClassNonZeroExit ErrorClass = "non_zero_exit"
	ErrorClassCanceled    ErrorClass = "canceled"
)

// TransportError wraps a transport-level failure with its classification.
type TransportError struct {
	Class ErrorClass
	Host  string
	Err   error
}

// Error implements the error interface.
func (e *TransportError) Error() string {
	if e.Host != "" {
		return string(e.Class) + ": " + e.Host + ": " + e.Err.Error()
	}
	return string(e.Class) + ": " + e.Err.Error()
}

// Unwrap allows errors.Is / errors.As to see through TransportError.
func (e *TransportError) Unwrap() error { return e.Err }

// Retryable reports whether the error class represents a transient network condition
// worth retrying. Non-zero exit codes and authentication failures are never retryable.
func (e *TransportError) Retryable() bool {
	switch e.Class {
	case ErrorClassConnection, ErrorClassTimeout:
		return true
	default:
		return false
	}
}

// RunOptions configures how a command is executed in a Session.
type RunOptions struct {
	// Stdout and Stderr, if non-nil, receive the command's output line by line as it
	// is produced.
	Stdout io.Writer
	Stderr io.Writer

	// PTY requests a pseudo-terminal be allocated for the command.
	PTY bool

	// Sudo requests the command run under non-interactive sudo.
	Sudo bool

	// CommandTimeout bounds the total time the command may run. Zero means no timeout
	// beyond the context passed to Run.
	CommandTimeout time.Duration

	// MaxOutputBytes truncates captured output beyond this size per stream. Zero means
	// use the transport's default.
	MaxOutputBytes int64
}

// Result is the outcome of running a command in a Session.
type Result struct {
	ExitCode  int
	Duration  time.Duration
	Stdout    string
	Stderr    string
	Truncated bool
}

// Session represents a single remote command-execution context against one host.
// Closing a session releases any underlying connection resources.
type Session interface {
	// Run executes command and blocks until it completes, the context is canceled, or
	// CommandTimeout elapses. Canceling ctx must close the remote command, not merely
	// stop waiting for it.
	Run(ctx context.Context, command string, opts RunOptions) (*Result, error)

	// Close releases resources held by the session.
	Close() error
}

// Transport opens Sessions against a host. A Transport implementation owns connection
// pooling, authentication, and host key verification for the hosts it serves.
type Transport interface {
	// Open returns a Session for host. The returned Session must be closed by the
	// caller when no longer needed.
	Open(ctx context.Context, host inventory.Host) (Session, error)

	// Close releases all resources held by the transport, closing any pooled
	// connections.
	Close() error
}
