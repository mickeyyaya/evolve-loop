package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func poolCoordinator(t *testing.T, st core.Storage, stderr io.Writer) *loopBatchCoordinator {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	b := &loopBatchCoordinator{ctx: context.Background(), cfg: loopConfig{ProjectRoot: root, EvolveDir: evolveDir}, result: &loopResult{}, stdout: io.Discard, stderr: stderr, loadGraph: noModuleGraph}
	b.deps.Storage = st
	return b
}

func lastAllocated(t *testing.T, st core.Storage) int {
	t.Helper()
	s, err := st.ReadState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return s.LastAllocatedCycleNumber
}

func TestCycleRunPoolArgs_APoolAddsTheFlagAndNoPoolKeepsTheBaseline(t *testing.T) {
	t.Parallel()
	base := cycleRunArgs("abc123", "", "goal", false, "/proj")
	launch := cycleLaunch{GoalHash: "abc123", GoalText: "goal", Root: "/proj"}
	if got := cycleRunPoolArgs(launch, ""); strings.Join(got, " ") != strings.Join(base, " ") {
		t.Errorf("pool 0 = %q, want the baseline %q", got, base)
	}
	launch.Pool = 41
	got := strings.Join(cycleRunPoolArgs(launch, ""), " ")
	if want := strings.Join(base, " ") + " --pool 41"; got != want {
		t.Errorf("pool 41 = %q, want %q", got, want)
	}
}

func TestParseCycleRunFlags_PoolIsReadAndANegativePoolIsRefused(t *testing.T) {
	t.Parallel()
	f, ok := parseCycleRunFlags([]string{"--goal-hash", "abcd1234", "--pool", "41"}, io.Discard)
	if !ok || f.pool != 41 {
		t.Fatalf("parse = %+v, %v, want pool 41", f, ok)
	}
	var stderr bytes.Buffer
	if _, ok := parseCycleRunFlags([]string{"--goal-hash", "abcd1234", "--pool", "-3"}, &stderr); ok || stderr.String() != "evolve cycle run: --pool -3 is negative\n" {
		t.Errorf("negative pool: ok=%v stderr=%q", ok, stderr.String())
	}
}

func TestRunCycleRun_PoolReachesTheWiringAsTheRoutingRunPool(t *testing.T) {
	old := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = old })
	var got routingRun
	wireOrchestratorDepsFn = func(_, _ string, _ io.Writer, run routingRun) orchDeps {
		got = run
		return orchDeps{RoutingErr: errors.New("stop here")}
	}
	var stderr bytes.Buffer
	rc := runCycleRun([]string{"--project-root", t.TempDir(), "--goal-hash", "abcd1234", "--pool", "41"}, io.Discard, &stderr)
	if rc != exitRoutingRefused || got.pool != 41 {
		t.Fatalf("rc=%d pool=%d, want the routing refusal and pool 41", rc, got.pool)
	}
}

func TestRunCycleRun_AnEnforceFleetLaneWithNoPoolStopsAtStartup(t *testing.T) {
	t.Setenv(ipcenv.FleetKey, "1")
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"fleet":{"integration":{"stage":"enforce"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	rc := runCycleRun([]string{"--project-root", root, "--goal-hash", "abcd1234"}, io.Discard, &stderr)
	want := "evolve cycle run: " + errEnforceLaneNoPool.Error() + "\n"
	if rc != exitRoutingRefused || stderr.String() != want {
		t.Fatalf("rc=%d stderr=%q, want exit %d and %q", rc, stderr.String(), exitRoutingRefused, want)
	}
}

func TestMintPoolID_APoolOrWaveRouteMintsOneNumberAndASequentialRouteMintsNone(t *testing.T) {
	t.Parallel()
	pool := policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "pool"}
	wave := policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "wave"}
	sequential := policy.FleetConfig{Count: 1, PlanSource: "triage", Scheduling: "pool"}
	for _, tc := range []struct {
		name string
		fc   policy.FleetConfig
		want int
	}{{"pool", pool, 1}, {"wave", wave, 1}, {"sequential", sequential, 0}} {
		st := storage.New(filepath.Join(t.TempDir(), ".evolve"))
		b := poolCoordinator(t, st, io.Discard)
		id := b.mintPoolID(tc.fc)
		if id != tc.want || lastAllocated(t, st) != tc.want {
			t.Errorf("%s: id=%d allocated=%d, want both %d", tc.name, id, lastAllocated(t, st), tc.want)
		}
	}
}

func TestMintPoolID_TwoPoolsGetTwoIncreasingIDsAboveEveryLaneNumber(t *testing.T) {
	t.Parallel()
	st := storage.New(filepath.Join(t.TempDir(), ".evolve"))
	b := poolCoordinator(t, st, io.Discard)
	fc := policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "pool"}
	first, second := b.mintPoolID(fc), b.mintPoolID(fc)
	if first < 1 || second != first+1 {
		t.Fatalf("pool ids = %d, %d, want consecutive numbers from 1", first, second)
	}
}

