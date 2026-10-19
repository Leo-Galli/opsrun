// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/ed25519"
	"fmt"
	"os"
)

// loadAuditSigner reads a raw 64-byte Ed25519 private key from path, used to sign audit
// log records. An empty path disables signing. Generate a key with:
//
//	opsrun audit keygen --out audit.key --out-pub audit.pub
func loadAuditSigner(path string) (ed25519.PrivateKey, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read audit signing key %q: %w", path, err)
	}
	if len(data) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("audit signing key %q: expected %d raw bytes, got %d", path, ed25519.PrivateKeySize, len(data))
	}
	return ed25519.PrivateKey(data), nil
}

// loadAuditPublicKey reads a raw 32-byte Ed25519 public key from path, used by
// `opsrun audit verify --public-key`.
func loadAuditPublicKey(path string) (ed25519.PublicKey, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read audit public key %q: %w", path, err)
	}
	if len(data) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("audit public key %q: expected %d raw bytes, got %d", path, ed25519.PublicKeySize, len(data))
	}
	return ed25519.PublicKey(data), nil
}
