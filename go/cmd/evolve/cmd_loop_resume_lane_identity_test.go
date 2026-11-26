package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type laneIdentityWallRunner struct {
	name  string
	calls *atomic.Int32
}

func (r laneIdentityWallRunner) Name() string { return r.name }
func (r laneIdentityWallRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	r.calls.Add(1)
	wall := core.Wall{Ends: []core.FamilyEnd{{CLI: "agy", Pattern: "rate_limit"}}}
	return core.PhaseResponse{}, core.WalkError{Wall: wall, Err: fmt.Errorf("bridge: launch exit=85: %w", core.ErrTransientBridgeFailure)}
}

type laneIdentityCheckpoint struct {
	stage  string
	pool   int
	scope  []string
	legacy bool
}

type laneIdentityResume struct {
	root      string
	statePath string
	rc        int
	stdout    string
	stderr    string
	wired     []routingRun
	calls     int32
}

func resumeLaneIdentityCheckpoint(t *testing.T, in laneIdentityCheckpoint) laneIdentityResume {
	t.Helper()
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	root := t.TempDir()
	initLoopContractRepo(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	ws := core.RunWorkspacePath(root, 7)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if in.stage != "" {
		body := fmt.Sprintf(`{"fleet":{"integration":{"stage":%q}}}`, in.stage)
		if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if len(in.scope) > 0 {
		pin, err := json.Marshal(core.LaneScope{TodoIDs: in.scope, GoalHash: "g"})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, core.LaneScopeFile), pin, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := storage.New(evolveDir)
	ctx := context.Background()
	cs := core.CycleState{CycleID: 7, RunID: "paused-run", Phase: "build", WorkspacePath: ws, CompletedPhases: []string{"scout", "triage"}}
	if err := st.WriteState(ctx, core.State{LastCycleNumber: 7}); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteCycleState(ctx, cs); err != nil {
		t.Fatal(err)
	}
	unbind := checkpoint.BindLane(in.pool)
	cp := checkpoint.Compose(cs, checkpoint.ReasonQuotaLikely, 0, "", time.Now())
	unbind()
	if in.legacy {
		cp.PoolID, cp.LaneCycle, cp.Scope = 0, 0, nil
	}
	statePath := filepath.Join(evolveDir, core.CycleStateFile)
	if err := checkpoint.ApplyToStateFile(statePath, cp); err != nil {
		t.Fatal(err)
	}

	out := laneIdentityResume{root: root, statePath: statePath}
	var calls atomic.Int32
	old := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = old })
	wireOrchestratorDepsFn = func(_, _ string, _ io.Writer, run routingRun) orchDeps {
		out.wired = append(out.wired, run)
		ledger := newFakeLedger()
		runners := map[core.Phase]core.PhaseRunner{}
		for _, p := range []core.Phase{core.PhaseBuild, core.PhaseAudit, core.PhaseShip} {
			runners[p] = laneIdentityWallRunner{name: string(p), calls: &calls}
		}
		orch := core.NewOrchestrator(st, ledger, runners, core.WithRetryConfig(policy.RetryConfig{PhaseMaxAttempts: 2}))
		return orchDeps{Storage: st, Ledger: ledger, Orchestrator: orch}
	}
	var stdout, stderr bytes.Buffer
	out.rc = runLoop([]string{"--project-root", root, "--resume"}, nil, &stdout, &stderr)
	out.stdout, out.stderr, out.calls = stdout.String(), stderr.String(), calls.Load()
	return out
}

func TestLaneIdentity_AResumedLaneCheckpointAtEnforceTakesItsPoolAndScopeFromTheCheckpoint(t *testing.T) {
	r := resumeLaneIdentityCheckpoint(t, laneIdentityCheckpoint{stage: policy.FleetIntegrationEnforce, pool: 41, scope: []string{"item-a"}})

	if r.calls == 0 {
		t.Fatalf("the lane checkpoint did not resume: rc=%d stderr=%s", r.rc, r.stderr)
	}
	if len(r.wired) != 1 || r.wired[0].pool != 41 || !reflect.DeepEqual(r.wired[0].scope, []string{"item-a"}) {
		t.Fatalf("the resumed orchestrator was wired with %+v, want one wiring that carries pool 41 and scope [item-a] so ship delivers to the wave record and stops before main", r.wired)
	}
}

func TestLaneIdentity_AResumedLaneCheckpointAtShadowTakesItsPoolAndScopeFromTheCheckpoint(t *testing.T) {
	r := resumeLaneIdentityCheckpoint(t, laneIdentityCheckpoint{stage: policy.FleetIntegrationShadow, pool: 41, scope: []string{"item-a", "item-b"}})

	if r.calls == 0 || len(r.wired) != 1 || r.wired[0].pool != 41 || !reflect.DeepEqual(r.wired[0].scope, []string{"item-a", "item-b"}) {
		t.Fatalf("calls=%d wired=%+v stderr=%s; want the shadow resume wired with pool 41 and the lane scope", r.calls, r.wired, r.stderr)
	}
}

