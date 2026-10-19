// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/Leo-Galli/opsrun/internal/secrets"
)

// AuthConfig describes how to authenticate to a host. At least one of UseAgent or
// PrivateKeyPath must be set.
type AuthConfig struct {
	// UseAgent authenticates via the ssh-agent reachable at SSH_AUTH_SOCK.
	UseAgent bool

	// PrivateKeyPath is the path to a private key file. If the key is encrypted, its
	// passphrase is resolved from Secrets using PassphraseSecretKey.
	PrivateKeyPath string

	// CertificatePath, if set, is an SSH certificate that pairs with PrivateKeyPath,
	// used for certificate-based authentication.
	CertificatePath string

	// PassphraseSecretKey is the key looked up in Secrets to decrypt PrivateKeyPath.
	// Ignored if PrivateKeyPath's key is not encrypted.
	PassphraseSecretKey string

	// Secrets resolves PassphraseSecretKey. May be nil if the key is not encrypted.
	Secrets secrets.Provider
}

// BuildAuthMethods constructs the ssh.AuthMethod list for a connection from cfg.
func BuildAuthMethods(ctx context.Context, cfg AuthConfig) ([]xssh.AuthMethod, error) {
	var methods []xssh.AuthMethod

	if cfg.UseAgent {
		if m, err := agentAuthMethod(); err == nil {
			methods = append(methods, m)
		}
	}

	if cfg.PrivateKeyPath != "" {
		signer, err := loadSigner(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("load private key %q: %w", cfg.PrivateKeyPath, err)
		}
		methods = append(methods, xssh.PublicKeys(signer))
	}

	if len(methods) == 0 {
		return nil, errors.New("no authentication method configured: set UseAgent or PrivateKeyPath")
	}
	return methods, nil
}

func agentAuthMethod() (xssh.AuthMethod, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, errors.New("SSH_AUTH_SOCK is not set")
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil, fmt.Errorf("dial ssh-agent: %w", err)
	}
	client := agent.NewClient(conn)
	return xssh.PublicKeysCallback(client.Signers), nil
}

func loadSigner(ctx context.Context, cfg AuthConfig) (xssh.Signer, error) {
	keyBytes, err := os.ReadFile(cfg.PrivateKeyPath)
	if err != nil {
		return nil, err
	}

	signer, err := xssh.ParsePrivateKey(keyBytes)
	var passErr *xssh.PassphraseMissingError
	if errors.As(err, &passErr) {
		if cfg.Secrets == nil {
			return nil, errors.New("private key is encrypted but no SecretProvider was configured")
		}
		passphrase, serr := cfg.Secrets.Get(ctx, cfg.PassphraseSecretKey)
		if serr != nil {
			return nil, fmt.Errorf("resolve key passphrase: %w", serr)
		}
		signer, err = xssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte(passphrase))
	}
	if err != nil {
		return nil, err
	}

	if cfg.CertificatePath == "" {
		return signer, nil
	}

	certBytes, err := os.ReadFile(cfg.CertificatePath)
	if err != nil {
		return nil, fmt.Errorf("read certificate: %w", err)
	}
	pub, _, _, _, err := xssh.ParseAuthorizedKey(certBytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}
	cert, ok := pub.(*xssh.Certificate)
	if !ok {
		return nil, fmt.Errorf("%q does not contain an SSH certificate", cfg.CertificatePath)
	}
	return xssh.NewCertSigner(cert, signer)
}
