// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExportMarkdown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	sink, err := NewFileSink(path, nil)
	require.NoError(t, err)
	exit := 0
	require.NoError(t, sink.Write(context.Background(), Event{
		Type: EventStepFinished, Timestamp: time.Now(), RunID: "run-1",
		StepID: "step-1", Host: "web-1", Command: "systemctl restart app", ExitCode: &exit,
	}))
	require.NoError(t, sink.Write(context.Background(), Event{
		Type: EventStepFinished, Timestamp: time.Now(), RunID: "run-2",
		StepID: "step-x", Host: "web-2", Command: "echo other-run",
	}))
	require.NoError(t, sink.Close())

	var buf bytes.Buffer
	require.NoError(t, ExportMarkdown(path, "run-1", &buf))
	out := buf.String()
	require.Contains(t, out, "systemctl restart app")
	require.Contains(t, out, "run-1")
	require.NotContains(t, out, "other-run")
}

func TestExportMarkdownAllRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	sink, err := NewFileSink(path, nil)
	require.NoError(t, err)
	require.NoError(t, sink.Write(context.Background(), Event{Type: EventRunStarted, Timestamp: time.Now(), RunID: "run-1"}))
	require.NoError(t, sink.Write(context.Background(), Event{Type: EventRunStarted, Timestamp: time.Now(), RunID: "run-2"}))
	require.NoError(t, sink.Close())

	var buf bytes.Buffer
	require.NoError(t, ExportMarkdown(path, "", &buf))
	out := buf.String()
	require.Contains(t, out, "run-1")
	require.Contains(t, out, "run-2")
}

func TestMdCellEscaping(t *testing.T) {
	require.Equal(t, `a\|b`, mdCell("a|b"))
	require.Equal(t, "a b", mdCell("a\nb"))
	require.Equal(t, "", mdCell(""))
}
