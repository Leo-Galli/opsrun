// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"
	"os"
)

// EnvProvider resolves secrets from environment variables, optionally prefixed.
type EnvProvider struct {
	Prefix string
}

// Get implements Provider.
func (p *EnvProvider) Get(_ context.Context, key string) (string, error) {
	v, ok := os.LookupEnv(p.Prefix + key)
	if !ok {
		return "", &ErrNotFound{Key: key}
	}
	return v, nil
}
