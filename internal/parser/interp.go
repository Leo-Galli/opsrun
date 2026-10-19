// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Leo-Galli/opsrun/internal/runbook"
)

var varRefPattern = regexp.MustCompile(`\{\{\s*\.Vars\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

// ExtractVarRefs returns the distinct variable names referenced in command text via the
// {{ .Vars.name }} syntax, in order of first appearance.
func ExtractVarRefs(command string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, m := range varRefPattern.FindAllStringSubmatch(command, -1) {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// Interpolate replaces every {{ .Vars.name }} reference in command with its resolved,
// shell-escaped value. It returns an error if a reference is not declared in declared,
// or if a declared, required variable has no value and no default.
func Interpolate(command string, declared map[string]runbook.Variable, values map[string]any) (string, error) {
	var firstErr error
	result := varRefPattern.ReplaceAllStringFunc(command, func(match string) string {
		if firstErr != nil {
			return match
		}
		name := varRefPattern.FindStringSubmatch(match)[1]
		v, ok := declared[name]
		if !ok {
			firstErr = fmt.Errorf("undeclared variable %q", name)
			return match
		}
		val, ok := values[name]
		if !ok {
			val = v.Default
		}
		if val == nil {
			if v.Required {
				firstErr = fmt.Errorf("missing required variable %q", name)
				return match
			}
			val = ""
		}
		str, err := formatValue(v.Type, val)
		if err != nil {
			firstErr = fmt.Errorf("variable %q: %w", name, err)
			return match
		}
		return shellEscape(str)
	})
	if firstErr != nil {
		return "", firstErr
	}
	return result, nil
}

func formatValue(t runbook.VariableType, val any) (string, error) {
	switch t {
	case runbook.VariableTypeInt:
		switch n := val.(type) {
		case int:
			return strconv.Itoa(n), nil
		case int64:
			return strconv.FormatInt(n, 10), nil
		case float64:
			return strconv.Itoa(int(n)), nil
		case string:
			if _, err := strconv.Atoi(n); err != nil {
				return "", fmt.Errorf("invalid int value %q", n)
			}
			return n, nil
		default:
			return "", fmt.Errorf("invalid int value %v", val)
		}
	case runbook.VariableTypeBool:
		switch b := val.(type) {
		case bool:
			return strconv.FormatBool(b), nil
		case string:
			parsed, err := strconv.ParseBool(b)
			if err != nil {
				return "", fmt.Errorf("invalid bool value %q", b)
			}
			return strconv.FormatBool(parsed), nil
		default:
			return "", fmt.Errorf("invalid bool value %v", val)
		}
	case runbook.VariableTypeDuration:
		switch d := val.(type) {
		case string:
			if _, err := time.ParseDuration(d); err != nil {
				return "", fmt.Errorf("invalid duration value %q", d)
			}
			return d, nil
		case time.Duration:
			return d.String(), nil
		default:
			return "", fmt.Errorf("invalid duration value %v", val)
		}
	default: // VariableTypeString and unset
		return fmt.Sprintf("%v", val), nil
	}
}

// shellEscape wraps s in single quotes, safe for POSIX shells, escaping any embedded
// single quotes.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
