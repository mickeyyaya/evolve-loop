package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

// writeAuditFailReason writes a workspace/audit-fail-reason.json fixture in
// the coherence-floor schema.
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
		{"c1329_ship_repo_contract_gate", "ship",
			"repo-contract scanner pack RED in the lane worktree (exit status 1) — pushing would red main; " +
				"fix the violation in-lane (the four suites: phasespec, profiles, phasecoherence, routingtest)",
			"gate-block"},
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

func TestAssembler_RecurrenceFromLedger(t *testing.T) {
	dir := t.TempDir()
	writeAuditFailReason(t, dir, "audit", "EGPS floor blocked ship: red_count=1")

	base, err := AssembleFailureDigest(1034, dir, recurrence.NewLedger())
	if err != nil {
		t.Fatalf("base assemble: %v", err)
	}
	if base.Recurrence != 0 {
		t.Fatalf("unseen fingerprint recurrence = %d, want 0", base.Recurrence)
	}

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

func TestAssembler_MissingArtifactsDegradeToUnknown(t *testing.T) {
	dir := t.TempDir()

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

// emptyCounter is a RecurrenceCounter that reports every fingerprint as unseen.
type emptyCounter struct{}

func (emptyCounter) Count(string) int { return 0 }
