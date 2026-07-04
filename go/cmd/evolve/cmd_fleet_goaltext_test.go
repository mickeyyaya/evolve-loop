package main

import (
	"strings"
	"testing"
)

// cmd_fleet_goaltext_test.go pins the loop-goaltext-wave-lane-propagation fix
// (inbox loop-goaltext-not-propagated-to-wave-lanes, weight 0.90): `evolve
// loop --goal-text` is parsed into loopConfig.GoalText but the wave-launch
// path never threads it — cmd_loop.go:409 calls productionWaveLauncher with
// only cfg.GoalHash, so every wave lane whose CycleSpec.OutputContract is
// empty silently drops the operator's --goal-text. cycleRunArgs is the pure
// seam where the --goal flag is decided; it grows a new goalText fallback
// parameter that ONLY applies when outputContract is empty (the per-todo
// contract must never be overridden).
func TestCycleRunArgs_FallsBackToLoopGoalText_WhenLaneHasNoOutputContract(t *testing.T) {
	args := cycleRunArgs("abc123", "", "fix the flaky bridge test", false, "")
	got := strings.Join(args, " ")
	want := "cycle run --goal-hash abc123 --goal fix the flaky bridge test"
	if got != want {
		t.Fatalf("cycleRunArgs = %q, want %q — a lane with no OutputContract must fall back to the loop-level --goal-text", got, want)
	}
}

// TestCycleRunArgs_OutputContractTakesPrecedenceOverGoalText is the guard
// against the fallback overriding a real per-todo contract: the plan's
// OutputContract is the lane's binding goal (cmd_fleet.go's existing
// cycleRunArgs doc) and must win even when an operator-level --goal-text is
// also present.
func TestCycleRunArgs_OutputContractTakesPrecedenceOverGoalText(t *testing.T) {
	args := cycleRunArgs("abc123", "planned removal task", "operator free text goal", false, "")
	got := strings.Join(args, " ")
	want := "cycle run --goal-hash abc123 --goal planned removal task"
	if got != want {
		t.Fatalf("cycleRunArgs = %q, want %q — the per-todo OutputContract must win over --goal-text, never be overridden", got, want)
	}
}

// TestCycleRunArgs_BothOutputContractAndGoalTextEmpty_OmitsGoalFlag pins the
// regression-safe baseline (inbox acceptance #3): when neither is set, the
// argv must be byte-identical to today's output — no fabricated --goal flag.
func TestCycleRunArgs_BothOutputContractAndGoalTextEmpty_OmitsGoalFlag(t *testing.T) {
	args := cycleRunArgs("abc123", "", "", false, "")
	got := strings.Join(args, " ")
	want := "cycle run --goal-hash abc123"
	if got != want {
		t.Fatalf("cycleRunArgs = %q, want %q — byte-identical baseline when both outputContract and goalText are empty", got, want)
	}
	// Guard against a bare `--goal` flag with an empty value. Check the argv
	// tokens directly, not a substring: `--goal-hash` contains "--goal", so a
	// naive strings.Contains would false-trip on the always-present goal-hash.
	for _, a := range args {
		if a == "--goal" {
			t.Errorf("cycleRunArgs = %q, the --goal flag must be entirely omitted, not emitted with an empty value", got)
		}
	}
}
