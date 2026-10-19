// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// runCLI executes args against a fresh root command and returns combined stdout,
// combined stderr, and the error (if any) from Execute.
func runCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCmd()
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestCLIVersion(t *testing.T) {
	out, _, err := runCLI(t, "version")
	require.NoError(t, err)
	require.Contains(t, out, "opsrun")
}

func TestCLIValidateValidRunbook(t *testing.T) {
	path := writeTempRunbookFile(t, validRunbookMD)
	out, _, err := runCLI(t, "validate", path)
	require.NoError(t, err)
	require.Contains(t, out, "[OK] runbook is valid")
}

func TestCLIValidateInvalidRunbookExitsNonZero(t *testing.T) {
	path := writeTempRunbookFile(t, dangerousRunbookMD)
	out, _, err := runCLI(t, "validate", path)
	require.Error(t, err)
	var ece *exitCodeError
	require.ErrorAs(t, err, &ece)
	require.Equal(t, ExitValidationFailed, ece.code)
	require.Contains(t, out, "[FAIL]")
}

func TestCLIValidateMissingFile(t *testing.T) {
	_, _, err := runCLI(t, "validate", filepath.Join(t.TempDir(), "nope.md"))
	require.Error(t, err)
	var ece *exitCodeError
	require.ErrorAs(t, err, &ece)
	require.Equal(t, ExitGenericError, ece.code)
}

func TestCLIValidateJSONOutput(t *testing.T) {
	path := writeTempRunbookFile(t, dangerousRunbookMD)
	out, _, err := runCLI(t, "validate", path, "--output", "json")
	require.Error(t, err)
	require.Contains(t, out, `"severity"`)
}

func TestCLIPlanShowsHostsFromConfig(t *testing.T) {
	runbookPath := writeTempRunbookFile(t, validRunbookMD)
	cfgPath := writeTempConfigFile(t, sampleConfigYAML)

	out, _, err := runCLI(t, "plan", runbookPath, "--config", cfgPath)
	require.NoError(t, err)
	require.Contains(t, out, "Level 1:")
	require.Contains(t, out, "web-1")
	require.Contains(t, out, "echo hello")
}

func TestCLIPlanJSONOutput(t *testing.T) {
	runbookPath := writeTempRunbookFile(t, validRunbookMD)
	cfgPath := writeTempConfigFile(t, sampleConfigYAML)

	out, _, err := runCLI(t, "plan", runbookPath, "--config", cfgPath, "--output", "json")
	require.NoError(t, err)
	require.Contains(t, out, `"id": "only-step"`)
}

func TestCLIHostsText(t *testing.T) {
	cfgPath := writeTempConfigFile(t, sampleConfigYAML)
	out, _, err := runCLI(t, "hosts", "--config", cfgPath)
	require.NoError(t, err)
	require.Contains(t, out, "web-1")
	require.Contains(t, out, "web-2")
}

func TestCLIHostsNoConfig(t *testing.T) {
	out, _, err := runCLI(t, "hosts")
	require.NoError(t, err)
	require.Contains(t, out, "No hosts configured.")
}

func TestCLIAuditVerifyEmptyLog(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "audit.jsonl")
	require.NoError(t, os.WriteFile(logPath, nil, 0o600))
	out, _, err := runCLI(t, "audit", "verify", "--log", logPath)
	require.NoError(t, err)
	require.Contains(t, out, "[OK]")
}

func TestCLIAuditKeygenAndVerifyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	privPath := filepath.Join(dir, "audit.key")
	pubPath := filepath.Join(dir, "audit.pub")
	_, _, err := runCLI(t, "audit", "keygen", "--out", privPath, "--out-pub", pubPath)
	require.NoError(t, err)

	data, err := os.ReadFile(privPath)
	require.NoError(t, err)
	require.Len(t, data, 64)

	pubData, err := os.ReadFile(pubPath)
	require.NoError(t, err)
	require.Len(t, pubData, 32)
}

func TestCLIRunRequiresRunbookArg(t *testing.T) {
	_, _, err := runCLI(t, "run")
	require.Error(t, err)
}

const validRunbookMD = `---
name: cli-test
targets:
  - group: web
---

## Step: only-step - Do a thing

` + "```bash opsrun:run target=web timeout=5s\necho hello\n```" + `
`

const dangerousRunbookMD = `---
name: cli-dangerous
---

## Step: wipe - Wipe disk

` + "```bash opsrun:run target=web\nrm -rf /\n```" + `
`

const sampleConfigYAML = `
hosts:
  - name: web-1
    address: 10.0.0.1
    user: ops
    groups: [web]
  - name: web-2
    address: 10.0.0.2
    user: ops
    groups: [web]
concurrency:
  global_max_connections: 10
  per_host_max_sessions: 2
  max_parallel_steps: 5
`

func writeTempRunbookFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runbook.md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func writeTempConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opsrun.yaml")
	require.NoError(t, os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600))
	return path
}