func TestMintPoolID_AStorageThatCannotAllocateWarnsAndLeavesTheLanesWithoutAPool(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	b := poolCoordinator(t, &fixtures.FakeStorage{}, &stderr)
	id := b.mintPoolID(policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "pool"})
	want := "[loop] WARN: fleet: the state store cannot allocate a pool id; the lanes of this pool run without one\n"
	if id != 0 || stderr.String() != want {
		t.Fatalf("id=%d stderr=%q, want 0 and %q", id, stderr.String(), want)
	}
}

type failingAllocator struct{ core.Storage }

func (failingAllocator) UpdateState(context.Context, func(*core.State)) (core.State, error) {
	return core.State{}, errors.New("disk full")
}

func TestMintPoolID_AFailedAllocationWarnsWithTheCause(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	b := poolCoordinator(t, failingAllocator{&fixtures.FakeStorage{}}, &stderr)
	id := b.mintPoolID(policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "wave"})
	want := "[loop] WARN: fleet: pool id not allocated (allocate cycle number: disk full); the lanes of this pool run without one\n"
	if id != 0 || stderr.String() != want {
		t.Fatalf("id=%d stderr=%q, want 0 and %q", id, stderr.String(), want)
	}
}

func TestDispatchFleetIteration_MintsOneIDForAPoolAndNoneForASequentialConfig(t *testing.T) {
	t.Parallel()
	root := wave20ShapedRoot(t)
	evolveDir := filepath.Join(root, ".evolve")
	st := storage.New(evolveDir)
	b := poolCoordinator(t, st, io.Discard)
	b.cfg = loopConfig{ProjectRoot: root, EvolveDir: evolveDir}
	b.deps.Signals = newRootSignalCenter(root, evolveDir, io.Discard)
	var starvation fleet.StarvationTracker
	sequential := policy.FleetConfig{Count: 1}
	if d := b.dispatchFleetIteration(0, sequential, "/nonexistent/evolve", &starvation); d.flow != batchProceed {
		t.Fatalf("sequential flow = %v", d.flow)
	}
	if got := lastAllocated(t, st); got != 0 {
		t.Fatalf("a sequential config burned cycle number %d", got)
	}
	wave := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "wave", StarvationK: 2, StarvationWeight: 0.9}
	b.dispatchFleetIteration(0, wave, "/nonexistent/evolve", &starvation)
	if got := lastAllocated(t, st); got != 1 {
		t.Fatalf("one wave dispatch allocated %d numbers, want 1", got)
	}
}

type laneShipRunner struct {
	tb    testing.TB
	real  core.PhaseRunner
	lane  shiptest.Lane
	state []byte
}

func (r laneShipRunner) Name() string { return string(core.PhaseShip) }

func (r laneShipRunner) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	statePath := filepath.Join(r.lane.Repo, ".evolve", "cycle-state.json")
	if err := os.WriteFile(statePath, r.state, 0o644); err != nil {
		return core.PhaseResponse{}, err
	}
	shiptest.SeedAudit(r.tb, r.lane.Repo, "PASS")
	if err := r.sealAuditWithSentinel(); err != nil {
		return core.PhaseResponse{}, err
	}
	req.Cycle, req.RunID, req.AuditRound = shiptest.LandingCycle, shiptest.LandingRunID, 1
	req.ProjectRoot, req.Workspace, req.Worktree, req.WorktreeBaseSHA = r.lane.Repo, r.lane.WS, r.lane.WT, r.lane.Base
	req.ExplanationDocumentationVersion, req.BuildExplanation = explanationdocs.CurrentContractVersion, r.lane.View
	req.Context = map[string]string{"commit_message": "evolve-cycle 1830"}
	req.Env = map[string]string{"EVOLVE_PLUGIN_ROOT": r.lane.Repo}
	return r.real.Run(ctx, req)
}

func (r laneShipRunner) sealAuditWithSentinel() error {
	report := filepath.Join(r.lane.WS, "audit-report.md")
	body := "<!-- challenge-token: testtoken123 -->\n# Audit Report\n\n" + phasecontract.RenderVerdictSentinel("audit", "PASS") + "\n"
	if err := os.WriteFile(report, []byte(body), 0o644); err != nil {
		return err
	}
	shiptest.SealTestPredicateEvidence(r.tb, acssuite.EvidenceIdentity{Cycle: shiptest.LandingCycle, RunID: shiptest.LandingRunID, Round: 1, TreeSHA: r.lane.Audited}, report)
	ledgerPath := filepath.Join(r.lane.Repo, ".evolve", "ledger.jsonl")
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		return err
	}
	var rewritten []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return err
		}
		if entry["role"] == "auditor" {
			entry["artifact_sha256"] = shiptest.MustHashFile(r.tb, report)
		}
		out, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		rewritten = append(rewritten, string(out))
	}
	return os.WriteFile(ledgerPath, []byte(strings.Join(rewritten, "\n")+"\n"), 0o644)
}

