// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExampleRunbooksParseAndValidate(t *testing.T) {
	dir := "../../examples/runbooks"
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(entries), 3, "expected at least 3 example runbooks")

	v, err := NewValidator(nil)
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
			require.NotEmpty(t, rb.Steps)

			for _, ve := range v.Validate(rb) {
				require.NotEqual(t, SeverityError, ve.Severity, "unexpected validation error in %s: %s", e.Name(), ve.Message)
			}
		})
	}
}
