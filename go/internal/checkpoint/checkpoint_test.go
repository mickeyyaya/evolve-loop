// Package checkpoint ports the pre-emptive cycle-checkpoint logic from
// scripts/lifecycle/cycle-state.sh:cycle_state_checkpoint. The
// checkpoint block is written INTO cycle-state.json (additive schema)
// — there is no separate checkpoint.json on disk despite earlier docs.
package checkpoint

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// fixedTime returns a deterministic timestamp for tests.
func fixedTime() time.Time {
	return time.Date(2026, 5, 22, 17, 0, 0, 0, time.UTC)
}

// TestTriggerDecide_Tiers exercises the 3-tier decision: <warn → None;
// [warn, checkpoint) → Warn; ≥checkpoint → Checkpoint. Mirrors bash
// evolve-loop-dispatch.sh:1057-1072 thresholds.
func TestTriggerDecide_Tiers(t *testing.T) {
	tr := Trigger{WarnAtPct: 80, CheckpointAtPct: 95}
	cases := []struct {
		pct  float64
		want Decision
	}{
		{0, DecisionNone},
		{50, DecisionNone},
		{79.9, DecisionNone},
		{80.0, DecisionWarn},
		{94.9, DecisionWarn},
		{95.0, DecisionCheckpoint},
		{120, DecisionCheckpoint},
	}
	for _, c := range cases {
		got := tr.Decide(c.pct)
		if got != c.want {
			t.Errorf("Decide(%g)=%v, want %v", c.pct, got, c.want)
		}
	}
}

// TestTriggerDecide_DisabledShortCircuits — EVOLVE_CHECKPOINT_DISABLE=1
// disables both thresholds. Even 99% returns None.
func TestTriggerDecide_DisabledShortCircuits(t *testing.T) {
	tr := Trigger{WarnAtPct: 80, CheckpointAtPct: 95, Disabled: true}
	if got := tr.Decide(99); got != DecisionNone {
		t.Errorf("Decide(99) when disabled=%v, want None", got)
	}
}

// TestTriggerFromEnv_Defaults — unset env yields the CLAUDE.md defaults:
// warn=80, checkpoint=95, disabled=false.
func TestTriggerFromEnv_Defaults(t *testing.T) {
	t.Setenv("EVOLVE_CHECKPOINT_WARN_AT_PCT", "")
	t.Setenv("EVOLVE_CHECKPOINT_AT_PCT", "")
	t.Setenv("EVOLVE_CHECKPOINT_DISABLE", "")
	tr := TriggerFromEnv()
	if tr.WarnAtPct != 80 || tr.CheckpointAtPct != 95 || tr.Disabled {
		t.Errorf("TriggerFromEnv defaults wrong: %+v", tr)
	}
}

// TestTriggerFromEnv_Overrides — env values are honored.
func TestTriggerFromEnv_Overrides(t *testing.T) {
	t.Setenv("EVOLVE_CHECKPOINT_WARN_AT_PCT", "70")
	t.Setenv("EVOLVE_CHECKPOINT_AT_PCT", "90")
	t.Setenv("EVOLVE_CHECKPOINT_DISABLE", "1")
	tr := TriggerFromEnv()
	if tr.WarnAtPct != 70 || tr.CheckpointAtPct != 90 || !tr.Disabled {
		t.Errorf("TriggerFromEnv overrides wrong: %+v", tr)
	}
}

// TestTriggerFromEnv_NumberParseFallsBack — malformed env values fall
// back to defaults (bash :- treats empty same as absent).
func TestTriggerFromEnv_NumberParseFallsBack(t *testing.T) {
	t.Setenv("EVOLVE_CHECKPOINT_AT_PCT", "not-a-number")
	tr := TriggerFromEnv()
	if tr.CheckpointAtPct != 95 {
		t.Errorf("malformed CheckpointAtPct → %d, want 95 default", tr.CheckpointAtPct)
	}
}

