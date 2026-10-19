// SPDX-License-Identifier: Apache-2.0

package secrets

import "context"

// ChainProvider tries each Provider in order and returns the first successful result.
// This is the extension point for additional backends: a Vault-backed Provider can be
// placed anywhere in the chain without changing callers, since it only needs to satisfy
// the Provider interface.
type ChainProvider struct {
	Providers []Provider
}

// Get implements Provider.
func (c *ChainProvider) Get(ctx context.Context, key string) (string, error) {
	var lastErr error
	for _, p := range c.Providers {
		v, err := p.Get(ctx, key)
		if err == nil {
			return v, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = &ErrNotFound{Key: key}
	}
	return "", lastErr
}
