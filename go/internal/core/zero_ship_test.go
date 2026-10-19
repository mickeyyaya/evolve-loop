package core

import (
	"os"
	"path/filepath"
	"testing"
)

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

func writeShipTiming(t *testing.T, evolveDir string, dir, timing string) {
	t.Helper()
	d := filepath.Join(evolveDir, "runs", dir)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "phase-timing.json"), []byte(timing), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCollectCycleShips_ClassifiesRangeAndSortsByCycle(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	writeShipTiming(t, evolveDir, "cycle-12", `[{"phase":"triage","verdict":"SKIPPED","abort_reason":"triage-empty-commitment"}]`)
	writeShipTiming(t, evolveDir, "cycle-10", `[{"phase":"ship","verdict":"PASS"}]`)
	writeShipTiming(t, evolveDir, "cycle-3", `[{"phase":"ship","verdict":"PASS"}]`)
	writeShipTiming(t, evolveDir, "not-a-cycle", `[{"phase":"ship","verdict":"PASS"}]`)

	got := CollectCycleShips(evolveDir, 10)

	want := []CycleShip{{Cycle: 10, Shipped: true}, {Cycle: 12, Shipped: false}}
	if len(got) != len(want) {
		t.Fatalf("CollectCycleShips = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("CollectCycleShips[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestCollectCycleShips_MissingRunsDirIsEmpty(t *testing.T) {
	t.Parallel()
	if got := CollectCycleShips(t.TempDir(), 0); got != nil {
		t.Errorf("CollectCycleShips without a runs dir = %+v, want nil", got)
	}
}

func TestCountShips(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		cycles []CycleShip
		want   int
	}{
		{"none", nil, 0},
		{"all zero-ship", []CycleShip{{Cycle: 1}, {Cycle: 2}}, 0},
		{"mixed", []CycleShip{{Cycle: 1, Shipped: true}, {Cycle: 2}, {Cycle: 3, Shipped: true}}, 2},
	}
	for _, tc := range cases {
		if got := CountShips(tc.cycles); got != tc.want {
			t.Errorf("%s: CountShips = %d, want %d", tc.name, got, tc.want)
		}
	}
}
