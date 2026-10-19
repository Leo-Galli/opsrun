// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"sync"
	"time"
)

// FileSink is an append-only, hash-chained JSON Lines AuditSink. Every record embeds
// the SHA-256 hash of the previous record, so the file can be verified for tampering
// with Verify. If Signer is set, every record's hash is also signed with Ed25519.
type FileSink struct {
	mu        sync.Mutex
	f         *os.File
	lastHash  string
	seq       int64
	signer    ed25519.PrivateKey
	localUser string
}

// NewFileSink opens (creating if necessary) the JSON Lines audit log at path, appending
// to any existing content and resuming the hash chain from its last record. signer may
// be nil to disable signing.
func NewFileSink(path string, signer ed25519.PrivateKey) (*FileSink, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open audit log %q: %w", path, err)
	}

	lastHash, seq, err := lastRecord(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("read existing audit log %q: %w", path, err)
	}
	if _, err := f.Seek(0, os.SEEK_END); err != nil {
		f.Close()
		return nil, fmt.Errorf("seek audit log %q: %w", path, err)
	}

	return &FileSink{
		f:         f,
		lastHash:  lastHash,
		seq:       seq,
		signer:    signer,
		localUser: currentUser(),
	}, nil
}

func currentUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "unknown"
}

// lastRecord scans f from the start and returns the hash and sequence number of its
// last record, or (genesisHash, 0) if f is empty.
func lastRecord(f *os.File) (string, int64, error) {
	if _, err := f.Seek(0, os.SEEK_SET); err != nil {
		return "", 0, err
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	hash := genesisHash
	var seq int64
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			return "", 0, fmt.Errorf("corrupt record: %w", err)
		}
		hash = r.Hash
		seq = r.Seq
	}
	if err := scanner.Err(); err != nil {
		return "", 0, err
	}
	return hash, seq, nil
}

// Write implements Sink.
func (s *FileSink) Write(_ context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	var exitCode *int
	if event.ExitCode != nil {
		ec := *event.ExitCode
		exitCode = &ec
	}
	rec := Record{
		Seq:        s.seq,
		Timestamp:  event.Timestamp.UTC().Format(time.RFC3339Nano),
		Type:       event.Type,
		RunID:      event.RunID,
		LocalUser:  firstNonEmpty(event.LocalUser, s.localUser),
		Host:       event.Host,
		StepID:     event.StepID,
		Command:    event.Command,
		Approver:   event.Approver,
		ExitCode:   exitCode,
		Duration:   event.Duration.String(),
		OutputHash: event.OutputHash,
		PrevHash:   s.lastHash,
	}
	rec.Hash = computeHash(rec)
	if s.signer != nil {
		rec.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(s.signer, []byte(rec.Hash)))
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal audit record: %w", err)
	}
	data = append(data, '\n')
	if _, err := s.f.Write(data); err != nil {
		s.seq--
		return fmt.Errorf("write audit record: %w", err)
	}
	if err := s.f.Sync(); err != nil {
		return fmt.Errorf("sync audit log: %w", err)
	}
	s.lastHash = rec.Hash
	return nil
}

// Close implements Sink.
func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.f.Close()
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
