// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// FuzzParse feeds arbitrary bytes into the parser and asserts only that it never panics
// and never hangs; a non-nil error is an entirely acceptable outcome for malformed input.
func FuzzParse(f *testing.F) {
	for _, dir := range []string{
		"../../examples/runbooks",
		"../../testdata/parser/valid",
		"../../testdata/parser/invalid",
		"../../testdata/parser/parseerr",
	} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) != ".md" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err == nil {
				f.Add(data)
			}
		}
	}
	f.Add([]byte(""))
	f.Add([]byte("---\n---\n"))
	f.Add([]byte("## Step: a - b\n"))

	p := NewMarkdownParser()
	f.Fuzz(func(t *testing.T, data []byte) {
		rb, err := p.Parse(context.Background(), bytes.NewReader(data))
		if err != nil {
			return
		}
		if rb == nil {
			t.Fatal("Parse returned nil runbook with nil error")
		}
	})
}
