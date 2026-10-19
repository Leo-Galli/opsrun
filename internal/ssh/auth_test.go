// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	xssh "golang.org/x/crypto/ssh"

	"github.com/Leo-Galli/opsrun/internal/secrets"
)

func pemEncode(block *pem.Block) []byte {
	return pem.EncodeToMemory(block)
}

func generateTestKeyFiles(t *testing.T, encrypt bool) (plainOrEncrypted string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	_ = pub

	var pemBytes []byte
	if encrypt {
		block, err := xssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte("s3cr3t"))
		require.NoError(t, err)
		pemBytes = pemEncode(block)
	} else {
		block, err := xssh.MarshalPrivateKey(priv, "")
		require.NoError(t, err)
		pemBytes = pemEncode(block)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "id_ed25519")
	require.NoError(t, os.WriteFile(path, pemBytes, 0o600))
	return path
}

func TestBuildAuthMethodsPlainKey(t *testing.T) {
	path := generateTestKeyFiles(t, false)
	methods, err := BuildAuthMethods(context.Background(), AuthConfig{PrivateKeyPath: path})
	require.NoError(t, err)
	require.Len(t, methods, 1)
}

func TestBuildAuthMethodsEncryptedKey(t *testing.T) {
	path := generateTestKeyFiles(t, true)
	provider := &secrets.EnvProvider{Prefix: ""}
	t.Setenv("KEY_PASSPHRASE", "s3cr3t")
	methods, err := BuildAuthMethods(context.Background(), AuthConfig{
		PrivateKeyPath:      path,
		PassphraseSecretKey: "KEY_PASSPHRASE",
		Secrets:             provider,
	})
	require.NoError(t, err)
	require.Len(t, methods, 1)
}

func TestBuildAuthMethodsEncryptedKeyNoProvider(t *testing.T) {
	path := generateTestKeyFiles(t, true)
	_, err := BuildAuthMethods(context.Background(), AuthConfig{PrivateKeyPath: path})
	require.Error(t, err)
}

func TestBuildAuthMethodsEncryptedKeyWrongPassphrase(t *testing.T) {
	path := generateTestKeyFiles(t, true)
	provider := &secrets.EnvProvider{Prefix: ""}
	t.Setenv("KEY_PASSPHRASE", "wrong")
	_, err := BuildAuthMethods(context.Background(), AuthConfig{
		PrivateKeyPath:      path,
		PassphraseSecretKey: "KEY_PASSPHRASE",
		Secrets:             provider,
	})
	require.Error(t, err)
}

func TestBuildAuthMethodsNoneConfigured(t *testing.T) {
	_, err := BuildAuthMethods(context.Background(), AuthConfig{})
	require.Error(t, err)
}

func TestBuildAuthMethodsMissingKeyFile(t *testing.T) {
	_, err := BuildAuthMethods(context.Background(), AuthConfig{PrivateKeyPath: "/nonexistent/key"})
	require.Error(t, err)
}

func TestBuildAuthMethodsAgentUnavailable(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	_, err := BuildAuthMethods(context.Background(), AuthConfig{UseAgent: true})
	require.Error(t, err, "no agent and no key configured must fail")
}
