// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bufio"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
)

// ConfigEntry holds the subset of ~/.ssh/config directives OpsRun understands for a
// matched Host pattern.
type ConfigEntry struct {
	HostName              string
	User                  string
	Port                  int
	IdentityFile          string
	ProxyJump             string
	StrictHostKeyChecking string
}

type configBlock struct {
	patterns []string
	entry    ConfigEntry
}

// Config is a parsed ~/.ssh/config, queryable by host alias.
type Config struct {
	blocks []configBlock
}

// LoadSSHConfig reads and parses the SSH client config file at path. A missing file is
// not an error: it is treated as an empty configuration.
func LoadSSHConfig(path string) (*Config, error) {
	f, err := os.Open(path) //nolint:gosec // path is an operator-configured ~/.ssh/config location, not untrusted input
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	defer f.Close()
	return ParseSSHConfig(f)
}

// ParseSSHConfig parses an OpenSSH client config from r.
func ParseSSHConfig(r io.Reader) (*Config, error) {
	cfg := &Config{}
	var cur *configBlock

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := splitDirective(line)
		if !ok {
			continue
		}
		switch strings.ToLower(key) {
		case "host":
			cfg.blocks = append(cfg.blocks, configBlock{patterns: strings.Fields(value)})
			cur = &cfg.blocks[len(cfg.blocks)-1]
		case "hostname":
			if cur != nil {
				cur.entry.HostName = value
			}
		case "user":
			if cur != nil {
				cur.entry.User = value
			}
		case "port":
			if cur != nil {
				if p, err := strconv.Atoi(value); err == nil {
					cur.entry.Port = p
				}
			}
		case "identityfile":
			if cur != nil {
				cur.entry.IdentityFile = value
			}
		case "proxyjump":
			if cur != nil {
				cur.entry.ProxyJump = value
			}
		case "stricthostkeychecking":
			if cur != nil {
				cur.entry.StrictHostKeyChecking = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func splitDirective(line string) (key, value string, ok bool) {
	fields := strings.SplitN(line, " ", 2)
	if len(fields) < 2 {
		fields = strings.SplitN(line, "\t", 2)
	}
	if len(fields) < 2 {
		if eq := strings.SplitN(line, "=", 2); len(eq) == 2 {
			return strings.TrimSpace(eq[0]), strings.Trim(strings.TrimSpace(eq[1]), `"`), true
		}
		return "", "", false
	}
	return strings.TrimSpace(fields[0]), strings.Trim(strings.TrimSpace(fields[1]), `"`), true
}

// Lookup merges every block whose pattern matches host, in file order, with earlier
// blocks taking precedence for any field they set, matching OpenSSH's "first obtained
// value wins" semantics.
func (c *Config) Lookup(host string) ConfigEntry {
	var result ConfigEntry
	for _, b := range c.blocks {
		if !matchesAny(b.patterns, host) {
			continue
		}
		if result.HostName == "" {
			result.HostName = b.entry.HostName
		}
		if result.User == "" {
			result.User = b.entry.User
		}
		if result.Port == 0 {
			result.Port = b.entry.Port
		}
		if result.IdentityFile == "" {
			result.IdentityFile = b.entry.IdentityFile
		}
		if result.ProxyJump == "" {
			result.ProxyJump = b.entry.ProxyJump
		}
		if result.StrictHostKeyChecking == "" {
			result.StrictHostKeyChecking = b.entry.StrictHostKeyChecking
		}
	}
	return result
}

func matchesAny(patterns []string, host string) bool {
	matched := false
	for _, p := range patterns {
		negate := strings.HasPrefix(p, "!")
		if negate {
			p = p[1:]
		}
		ok, _ := path.Match(p, host)
		if !ok {
			continue
		}
		if negate {
			return false
		}
		matched = true
	}
	return matched
}
