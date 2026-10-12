package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type runCycleWritingRunnerE4 struct {
	name string
	run  func(PhaseRequest) error
}

func (r *runCycleWritingRunnerE4) Name() string { return r.name }
func (r *runCycleWritingRunnerE4) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	if r.run != nil {
		if err := r.run(req); err != nil {
			return PhaseResponse{}, err
		}
	}
	return PhaseResponse{Phase: r.name, Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func TestRunCycle_ABlockedActivationDirectoryStopsTheCycleBeforeAnyPhase(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".evolve", "build-explanation-contracts"), "a file where the marker directory belongs\n")
	runners := buildRunners(nil)
	o := NewOrchestrator(&fakeStorage{enforceExplanationContract: true}, &fakeLedger{}, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root})

	if err == nil || !strings.Contains(err.Error(), "activate Build explanation contract:") {
		t.Fatalf("err = %v, want the activation failure named", err)
	}
	if calls := runners[PhaseScout].(*fakeRunner).calls; calls != 0 {
		t.Fatalf("scout ran %d times; no phase may start without the contract marker", calls)
	}
}

func TestRunCycle_ABuildCannotStartWithoutASealedWorktreeContext(t *testing.T) {
	t.Parallel()

	runners := buildRunners(nil)
	o := NewOrchestrator(&fakeStorage{enforceExplanationContract: true}, &fakeLedger{}, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})

	if err == nil || !strings.Contains(err.Error(), "seal Build explanation context: active Build explanation contract requires worktree and base SHA") {
		t.Fatalf("err = %v, want the seal refusal", err)
	}
	if calls := runners[PhaseBuild].(*fakeRunner).calls; calls != 0 {
		t.Fatalf("build ran %d times without a sealed context", calls)
	}
	if calls := runners[PhaseScout].(*fakeRunner).calls; calls != 1 {
		t.Fatalf("scout ran %d times, want the spine to reach build first", calls)
	}
}

func TestRunCycle_AnUnwritableInteractionSummaryWarnsAndTheCycleStillCompletes(t *testing.T) {
	runners := buildRunners(nil)
	runners[PhaseScout] = &runCycleWritingRunnerE4{name: "scout", run: func(req PhaseRequest) error {
		writeFile(t, filepath.Join(req.Workspace, "scout-interactions.ndjson"), `{"kind":"clarify","result":"answered"}`+"\n")
		return os.MkdirAll(filepath.Join(req.Workspace, "interaction-summary.json"), 0o755)
	}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners)

	var err error
	log := captureStderr(t, func() { _, err = o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()}) })

	if err != nil {
		t.Fatalf("RunCycle: %v, a lost rollup must not fail the cycle", err)
	}
	if !strings.Contains(log, "WARN interaction-summary write:") {
		t.Fatalf("the lost rollup must be loud:\n%s", log)
	}
}

func evaluateBatchRunCycleE4(t *testing.T, first, second PhaseRunner) (*Orchestrator, map[Phase]PhaseRunner) {
	t.Helper()
	runners := buildRunners(nil)
	runners[Phase("check-a")], runners[Phase("check-b")] = first, second
	catalog, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{
		{Name: "check-a", Optional: true, After: "build", Role: string(phasespec.RoleEvaluate)},
		{Name: "check-b", Optional: true, After: "build", Role: string(phasespec.RoleEvaluate)},
	})
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	cfg := shadowCfg(config.StageAdvisory)
	cfg.Mode = config.ModeDynamicLLM
	cfg.ParallelEvaluate = config.StageEnforce
	cfg.ParallelEvaluateConcurrency = 2
	cfg.Order = []string{"scout", "triage", "tdd", "build-planner", "build", "check-a", "check-b", "audit", "ship"}
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "tdd", Run: true}, {Phase: "build", Run: true},
		{Phase: "check-a", Run: true}, {Phase: "check-b", Run: true}, {Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithRouting(cfg, router.StaticPreset{}), WithCatalog(catalog), WithPlanner(&fixedPlanner{plan: plan}))
	return o, runners
}

func TestRunCycle_AContiguousEvaluateRunIsDispatchedAsOneBatchAndTheCycleShips(t *testing.T) {
	t.Parallel()

	a := &fakeRunner{name: "check-a"}
	b := &fakeRunner{name: "check-b"}
	o, runners := evaluateBatchRunCycleE4(t, a, b)

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true})

	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if a.calls != 1 || b.calls != 1 {
		t.Fatalf("batch members ran a=%d b=%d times, want exactly once each", a.calls, b.calls)
	}
	if runners[PhaseAudit].(*fakeRunner).calls != 1 || runners[PhaseShip].(*fakeRunner).calls != 1 {
		t.Fatal("the walk must continue past the batch to audit and ship")
	}
	seen := map[Phase]bool{}
	for _, p := range res.PhasesRun {
		seen[p] = true
	}
	if !seen["check-a"] || !seen["check-b"] {
		t.Fatalf("PhasesRun = %v, want both batch members recorded", res.PhasesRun)
	}
}

func TestRunCycle_AFailingBatchMemberAbortsTheCycleNamingThePhaseAndNeverReachesAudit(t *testing.T) {
	t.Parallel()

	a := &fakeRunner{name: "check-a"}
	b := &fakeRunner{name: "check-b", failErr: errPlainE4("check-b: deliverable forged challenge token"), failUntil: 99}
	o, runners := evaluateBatchRunCycleE4(t, a, b)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true})

	if err == nil || !strings.Contains(err.Error(), "check-b") {
		t.Fatalf("err = %v, want the failing batch member named", err)
	}
	if runners[PhaseAudit].(*fakeRunner).calls != 0 {
		t.Fatal("audit must not run after an aborted batch")
	}
}

type errPlainE4 string

func (e errPlainE4) Error() string { return string(e) }
