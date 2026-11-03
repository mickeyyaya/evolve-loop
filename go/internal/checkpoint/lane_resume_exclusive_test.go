package checkpoint

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func pausedLaneState(t *testing.T) (workspace, statePath string) {
	t.Helper()
	workspace = filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	statePath = filepath.Join(workspace, core.CycleStateFile)
	if err := os.WriteFile(statePath, []byte(`{"cycle_id":7,"phase":"build"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := core.CycleState{CycleID: 7, Phase: "build", WorkspacePath: workspace}
	if err := ApplyToStateFile(statePath, Compose(cs, ReasonQuotaLikely, 0, "", time.Now())); err != nil {
		t.Fatal(err)
	}
	return workspace, statePath
}

func TestRecordLaneResume_TwoConcurrentLaneResumesOfOnePausedCycleAdmitExactlyOne(t *testing.T) {
	workspace, statePath := pausedLaneState(t)
	const lanes = 2
	var wg sync.WaitGroup
	admitted := make([]bool, lanes)
	errs := make([]error, lanes)
	for i := range lanes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, admitted[i], errs[i] = RecordLaneResume(statePath, true)
		}()
	}
	wg.Wait()

	winners, refusals := 0, 0
	for i := range lanes {
		var held *core.ResumeHeldError
		switch {
		case errs[i] == nil && admitted[i]:
			winners++
		case errors.As(errs[i], &held) && held.Holder.OwnerPID == os.Getpid() && held.Workspace == workspace:
			refusals++
		default:
			t.Errorf("lane %d = (admitted %v, err %v), want an admission or a typed refusal that names the holder", i, admitted[i], errs[i])
		}
	}
	if winners != 1 || refusals != 1 {
		t.Fatalf("winners = %d, refusals = %d; want exactly one lane to adopt the paused cycle", winners, refusals)
	}
	if got := readBlock(t, statePath).AutoResumeAttempts; got != 1 {
		t.Errorf("auto-resume attempts = %d, want 1: the refused lane spends no attempt", got)
	}
	if l, ok, err := runlease.Read(workspace); err != nil || !ok || !runlease.Fresh(l, time.Now(), 0) {
		t.Errorf("lease after admission = (%+v, %v, %v), want a fresh lease that refuses a third lane", l, ok, err)
	}
}

func TestRecordLaneResume_ASpentBudgetTakesNoLease(t *testing.T) {
	workspace, statePath := pausedLaneState(t)
	dropLease := func() {
		if err := os.Remove(runlease.PathIn(workspace)); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	for range DefaultAutoResumeAttempts {
		dropLease()
		if _, ok, err := RecordLaneResume(statePath, true); err != nil || !ok {
			t.Fatalf("auto-resume inside the budget = (%v, %v), want admitted", ok, err)
		}
	}
	dropLease()

	if _, ok, err := RecordLaneResume(statePath, true); err != nil || ok {
		t.Fatalf("auto-resume past the budget = (%v, %v), want refused without error", ok, err)
	}
	if _, ok, _ := runlease.Read(workspace); ok {
		t.Error("a refused resume wrote a lease and blocks the operator resume for a full TTL")
	}
}
