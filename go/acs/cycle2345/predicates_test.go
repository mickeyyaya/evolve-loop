//go:build acs

package cycle2345

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func digestFingerprint(t *testing.T, phase, reason string) string {
	t.Helper()
	workspace := t.TempDir()
	body, err := json.Marshal(map[string]any{"schema_version": 1, "phase": phase, "reasons": []string{reason}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "audit-fail-reason.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := core.AssembleFailureDigest(1, workspace, nil)
	if err != nil {
		t.Fatalf("AssembleFailureDigest: %v", err)
	}
	return digest.Fingerprint
}

func requireSameFingerprint(t *testing.T, phase, a, b string) {
	t.Helper()
	fa, fb := digestFingerprint(t, phase, a), digestFingerprint(t, phase, b)
	if fa != fb {
		t.Errorf("RED: the two reasons differ only in a per-cycle token but get two fingerprints\n a=%q -> %s\n b=%q -> %s", a, fa, b, fb)
	}
}

func requireDistinctFingerprint(t *testing.T, phase, a, b string) {
	t.Helper()
	fa, fb := digestFingerprint(t, phase, a), digestFingerprint(t, phase, b)
	if fa == fb {
		t.Errorf("RED: two different defects collapse into one fingerprint %s\n a=%q\n b=%q", fa, a, b)
	}
}

func TestC2345_001_CycleAssignFoldsAcrossCycles(t *testing.T) {
	a := "phase scout: bridge: launch exit=81: artifact-timeout: phase=scout cycle=2071 driver=claude-tmux artifact=\"scout-report.md\""
	b := "phase scout: bridge: launch exit=81: artifact-timeout: phase=scout cycle=2072 driver=claude-tmux artifact=\"scout-report.md\""
	requireSameFingerprint(t, "scout", a, b)
}

func TestC2345_002_CycleULIDSuffixFoldsAcrossCycles(t *testing.T) {
	a := "build handoff floor: [ste-lint] the changed documents fail the STE lint (first: docs/explain/builds/cycle-2049-01m5xtvnwk94g9ryfgzhqre9m0.md:8 STE-SENTENCE)"
	b := "build handoff floor: [ste-lint] the changed documents fail the STE lint (first: docs/explain/builds/cycle-2051-01m5xwqdjqyc9mhva9bxdvs52e.md:8 STE-SENTENCE)"
	requireSameFingerprint(t, "build", a, b)
}

func TestC2345_003_FindingCountFolds(t *testing.T) {
	a := "build handoff floor: [ste-lint] the changed documents fail the STE lint: 1 finding(s) in 1 file(s)"
	b := "build handoff floor: [ste-lint] the changed documents fail the STE lint: 2 finding(s) in 1 file(s)"
	requireSameFingerprint(t, "build", a, b)
	requireSameFingerprint(t, "build", "audit reported 12 findings in the diff", "audit reported 3 findings in the diff")
	requireSameFingerprint(t, "build", "audit reported 1 finding in the diff", "audit reported 7 findings in the diff")
}

func TestC2345_004_MeasuredNearDuplicateReasonsShareOneFingerprint(t *testing.T) {
	a := "review gate: phase \"build\" deliverable rejected after 1 correction(s): build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff:\n  [ste-lint] the changed documents fail the STE lint: 1 finding(s) in 1 file(s) (first: docs/explain/builds/cycle-2049-01m5xtvnwk94g9ryfgzhqre9m0.md:8 STE-SENTENCE); fix each finding"
	b := "review gate: phase \"build\" deliverable rejected after 1 correction(s): build handoff floor: 1 deterministic check failure(s) — fix these exactly before handoff:\n  [ste-lint] the changed documents fail the STE lint: 2 finding(s) in 1 file(s) (first: docs/explain/builds/cycle-2051-01m5xwqdjqyc9mhva9bxdvs52e.md:8 STE-SENTENCE); fix each finding"
	requireSameFingerprint(t, "build", a, b)
}

func TestC2345_005_WordEndingInCycleKeepsItsNumber(t *testing.T) {
	requireDistinctFingerprint(t, "build", "gate saw recycle=12 on the lane", "gate saw recycle=13 on the lane")
}

func TestC2345_006_CheckFailureCountStaysIdentityBearing(t *testing.T) {
	a := "build handoff floor: 2 deterministic check failure(s) — fix these exactly before handoff"
	b := "build handoff floor: 3 deterministic check failure(s) — fix these exactly before handoff"
	requireDistinctFingerprint(t, "build", a, b)
}

func TestC2345_007_DifferentDefectsStayDistinct(t *testing.T) {
	a := "build handoff floor: 1 deterministic check failure(s):\n  Explanation Documentation: Changed Areas does not explain material path go/internal/routingtest/engine.go"
	b := "build handoff floor: 1 deterministic check failure(s):\n  Explanation Documentation: Changed Areas does not explain material path go/internal/repocontract/repocontract.go"
	requireDistinctFingerprint(t, "build", a, b)
	requireDistinctFingerprint(t, "scout", "phase=scout cycle=2071 cause=submit_wedged", "phase=scout cycle=2071 cause=quota_wall")
	requireDistinctFingerprint(t, "build", "lint: 2 finding(s) in docs/a.md", "lint: 2 finding(s) in docs/b.md")
}
