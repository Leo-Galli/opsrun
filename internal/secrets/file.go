// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// FileProvider resolves secrets by reading one file per key from a directory, trimming
// a single trailing newline if present. This matches the convention used by Docker and
// Kubernetes secret mounts.
type FileProvider struct {
	Dir string
}

// Get implements Provider.
func (p *FileProvider) Get(_ context.Context, key string) (string, error) {
	data, err := os.ReadFile(filepath.Join(p.Dir, key)) //nolint:gosec // p.Dir is an operator-configured secrets directory, key is a known secret name, not untrusted input
	if err != nil {
		if os.IsNotExist(err) {
			return "", &ErrNotFound{Key: key}
		}
		return "", err
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}
