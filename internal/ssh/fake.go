// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"sync"

	"github.com/Leo-Galli/opsrun/internal/inventory"
)

// FakeHandler decides the outcome of running command against host. It is the single
// extension point FakeTransport exposes, so tests can script exact per-host, per-command
// behavior (success, failure, slowness) without a real process or network connection.
type FakeHandler func(host inventory.Host, command string, opts RunOptions) (*Result, error)

// FakeTransport is an in-memory Transport for deterministic, instantaneous tests of
// code that depends on Transport/Session, such as the executor.
type FakeTransport struct {
	Handler FakeHandler

	mu     sync.Mutex
	opens  []inventory.Host
	closed bool
}

// NewFakeTransport returns a FakeTransport that dispatches every Run call to handler. A
// nil handler makes every command succeed with exit code 0 and no output.
func NewFakeTransport(handler FakeHandler) *FakeTransport {
	return &FakeTransport{Handler: handler}
}

// Open implements Transport.
func (f *FakeTransport) Open(_ context.Context, host inventory.Host) (Session, error) {
	f.mu.Lock()
	f.opens = append(f.opens, host)
	f.mu.Unlock()
	return &fakeSession{transport: f, host: host}, nil
}

// Close implements Transport.
func (f *FakeTransport) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

// Opens returns every host Open was called with, in call order.
func (f *FakeTransport) Opens() []inventory.Host {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]inventory.Host(nil), f.opens...)
}

type fakeSession struct {
	transport *FakeTransport
	host      inventory.Host
}

// Run implements Session.
func (s *fakeSession) Run(ctx context.Context, command string, opts RunOptions) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, &TransportError{Class: ErrorClassCanceled, Host: s.host.Name, Err: err}
	}
	if s.transport.Handler == nil {
		return &Result{ExitCode: 0}, nil
	}
	return s.transport.Handler(s.host, command, opts)
}

// Close implements Session.
func (s *fakeSession) Close() error { return nil }
