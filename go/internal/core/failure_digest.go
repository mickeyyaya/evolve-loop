package core

// failure_digest.go — S1 failure-digest-assembler (cycle-1034, item
// failure-disposition-router). The deterministic post-FAIL / pre-retro step that
// converts a failed cycle's forensic artifacts into a STABLE failure identity the
// S2 disposition gate cross-checks against, so the retro agent can no longer
// INVENT the failure's identity (closes lesson_to_action_gap).
//
// SEAM (Core Rule 3): the fingerprint/bucket source is the single workspace SSOT
// artifact audit-fail-reason.json ({schema_version, phase, reasons[]}, emitted by
// the coherence floor), mirroring readFailureDecision's workspace-file boundary.
// Reading it is fail-SOFT: an absent/malformed artifact degrades to the "unknown"
// bucket and STILL writes a digest — a genuinely novel failure must always yield a
// triage artifact. Only a real write IO failure is returned as an error.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FailureDigest is the stable failure identity written to
// <workspace>/failure-digest.json and cross-checked by VerifyDisposition.
type FailureDigest struct {
	Cycle       int    `json:"cycle"`
	Fingerprint string `json:"fingerprint"`
	PreClass    string `json:"pre_class"`
	Recurrence  int    `json:"recurrence"`
}

// RecurrenceCounter is the minimal read view of the recurrence ledger the
// assembler consults (satisfied by *recurrence.Ledger via Count(string) int). The
// count is READ THROUGH this seam, never fabricated.
type RecurrenceCounter interface {
	Count(string) int
}

// auditFailReason mirrors the coherence-floor schema of audit-fail-reason.json.
type auditFailReason struct {
	SchemaVersion int      `json:"schema_version"`
	Phase         string   `json:"phase"`
	Reasons       []string `json:"reasons"`
}

// preClassRule maps a set of lowercase keyword needles to a pre-class bucket.
// Rules are evaluated in order; the FIRST rule with any matching needle wins, so
// the ordering encodes precedence when a reason could touch several classes.
type preClassRule struct {
	bucket  string
	needles []string
}

// preClassRules classify a failure reason into a coarse bucket from REAL reason
// text (not a hardcoded echo). Order = precedence: infra teardown is checked
// before the gate/verdict buckets because an infra-severed phase can also mention
// a floor.
var preClassRules = []preClassRule{
	{"infra-error", []string{"infra teardown", "quota", "bridge", "teardown"}},
	{"guard-abort", []string{"statemap severed", "guard aborted", "guard abort", "statemap"}},
	{"gate-block", []string{"egps", "floor blocked", "red_count", "gate block", "blocked ship"}},
	{"verdict-fail", []string{"failed to compile", "predicate", "verdict", "acs"}},
}

// classifyPreClass returns the bucket for a joined, lowercased reason string, or
// "unknown" when no rule matches (the fail-soft / novel-failure default).
func classifyPreClass(reasonLower string) string {
	for _, rule := range preClassRules {
		for _, n := range rule.needles {
			if strings.Contains(reasonLower, n) {
				return rule.bucket
			}
		}
	}
	return "unknown"
}

// AssembleFailureDigest reads <workspace>/audit-fail-reason.json, derives a stable
// phase-composed fingerprint and pre-class bucket, reads the recurrence count
// through rc, writes the digest atomically, and returns it. Reading the artifact
// is fail-soft (absent/malformed → "unknown", no abort); only a write failure is
// returned as an error.
func AssembleFailureDigest(cycle int, workspace string, rc RecurrenceCounter) (FailureDigest, error) {
	phase, reasons := readAuditFailReason(workspace)
	joined := strings.ToLower(strings.Join(reasons, "\n"))
	preClass := classifyPreClass(joined)

	digest := FailureDigest{
		Cycle:       cycle,
		Fingerprint: fingerprint(phase, preClass, reasons),
		PreClass:    preClass,
	}
	if rc != nil {
		digest.Recurrence = rc.Count(digest.Fingerprint)
	}

	b, err := json.Marshal(digest)
	if err != nil {
		return digest, fmt.Errorf("marshal failure digest: %w", err)
	}
	if err := writeArtifactAtomically(filepath.Join(workspace, "failure-digest.json"), b); err != nil {
		return digest, fmt.Errorf("write failure-digest.json: %w", err)
	}
	return digest, nil
}

// readAuditFailReason returns the phase and reasons from the workspace SSOT
// artifact. Absent or malformed → ("", nil) so the caller degrades to "unknown"
// rather than aborting (fail-soft boundary, mirrors readFailureDecision).
func readAuditFailReason(workspace string) (phase string, reasons []string) {
	raw, err := os.ReadFile(filepath.Join(workspace, "audit-fail-reason.json"))
	if err != nil {
		return "", nil
	}
	var a auditFailReason
	if json.Unmarshal(raw, &a) != nil {
		return "", nil
	}
	return a.Phase, a.Reasons
}

// fingerprint composes a DETERMINISTIC, phase-load-bearing identity:
// "<phase>|<preClass>|<hash>" where the hash also folds in phase+preClass+reasons.
// Phase is both a prefix and a hash input, so two failures differing only in phase
// never collapse to one id. No timestamp/random seed — identical artifacts always
// yield the identical fingerprint.
func fingerprint(phase, preClass string, reasons []string) string {
	sum := sha256.Sum256([]byte(phase + "\x00" + preClass + "\x00" + strings.Join(reasons, "\x00")))
	return phase + "|" + preClass + "|" + hex.EncodeToString(sum[:])[:12]
}
