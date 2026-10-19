// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	require.NoError(t, cfg.Validate())
	require.Equal(t, 50, cfg.Concurrency.GlobalMaxConnections)
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opsrun.yaml")
	content := `
hosts:
  - name: web-1
    address: 10.0.0.1
    user: ops
    groups: [web]
concurrency:
  global_max_connections: 100
  per_host_max_sessions: 4
  max_parallel_steps: 20
timeouts:
  connect: 5s
  command: 2m
  idle: 30s
audit:
  log_path: /var/log/opsrun/audit.jsonl
approval:
  mode: all
  timeout: 10m
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := Load(path)
	require.NoError(t, err)
	require.Len(t, cfg.Hosts, 1)
	require.Equal(t, "web-1", cfg.Hosts[0].Name)
	require.Equal(t, 100, cfg.Concurrency.GlobalMaxConnections)
	require.Equal(t, "all", cfg.Approval.Mode)

	hosts := cfg.InventoryHosts()
	require.Len(t, hosts, 1)
	require.Equal(t, []string{"web"}, hosts[0].Groups)
}

func TestLoadEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opsrun.yaml")
	require.NoError(t, os.WriteFile(path, []byte("audit:\n  log_path: /tmp/a.jsonl\n"), 0o600))

	t.Setenv("OPSRUN_AUDIT_LOG_PATH", "/tmp/override.jsonl")
	t.Setenv("OPSRUN_MAX_PARALLEL_STEPS", "7")

	cfg, err := Load(path)
	require.NoError(t, err)
	require.Equal(t, "/tmp/override.jsonl", cfg.Audit.LogPath)
	require.Equal(t, 7, cfg.Concurrency.MaxParallelSteps)
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	require.Error(t, err)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"valid defaults", func(c *Config) {}, false},
		{"duplicate host", func(c *Config) {
			c.Hosts = []HostConfig{{Name: "a", Address: "1.1.1.1"}, {Name: "a", Address: "2.2.2.2"}}
		}, true},
		{"host missing name", func(c *Config) {
			c.Hosts = []HostConfig{{Address: "1.1.1.1"}}
		}, true},
		{"host missing address", func(c *Config) {
			c.Hosts = []HostConfig{{Name: "a"}}
		}, true},
		{"zero global connections", func(c *Config) {
			c.Concurrency.GlobalMaxConnections = 0
		}, true},
		{"zero per host sessions", func(c *Config) {
			c.Concurrency.PerHostMaxSessions = 0
		}, true},
		{"zero max parallel steps", func(c *Config) {
			c.Concurrency.MaxParallelSteps = 0
		}, true},
		{"invalid approval mode", func(c *Config) {
			c.Approval.Mode = "sometimes"
		}, true},
		{"empty audit log path", func(c *Config) {
			c.Audit.LogPath = ""
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.mutate(cfg)
			err := cfg.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
