// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"bufio"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
)

// VerifyResult is the outcome of checking an audit log's hash chain (and, if a public
// key was supplied, its signatures) for tampering.
type VerifyResult struct {
	RecordCount int
	Valid       bool
	Violations  []string
}

// VerifyFile checks the audit log at path for a broken hash chain, tampered records, or
// (if publicKey is non-nil) invalid or missing signatures. It reads the whole file even
// after finding a violation, so a single call reports every problem, not just the first.
func VerifyFile(path string, publicKey ed25519.PublicKey) (*VerifyResult, error) {
	f, err := os.Open(path) //nolint:gosec // path is an operator-configured audit log location, not untrusted input
	if err != nil {
		return nil, fmt.Errorf("open audit log %q: %w", path, err)
	}
	defer f.Close()

	result := &VerifyResult{Valid: true}
	prevHash := genesisHash
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			result.Valid = false
			result.Violations = append(result.Violations, fmt.Sprintf("line %d: invalid JSON: %v", lineNo, err))
			continue
		}
		result.RecordCount++

		if rec.PrevHash != prevHash {
			result.Valid = false
			result.Violations = append(result.Violations, fmt.Sprintf("line %d (seq %d): prev_hash does not match the preceding record; the chain is broken", lineNo, rec.Seq))
		}
		if want := computeHash(rec); rec.Hash != want {
			result.Valid = false
			result.Violations = append(result.Violations, fmt.Sprintf("line %d (seq %d): hash does not match record content; the record was altered", lineNo, rec.Seq))
		}
		if publicKey != nil {
			if rec.Signature == "" {
				result.Valid = false
				result.Violations = append(result.Violations, fmt.Sprintf("line %d (seq %d): missing signature", lineNo, rec.Seq))
			} else if sig, err := base64.StdEncoding.DecodeString(rec.Signature); err != nil || !ed25519.Verify(publicKey, []byte(rec.Hash), sig) {
				result.Valid = false
				result.Violations = append(result.Violations, fmt.Sprintf("line %d (seq %d): invalid signature", lineNo, rec.Seq))
			}
		}

		prevHash = rec.Hash
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read audit log %q: %w", path, err)
	}
	return result, nil
}

// ReadAll reads every record from the audit log at path, in order. It does not verify
// the hash chain; use VerifyFile for that.
func ReadAll(path string) ([]Record, error) {
	f, err := os.Open(path) //nolint:gosec // path is an operator-configured audit log location, not untrusted input
	if err != nil {
		return nil, fmt.Errorf("open audit log %q: %w", path, err)
	}
	defer f.Close()

	var records []Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, fmt.Errorf("corrupt record: %w", err)
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read audit log %q: %w", path, err)
	}
	return records, nil
}
