package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// writeAuditWithFailure writes an audit-report.md carrying a v2 verdict
// sentinel with a structured failure block — the shape the dossier parses to
// surface the audit's self-declared class/defects.
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

func TestFailureDossier(t *testing.T) {
	fp := policy.DefaultSystemFailurePolicy()

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

	t.Run("coherent_red_audit_no_floor_candidate", func(t *testing.T) {
		dir := t.TempDir()
		writeVerdicts(t, dir, "FAIL", "FAIL")
		cs := CycleState{CycleID: 1002, WorkspacePath: dir}

		d := buildFailureDossier(cs, VerdictFAIL, fp)
		if d.FloorCandidate != "" {
			t.Errorf("FloorCandidate = %q, want empty (coherent RED audit)", d.FloorCandidate)
		}
	})

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

	t.Run("ship_phase_explained_no_floor_candidate", func(t *testing.T) {
		dir := t.TempDir()
		writeVerdicts(t, dir, "PASS", "PASS")
		cs := CycleState{CycleID: 1329, WorkspacePath: dir,
			ShipFailReasons: []string{
				"repo-contract scanner pack RED in the lane worktree (exit status 1) — pushing would red main",
			}}

		d := buildFailureDossier(cs, VerdictFAIL, fp)
		if d.FloorCandidate != "" {
			t.Errorf("FloorCandidate = %q, want empty (ship-phase failure is diagnosed, not forged)", d.FloorCandidate)
		}
	})

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
