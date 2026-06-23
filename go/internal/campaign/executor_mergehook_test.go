package campaign

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/fleet"
)

// The AfterWaveComplete hook is the merge-to-main gate's seam: it fires once per
// newly-completed wave, AFTER progress is durably saved, so a promotion attempt
// always runs against a checkpointed boundary. A nil hook is a no-op (legacy
// behavior). Resumed (already-complete) waves must NOT re-fire it.

func TestRunWaves_AfterWaveCompleteFiresPerCompletedWave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	waves := [][]fleet.CycleSpec{mkwave("a"), mkwave("b", "c")}
	r := &recordingRunner{}

	var firedWaves, doneCounts []int
	opts := RunOptions{
		ProgressPath: path, PlanSHA: "P",
		AfterWaveComplete: func(w int, prog *CampaignProgress) {
			firedWaves = append(firedWaves, w)
			doneCounts = append(doneCounts, len(prog.CompletedWaves)) // hook sees durable progress
		},
	}
	if err := RunWaves(context.Background(), waves, r.run, opts); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(firedWaves, []int{0, 1}) {
		t.Errorf("hook fired for waves %v, want [0 1]", firedWaves)
	}
	if !reflect.DeepEqual(doneCounts, []int{1, 2}) {
		t.Errorf("completed-wave counts at hook time = %v, want [1 2] (hook runs after Save)", doneCounts)
	}
}

func TestRunWaves_AfterWaveCompleteSkipsResumedWaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	if err := (&CampaignProgress{PlanSHA: "P", CompletedWaves: []int{0}}).Save(path); err != nil {
		t.Fatal(err)
	}
	waves := [][]fleet.CycleSpec{mkwave("a"), mkwave("b")}
	r := &recordingRunner{}

	var fired []int
	opts := RunOptions{
		ProgressPath: path, PlanSHA: "P", Resume: true,
		AfterWaveComplete: func(w int, _ *CampaignProgress) { fired = append(fired, w) },
	}
	if err := RunWaves(context.Background(), waves, r.run, opts); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fired, []int{1}) {
		t.Errorf("hook fired for %v, want [1] (resumed wave 0 must not re-fire)", fired)
	}
}

func TestRunWaves_NilAfterWaveCompleteIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	waves := [][]fleet.CycleSpec{mkwave("a")}
	r := &recordingRunner{}
	// No AfterWaveComplete set: must behave exactly like before (no panic, wave runs).
	if err := RunWaves(context.Background(), waves, r.run, RunOptions{ProgressPath: path, PlanSHA: "P"}); err != nil {
		t.Fatal(err)
	}
}
