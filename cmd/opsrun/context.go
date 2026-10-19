// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	xssh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"gopkg.in/yaml.v3"

	"github.com/Leo-Galli/opsrun/internal/audit"
	"github.com/Leo-Galli/opsrun/internal/config"
	"github.com/Leo-Galli/opsrun/internal/executor"
	"github.com/Leo-Galli/opsrun/internal/inventory"
	"github.com/Leo-Galli/opsrun/internal/parser"
	"github.com/Leo-Galli/opsrun/internal/rollback"
	"github.com/Leo-Galli/opsrun/internal/runbook"
	"github.com/Leo-Galli/opsrun/internal/secrets"
	"github.com/Leo-Galli/opsrun/internal/ssh"
)

// loadConfig reads opsrun.yaml at path, or returns safe defaults if path is empty.
func loadConfig(path string) (*config.Config, error) {
	if path == "" {
		return config.Default(), nil
	}
	return config.Load(path)
}

// parseRunbookFile parses the runbook at path.
func parseRunbookFile(path string) (*runbook.Runbook, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open runbook %q: %w", path, err)
	}
	defer f.Close()
	rb, err := parser.NewMarkdownParser().Parse(context.Background(), f)
	if err != nil {
		return nil, fmt.Errorf("parse runbook %q: %w", path, err)
	}
	return rb, nil
}

// resolveVars merges --vars-file content with --var key=value overrides, the latter
// taking precedence.
func resolveVars(varsFile string, varFlags []string) (map[string]any, error) {
	vars := map[string]any{}
	if varsFile != "" {
		data, err := os.ReadFile(varsFile)
		if err != nil {
			return nil, fmt.Errorf("read vars file %q: %w", varsFile, err)
		}
		if err := yaml.Unmarshal(data, &vars); err != nil {
			return nil, fmt.Errorf("parse vars file %q: %w", varsFile, err)
		}
	}
	for _, kv := range varFlags {
		key, value, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --var %q: expected key=value", kv)
		}
		vars[key] = value
	}
	return vars, nil
}

// secretsProvider builds the chain used to resolve encrypted private key passphrases:
// environment variables first, then files under cfg.Secrets.FilesDir if configured.
func secretsProvider(cfg *config.Config) secrets.Provider {
	chain := &secrets.ChainProvider{Providers: []secrets.Provider{
		&secrets.EnvProvider{Prefix: cfg.Secrets.EnvPrefix},
	}}
	if cfg.Secrets.FilesDir != "" {
		chain.Providers = append(chain.Providers, &secrets.FileProvider{Dir: cfg.Secrets.FilesDir})
	}
	return chain
}

// buildHostKeyCallback loads ~/.ssh/known_hosts (or $SSH_KNOWN_HOSTS), failing closed if
// it cannot, unless insecureSkipHostKeyVerification is set, in which case it warns
// loudly and accepts any host key.
func buildHostKeyCallback(insecureSkipHostKeyVerification bool, out *os.File) (xssh.HostKeyCallback, error) {
	if insecureSkipHostKeyVerification {
		fmt.Fprintln(out, "[WARN] host key verification is DISABLED (--insecure-skip-host-key-verification); this run is vulnerable to man-in-the-middle attacks")
		return xssh.InsecureIgnoreHostKey(), nil //nolint:gosec
	}
	path := os.Getenv("SSH_KNOWN_HOSTS")
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("determine home directory for known_hosts: %w", err)
		}
		path = filepath.Join(home, ".ssh", "known_hosts")
	}
	cb, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts %q: %w (use --insecure-skip-host-key-verification to bypass, not recommended)", path, err)
	}
	return cb, nil
}

// buildTransport constructs the SSH transport used for every host in cfg, with
// authentication resolved per host from its configured identity file (falling back to
// ssh-agent) and the connection pool limits from cfg.Concurrency.
func buildTransport(cfg *config.Config, insecureSkipHostKeyVerification bool, out *os.File) (ssh.Transport, error) {
	hostKeyCallback, err := buildHostKeyCallback(insecureSkipHostKeyVerification, out)
	if err != nil {
		return nil, err
	}
	provider := secretsProvider(cfg)

	poolCfg := ssh.PoolConfig{
		GlobalMaxConnections: cfg.Concurrency.GlobalMaxConnections,
		PerHostMaxSessions:   cfg.Concurrency.PerHostMaxSessions,
		ConnectTimeout:       cfg.Timeouts.Connect.Value(),
		IdleTimeout:          cfg.Timeouts.Idle.Value(),
		KeepAliveInterval:    30 * cfg.Timeouts.Idle.Value() / 100, // a fraction of the idle timeout, a reasonable default
		HostKeyCallback:      hostKeyCallback,
		AuthForHost: func(ctx context.Context, host inventory.Host) ([]xssh.AuthMethod, error) {
			authCfg := ssh.AuthConfig{
				UseAgent:            true,
				PrivateKeyPath:      host.IdentityFile,
				PassphraseSecretKey: host.Name + "_passphrase",
				Secrets:             provider,
			}
			if authCfg.PrivateKeyPath == "" {
				authCfg.UseAgent = true
			}
			return ssh.BuildAuthMethods(ctx, authCfg)
		},
	}
	return ssh.NewSSHTransport(poolCfg)
}

// buildExecutor wires a DefaultExecutor from cfg, ready to Run or Resume. emitters are
// combined with the audit sink (if any) into a single Emitter. The returned closer must
// be called (even on error paths after it is non-nil) to flush the audit log.
func buildExecutor(cfg *config.Config, insecureSkipHostKeyVerification bool, out *os.File, extraEmitters ...executor.Emitter) (*executor.DefaultExecutor, audit.Sink, error) {
	transport, err := buildTransport(cfg, insecureSkipHostKeyVerification, out)
	if err != nil {
		return nil, nil, err
	}

	var auditSink audit.Sink
	if cfg.Audit.LogPath != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.Audit.LogPath), 0o755); err != nil {
			return nil, nil, fmt.Errorf("create audit log directory: %w", err)
		}
		signer, err := loadAuditSigner(cfg.Audit.SigningKeyPath)
		if err != nil {
			return nil, nil, err
		}
		auditSink, err = audit.NewFileSink(cfg.Audit.LogPath, signer)
		if err != nil {
			return nil, nil, err
		}
	}

	emitters := executor.MultiEmitter{}
	if auditSink != nil {
		emitters = append(emitters, &executor.AuditEmitter{Sink: auditSink})
	}
	emitters = append(emitters, extraEmitters...)

	exec := &executor.DefaultExecutor{
		Transport: transport,
		Inventory: inventory.NewStatic(cfg.InventoryHosts()),
		Rollback:  &rollback.DefaultManager{Transport: transport, Timeout: cfg.Timeouts.Command.Value()},
		Emitter:   emitters,
		Parser:    parser.NewMarkdownParser(),
	}
	return exec, auditSink, nil
}
