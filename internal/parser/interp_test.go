// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Leo-Galli/opsrun/internal/runbook"
)

func TestExtractVarRefs(t *testing.T) {
	refs := ExtractVarRefs("echo {{ .Vars.a }} {{.Vars.b}} {{ .Vars.a }}")
	require.Equal(t, []string{"a", "b"}, refs)
	require.Empty(t, ExtractVarRefs("echo hello"))
}

func TestInterpolate(t *testing.T) {
	declared := map[string]runbook.Variable{
		"name":     {Name: "name", Type: runbook.VariableTypeString},
		"count":    {Name: "count", Type: runbook.VariableTypeInt, Default: 5},
		"enabled":  {Name: "enabled", Type: runbook.VariableTypeBool, Default: true},
		"timeout":  {Name: "timeout", Type: runbook.VariableTypeDuration, Default: "30s"},
		"required": {Name: "required", Type: runbook.VariableTypeString, Required: true},
	}

	out, err := Interpolate("echo {{ .Vars.name }}", declared, map[string]any{"name": "it's a test"})
	require.NoError(t, err)
	require.Equal(t, `echo 'it'\''s a test'`, out)

	out, err = Interpolate("count={{ .Vars.count }}", declared, nil)
	require.NoError(t, err)
	require.Equal(t, "count='5'", out)

	out, err = Interpolate("count={{ .Vars.count }}", declared, map[string]any{"count": 42})
	require.NoError(t, err)
	require.Equal(t, "count='42'", out)

	out, err = Interpolate("count={{ .Vars.count }}", declared, map[string]any{"count": int64(7)})
	require.NoError(t, err)
	require.Equal(t, "count='7'", out)

	out, err = Interpolate("count={{ .Vars.count }}", declared, map[string]any{"count": float64(9)})
	require.NoError(t, err)
	require.Equal(t, "count='9'", out)

	out, err = Interpolate("count={{ .Vars.count }}", declared, map[string]any{"count": "11"})
	require.NoError(t, err)
	require.Equal(t, "count='11'", out)

	_, err = Interpolate("count={{ .Vars.count }}", declared, map[string]any{"count": "nope"})
	require.Error(t, err)

	_, err = Interpolate("count={{ .Vars.count }}", declared, map[string]any{"count": true})
	require.Error(t, err)

	out, err = Interpolate("enabled={{ .Vars.enabled }}", declared, nil)
	require.NoError(t, err)
	require.Equal(t, "enabled='true'", out)

	out, err = Interpolate("enabled={{ .Vars.enabled }}", declared, map[string]any{"enabled": "false"})
	require.NoError(t, err)
	require.Equal(t, "enabled='false'", out)

	_, err = Interpolate("enabled={{ .Vars.enabled }}", declared, map[string]any{"enabled": "notabool"})
	require.Error(t, err)

	_, err = Interpolate("enabled={{ .Vars.enabled }}", declared, map[string]any{"enabled": 1})
	require.Error(t, err)

	out, err = Interpolate("timeout={{ .Vars.timeout }}", declared, nil)
	require.NoError(t, err)
	require.Equal(t, "timeout='30s'", out)

	_, err = Interpolate("timeout={{ .Vars.timeout }}", declared, map[string]any{"timeout": "notaduration"})
	require.Error(t, err)

	_, err = Interpolate("timeout={{ .Vars.timeout }}", declared, map[string]any{"timeout": 5})
	require.Error(t, err)

	_, err = Interpolate("x={{ .Vars.nope }}", declared, nil)
	require.Error(t, err)

	_, err = Interpolate("x={{ .Vars.required }}", declared, nil)
	require.Error(t, err)

	out, err = Interpolate("plain text, no vars", declared, nil)
	require.NoError(t, err)
	require.Equal(t, "plain text, no vars", out)
}

func TestInterpolateRedacted(t *testing.T) {
	declared := map[string]runbook.Variable{
		"password": {Name: "password", Type: runbook.VariableTypeString, Secret: true},
		"name":     {Name: "name", Type: runbook.VariableTypeString},
	}
	out, err := InterpolateRedacted("login --user {{ .Vars.name }} --pass {{ .Vars.password }}", declared, map[string]any{
		"name":     "alice",
		"password": "hunter2",
	})
	require.NoError(t, err)
	require.Equal(t, "login --user 'alice' --pass '***REDACTED***'", out)
	require.NotContains(t, out, "hunter2")

	real, err := Interpolate("login --user {{ .Vars.name }} --pass {{ .Vars.password }}", declared, map[string]any{
		"name":     "alice",
		"password": "hunter2",
	})
	require.NoError(t, err)
	require.Contains(t, real, "hunter2")
}

func TestShellEscape(t *testing.T) {
	require.Equal(t, "'hello'", shellEscape("hello"))
	require.Equal(t, `'it'\''s'`, shellEscape("it's"))
}
