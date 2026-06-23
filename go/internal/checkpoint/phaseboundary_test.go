// phaseboundary_test.go — cycle-234 task `phase-boundary-checkpoint` (RED).
//
// AC1: a new ReasonPhaseComplete constant joins the canonical reason set so
// the orchestrator can write a checkpoint block at EVERY phase boundary
// (campaign retro Invariant 3 — three failed `--resume` attempts in the
// 215-231 campaign because checkpoints existed only at the quota wall).
//
// These tests are authored by the TDD-Engineer BEFORE the constant exists:
// the compile error "undefined: ReasonPhaseComplete" is the intended RED
// signal. Builder adds the constant + IsValid arm; do NOT modify the tests.
package checkpoint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// TestReasonPhaseComplete_IsValid pins the new reason's identity:
// the canonical wire value is "phase-complete" and IsValid accepts it.
func TestReasonPhaseComplete_IsValid(t *testing.T) {
	if got := string(ReasonPhaseComplete); got != "phase-complete" {
		t.Errorf("ReasonPhaseComplete = %q, want \"phase-complete\"", got)
	}
	if !ReasonPhaseComplete.IsValid() {
		t.Error("ReasonPhaseComplete.IsValid() = false, want true")
	}
}

// Negative (adversarial diversity): near-miss spellings must stay invalid —
// an IsValid that just returns true would pass the positive test above but
// fail here.
func TestReason_NearMissSpellings_StayInvalid(t *testing.T) {
	for _, bogus := range []Reason{"phase-completed", "phasecomplete", "phase_complete", ""} {
		if bogus.IsValid() {
			t.Errorf("Reason(%q).IsValid() = true, want false", bogus)
		}
	}
}

// TestComposeChecked_AcceptsPhaseComplete: the validating constructor must
// admit the new reason (it is the orchestrator's entry point for untrusted
// reason origins) and carry it through to the composed block.
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

// TestPhaseBoundaryCheckpointer_YieldsToEscalation pins the overwrite policy
// (found landing cycle-234/236 work: the production wiring of the hook broke
// TestRunLoop_QuotaPause_Rc5 because a routine phase-complete checkpoint
// clobbered the seeded quota-likely block before detectQuotaPause read it).
// phase-complete is the LOWEST-priority reason: the hook must never overwrite
// an existing escalation checkpoint (quota-likely, batch-cap-near,
// operator-requested, stall-inactivity) — those must survive until consumed.
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

// TestPhaseBoundaryCheckpointer_OverwritesRoutine: a phase-complete (or absent)
// checkpoint IS refreshed — that is the breadcrumb behavior the feature exists for.
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
