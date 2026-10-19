// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const sampleSSHConfig = `
# comment line
Host web-*
    HostName internal.example.com
    User deploy
    Port 2222
    IdentityFile ~/.ssh/web_key
    ProxyJump bastion.example.com:22

Host db-1
    HostName db-internal.example.com
    User dbadmin
    StrictHostKeyChecking yes
`

func TestParseSSHConfig(t *testing.T) {
	cfg, err := ParseSSHConfig(strings.NewReader(sampleSSHConfig))
	require.NoError(t, err)

	e := cfg.Lookup("web-1")
	require.Equal(t, "internal.example.com", e.HostName)
	require.Equal(t, "deploy", e.User)
	require.Equal(t, 2222, e.Port)
	require.Equal(t, "~/.ssh/web_key", e.IdentityFile)
	require.Equal(t, "bastion.example.com:22", e.ProxyJump)

	e2 := cfg.Lookup("db-1")
	require.Equal(t, "db-internal.example.com", e2.HostName)
	require.Equal(t, "dbadmin", e2.User)
	require.Equal(t, "yes", e2.StrictHostKeyChecking)

	e3 := cfg.Lookup("unknown-host")
	require.Equal(t, ConfigEntry{}, e3)
}

func TestParseSSHConfigPrecedence(t *testing.T) {
	// Matches real OpenSSH semantics: for each parameter, the first Host block in file
	// order that both matches and sets it wins, so specific blocks must precede
	// wildcard fallbacks to take precedence.
	const cfgText = `
Host specific
    User precise

Host *
    User fallback
`
	cfg, err := ParseSSHConfig(strings.NewReader(cfgText))
	require.NoError(t, err)

	e := cfg.Lookup("specific")
	require.Equal(t, "precise", e.User, "first matching block in file order wins")

	e2 := cfg.Lookup("other")
	require.Equal(t, "fallback", e2.User)
}

func TestLoadSSHConfigMissingFile(t *testing.T) {
	cfg, err := LoadSSHConfig("/nonexistent/path/to/config")
	require.NoError(t, err)
	require.Equal(t, ConfigEntry{}, cfg.Lookup("anything"))
}

func TestMatchesAnyNegation(t *testing.T) {
	const cfgText = `
Host web-* !web-excluded
    User deploy
`
	cfg, err := ParseSSHConfig(strings.NewReader(cfgText))
	require.NoError(t, err)

	require.Equal(t, "deploy", cfg.Lookup("web-1").User)
	require.Equal(t, "", cfg.Lookup("web-excluded").User)
}
