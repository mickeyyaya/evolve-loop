// cmd_fleet_lane_goalhash_test.go — regression for the fleet-lane goal-hash
// defect (inbox loop-goaltext-not-propagated-to-wave-lanes, 0.9); see
// laneGoalHash's doc-comment in cmd_fleet.go for the root cause.
package main

import "testing"

func TestLaneGoalHash_FallsBackToLaunchGoalHashWhenSpecEmpty(t *testing.T) {
	for _, tc := range []struct {
		name     string
		specGH   string
		fallback string
		want     string
	}{
		// The bug: a planner-built spec has no GoalHash → the lane must still
		// receive the wave's goal-hash, not an empty string.
		{"empty spec falls back", "", "wave-goal-hash", "wave-goal-hash"},
		// A spec that DOES carry its own goal identity wins (future planners /
		// campaign specs that set GoalHash keep control).
		{"spec goal-hash wins", "spec-goal-hash", "wave-goal-hash", "spec-goal-hash"},
		// Both empty stays empty — the caller (evolve cycle run) then errors
		// loudly with "--goal-hash is required" rather than launching unscoped.
		{"both empty stays empty", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := laneGoalHash(tc.specGH, tc.fallback); got != tc.want {
				t.Errorf("laneGoalHash(%q, %q) = %q, want %q", tc.specGH, tc.fallback, got, tc.want)
			}
		})
	}
}
