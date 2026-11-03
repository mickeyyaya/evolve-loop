//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type countingPhaseRunner struct {
	name  string
	calls *atomic.Int32
}

func (r countingPhaseRunner) Name() string { return r.name }
func (r countingPhaseRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.calls.Add(1)
	return cycleRunQuotaRunner{}.Run(ctx, req)
}

func TestCycleRunRoot_AFleetLaneResumesTheQuotaPausedCycleOfItsTodoAtThePausedPhaseWhileASiblingHoldsTheProjectLock(t *testing.T) {
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	root := t.TempDir()
	initLoopContractRepo(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	ws := core.RunWorkspacePath(root, 7)
	inbox := filepath.Join(evolveDir, "inbox")
	for _, d := range []string{ws, inbox} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	item, _ := json.Marshal(map[string]any{"id": "paused-item", "title": "fixture paused-item", "kind": "bug"})
	scope, _ := json.Marshal(core.LaneScope{TodoIDs: []string{"paused-item"}, GoalHash: "paused-goal"})
	for path, body := range map[string][]byte{filepath.Join(inbox, "2026-11-02T00-00-00Z-paused-item.json"): item, filepath.Join(ws, core.LaneScopeFile): scope} {
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	statePath := filepath.Join(ws, core.CycleStateFile)
	t.Setenv(ipcenv.CycleStateFileKey, statePath)
	st := storage.New(evolveDir)
	ctx := context.Background()
	cs := core.CycleState{CycleID: 7, RunID: "paused-run", Phase: "build", WorkspacePath: ws, GoalHash: "paused-goal", CompletedPhases: []string{"scout", "triage"}}
	if err := st.WriteState(ctx, core.State{LastCycleNumber: 7}); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteCycleState(ctx, cs); err != nil {
		t.Fatal(err)
	}
	if err := checkpoint.ApplyToStateFile(statePath, checkpoint.Compose(cs, checkpoint.ReasonQuotaLikely, 0, "", time.Now())); err != nil {
		t.Fatal(err)
	}
	os.Unsetenv(ipcenv.CycleStateFileKey)
	t.Setenv(ipcenv.FleetKey, "1")
	t.Setenv(ipcenv.FleetScopeKey, "paused-item")
	releaseSiblingLock, err := st.AcquireLock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = releaseSiblingLock() })
	var scoutCalls, buildCalls atomic.Int32
	old := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = old })
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		ledger := newFakeLedger()
		runners := map[core.Phase]core.PhaseRunner{
			core.PhaseScout: countingPhaseRunner{name: "scout", calls: &scoutCalls},
			core.PhaseBuild: countingPhaseRunner{name: "build", calls: &buildCalls},
		}
		orch := core.NewOrchestrator(st, ledger, runners, core.WithRetryConfig(policy.RetryConfig{PhaseMaxAttempts: 2}))
		return orchDeps{Storage: st, Ledger: ledger, Orchestrator: orch}
	}
	var stdout, stderr bytes.Buffer

	rc := runCycleRun([]string{"--project-root", root, "--goal-hash", "next-wave-goal", "--goal", "next wave"}, &stdout, &stderr)

	if rc != fleet.ExitDeferred {
		t.Fatalf("rc = %d, want fleet.ExitDeferred from the walled resume; stderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "lane paused-item resumes cycle 7 at phase build (auto-resume 1 of 3)") {
		t.Errorf("stderr = %s, want the lane to resume cycle 7", stderr.String())
	}
	if scoutCalls.Load() != 0 || buildCalls.Load() == 0 {
		t.Errorf("scout ran %d times and build %d times; want build only, at the paused phase", scoutCalls.Load(), buildCalls.Load())
	}
	cp := readCheckpoint(t, statePath)
	if cp.AutoResumeAttempts != 1 || cp.Reason != checkpoint.ReasonQuotaLikely || cp.ResumeFromPhase != "build" {
		t.Errorf("checkpoint = %+v, want the walled resume to keep a quota checkpoint at build with 1 auto-resume spent", cp)
	}
}