func TestLaneIdentity_ALegacyLaneCheckpointAtEnforceIsRefusedBeforeAnyPhaseRuns(t *testing.T) {
	r := resumeLaneIdentityCheckpoint(t, laneIdentityCheckpoint{stage: policy.FleetIntegrationEnforce, scope: []string{"item-a"}, legacy: true})

	if r.rc == 0 || r.rc == fleet.ExitDeferred {
		t.Fatalf("rc = %d, want a refusal for a lane checkpoint with no identity at enforce; stderr=%s", r.rc, r.stderr)
	}
	if r.calls != 0 {
		t.Errorf("%d phases ran after the refusal; a legacy lane checkpoint must never reach ship at enforce", r.calls)
	}
	if !strings.Contains(r.stderr, r.statePath) {
		t.Errorf("the refusal does not name the checkpoint %s:\n%s", r.statePath, r.stderr)
	}
}

func TestLaneIdentity_ALegacyLaneCheckpointAtShadowStillResumesAsBefore(t *testing.T) {
	r := resumeLaneIdentityCheckpoint(t, laneIdentityCheckpoint{stage: policy.FleetIntegrationShadow, scope: []string{"item-a"}, legacy: true})

	if r.calls == 0 || r.rc != fleet.ExitDeferred {
		t.Fatalf("rc=%d calls=%d stderr=%s; shadow keeps resuming a legacy file", r.rc, r.calls, r.stderr)
	}
}

func TestLaneIdentity_ASequentialCheckpointAtEnforceStillResumesWithoutAPool(t *testing.T) {
	r := resumeLaneIdentityCheckpoint(t, laneIdentityCheckpoint{stage: policy.FleetIntegrationEnforce})

	if r.calls == 0 || r.rc != fleet.ExitDeferred {
		t.Fatalf("rc=%d calls=%d stderr=%s; a sequential cycle is no lane and resumes at enforce", r.rc, r.calls, r.stderr)
	}
	if len(r.wired) != 1 || r.wired[0].pool != 0 || len(r.wired[0].scope) != 0 {
		t.Errorf("a sequential resume was wired with %+v, want no pool and no scope", r.wired)
	}
}

type laneIdentityProbeRunner struct {
	pool *int
}

func (laneIdentityProbeRunner) Name() string { return "scout" }
func (r laneIdentityProbeRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	*r.pool = checkpoint.Compose(core.CycleState{CycleID: 1}, checkpoint.ReasonOperatorRequest, 0, "", time.Now()).PoolID
	return core.PhaseResponse{}, errors.New("probe stop")
}

func TestLaneIdentity_TheCycleRunBindsItsPoolIdForEveryCheckpointItWritesAndReleasesItAtTheEnd(t *testing.T) {
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	root := t.TempDir()
	initLoopContractRepo(t, root)
	st := storage.New(filepath.Join(root, ".evolve"))
	if err := st.WriteState(context.Background(), core.State{}); err != nil {
		t.Fatal(err)
	}
	seen := -1
	old := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = old })
	wireOrchestratorDepsFn = func(string, string, io.Writer, routingRun) orchDeps {
		ledger := newFakeLedger()
		orch := core.NewOrchestrator(st, ledger, map[core.Phase]core.PhaseRunner{core.PhaseScout: laneIdentityProbeRunner{pool: &seen}})
		return orchDeps{Storage: st, Ledger: ledger, Orchestrator: orch}
	}
	var stdout, stderr bytes.Buffer

	runCycleRun([]string{"--project-root", root, "--goal-hash", "abcd1234", "--goal", "g", "--pool", "41"}, &stdout, &stderr)

	if seen != 41 {
		t.Errorf("a checkpoint composed while the lane ran carries pool %d, want 41; stderr=%s", seen, stderr.String())
	}
	after := checkpoint.Compose(core.CycleState{CycleID: 1}, checkpoint.ReasonOperatorRequest, 0, "", time.Now()).PoolID
	if after != 0 {
		t.Errorf("the pool %d stays bound after the lane ended", after)
	}
}

func TestLaneIdentity_AnEnforceLaneWiredWithAScopeAndNoPoolStopsBeforeAnyDispatch(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"fleet":{"integration":{"stage":"enforce"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{scope: []string{"item-a"}})
	if deps.CloseSignals != nil {
		t.Cleanup(deps.CloseSignals)
	}

	if deps.RoutingErr == nil || deps.RoutingErr.Error() != errEnforceLaneNoPool.Error() || deps.Orchestrator != nil {
		t.Fatalf("deps = err %v, orchestrator wired %v; want no orchestrator and %q", deps.RoutingErr, deps.Orchestrator != nil, errEnforceLaneNoPool)
	}
}

func TestLaneIdentity_AnEnforceLaneWiredWithAPoolAndAScopeBuildsItsOrchestrator(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"fleet":{"integration":{"stage":"enforce"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	deps := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{pool: 41, scope: []string{"item-a"}})
	if deps.CloseSignals != nil {
		t.Cleanup(deps.CloseSignals)
	}

	if deps.RoutingErr != nil || deps.Orchestrator == nil {
		t.Fatalf("deps = err %v, orchestrator wired %v; want a wired orchestrator for a lane that holds its pool", deps.RoutingErr, deps.Orchestrator != nil)
	}
}
