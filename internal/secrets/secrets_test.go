// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvProvider(t *testing.T) {
	t.Setenv("OPSRUN_SECRET_DB_PASSWORD", "hunter2")
	p := &EnvProvider{Prefix: "OPSRUN_SECRET_"}
	v, err := p.Get(context.Background(), "DB_PASSWORD")
	require.NoError(t, err)
	require.Equal(t, "hunter2", v)

	_, err = p.Get(context.Background(), "MISSING")
	require.Error(t, err)
	var notFound *ErrNotFound
	require.ErrorAs(t, err, &notFound)
}

func TestFileProvider(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "api_key"), []byte("secret-value\n"), 0o600))
	p := &FileProvider{Dir: dir}

	v, err := p.Get(context.Background(), "api_key")
	require.NoError(t, err)
	require.Equal(t, "secret-value", v)

	_, err = p.Get(context.Background(), "missing")
	require.Error(t, err)
	var notFound *ErrNotFound
	require.ErrorAs(t, err, &notFound)
}

func TestChainProvider(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "only_in_file"), []byte("from-file"), 0o600))
	t.Setenv("OPSRUN_SECRET_ONLY_IN_ENV", "from-env")

	chain := &ChainProvider{Providers: []Provider{
		&EnvProvider{Prefix: "OPSRUN_SECRET_"},
		&FileProvider{Dir: dir},
	}}

	v, err := chain.Get(context.Background(), "ONLY_IN_ENV")
	require.NoError(t, err)
	require.Equal(t, "from-env", v)

	// env lookup fails for this key, so the chain falls through to the file provider.
	v, err = chain.Get(context.Background(), "only_in_file")
	require.NoError(t, err)
	require.Equal(t, "from-file", v)

	_, err = chain.Get(context.Background(), "nowhere")
	require.Error(t, err)
}
