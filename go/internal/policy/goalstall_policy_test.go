package policy

import "testing"

// The goal-stall threshold/weight must come from policy with a compiled-in safe
// default when the block is absent or non-positive (never a Go literal at the
// call site).
func TestGoalStallThreshold_DefaultAndOverride(t *testing.T) {
	if got := (Policy{}).GoalStallThreshold(); got != 3 {
		t.Errorf("absent block: threshold = %d, want compiled default 3", got)
	}
	if got := (Policy{GoalStall: &GoalStallPolicy{Threshold: 5}}).GoalStallThreshold(); got != 5 {
		t.Errorf("override: threshold = %d, want 5", got)
	}
	if got := (Policy{GoalStall: &GoalStallPolicy{Threshold: 0}}).GoalStallThreshold(); got != 3 {
		t.Errorf("non-positive: threshold = %d, want default 3", got)
	}
}

// The non-progress threshold is the UNION breaker's ceiling (any non-shipping
// outcome, FAIL included). It must be config-driven with its own compiled
// default, and that default must sit ABOVE the empty-only goal-stall threshold —
// a mixed FAIL/EMPTY streak is noisier evidence than a clean empty streak, so it
// gets more rope before escalating.
func TestGoalStallNonprogressThreshold_DefaultAndOverride(t *testing.T) {
	got := (Policy{}).GoalStallNonprogressThreshold()
	if got != 5 {
		t.Errorf("absent block: nonprogress threshold = %d, want compiled default 5", got)
	}
	if base := (Policy{}).GoalStallThreshold(); got <= base {
		t.Errorf("nonprogress default %d must exceed the empty-only goal-stall default %d", got, base)
	}
	if got := (Policy{GoalStall: &GoalStallPolicy{NonprogressThreshold: 8}}).GoalStallNonprogressThreshold(); got != 8 {
		t.Errorf("override: nonprogress threshold = %d, want 8", got)
	}
	if got := (Policy{GoalStall: &GoalStallPolicy{NonprogressThreshold: -1}}).GoalStallNonprogressThreshold(); got != 5 {
		t.Errorf("non-positive: nonprogress threshold = %d, want default 5", got)
	}
}

func TestGoalStallWeight_DefaultAndOverride(t *testing.T) {
	if got := (Policy{}).GoalStallWeight(); got != 0.9 {
		t.Errorf("absent block: weight = %v, want compiled default 0.9", got)
	}
	if got := (Policy{GoalStall: &GoalStallPolicy{Weight: 0.95}}).GoalStallWeight(); got != 0.95 {
		t.Errorf("override: weight = %v, want 0.95", got)
	}
	if got := (Policy{GoalStall: &GoalStallPolicy{Weight: -1}}).GoalStallWeight(); got != 0.9 {
		t.Errorf("non-positive: weight = %v, want default 0.9", got)
	}
}
