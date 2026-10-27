package intent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
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
	}
}

var delta = hooks{delta: true}

func TestIntentShouldSkip_UnchangedGoal_DeterministicSkip(t *testing.T) {
	root := t.TempDir()
	writeBatchState(t, root, "abc123")
	skipped, verdict, next, _ := delta.ShouldSkip(deltaReq(root, "abc123"))
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

func TestIntentShouldSkip_ChangedGoal_RunsIntent(t *testing.T) {
	root := t.TempDir()
	writeBatchState(t, root, "OLD")
	if skipped, _, _, _ := delta.ShouldSkip(deltaReq(root, "NEW")); skipped {
		t.Error("changed goal hash must NOT skip — intent runs for delta synthesis")
	}
}

func TestIntentShouldSkip_NotDeltaMode_NeverSkips(t *testing.T) {
	root := t.TempDir()
	writeBatchState(t, root, "abc")
	req := deltaReq(root, "abc")
	req.Env = map[string]string{"EVOLVE_INTENT_DELTA": "1"}
	if skipped, _, _, _ := (hooks{}).ShouldSkip(req); skipped {
		t.Error("full (non-delta) intent must never skip, and the environment does not turn delta mode on")
	}
}

func TestIntentShouldSkip_NoState_FailOpenRuns(t *testing.T) {
	if skipped, _, _, _ := delta.ShouldSkip(deltaReq(t.TempDir(), "abc")); skipped {
		t.Error("absent state.json must fail open (run intent), not skip")
	}
}
