package core

import "testing"

func TestEvaluateZeroShip(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		waves     []int
		ceiling   int
		wantHalt  bool
		wantCount int
	}{
		{"no waves yet", nil, 2, false, 0},
		{"one zero-ship wave is below the ceiling", []int{0}, 2, false, 0},
		{"two consecutive zero-ship waves halt", []int{0, 0}, 2, true, 2},
		{"a ship between two zero-ship waves resets the count", []int{0, 1, 0}, 2, false, 0},
		{"a ship at the end of the history resets the count", []int{0, 0, 1}, 2, false, 0},
		{"the trailing run after an earlier ship halts", []int{3, 0, 0}, 2, true, 2},
		{"a wave with several ships is one productive wave", []int{2, 0}, 2, false, 0},
		{"two zero-ship waves stay below a ceiling of three", []int{0, 0}, 3, false, 0},
		{"three zero-ship waves halt at a ceiling of three", []int{0, 0, 0}, 3, true, 3},
		{"a long run reports its full length", []int{1, 0, 0, 0, 0}, 2, true, 4},
		{"a zero ceiling disables the rule", []int{0, 0, 0}, 0, false, 0},
		{"a negative ceiling disables the rule", []int{0, 0, 0}, -1, false, 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			v := EvaluateZeroShip(tc.waves, tc.ceiling)
			if v.Halt != tc.wantHalt {
				t.Fatalf("EvaluateZeroShip(%v, %d).Halt = %v, want %v (%+v)", tc.waves, tc.ceiling, v.Halt, tc.wantHalt, v)
			}
			if !tc.wantHalt {
				if v != (BlockerVerdict{}) {
					t.Errorf("a non-halt verdict is the zero verdict, got %+v", v)
				}
				return
			}
			if v.Rule != "zero-ship" {
				t.Errorf("Rule = %q, want %q", v.Rule, "zero-ship")
			}
			if v.Count != tc.wantCount {
				t.Errorf("Count = %d, want %d", v.Count, tc.wantCount)
			}
			if v.Reason == "" {
				t.Error("a halt names its reason")
			}
		})
	}
}

func TestEvaluateZeroShip_DoesNotMutateItsInput(t *testing.T) {
	t.Parallel()
	waves := []int{0, 1, 0, 0}
	_ = EvaluateZeroShip(waves, 2)
	want := []int{0, 1, 0, 0}
	for i := range want {
		if waves[i] != want[i] {
			t.Fatalf("input mutated: %v, want %v", waves, want)
		}
	}
}

func TestEvaluateZeroShip_ReasonStatesTheRunAndTheCeilingAndNothingElse(t *testing.T) {
	t.Parallel()
	got := EvaluateZeroShip([]int{1, 0, 0, 0}, 2).Reason
	if want := "3 consecutive waves landed no ship (ceiling 2)"; got != want {
		t.Errorf("Reason = %q, want %q", got, want)
	}
}
