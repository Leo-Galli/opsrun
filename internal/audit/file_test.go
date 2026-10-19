// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func writeSample(t *testing.T, sink *FileSink, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		exit := 0
		err := sink.Write(context.Background(), Event{
			Type: EventStepFinished, Timestamp: time.Now(), RunID: "run-1",
			StepID: "step", Host: "host", Command: "echo hi", ExitCode: &exit,
		})
		require.NoError(t, err)
	}
}

func TestFileSinkWriteAndChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	sink, err := NewFileSink(path, nil)
	require.NoError(t, err)
	writeSample(t, sink, 3)
	require.NoError(t, sink.Close())

	records, err := ReadAll(path)
	require.NoError(t, err)
	require.Len(t, records, 3)
	require.Equal(t, genesisHash, records[0].PrevHash)
	require.Equal(t, records[0].Hash, records[1].PrevHash)
	require.Equal(t, records[1].Hash, records[2].PrevHash)
	require.Equal(t, int64(1), records[0].Seq)
	require.Equal(t, int64(3), records[2].Seq)
}

func TestFileSinkResumesChainAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	sink1, err := NewFileSink(path, nil)
	require.NoError(t, err)
	writeSample(t, sink1, 2)
	require.NoError(t, sink1.Close())

	sink2, err := NewFileSink(path, nil)
	require.NoError(t, err)
	writeSample(t, sink2, 1)
	require.NoError(t, sink2.Close())

	records, err := ReadAll(path)
	require.NoError(t, err)
	require.Len(t, records, 3)
	require.Equal(t, int64(3), records[2].Seq)
	require.Equal(t, records[1].Hash, records[2].PrevHash)
}

func TestVerifyFileValid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	sink, err := NewFileSink(path, nil)
	require.NoError(t, err)
	writeSample(t, sink, 5)
	require.NoError(t, sink.Close())

	result, err := VerifyFile(path, nil)
	require.NoError(t, err)
	require.True(t, result.Valid)
	require.Empty(t, result.Violations)
	require.Equal(t, 5, result.RecordCount)
}

func TestVerifyFileDetectsTamperedField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	sink, err := NewFileSink(path, nil)
	require.NoError(t, err)
	writeSample(t, sink, 3)
	require.NoError(t, sink.Close())

	tamperLine(t, path, 1, func(r *Record) { r.Command = "rm -rf /" })

	result, err := VerifyFile(path, nil)
	require.NoError(t, err)
	require.False(t, result.Valid)
	require.NotEmpty(t, result.Violations)
}

func TestVerifyFileDetectsBrokenChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	sink, err := NewFileSink(path, nil)
	require.NoError(t, err)
	writeSample(t, sink, 3)
	require.NoError(t, sink.Close())

	tamperLine(t, path, 2, func(r *Record) { r.PrevHash = "deadbeef" })

	result, err := VerifyFile(path, nil)
	require.NoError(t, err)
	require.False(t, result.Valid)
}

func TestVerifyFileDetectsDeletedRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	sink, err := NewFileSink(path, nil)
	require.NoError(t, err)
	writeSample(t, sink, 3)
	require.NoError(t, sink.Close())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	require.Len(t, lines, 3)
	remaining := lines[0] + "\n" + lines[2] + "\n"
	require.NoError(t, os.WriteFile(path, []byte(remaining), 0o600))

	result, err := VerifyFile(path, nil)
	require.NoError(t, err)
	require.False(t, result.Valid)
}

func TestFileSinkWithSigning(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "audit.jsonl")
	sink, err := NewFileSink(path, priv)
	require.NoError(t, err)
	writeSample(t, sink, 2)
	require.NoError(t, sink.Close())

	result, err := VerifyFile(path, pub)
	require.NoError(t, err)
	require.True(t, result.Valid)

	records, err := ReadAll(path)
	require.NoError(t, err)
	for _, r := range records {
		require.NotEmpty(t, r.Signature)
	}
}

func TestVerifyFileDetectsBadSignature(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	otherPub, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "audit.jsonl")
	sink, err := NewFileSink(path, priv)
	require.NoError(t, err)
	writeSample(t, sink, 2)
	require.NoError(t, sink.Close())

	result, err := VerifyFile(path, otherPub)
	require.NoError(t, err)
	require.False(t, result.Valid)
}

func TestVerifyFileRequiresSignatureWhenPublicKeyGiven(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "audit.jsonl")
	sink, err := NewFileSink(path, nil) // unsigned
	require.NoError(t, err)
	writeSample(t, sink, 1)
	require.NoError(t, sink.Close())

	result, err := VerifyFile(path, pub)
	require.NoError(t, err)
	require.False(t, result.Valid)
}

func TestVerifyFileMissing(t *testing.T) {
	_, err := VerifyFile(filepath.Join(t.TempDir(), "nope.jsonl"), nil)
	require.Error(t, err)
}

// tamperLine rewrites the n-th (1-based) line of an audit log, mutating its decoded
// Record with mutate before re-encoding, without recomputing Hash -- simulating an
// attacker editing the file directly.
func tamperLine(t *testing.T, path string, n int, mutate func(*Record)) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	require.GreaterOrEqual(t, len(lines), n)

	var rec Record
	require.NoError(t, json.Unmarshal([]byte(lines[n-1]), &rec))
	mutate(&rec)
	newLine, err := json.Marshal(rec)
	require.NoError(t, err)
	lines[n-1] = string(newLine)

	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
}
