package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Failure artifacts written by internal/core; the dossier only reads them.
const (
	auditFailReasonFile = "audit-fail-reason.json"
	failureDigestFile   = "failure-digest.json"
)

// The committed record carries a legible head of the reasons; the full trail
// stays in the workspace artifacts.
const (
	maxFailureReasons     = 5
	maxFailureReasonBytes = 200
)

// FailureRecord is a FAIL cycle's failure identity: the digest fingerprint and
// pre-class plus the head of the audit reasons. Each field is best-effort.
type FailureRecord struct {
	Fingerprint string   `json:"fingerprint,omitempty"`
	PreClass    string   `json:"pre_class,omitempty"`
	Reasons     []string `json:"reasons,omitempty"`
}

// failureRecord reads the failure identity; ok is false when neither artifact
// yields content. A digest whose cycle field differs is a stale leftover and is
// ignored, because a false identity is worse than none.
func failureRecord(workspace string, cycle int) (*FailureRecord, bool) {
	rec := &FailureRecord{Reasons: failureReasons(workspace)}
	if raw, err := os.ReadFile(filepath.Join(workspace, failureDigestFile)); err == nil {
		var d struct {
			Cycle       int    `json:"cycle"`
			Fingerprint string `json:"fingerprint"`
			PreClass    string `json:"pre_class"`
		}
		if json.Unmarshal(raw, &d) == nil && d.Cycle == cycle {
			rec.Fingerprint = strings.TrimSpace(d.Fingerprint)
			rec.PreClass = strings.TrimSpace(d.PreClass)
		}
	}
	if rec.Fingerprint == "" && rec.PreClass == "" && len(rec.Reasons) == 0 {
		return nil, false
	}
	return rec, true
}

// failureReasons returns audit-fail-reason.json's non-blank reasons, bounded to
// maxFailureReasons head-truncated entries; nil when the artifact is unusable.
func failureReasons(workspace string) []string {
	raw, err := os.ReadFile(filepath.Join(workspace, auditFailReasonFile))
	if err != nil {
		return nil
	}
	var a struct {
		Reasons []string `json:"reasons"`
	}
	if json.Unmarshal(raw, &a) != nil {
		return nil
	}
	var out []string
	for _, r := range a.Reasons {
		// A newline in a reason would put a fake "## " heading into the
		// markdown record, so whitespace collapses before the byte bound.
		r = strings.Join(strings.Fields(r), " ")
		if r == "" {
			continue
		}
		if len(r) > maxFailureReasonBytes {
			// Keep the head, where a reason's identifying content leads, and
			// drop any rune the byte cut split.
			r = strings.ToValidUTF8(r[:maxFailureReasonBytes], "")
		}
		out = append(out, r)
		if len(out) == maxFailureReasons {
			break
		}
	}
	return out
}
