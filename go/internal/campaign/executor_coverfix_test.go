package campaign

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

func TestCoverfixRunWavesResumeRefusesCorruptProgress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &recordingRunner{}
	err := RunWaves(context.Background(), [][]fleet.CycleSpec{mkwave("a")}, r.run, RunOptions{ProgressPath: path, PlanSHA: "P", Resume: true})
	if err == nil {
		t.Fatal("corrupt progress on resume must fail, not restart silently")
	}
	if r.calls != 0 {
		t.Errorf("no wave may run after a progress load failure, ran %d", r.calls)
	}
}

func TestCoverfixRunWavesReportsProgressSaveFailure(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	r := &recordingRunner{}
	err := RunWaves(context.Background(), [][]fleet.CycleSpec{mkwave("a"), mkwave("b")}, r.run,
		RunOptions{ProgressPath: filepath.Join(blocker, "prog.json"), PlanSHA: "P"})
	if err == nil {
		t.Fatal("unwritable progress must fail the campaign")
	}
	if r.calls != 1 {
		t.Errorf("campaign must stop at the first unsaved wave, ran %d waves", r.calls)
	}
}

func TestCoverfixCooldownSkipsNonPositiveDelays(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second} {
		slept := false
		r := &recordingRunner{failTimes: map[string]int{"a": 1}}
		opts := RunOptions{
			ProgressPath: filepath.Join(t.TempDir(), "prog.json"), PlanSHA: "P", MaxRetries: 1,
			Cooldown: func() time.Duration { return d },
			Sleep:    func(time.Duration) { slept = true },
		}
		if err := RunWaves(context.Background(), [][]fleet.CycleSpec{mkwave("a")}, r.run, opts); err != nil {
			t.Fatal(err)
		}
		if slept {
			t.Errorf("cooldown %v must not sleep", d)
		}
		if r.calls != 2 {
			t.Errorf("cooldown %v: want a retry (2 runs), got %d", d, r.calls)
		}
	}
}

func TestCoverfixCooldownFallsBackToRealSleepWithoutSeam(t *testing.T) {
	r := &recordingRunner{failTimes: map[string]int{"a": 1}}
	opts := RunOptions{
		ProgressPath: filepath.Join(t.TempDir(), "prog.json"), PlanSHA: "P", MaxRetries: 1,
		Cooldown: func() time.Duration { return 20 * time.Millisecond },
	}
	start := time.Now()
	if err := RunWaves(context.Background(), [][]fleet.CycleSpec{mkwave("a")}, r.run, opts); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed < 20*time.Millisecond {
		t.Errorf("retry fired after %v, before the 20ms cooldown", elapsed)
	}
}

func TestCoverfixSkippedOptionalCycleIsFailedNotCompleted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prog.json")
	wave := []fleet.CycleSpec{{Scope: []string{"a"}}, {Scope: []string{"b"}, Optional: true}}
	r := &recordingRunner{failForever: map[string]bool{"b": true}}
	if err := RunWaves(context.Background(), [][]fleet.CycleSpec{wave}, r.run, RunOptions{ProgressPath: path, PlanSHA: "P"}); err != nil {
		t.Fatal(err)
	}
	prog, err := LoadProgress(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(prog.CompletedCycleIDs, []string{"a"}) {
		t.Errorf("completed = %v, want [a]", prog.CompletedCycleIDs)
	}
	if !reflect.DeepEqual(prog.FailedCycleIDs, []string{"b"}) {
		t.Errorf("failed = %v, want [b]", prog.FailedCycleIDs)
	}
}
