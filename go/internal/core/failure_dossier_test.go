package core

// failure_dossier_test.go — cycle-1002 RED contract for ADR-0072 S4 Task 1
// (evidence-dossier-builder). The dossier composes INDEPENDENT evidence — the
// coherence signal, the audit's self-declared failure envelope, and the
// non-progress counters — never the recorded verdict alone (the forged-verdict
// lesson). These tests fail RED until Builder adds buildFailureDossier /
// writeFailureDossier + the failureDossier type in failure_dossier.go.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// writeAuditWithFailure writes an audit-report.md carrying a v2 verdict sentinel
// with a structured failure block (ADR-0039 §7) — the shape the dossier parses
// to surface the audit's SELF-declared class/defects.
func writeAuditWithFailure(t *testing.T, dir, verdict, class string, defects ...string) {
	t.Helper()
	blk := map[string]any{"class": class}
	if len(defects) > 0 {
		blk["defects"] = defects
	}
	sent := map[string]any{"phase": "audit", "verdict": verdict, "schema_version": 2, "failure": blk}
	raw, err := json.Marshal(sent)
	if err != nil {
		t.Fatal(err)
	}
	body := "## Verdict\n<!-- evolve-verdict: " + string(raw) + " -->\n"
	if err := os.WriteFile(filepath.Join(dir, "audit-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestFailureDossier is the Task-1 acceptance surface.
func TestFailureDossier(t *testing.T) {
	fp := policy.DefaultSystemFailurePolicy()

	// (a) INCOHERENT: recorded FAIL but on-disk audit=PASS and acs=PASS with no
	// substantive error → verdict-incoherence floor candidate. Also exercises
	// the non-progress counters (composed from cs.FailedAt + policy thresholds)
	// and the write→read-back of the emitted artifact.
	t.Run("incoherent_verdict_floor_candidate_and_artifact", func(t *testing.T) {
		dir := t.TempDir()
		writeVerdicts(t, dir, "PASS", "PASS") // green artifacts contradict a recorded FAIL
		cs := CycleState{CycleID: 1002, WorkspacePath: dir, FailedAt: []FailedRecord{
			{Cycle: 1000, Verdict: "FAIL", Classification: "code-audit-fail"},
			{Cycle: 1001, Verdict: "FAIL", Classification: "code-audit-fail"},
		}}

		d := buildFailureDossier(cs, VerdictFAIL, fp)
		if d == nil {
			t.Fatal("buildFailureDossier returned nil")
		}
		if d.FloorCandidate != "verdict-incoherence" {
			t.Errorf("FloorCandidate = %q, want verdict-incoherence", d.FloorCandidate)
		}

		if err := writeFailureDossier(dir, d); err != nil {
			t.Fatalf("writeFailureDossier: %v", err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, "failure-dossier.json"))
		if err != nil {
			t.Fatalf("failure-dossier.json must be written per cycle: %v", err)
		}
		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("failure-dossier.json is not valid JSON: %v", err)
		}
		if got["floor_candidate"] != "verdict-incoherence" {
			t.Errorf("artifact floor_candidate = %v, want verdict-incoherence", got["floor_candidate"])
		}
		if got["recorded_verdict"] != VerdictFAIL {
			t.Errorf("artifact recorded_verdict = %v, want %s", got["recorded_verdict"], VerdictFAIL)
		}
		np, ok := got["non_progress"].(map[string]any)
		if !ok {
			t.Fatalf("artifact must carry a non_progress object, got %T", got["non_progress"])
		}
		// Policy thresholds are composed into the dossier (deterministic).
		if rc, _ := np["repeat_ceiling"].(float64); int(rc) != fp.Thresholds.RepeatCeiling {
			t.Errorf("non_progress.repeat_ceiling = %v, want %d", np["repeat_ceiling"], fp.Thresholds.RepeatCeiling)
		}
		// The same-class recurrence counter reflects the injected history.
		if scs, _ := np["same_class_streak"].(float64); int(scs) < 1 {
			t.Errorf("non_progress.same_class_streak = %v, want >= 1 for two same-class fails", np["same_class_streak"])
		}
	})

	// (b) COHERENT: recorded FAIL with a RED on-disk audit → the negative is
	// earned, not forged → no floor candidate.
	t.Run("coherent_red_audit_no_floor_candidate", func(t *testing.T) {
		dir := t.TempDir()
		writeVerdicts(t, dir, "FAIL", "FAIL")
		cs := CycleState{CycleID: 1002, WorkspacePath: dir}

		d := buildFailureDossier(cs, VerdictFAIL, fp)
		if d.FloorCandidate != "" {
			t.Errorf("FloorCandidate = %q, want empty (coherent RED audit)", d.FloorCandidate)
		}
	})

	// (c) CYCLE-1001 PROSE-ONLY: the audit self-declares a SYSTEM-class fault in
	// its defects PROSE while the structured class stays task-level
	// (code-audit-fail). The deterministic floor cannot catch it (FloorCandidate
	// empty) — but the dossier MUST surface the class + defects so the
	// orchestrator judgment layer can classify it. This is the exact shape the
	// live cycle-1001 case looped through as task-level.
	t.Run("cycle1001_prose_system_surfaced_for_judgment", func(t *testing.T) {
		dir := t.TempDir()
		writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail",
			"SYSTEM-class shared-state lost write: state.json carryoverTodos clobbered",
			"ACS gate fail-opens on missing predicate file")
		cs := CycleState{CycleID: 1001, WorkspacePath: dir, AuditFailReasons: []string{"gate downgrade"}}

		d := buildFailureDossier(cs, VerdictFAIL, fp)
		if d.AuditDeclared.Class != "code-audit-fail" {
			t.Errorf("AuditDeclared.Class = %q, want code-audit-fail", d.AuditDeclared.Class)
		}
		joined := strings.Join(d.AuditDeclared.Defects, " | ")
		if !strings.Contains(joined, "SYSTEM-class") {
			t.Errorf("AuditDeclared.Defects must surface the SYSTEM-class prose for judgment, got %q", joined)
		}
		if d.FloorCandidate != "" {
			t.Errorf("FloorCandidate = %q, want empty (prose-only system class is NOT deterministically floorable)", d.FloorCandidate)
		}
	})

	// (d) STRUCTURED SYSTEM: the audit self-declares a system-level class
	// structurally → AuditDeclared.Level maps to system via the policy table →
	// the dossier proposes the infra-systemic floor candidate deterministically
	// (caught even in orchestrator-absent fallback).
	t.Run("structured_system_class_yields_infra_systemic_candidate", func(t *testing.T) {
		dir := t.TempDir()
		writeAuditWithFailure(t, dir, "FAIL", "infra-systemic",
			"all CLI families exhausted; systemic infrastructure teardown")
		cs := CycleState{CycleID: 1002, WorkspacePath: dir}

		d := buildFailureDossier(cs, VerdictFAIL, fp)
		if d.AuditDeclared.Level != policy.LevelSystem {
			t.Errorf("AuditDeclared.Level = %q, want system", d.AuditDeclared.Level)
		}
		if d.FloorCandidate != policy.CategoryInfraSystemic {
			t.Errorf("FloorCandidate = %q, want infra-systemic", d.FloorCandidate)
		}
	})
}
