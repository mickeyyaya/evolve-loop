package main

import (
	"strings"
	"testing"
)

func TestCycleRunArgs_FallsBackToLoopGoalText_WhenLaneHasNoOutputContract(t *testing.T) {
	args := cycleRunArgs("abc123", "", "fix the flaky bridge test", false, "")
	got := strings.Join(args, " ")
	want := "cycle run --goal-hash abc123 --goal fix the flaky bridge test"
	if got != want {
		t.Fatalf("cycleRunArgs = %q, want %q — a lane with no OutputContract must fall back to the loop-level --goal-text", got, want)
	}
}

func TestCycleRunArgs_OutputContractTakesPrecedenceOverGoalText(t *testing.T) {
	args := cycleRunArgs("abc123", "planned removal task", "operator free text goal", false, "")
	got := strings.Join(args, " ")
	want := "cycle run --goal-hash abc123 --goal planned removal task"
	if got != want {
		t.Fatalf("cycleRunArgs = %q, want %q — the per-todo OutputContract must win over --goal-text, never be overridden", got, want)
	}
}

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
