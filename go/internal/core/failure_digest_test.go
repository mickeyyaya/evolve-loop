package core

// failure_digest_test.go — RED contract for the S1 failure-digest-assembler
// (cycle-1034, item failure-disposition-router slice S1).
//
// The assembler is the deterministic post-FAIL / pre-retro step that converts a
// failed cycle's forensic artifacts into a stable failure IDENTITY the S2
// disposition gate can cross-check against — closing lesson_to_action_gap (the
// agent can no longer INVENT the failure's identity in retro).
//
// SEAM CHOICE (surfaced per Core Rule 3, "no silent changes"): scout-report.md
// lists a rich input set (audit-fail-reason.json + CycleResult.FailReasons +
// phase outcomes + dossier + git state + infra signals). This contract reads the
// single workspace SSOT artifact `audit-fail-reason.json` (schema:
// {schema_version, phase, reasons[]}, already emitted by the coherence floor —
// system_failure_test.go:202) as the fingerprint/bucket source, mirroring
// readFailureDecision's workspace-file boundary. Folding the other signals into
// that one artifact keeps the input surface minimal (Rule 2) without weakening
// the identity — the Builder MAY widen the input later, but the tests below pin
// only the observable contract (bucket, determinism, recurrence, fail-soft,
// atomic write), never an internal input shape.
//
// RED today: AssembleFailureDigest / FailureDigest / RecurrenceCounter do not
// exist, so this file fails to COMPILE — the correct RED for a new-surface task.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

// writeAuditFailReason writes a workspace/audit-fail-reason.json fixture in the
// coherence-floor schema. Empty reasons + skip=true writes no file at all (the
// AC4 absent-artifact case).
func writeAuditFailReason(t *testing.T, dir, phase string, reasons ...string) {
	t.Helper()
	body := map[string]any{"schema_version": 1, "phase": phase, "reasons": reasons}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "audit-fail-reason.json"), b, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

// AC1 — pre-class buckets derived from REAL artifacts (not a hardcoded string).
// Fixtures shaped from real cycles: 1028 (EGPS red → gate-block), 999 (statemap
// guard-abort), 949 (predicate compile → verdict-fail), plus an infra fixture
// and the unknown default. Each fixture's reason keywords map to exactly one
// bucket, so the assertion is on the CLASSIFIER output, not on echoed text.
func TestAssembler_PreClassBucketsFromRealArtifacts(t *testing.T) {
	cases := []struct {
		name   string
		phase  string
		reason string
		want   string
	}{
		{"c1028_egps_red", "audit", "EGPS floor blocked ship: red_count=1 (egps-v11)", "gate-block"},
		{"c999_statemap_severed", "build", "statemap severed: guard aborted the build->audit transition", "guard-abort"},
		{"c949_predicate_compile", "audit", "ACS predicates failed to compile (predicates_test.go build error)", "verdict-fail"},
		{"infra_quota", "build", "bridge quota exhausted (85); infra teardown mid-phase", "infra-error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeAuditFailReason(t, dir, tc.phase, tc.reason)
			got, err := AssembleFailureDigest(1034, dir, emptyCounter{})
			if err != nil {
				t.Fatalf("AssembleFailureDigest returned error: %v", err)
			}
			if got.PreClass != tc.want {
				t.Errorf("pre_class = %q, want %q (reason %q)", got.PreClass, tc.want, tc.reason)
			}
		})
	}
}

// AC2 — fingerprint is STABLE and composed such that phase is load-bearing.
// Determinism: two runs over identical artifacts yield the same fingerprint (a
// random/timestamp-seeded id — the gaming fake — fails this). Composition: a run
// differing ONLY in phase yields a DIFFERENT fingerprint (phase is part of the
// gate+reason-class+phase key), so distinct failures never collapse to one id.
func TestAssembler_FingerprintComposition(t *testing.T) {
	dir := t.TempDir()
	writeAuditFailReason(t, dir, "audit", "EGPS floor blocked ship: red_count=1")

	first, err := AssembleFailureDigest(1034, dir, emptyCounter{})
	if err != nil {
		t.Fatalf("first assemble: %v", err)
	}
	second, err := AssembleFailureDigest(1034, dir, emptyCounter{})
	if err != nil {
		t.Fatalf("second assemble: %v", err)
	}
	if first.Fingerprint == "" {
		t.Fatal("fingerprint is empty for a populated artifact")
	}
	if first.Fingerprint != second.Fingerprint {
		t.Errorf("fingerprint not deterministic: %q vs %q (random/timestamp seed?)", first.Fingerprint, second.Fingerprint)
	}

	other := t.TempDir()
	writeAuditFailReason(t, other, "build", "EGPS floor blocked ship: red_count=1")
	diffPhase, err := AssembleFailureDigest(1034, other, emptyCounter{})
	if err != nil {
		t.Fatalf("diff-phase assemble: %v", err)
	}
	if diffPhase.Fingerprint == first.Fingerprint {
		t.Errorf("fingerprint ignored phase: audit and build produced the same id %q", first.Fingerprint)
	}
}

