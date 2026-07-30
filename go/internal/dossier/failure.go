package dossier

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// failure.go — FAIL-cycle failure-identity ingestion (dossier-carries-failure-
// reason). A FAIL dossier used to be content-free ("see audit-report.md"): the
// knowledge base recorded THAT a cycle failed but not WHY, so convergence
// briefs and cross-batch forensics needed workspace archaeology in gitignored
// runtime. By dossier time the identity already exists on disk — the coherence
// floor / core.ensureFailureDigest write audit-fail-reason.json and
// core.AssembleFailureDigest writes failure-digest.json — so Build projects
// them into the committed record. Ingestion is best-effort PER ARTIFACT
// (mirrors ciWatchRecord): an absent/malformed file shrinks the block, never
// fails the write, and nothing is ever fabricated.

// Workspace artifact names. Unexported: the writers live in internal/core and
// name the files themselves; the dossier only reads them.
const (
	auditFailReasonFile = "audit-fail-reason.json"
	failureDigestFile   = "failure-digest.json"
)

// Reason bounds: the dossier carries a legible head of the evidence, not the
// whole forensic trail (which stays in the workspace artifacts) — so a
// pathological reason set can never bloat the committed knowledge base. The
// writers of both artifacts live in internal/core (which imports this package,
// so the read cannot be shared — no import cycle available).
const (
	maxFailureReasons = 5
	// BYTES, not runes (len(r)): a multibyte reason carries proportionally
	// fewer characters. Named for what it measures (review LOW).
	maxFailureReasonBytes = 200
)

// FailureRecord is the failure identity of a FAIL cycle: the stable digest
// fingerprint + pre-class bucket (failure-digest.json) plus the verbatim-head
// reasons (audit-fail-reason.json). Every field is best-effort — present only
// when its source artifact yielded content.
type FailureRecord struct {
	Fingerprint string   `json:"fingerprint,omitempty"`
	PreClass    string   `json:"pre_class,omitempty"`
	Reasons     []string `json:"reasons,omitempty"`
}

// failureRecord reads the failure identity from the workspace artifacts.
// Returns ok=false when neither artifact yields content, so Build leaves
// Dossier.Failure nil rather than committing an empty block.
//
// cycle guards the digest: its own `cycle` field must match, or the digest is
// a STALE leftover (this cycle's own write WARNed) and adopting it would
// commit a FALSE forensic identity — strictly worse than an absent one. Same
// distrust core.EvaluateBlockerBreaker applies when it filters digests by
// their cycle field rather than trusting the containing path (review MEDIUM).
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

// failureReasons reads audit-fail-reason.json's reasons[], dropping blanks and
// bounding the carried evidence to maxFailureReasons head-truncated entries.
// Absent/malformed artifact ⇒ nil (fail-soft, mirrors core.readAuditFailReason).
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
		// Collapse ALL whitespace runs to single spaces FIRST: reasons are built
		// from test-output excerpts, and a newline-bearing reason rendered into
		// the md bullet list corrupts the committed record (a "## " line inside
		// a reason becomes a fake heading the retro reads). Collapsing also
		// makes the byte bound meaningful (review MEDIUM).
		r = strings.Join(strings.Fields(r), " ")
		if r == "" {
			continue
		}
		if len(r) > maxFailureReasonBytes {
			// Head-kept, UTF-8-safe cut (same idiom as core.defectHead): the
			// defect-identifying content of a reason leads; the tail is chrome.
			r = strings.ToValidUTF8(r[:maxFailureReasonBytes], "")
		}
		out = append(out, r)
		if len(out) == maxFailureReasons {
			break
		}
	}
	return out
}
