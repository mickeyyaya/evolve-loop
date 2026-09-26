package main

import "testing"

func TestLaneGoalHash_FallsBackToLaunchGoalHashWhenSpecEmpty(t *testing.T) {
	for _, tc := range []struct {
		name     string
		specGH   string
		fallback string
		want     string
	}{
		{"empty spec falls back", "", "wave-goal-hash", "wave-goal-hash"},
		{"spec goal-hash wins", "spec-goal-hash", "wave-goal-hash", "spec-goal-hash"},
		{"both empty stays empty", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := laneGoalHash(tc.specGH, tc.fallback); got != tc.want {
				t.Errorf("laneGoalHash(%q, %q) = %q, want %q", tc.specGH, tc.fallback, got, tc.want)
			}
		})
	}
}