// AC3 — recurrence count is READ THROUGH the ledger, never invented. An empty
// ledger reports 0; a real recurrence.Ledger pre-seeded with the SAME fingerprint
// across two cycles (Count==2) makes the digest report recurrence=2. Using the
// real ledger (which satisfies RecurrenceCounter via Count(string) int) proves
// the assembler consults the ledger rather than fabricating the number.
func TestAssembler_RecurrenceFromLedger(t *testing.T) {
	dir := t.TempDir()
	writeAuditFailReason(t, dir, "audit", "EGPS floor blocked ship: red_count=1")

	// First pass with an empty ledger discovers the fingerprint and proves the
	// unseen-fingerprint path reports 0.
	base, err := AssembleFailureDigest(1034, dir, recurrence.NewLedger())
	if err != nil {
		t.Fatalf("base assemble: %v", err)
	}
	if base.Recurrence != 0 {
		t.Fatalf("unseen fingerprint recurrence = %d, want 0", base.Recurrence)
	}

	// Seed the real ledger with that fingerprint across two distinct cycles →
	// Count == 2, then re-assemble and require the digest to reflect it.
	led := recurrence.NewLedger()
	pol := recurrence.DefaultEscalationPolicy()
	if err := led.RecordClosure(base.Fingerprint, 1001, nil, nil, pol); err != nil {
		t.Fatalf("seed closure 1: %v", err)
	}
	if err := led.RecordClosure(base.Fingerprint, 1002, nil, nil, pol); err != nil {
		t.Fatalf("seed closure 2: %v", err)
	}
	if led.Count(base.Fingerprint) != 2 {
		t.Fatalf("ledger seeding wrong: Count=%d, want 2", led.Count(base.Fingerprint))
	}

	seeded, err := AssembleFailureDigest(1034, dir, led)
	if err != nil {
		t.Fatalf("seeded assemble: %v", err)
	}
	if seeded.Recurrence != 2 {
		t.Errorf("recurrence = %d, want 2 (must come from the ledger, not be invented)", seeded.Recurrence)
	}
}

// AC4 (negative) — malformed/absent artifacts NEVER abort. With no
// audit-fail-reason.json at all, the assembler degrades to the "unknown" bucket,
// still writes the digest, and returns no cycle-aborting error (fail-soft,
// mirroring readFailureDecision's boundary). This is the strongest anti-brittle
// signal: a genuinely novel failure must still produce a triage artifact.
func TestAssembler_MissingArtifactsDegradeToUnknown(t *testing.T) {
	dir := t.TempDir() // deliberately empty — no audit-fail-reason.json

	got, err := AssembleFailureDigest(1034, dir, emptyCounter{})
	if err != nil {
		t.Fatalf("missing artifacts must NOT return an aborting error, got: %v", err)
	}
	if got.PreClass != "unknown" {
		t.Errorf("pre_class = %q, want \"unknown\" for absent artifacts", got.PreClass)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "failure-digest.json")); statErr != nil {
		t.Errorf("failure-digest.json not written on the fail-soft path: %v", statErr)
	}
}

// AC5 (edge) — the digest is written to a real path as valid JSON carrying the
// four contract fields. t.TempDir() ONLY — never mutates the live repo tree
// (the goal invariant).
func TestAssembler_WritesDigestArtifact(t *testing.T) {
	dir := t.TempDir()
	writeAuditFailReason(t, dir, "audit", "EGPS floor blocked ship: red_count=1")

	got, err := AssembleFailureDigest(1034, dir, emptyCounter{})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	path := filepath.Join(dir, "failure-digest.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read digest: %v", err)
	}
	var onDisk FailureDigest
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("digest is not valid JSON: %v", err)
	}
	if onDisk.Cycle != 1034 {
		t.Errorf("digest cycle = %d, want 1034", onDisk.Cycle)
	}
	if onDisk.Fingerprint == "" || onDisk.PreClass == "" {
		t.Errorf("digest missing fingerprint/pre_class: %+v", onDisk)
	}
	if onDisk.Fingerprint != got.Fingerprint || onDisk.PreClass != got.PreClass {
		t.Errorf("on-disk digest %+v disagrees with returned %+v", onDisk, got)
	}
}

// emptyCounter is a RecurrenceCounter that reports every fingerprint as unseen —
// isolates the bucket/fingerprint/write ACs from ledger state.
type emptyCounter struct{}

func (emptyCounter) Count(string) int { return 0 }
