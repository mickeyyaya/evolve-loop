package intent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

func writeBatchState(t *testing.T, root, goalHash string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	js := `{"currentBatch":{"goalHash":"` + goalHash + `"}}`
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}
}

func deltaReq(root, goalHash string) core.PhaseRequest {
	return core.PhaseRequest{
		ProjectRoot: root,
		GoalHash:    goalHash,
		Env:         map[string]string{"EVOLVE_INTENT_DELTA": "1"},
	}
}

// TestIntentShouldSkip_UnchangedGoal_DeterministicSkip is the T1.5 fix: the
// "is the goal unchanged?" decision is a DETERMINISTIC goal-hash comparison in
// code (Core Rule 5), not an LLM judgment. A delta-mode cycle whose goal hash
// matches the batch's recorded hash skips intent (no LLM dispatch) → scout.
func TestIntentShouldSkip_UnchangedGoal_DeterministicSkip(t *testing.T) {
	root := t.TempDir()
	writeBatchState(t, root, "abc123")
	skipped, verdict, next, _ := (hooks{}).ShouldSkip(deltaReq(root, "abc123"))
	if !skipped {
		t.Fatal("matching goal hash in delta mode must skip intent deterministically")
	}
	if verdict != core.VerdictSKIPPED {
		t.Errorf("verdict=%q, want SKIPPED", verdict)
	}
	if next != string(core.PhaseScout) {
		t.Errorf("next=%q, want scout", next)
	}
}

// TestIntentShouldSkip_ChangedGoal_RunsIntent: a changed goal must NOT skip —
// the LLM still runs for genuine delta synthesis.
func TestIntentShouldSkip_ChangedGoal_RunsIntent(t *testing.T) {
	root := t.TempDir()
	writeBatchState(t, root, "OLD")
	if skipped, _, _, _ := (hooks{}).ShouldSkip(deltaReq(root, "NEW")); skipped {
		t.Error("changed goal hash must NOT skip — intent runs for delta synthesis")
	}
}

// TestIntentShouldSkip_NotDeltaMode_NeverSkips: full intent always runs.
func TestIntentShouldSkip_NotDeltaMode_NeverSkips(t *testing.T) {
	root := t.TempDir()
	writeBatchState(t, root, "abc")
	req := deltaReq(root, "abc")
	req.Env = nil // not delta mode
	if skipped, _, _, _ := (hooks{}).ShouldSkip(req); skipped {
		t.Error("full (non-delta) intent must never skip")
	}
}

// TestIntentShouldSkip_NoState_FailOpenRuns: an absent/unreadable state.json
// fails OPEN (run intent), never skips on uncertainty.
func TestIntentShouldSkip_NoState_FailOpenRuns(t *testing.T) {
	if skipped, _, _, _ := (hooks{}).ShouldSkip(deltaReq(t.TempDir(), "abc")); skipped {
		t.Error("absent state.json must fail open (run intent), not skip")
	}
}
