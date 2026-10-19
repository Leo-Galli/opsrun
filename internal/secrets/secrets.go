// SPDX-License-Identifier: Apache-2.0

// Package secrets resolves sensitive values (key passphrases, vault tokens, and the
// like) OpsRun needs without ever writing them to disk or logs.
package secrets

import "context"

// Provider resolves a named secret to its value. Implementations must not log the
// resolved value.
type Provider interface {
	Get(ctx context.Context, key string) (string, error)
}

// ErrNotFound is returned by a Provider when the requested key does not exist.
type ErrNotFound struct {
	Key string
}

// Error implements the error interface.
func (e *ErrNotFound) Error() string {
	return "secret not found: " + e.Key
}