// TestCompose_FromCycleState pulls all the load-bearing fields from a
// CycleState and bash semantics. resumeFromPhase comes from .phase;
// completedPhases from .completed_phases; worktreePath from
// .active_worktree.
func TestCompose_FromCycleState(t *testing.T) {
	cs := core.CycleState{
		CycleID:         42,
		Phase:           "build",
		ActiveWorktree:  ".evolve/worktrees/abc",
		CompletedPhases: []string{"intent", "scout", "tdd"},
	}
	cp := Compose(cs, ReasonBatchCapNear, 12.34, "deadbeef", fixedTime())
	if !cp.Enabled {
		t.Error("Enabled=false")
	}
	if cp.Reason != ReasonBatchCapNear {
		t.Errorf("Reason=%q, want batch-cap-near", cp.Reason)
	}
	if cp.ResumeFromPhase != "build" {
		t.Errorf("ResumeFromPhase=%q, want build", cp.ResumeFromPhase)
	}
	if cp.WorktreePath != ".evolve/worktrees/abc" {
		t.Errorf("WorktreePath=%q", cp.WorktreePath)
	}
	if len(cp.CompletedPhases) != 3 {
		t.Errorf("CompletedPhases=%v", cp.CompletedPhases)
	}
	if cp.GitHead != "deadbeef" {
		t.Errorf("GitHead=%q", cp.GitHead)
	}
	if cp.CostAtCheckpoint != 12.34 {
		t.Errorf("Cost=%g", cp.CostAtCheckpoint)
	}
	if cp.SavedAt == "" {
		t.Error("SavedAt empty")
	}
	if cp.AutoResumeMaxAttempts != 3 {
		t.Errorf("AutoResumeMaxAttempts=%d, want 3 default", cp.AutoResumeMaxAttempts)
	}
}

// TestCompose_InvalidReason_Errors — reason is validated by Reason
// type, but Compose with a bare string must surface an error.
func TestCompose_InvalidReason_Panics(t *testing.T) {
	_, err := ComposeChecked(core.CycleState{CycleID: 1, Phase: "x"}, Reason("nope"), 0, "", fixedTime())
	if err == nil {
		t.Error("ComposeChecked(invalid reason): want error")
	}
}

// TestApplyToStateFile_AddsCheckpointBlock — read cycle-state.json,
// add checkpoint block via Apply, write back; verify round-trip.
func TestApplyToStateFile_AddsCheckpointBlock(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cycle-state.json")
	original := `{"cycle_id":7,"phase":"build","active_worktree":".evolve/worktrees/a","completed_phases":["intent","scout","tdd"],"workspace_path":".evolve/runs/cycle-7","intent_required":false}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	cp := Checkpoint{
		Enabled:               true,
		Reason:                ReasonBatchCapNear,
		SavedAt:               "2026-05-22T17:00:00Z",
		ResumeFromPhase:       "build",
		WorktreePath:          ".evolve/worktrees/a",
		CompletedPhases:       []string{"intent", "scout", "tdd"},
		GitHead:               "abcdef",
		CostAtCheckpoint:      5.0,
		AutoResumeAttempts:    0,
		AutoResumeMaxAttempts: 3,
	}
	if err := ApplyToStateFile(path, cp); err != nil {
		t.Fatalf("ApplyToStateFile: %v", err)
	}
	// Re-read and verify checkpoint sub-block exists with required fields.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	// Pre-existing fields preserved.
	if got["phase"] != "build" {
		t.Errorf("phase mutated: %v", got["phase"])
	}
	// Checkpoint block present.
	bl, ok := got["checkpoint"].(map[string]any)
	if !ok {
		t.Fatalf("checkpoint block missing or wrong type: %v", got["checkpoint"])
	}
	if bl["reason"] != "batch-cap-near" {
		t.Errorf("checkpoint.reason=%v", bl["reason"])
	}
	if bl["enabled"] != true {
		t.Errorf("checkpoint.enabled=%v", bl["enabled"])
	}
}

// TestApplyToStateFile_AtomicReplace — failed rename leaves no tmp.
func TestApplyToStateFile_AtomicReplace(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cs.json")
	if err := os.WriteFile(path, []byte(`{"cycle_id":1,"phase":"build"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cp := Checkpoint{
		Enabled: true, Reason: ReasonQuotaLikely, ResumeFromPhase: "build",
		AutoResumeMaxAttempts: 3,
	}
	if err := ApplyToStateFile(path, cp); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover tmp file: %s", e.Name())
		}
	}
}

// TestApplyToStateFile_MissingFile_Errors — checkpoint can't be applied
// without an existing cycle-state.json (bash:486 same precondition).
func TestApplyToStateFile_MissingFile_Errors(t *testing.T) {
	err := ApplyToStateFile("/no/such/file", Checkpoint{Reason: ReasonBatchCapNear})
	if err == nil {
		t.Error("ApplyToStateFile(missing): want error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err=%v, want os.ErrNotExist", err)
	}
}

// TestReason_Validate — only the 4 bash-canonical reasons are accepted.
func TestReason_Validate(t *testing.T) {
	for _, ok := range []Reason{ReasonQuotaLikely, ReasonBatchCapNear, ReasonOperatorRequest, ReasonStallInactivity} {
		if !ok.IsValid() {
			t.Errorf("%q should be valid", ok)
		}
	}
	if Reason("bogus").IsValid() {
		t.Error("'bogus' should be invalid")
	}
}
