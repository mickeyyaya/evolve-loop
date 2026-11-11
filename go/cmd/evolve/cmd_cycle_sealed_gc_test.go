package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func (e *sealGCEnv) simulatedLaneCycle(sealedCycle int) (int, string) {
	e.t.Helper()
	if err := storage.New(e.evolveDir).WriteState(context.Background(), core.State{LastCycleNumber: sealedCycle - 1}); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.projectRoot, ".gitignore"), []byte(".evolve/\n"), 0o644); err != nil {
		e.t.Fatal(err)
	}
	gcGit(e.t, e.projectRoot, "add", ".gitignore")
	gcGit(e.t, e.projectRoot, "commit", "-q", "-m", "ignore the runtime tree")
	e.t.Setenv(ipcenv.FleetKey, "1")
	var stdout, stderr bytes.Buffer
	rc := runCycleRun([]string{"--project-root", e.projectRoot, "--goal-hash", "abcd1234", "--simulate"}, &stdout, &stderr)
	var result core.CycleResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || result.Cycle != sealedCycle {
		e.t.Fatalf("the simulated lane cycle = %d (err %v), want cycle %d\nstdout:\n%s", result.Cycle, err, sealedCycle, stdout.String())
	}
	return rc, stderr.String()
}

func TestRunCycleRun_ASealedFleetLaneCycleHasItsDirtyLaneBranchAndOrphanCleanedAtOnce(t *testing.T) {
	e := newSealGCEnv(t, "enforce")
	sealed := e.lane(800, true)
	other := e.lane(801, true)
	e.orphansIn(filepath.Join(sealed, "go"), other)

	rc, stderr := e.simulatedLaneCycle(800)

	if rc != 0 {
		t.Fatalf("evolve cycle run exit = %d, want 0\nstderr:\n%s", rc, stderr)
	}
	e.assertReleased(sealed, 800)
	e.assertKept(other, 801)
	if !reflect.DeepEqual(e.signaled, []int{20}) {
		t.Errorf("signaled %v, want [20]: the orphan of the sealed lane, never the orphan of cycle 801", e.signaled)
	}
}

func TestRunCycleRun_TheGCModeGovernsTheSealedFleetLaneSweep(t *testing.T) {
	for _, mode := range []string{"shadow", "off"} {
		t.Run(mode, func(t *testing.T) {
			e := newSealGCEnv(t, mode)
			sealed := e.lane(800, true)
			e.orphansIn(sealed)

			rc, stderr := e.simulatedLaneCycle(800)

			if rc != 0 {
				t.Fatalf("evolve cycle run exit = %d, want 0\nstderr:\n%s", rc, stderr)
			}
			e.assertKept(sealed, 800)
			if len(e.signaled) != 0 {
				t.Errorf("gc.mode %s must signal nothing, signaled %v", mode, e.signaled)
			}
		})
	}
}

func TestRunCycleRun_TheLiveLaneOfAnotherCycleIsNotTouchedWhenAFleetLaneCycleSeals(t *testing.T) {
	e := newSealGCEnv(t, "enforce")
	sealed := e.lane(800, true)
	live := e.lane(801, true)
	liveRun := filepath.Join(e.evolveDir, "runs", "cycle-801")
	if err := os.MkdirAll(liveRun, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(liveRun, runlease.Lease{RunID: "cycle-801", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
	e.orphansIn(live)

	rc, stderr := e.simulatedLaneCycle(800)

	if rc != 0 {
		t.Fatalf("evolve cycle run exit = %d, want 0\nstderr:\n%s", rc, stderr)
	}
	e.assertKept(live, 801)
	if _, err := os.Stat(sealed); !os.IsNotExist(err) {
		t.Errorf("the sealed lane of cycle 800 must be released: %v", err)
	}
	if len(e.signaled) != 0 {
		t.Errorf("the orphan sits in the live lane of cycle 801, signaled %v", e.signaled)
	}
}
