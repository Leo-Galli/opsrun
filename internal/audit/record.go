// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// genesisHash is the PrevHash of the first record in a log: 64 hex zeros, the same
// length as a real SHA-256 hash.
var genesisHash = strings.Repeat("0", sha256.Size*2)

// Record is the on-disk, hash-chained representation of one Event. Each record embeds
// the SHA-256 hash of the previous record (PrevHash) and its own hash (Hash), computed
// over every other field, so altering or reordering any past record is detectable by
// recomputing the chain (see Verify).
type Record struct {
	Seq        int64     `json:"seq"`
	Timestamp  string    `json:"timestamp"` // RFC 3339, UTC
	Type       EventType `json:"type"`
	RunID      string    `json:"run_id"`
	LocalUser  string    `json:"local_user,omitempty"`
	Host       string    `json:"host,omitempty"`
	StepID     string    `json:"step_id,omitempty"`
	Command    string    `json:"command,omitempty"`
	Approver   string    `json:"approver,omitempty"`
	ExitCode   *int      `json:"exit_code,omitempty"`
	Duration   string    `json:"duration,omitempty"`
	OutputHash string    `json:"output_hash,omitempty"`
	PrevHash   string    `json:"prev_hash"`
	Hash       string    `json:"hash"`
	Signature  string    `json:"signature,omitempty"` // base64 Ed25519 signature over Hash
}

// hashInput is the subset of Record fields that feed the hash, in a fixed field order so
// that json.Marshal produces identical bytes for identical records.
type hashInput struct {
	Seq        int64     `json:"seq"`
	Timestamp  string    `json:"timestamp"`
	Type       EventType `json:"type"`
	RunID      string    `json:"run_id"`
	LocalUser  string    `json:"local_user,omitempty"`
	Host       string    `json:"host,omitempty"`
	StepID     string    `json:"step_id,omitempty"`
	Command    string    `json:"command,omitempty"`
	Approver   string    `json:"approver,omitempty"`
	ExitCode   *int      `json:"exit_code,omitempty"`
	Duration   string    `json:"duration,omitempty"`
	OutputHash string    `json:"output_hash,omitempty"`
	PrevHash   string    `json:"prev_hash"`
}

// computeHash returns the hex-encoded SHA-256 hash of r's content fields and PrevHash.
// It never panics: hashInput contains only JSON-marshalable types.
func computeHash(r Record) string {
	in := hashInput{
		Seq: r.Seq, Timestamp: r.Timestamp, Type: r.Type, RunID: r.RunID,
		LocalUser: r.LocalUser, Host: r.Host, StepID: r.StepID, Command: r.Command,
		Approver: r.Approver, ExitCode: r.ExitCode, Duration: r.Duration,
		OutputHash: r.OutputHash, PrevHash: r.PrevHash,
	}
	data, _ := json.Marshal(in)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
