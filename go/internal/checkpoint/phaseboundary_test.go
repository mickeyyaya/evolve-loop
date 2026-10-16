package checkpoint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestReasonPhaseComplete_IsValid(t *testing.T) {
	if got := string(ReasonPhaseComplete); got != "phase-complete" {
		t.Errorf("ReasonPhaseComplete = %q, want \"phase-complete\"", got)
	}
	if !ReasonPhaseComplete.IsValid() {
		t.Error("ReasonPhaseComplete.IsValid() = false, want true")
	}
}

func TestReason_NearMissSpellings_StayInvalid(t *testing.T) {
	for _, bogus := range []Reason{"phase-completed", "phasecomplete", "phase_complete", ""} {
		if bogus.IsValid() {
			t.Errorf("Reason(%q).IsValid() = true, want false", bogus)
		}
	}
}

func TestComposeChecked_AcceptsPhaseComplete(t *testing.T) {
	cs := core.CycleState{
		CycleID:         234,
		Phase:           "build",
		CompletedPhases: []string{"scout", "triage", "tdd"},
	}
	cp, err := ComposeChecked(cs, ReasonPhaseComplete, 0, "deadbeef", time.Unix(1770000000, 0))
	if err != nil {
		t.Fatalf("ComposeChecked(ReasonPhaseComplete) error: %v (the new reason must validate)", err)
	}
	if cp.Reason != ReasonPhaseComplete {
		t.Errorf("composed Reason = %q, want %q", cp.Reason, ReasonPhaseComplete)
	}
	if !cp.Enabled {
		t.Error("composed checkpoint must be Enabled")
	}
	if len(cp.CompletedPhases) != 3 || cp.CompletedPhases[0] != "scout" {
		t.Errorf("CompletedPhases = %v, want the CycleState's [scout triage tdd]", cp.CompletedPhases)
	}
}

func TestPhaseBoundaryCheckpointer_YieldsToEscalation(t *testing.T) {
	if core.PhaseBoundaryCheckpointer == nil {
		t.Fatal("core.PhaseBoundaryCheckpointer not registered (init() wiring missing)")
	}
	cs := core.CycleState{CycleID: 234, Phase: "build", CompletedPhases: []string{"scout"}}

	for _, escalation := range []string{"quota-likely", "batch-cap-near", "operator-requested", "stall-inactivity"} {
		root := t.TempDir()
		seedCycleState(t, root, escalation)
		if err := core.PhaseBoundaryCheckpointer(cs, root, time.Unix(1770000000, 0)); err != nil {
			t.Fatalf("hook error with existing %s: %v", escalation, err)
		}
		if got := readCheckpointReason(t, root); got != escalation {
			t.Errorf("existing %s checkpoint overwritten: reason now %q, want preserved", escalation, got)
		}
	}
}

func TestPhaseBoundaryCheckpointer_OverwritesRoutine(t *testing.T) {
	if core.PhaseBoundaryCheckpointer == nil {
		t.Fatal("core.PhaseBoundaryCheckpointer not registered")
	}
	cs := core.CycleState{CycleID: 234, Phase: "audit", CompletedPhases: []string{"scout", "build"}}

	for _, prior := range []string{"phase-complete", ""} {
		root := t.TempDir()
		seedCycleState(t, root, prior)
		if err := core.PhaseBoundaryCheckpointer(cs, root, time.Unix(1770000000, 0)); err != nil {
			t.Fatalf("hook error with prior %q: %v", prior, err)
		}
		if got := readCheckpointReason(t, root); got != "phase-complete" {
			t.Errorf("prior %q: reason = %q, want phase-complete refresh", prior, got)
		}
	}
}

func seedCycleState(t *testing.T, root, reason string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{"cycle_id": 234}
	if reason != "" {
		state["checkpoint"] = map[string]any{"enabled": true, "reason": reason, "quotaResetAt": "2026-05-23T20:00:00Z"}
	}
	b, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cycle-state.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readCheckpointReason(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".evolve", "cycle-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state struct {
		Checkpoint struct {
			Reason string `json:"reason"`
		} `json:"checkpoint"`
	}
	if err := json.Unmarshal(b, &state); err != nil {
		t.Fatal(err)
	}
	return state.Checkpoint.Reason
}
