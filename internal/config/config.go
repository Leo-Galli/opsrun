// SPDX-License-Identifier: Apache-2.0

// Package config loads and validates opsrun.yaml: the host inventory and groups, users,
// keys, jump hosts, concurrency limits, timeouts, audit log path, and approval policy.
// Values can be overridden by OPSRUN_-prefixed environment variables and, at the call
// site, by command-line flags.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// HostConfig is one inventory entry.
type HostConfig struct {
	Name         string   `yaml:"name"`
	Address      string   `yaml:"address"`
	Port         int      `yaml:"port"`
	User         string   `yaml:"user"`
	IdentityFile string   `yaml:"identity_file"`
	ProxyJump    string   `yaml:"proxy_jump"`
	Groups       []string `yaml:"groups"`
}

// ConcurrencyConfig bounds how many connections, sessions, and steps may run at once.
type ConcurrencyConfig struct {
	GlobalMaxConnections int `yaml:"global_max_connections"`
	PerHostMaxSessions   int `yaml:"per_host_max_sessions"`
	MaxParallelSteps     int `yaml:"max_parallel_steps"`
}

// TimeoutsConfig bounds how long transport operations may take.
type TimeoutsConfig struct {
	Connect Duration `yaml:"connect"`
	Command Duration `yaml:"command"`
	Idle    Duration `yaml:"idle"`
}

// AuditConfig configures where and how audit events are persisted.
type AuditConfig struct {
	LogPath        string `yaml:"log_path"`
	SigningKeyPath string `yaml:"signing_key_path"`
}

// ApprovalConfig is the default, runbook-wide approval policy.
type ApprovalConfig struct {
	Mode    string   `yaml:"mode"` // none, per_step, all
	Timeout Duration `yaml:"timeout"`
}

// SecretsConfig configures how secret values are resolved.
type SecretsConfig struct {
	EnvPrefix string `yaml:"env_prefix"`
	FilesDir  string `yaml:"files_dir"`
}

// Config is the root opsrun.yaml schema.
type Config struct {
	Hosts       []HostConfig      `yaml:"hosts"`
	Concurrency ConcurrencyConfig `yaml:"concurrency"`
	Timeouts    TimeoutsConfig    `yaml:"timeouts"`
	Audit       AuditConfig       `yaml:"audit"`
	Approval    ApprovalConfig    `yaml:"approval"`
	Secrets     SecretsConfig     `yaml:"secrets"`
}

// Default returns a Config with safe, conservative defaults.
func Default() *Config {
	return &Config{
		Concurrency: ConcurrencyConfig{
			GlobalMaxConnections: 50,
			PerHostMaxSessions:   2,
			MaxParallelSteps:     10,
		},
		Timeouts: TimeoutsConfig{
			Connect: Duration(10_000_000_000),  // 10s
			Command: Duration(300_000_000_000), // 5m
			Idle:    Duration(60_000_000_000),  // 1m
		},
		Audit: AuditConfig{
			LogPath: "./audit/opsrun-audit.jsonl",
		},
		Approval: ApprovalConfig{
			Mode: "per_step",
		},
		Secrets: SecretsConfig{
			EnvPrefix: "OPSRUN_SECRET_",
		},
	}
}

// Load reads and parses the YAML file at path, applies environment variable overrides,
// and validates the result.
func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	applyEnvOverrides(cfg)
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %q: %w", path, err)
	}
	return cfg, nil
}

// envOverrides maps OPSRUN_-prefixed environment variables to setter functions, keeping
// Load's override logic declarative and easy to extend.
func applyEnvOverrides(cfg *Config) {
	if v, ok := os.LookupEnv("OPSRUN_AUDIT_LOG_PATH"); ok {
		cfg.Audit.LogPath = v
	}
	if v, ok := os.LookupEnv("OPSRUN_APPROVAL_MODE"); ok {
		cfg.Approval.Mode = v
	}
	if v, ok := os.LookupEnv("OPSRUN_MAX_PARALLEL_STEPS"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Concurrency.MaxParallelSteps = n
		}
	}
	if v, ok := os.LookupEnv("OPSRUN_GLOBAL_MAX_CONNECTIONS"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Concurrency.GlobalMaxConnections = n
		}
	}
	if v, ok := os.LookupEnv("OPSRUN_PER_HOST_MAX_SESSIONS"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Concurrency.PerHostMaxSessions = n
		}
	}
}

// validApprovalModes lists the approval modes accepted in opsrun.yaml.
var validApprovalModes = map[string]bool{
	"none":     true,
	"per_step": true,
	"all":      true,
}

// Validate checks the configuration for internal consistency.
func (c *Config) Validate() error {
	seen := make(map[string]bool, len(c.Hosts))
	for _, h := range c.Hosts {
		if h.Name == "" {
			return fmt.Errorf("host entry missing name")
		}
		if seen[h.Name] {
			return fmt.Errorf("duplicate host name %q", h.Name)
		}
		seen[h.Name] = true
		if h.Address == "" {
			return fmt.Errorf("host %q missing address", h.Name)
		}
	}
	if c.Concurrency.GlobalMaxConnections <= 0 {
		return fmt.Errorf("concurrency.global_max_connections must be positive")
	}
	if c.Concurrency.PerHostMaxSessions <= 0 {
		return fmt.Errorf("concurrency.per_host_max_sessions must be positive")
	}
	if c.Concurrency.MaxParallelSteps <= 0 {
		return fmt.Errorf("concurrency.max_parallel_steps must be positive")
	}
	if c.Approval.Mode != "" && !validApprovalModes[strings.ToLower(c.Approval.Mode)] {
		return fmt.Errorf("approval.mode %q is not one of none, per_step, all", c.Approval.Mode)
	}
	if c.Audit.LogPath == "" {
		return fmt.Errorf("audit.log_path must not be empty")
	}
	return nil
}
