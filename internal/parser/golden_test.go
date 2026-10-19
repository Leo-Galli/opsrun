// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Leo-Galli/opsrun/internal/runbook"
)

var updateGolden = flag.Bool("update", false, "update golden files")

func summarizeRunbook(rb *runbook.Runbook) string {
	var b strings.Builder
	fmt.Fprintf(&b, "name=%s version=%s service=%s severity=%s author=%s\n", rb.Name, rb.Version, rb.Service, rb.Severity, rb.Author)

	targets := make([]string, 0, len(rb.Targets))
	for _, t := range rb.Targets {
		if t.Group != "" {
			targets = append(targets, "group:"+t.Group)
		} else {
			targets = append(targets, "host:"+t.Host)
		}
	}
	fmt.Fprintf(&b, "targets=%v\n", targets)

	varNames := make([]string, 0, len(rb.Vars))
	for name := range rb.Vars {
		varNames = append(varNames, name)
	}
	sort.Strings(varNames)
	for _, name := range varNames {
		v := rb.Vars[name]
		fmt.Fprintf(&b, "var %s type=%s default=%v required=%v\n", name, v.Type, v.Default, v.Required)
	}

	fmt.Fprintf(&b, "approval mode=%s timeout=%s\n", rb.Approval.Mode, rb.Approval.Timeout)

	for _, s := range rb.Steps {
		fmt.Fprintf(&b, "step id=%s title=%q depends_on=%v confirm=%v retries=%d continue_on_error=%v mode=%s read_only=%v\n",
			s.ID, s.Title, s.DependsOn, s.Confirm, s.Retries, s.ContinueOnError, s.Mode, s.ReadOnly)
		if s.Approval != nil {
			fmt.Fprintf(&b, "  approval_override mode=%s timeout=%s\n", s.Approval.Mode, s.Approval.Timeout)
		}
		if s.Run != nil {
			fmt.Fprintf(&b, "  run target=%s timeout=%s command=%q\n", s.Run.Target, s.Run.Timeout, s.Run.Command)
		}
		for _, c := range s.Checks {
			fmt.Fprintf(&b, "  check expect_exit=%d command=%q\n", c.ExpectExit, c.Command)
		}
		if s.Rollback != nil {
			fmt.Fprintf(&b, "  rollback target=%s timeout=%s command=%q\n", s.Rollback.Target, s.Rollback.Timeout, s.Rollback.Command)
		}
	}
	return b.String()
}

func summarizeValidation(errs []ValidationError) string {
	var b strings.Builder
	for _, e := range errs {
		fmt.Fprintf(&b, "%s step=%s msg=%s\n", e.Severity, e.StepID, e.Message)
	}
	return b.String()
}

func checkGolden(t *testing.T, path string, got string) {
	t.Helper()
	if *updateGolden {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "golden file missing; run with -update to create it")
	require.Equal(t, string(want), got)
}

func TestGoldenValid(t *testing.T) {
	dir := "../../testdata/parser/valid"
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".md" {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			f, err := os.Open(filepath.Join(dir, e.Name()))
			require.NoError(t, err)
			defer f.Close()

			rb, err := NewMarkdownParser().Parse(context.Background(), f)
			require.NoError(t, err)

			v, err := NewValidator(nil)
			require.NoError(t, err)
			errs := v.Validate(rb)
			for _, ve := range errs {
				require.NotEqual(t, SeverityError, ve.Severity, "unexpected validation error: %s", ve.Message)
			}

			golden := filepath.Join(dir, strings.TrimSuffix(e.Name(), ".md")+".golden")
			checkGolden(t, golden, summarizeRunbook(rb))
		})
	}
}

func TestGoldenInvalid(t *testing.T) {
	dir := "../../testdata/parser/invalid"
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".md" {
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			f, err := os.Open(filepath.Join(dir, e.Name()))
			require.NoError(t, err)
			defer f.Close()

			rb, err := NewMarkdownParser().Parse(context.Background(), f)
			require.NoError(t, err, "fixture must parse successfully; validation errors are checked separately")

			v, err := NewValidator(nil)
			require.NoError(t, err)
			errs := v.Validate(rb)
			require.NotEmpty(t, errs)

			golden := filepath.Join(dir, strings.TrimSuffix(e.Name(), ".md")+".golden")
			checkGolden(t, golden, summarizeValidation(errs))
		})
	}
}

func TestParseErrors(t *testing.T) {
	dir := "../../testdata/parser/parseerr"
	tests := []struct {
		file      string
		substring string
	}{
		{"no-frontmatter.md", "front matter"},
		{"unterminated-frontmatter.md", "no closing delimiter"},
		{"unknown-frontmatter-field.md", "not_a_real_field"},
		{"bad-directive-type.md", "invalid directive block"},
		{"unknown-action-kind.md", "unknown action kind"},
		{"bad-timeout.md", "invalid timeout"},
		{"malformed-attribute.md", "malformed code block attribute"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			f, err := os.Open(filepath.Join(dir, tt.file))
			require.NoError(t, err)
			defer f.Close()

			_, err = NewMarkdownParser().Parse(context.Background(), f)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.substring)
		})
	}
}
