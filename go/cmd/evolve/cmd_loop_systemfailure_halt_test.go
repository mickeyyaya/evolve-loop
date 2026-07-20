package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

// ADR-0072 fleet-halt-unwired (inbox adr0072-fleet-halt-unwired, cycle 956).
//
// Today `result.SystemFailure` is read ONLY by the sequential single-cycle
// path in cmd_loop.go (line ~650). `runCycleRun` (cmd_cycle.go, the exact
// entrypoint every fleet lane subprocess runs) maps FinalVerdict to an exit
// code and never looks at SystemFailure, so a system-failure halt inside a
// fleet lane is indistinguishable from an ordinary FAIL (rc=2) to the parent
// wave loop — which in turn (cmd_loop_wave.go dispatchIteration / the
// cmd_loop.go wave/pool branches) only counts `ExitCode != 0` as "failed lane"
// and always continues to the next wave.
//
// These tests encode the exit-code contract as pure, unit-testable functions
// so both the single-cycle path and the fleet subprocess boundary can share
// ONE halt decision (AC2: no duplicated logic) instead of the sequential path
// re-implementing the check inline.

// cycleRunExitCode does not exist yet — this is the pure function
// runCycleRun (cmd_cycle.go) must consult instead of its current inline
// `if result.FinalVerdict == core.VerdictFAIL { return 2 }; return 0`.
func TestCycleRunExitCode_HaltsOnSystemFailureRegardlessOfVerdict(t *testing.T) {
	halted := cyclestate.CycleResult{
		FinalVerdict:  "PASS",
		SystemFailure: &cyclestate.SystemFailureSignal{Category: "verdict-incoherence", Level: "system", Halt: true},
	}
	if got := cycleRunExitCode(halted); got != systemFailureHaltExitCode {
		t.Errorf("cycleRunExitCode(SystemFailure.Halt=true, verdict=PASS) = %d, want %d", got, systemFailureHaltExitCode)
	}

	haltedFail := cyclestate.CycleResult{
		FinalVerdict:  "FAIL",
		SystemFailure: &cyclestate.SystemFailureSignal{Category: "verdict-incoherence", Level: "system", Halt: true},
	}
	if got := cycleRunExitCode(haltedFail); got != systemFailureHaltExitCode {
		t.Errorf("cycleRunExitCode(SystemFailure.Halt=true, verdict=FAIL) = %d, want %d (halt takes priority)", got, systemFailureHaltExitCode)
	}
}

func TestCycleRunExitCode_NoSystemFailure_FollowsOrdinaryVerdictMapping(t *testing.T) {
	cases := []struct {
		name string
		res  cyclestate.CycleResult
		want int
	}{
		{"nil-signal-fail", cyclestate.CycleResult{FinalVerdict: "FAIL"}, 2},
		{"nil-signal-pass", cyclestate.CycleResult{FinalVerdict: "PASS"}, 0},
		{"nil-signal-warn", cyclestate.CycleResult{FinalVerdict: "WARN"}, 0},
		{"signal-present-not-halt", cyclestate.CycleResult{
			FinalVerdict:  "FAIL",
			SystemFailure: &cyclestate.SystemFailureSignal{Category: "infra-transient", Level: "system", Halt: false},
		}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cycleRunExitCode(c.res); got != c.want {
				t.Errorf("cycleRunExitCode(%s) = %d, want %d (must be unchanged: existing rc=2/0 mapping)", c.name, got, c.want)
			}
		})
	}
}

// haltOnSystemFailure does not exist yet — it is the ONE shared function
// (AC2) both cmd_loop.go's sequential path and runCycleRun must call so the
// dossier + P0 inbox item + halt exit code are written identically on every
// code path, instead of the escalation logic being duplicated.
func TestHaltOnSystemFailure_WritesDossierAndP0AndReturnsHaltExitCode(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sf := &cyclestate.SystemFailureSignal{
		Category: "verdict-incoherence",
		Level:    "system",
		Evidence: "recorded=FAIL but audit=PASS and acs=PASS",
		Halt:     true,
	}

	got := haltOnSystemFailure(evolveDir, root, 956, filepath.Join(root, ".evolve/runs/cycle-956"), sf, os.Stderr)
	if got != systemFailureHaltExitCode {
		t.Errorf("haltOnSystemFailure return = %d, want %d", got, systemFailureHaltExitCode)
	}

	escB, err := os.ReadFile(filepath.Join(evolveDir, "pipeline-escalation.json"))
	if err != nil {
		t.Fatalf("pipeline-escalation.json not written: %v", err)
	}
	var esc map[string]any
	if err := json.Unmarshal(escB, &esc); err != nil {
		t.Fatalf("escalation not valid JSON: %v", err)
	}
	if esc["category"] != "verdict-incoherence" {
		t.Errorf("escalation category = %v, want verdict-incoherence", esc["category"])
	}

	inboxPath := filepath.Join(root, ".evolve", "inbox", "pipeline-defect-verdict-incoherence.json")
	if _, err := os.Stat(inboxPath); err != nil {
		t.Errorf("P0 pipeline-repair inbox item not filed at %s: %v", inboxPath, err)
	}
}

// anyLaneHaltedForSystemFailure does not exist yet — the wave/fleet dispatch
// loop (cmd_loop_wave.go dispatchIteration, and the wave/pool branches in
// cmd_loop.go around line 519) must consult it across ALL lane results and
// stop dispatching further waves when it is true, instead of only counting
// `r.Err != nil || r.ExitCode != 0` as an ordinary "failed lane".
func TestAnyLaneHaltedForSystemFailure_DetectsHaltExitCodeAmongLanes(t *testing.T) {
	results := []fleet.Result{
		{Index: 0, ExitCode: 0},
		{Index: 1, ExitCode: systemFailureHaltExitCode},
		{Index: 2, ExitCode: 0},
	}
	if !anyLaneHaltedForSystemFailure(results) {
		t.Error("anyLaneHaltedForSystemFailure = false, want true when one lane exits with the system-failure halt code")
	}
}

func TestAnyLaneHaltedForSystemFailure_OrdinaryLaneFailuresDoNotHalt(t *testing.T) {
	// Negative case: an ordinary FAIL (rc=2) or process error (rc=-1/1) must
	// NOT be conflated with a system-failure halt — only the batch loop
	// should stop; ordinary task-level failures keep the never-stop retry
	// semantics (ADR-0072 draws this line deliberately).
	results := []fleet.Result{
		{Index: 0, ExitCode: 2},
		{Index: 1, ExitCode: 1},
		{Index: 2, ExitCode: -1, Err: errTestLaneFailed},
	}
	if anyLaneHaltedForSystemFailure(results) {
		t.Error("anyLaneHaltedForSystemFailure = true, want false — ordinary lane failures must not trigger a batch halt")
	}
}

func TestAnyLaneHaltedForSystemFailure_EmptyResultsIsFalse(t *testing.T) {
	if anyLaneHaltedForSystemFailure(nil) {
		t.Error("anyLaneHaltedForSystemFailure(nil) = true, want false")
	}
}

var errTestLaneFailed = &laneTestError{"simulated lane launch error"}

type laneTestError struct{ msg string }

func (e *laneTestError) Error() string { return e.msg }
