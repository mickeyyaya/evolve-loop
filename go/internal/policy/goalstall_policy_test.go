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