func wireProductionShipOnLane(t *testing.T, lane shiptest.Lane) {
	t.Helper()
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	state, err := os.ReadFile(filepath.Join(lane.Repo, ".evolve", "cycle-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	old := wireOrchestratorDepsFn
	t.Cleanup(func() { wireOrchestratorDepsFn = old })
	wireOrchestratorDepsFn = func(root, evolveDir string, console io.Writer, run routingRun) orchDeps {
		d := wireOrchestratorDeps(root, evolveDir, console, run)
		if d.RoutingErr != nil {
			return d
		}
		runners := map[core.Phase]core.PhaseRunner{}
		for _, p := range []core.Phase{core.PhaseIntent, core.PhaseScout, core.PhaseTriage, core.PhaseTDD, core.PhaseBuildPlanner, core.PhaseBuild, core.PhaseAudit, core.PhaseRetro} {
			runners[p] = noopRunner{name: string(p)}
		}
		runners[core.PhaseShip] = laneShipRunner{tb: t, real: d.Runners[core.PhaseShip], lane: lane, state: state}
		d.Orchestrator = core.NewOrchestrator(d.Storage, newFakeLedger(), runners, core.WithRetryConfig(policy.RetryConfig{PhaseMaxAttempts: 1}))
		return d
	}
}

func runShippedFleetLane(t *testing.T, extra ...string) (shiptest.Lane, int, string) {
	t.Helper()
	lane := shiptest.LaneReadyToLand(t)
	exclude := filepath.Join(lane.Repo, ".git", "info", "exclude")
	if err := os.WriteFile(exclude, []byte("go/bin/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wireProductionShipOnLane(t, lane)
	t.Setenv(ipcenv.FleetKey, "1")
	args := append([]string{"--project-root", lane.Repo, "--goal-hash", "abcd1234"}, extra...)
	var stdout, stderr bytes.Buffer
	rc := runCycleRun(args, &stdout, &stderr)
	return lane, rc, stdout.String() + stderr.String()
}

func TestRunCycleRun_AFleetLaneWithAPoolWritesTheDeliverableRecordAtShipAndOneWithoutWritesNone(t *testing.T) {
	lane, rc, out := runShippedFleetLane(t, "--pool", "41")
	if rc != 0 {
		t.Fatalf("pooled lane rc=%d, want 0\n%s", rc, out)
	}
	rec, err := integration.ReadRecord(integration.RecordPath(integration.WaveDir(lane.Repo, "41"), shiptest.LandingCycle))
	if err != nil {
		t.Fatalf("no deliverable record under .evolve/waves/integration-41: %v\n%s", err, out)
	}
	if rec.Wave != "41" || rec.Cycle != shiptest.LandingCycle {
		t.Errorf("record identity = wave %q cycle %d, want wave 41 cycle %d", rec.Wave, rec.Cycle, shiptest.LandingCycle)
	}

	bare, rc, out := runShippedFleetLane(t)
	if rc != 0 {
		t.Fatalf("unpooled lane rc=%d, want 0\n%s", rc, out)
	}
	if matches, _ := filepath.Glob(filepath.Join(bare.Repo, ".evolve", "waves", "integration-*")); len(matches) != 0 {
		t.Errorf("a lane with no --pool wrote %v, want no record", matches)
	}
}

func TestRunCycleRun_AnEnforceFleetLaneWithNoPoolRefusesAtStartupWithTheWholeTextAndShipsNothing(t *testing.T) {
	lane := shiptest.LaneReadyToLand(t)
	wireProductionShipOnLane(t, lane)
	t.Setenv(ipcenv.FleetKey, "1")
	policyPath := filepath.Join(lane.Repo, ".evolve", "policy.json")
	if err := os.WriteFile(policyPath, []byte(`{"fleet":{"integration":{"stage":"enforce"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	rc := runCycleRun([]string{"--project-root", lane.Repo, "--goal-hash", "abcd1234"}, &stdout, &stderr)
	want := "evolve cycle run: fleet.integration.stage=enforce needs a pool id (--pool), and this fleet lane has none; start the lane through evolve loop\n"
	if rc != exitRoutingRefused || stderr.String() != want || stdout.Len() != 0 {
		t.Fatalf("rc=%d stderr=%q stdout=%q, want exit %d, exactly %q and no output", rc, stderr.String(), stdout.String(), exitRoutingRefused, want)
	}
	if matches, _ := filepath.Glob(filepath.Join(lane.Repo, ".evolve", "waves", "integration-*")); len(matches) != 0 {
		t.Errorf("a refused lane wrote %v, want no record", matches)
	}
}
