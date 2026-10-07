//go:build integration

package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/mintregistry"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type fleetLane struct {
	slug      string
	workspace string
	worktree  string
}

func writeLanePin(t *testing.T, workspace, slug string) {
	t.Helper()
	pin, err := json.Marshal(LaneScope{TodoIDs: []string{slug}, GoalHash: "g"})
	if err != nil {
		t.Fatal(err)
	}
	writeTreeFile(t, workspace, LaneScopeFile, string(pin))
}

func pinnedLaneWorkspace(t *testing.T, workspace, slug string) {
	t.Helper()
	writeLanePin(t, workspace, slug)
	writeTreeFile(t, workspace, evalsDir+slug+".md", "# Eval: "+slug+"\n- [code] `true`\n")
}

func twoLanesOnOneMainTree(t *testing.T) (string, fleetLane, fleetLane) {
	t.Helper()
	repo, laneTree := realWorktree(t)
	siblingTree := filepath.Join(t.TempDir(), "sibling-wt")
	gitInRepo(t, repo, "worktree", "add", "--detach", "-q", siblingTree, "HEAD")
	lane := fleetLane{slug: "router-silent-errors", workspace: RunWorkspacePath(repo, 1811), worktree: laneTree}
	sibling := fleetLane{slug: "phasespec-roots-silent-policy-error", workspace: RunWorkspacePath(repo, 1812), worktree: siblingTree}
	pinnedLaneWorkspace(t, lane.workspace, lane.slug)
	pinnedLaneWorkspace(t, sibling.workspace, sibling.slug)
	writeRunLease(t, sibling.workspace, time.Now())
	return repo, lane, sibling
}

func takePhaseSnapshot(t *testing.T, cr *cycleRun, dr *dispatchResult, phase Phase) {
	t.Helper()
	guard, before, err := cr.snapshotMainTree(phase)
	if err != nil {
		t.Fatalf("pre-phase snapshot: %v", err)
	}
	dr.treeGuard, dr.beforeDirty = guard, before
}

func (l fleetLane) phaseStart(t *testing.T, repo string, cycle int, phase Phase) (*cycleRun, *dispatchResult) {
	t.Helper()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	cr := &cycleRun{
		o:     o,
		ctx:   context.Background(),
		req:   CycleRequest{ProjectRoot: repo},
		cycle: cycle,
		cs:    CycleState{CycleID: cycle, RunID: "01lane", WorkspacePath: l.workspace, ActiveWorktree: l.worktree},
	}
	dr := &dispatchResult{phaseWorktree: l.worktree}
	takePhaseSnapshot(t, cr, dr, phase)
	return cr, dr
}

func (l fleetLane) evalPath() string { return evalsDir + l.slug + ".md" }

func requireTreeFile(t *testing.T, root, rel, want string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("%s must still be in %s: %v", rel, root, err)
	}
	if string(got) != want {
		t.Fatalf("%s in %s = %q, want %q", rel, root, got, want)
	}
}

func requireAbsent(t *testing.T, root, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
		t.Fatalf("%s must not be in %s (stat err=%v)", rel, root, err)
	}
}

func requireStaged(t *testing.T, worktree, rel string) {
	t.Helper()
	if diff := gitInRepo(t, worktree, "diff", "--cached", "--name-only"); !strings.Contains(diff, rel) {
		t.Fatalf("%s must be staged in the lane worktree; git diff --cached=%q", rel, diff)
	}
}

func TestRecoverBeforeReview_SiblingLaneEvalWrittenMidPhaseStaysInTheMainTree(t *testing.T) {
	t.Parallel()
	repo, lane, sibling := twoLanesOnOneMainTree(t)
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseTDD)
	writeTreeFile(t, repo, sibling.evalPath(), "sibling scout copy\n")

	if err := cr.recoverBeforeReview(PhaseTDD, dr); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	requireTreeFile(t, repo, sibling.evalPath(), "sibling scout copy\n")
	requireAbsent(t, lane.worktree, sibling.evalPath())
	if st := gitInRepo(t, lane.worktree, "status", "--porcelain", "-uall"); st != "" {
		t.Fatalf("the lane worktree must not carry the sibling's eval; status=%q", st)
	}
}

func TestRecoverBeforeReview_OwnLaneEvalLeakStillRelocates(t *testing.T) {
	t.Parallel()
	repo, lane, sibling := twoLanesOnOneMainTree(t)
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseTDD)
	writeTreeFile(t, repo, lane.evalPath(), "lane tdd eval\n")
	writeTreeFile(t, repo, sibling.evalPath(), "sibling scout copy\n")

	if err := cr.recoverBeforeReview(PhaseTDD, dr); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	requireAbsent(t, repo, lane.evalPath())
	requireTreeFile(t, lane.worktree, lane.evalPath(), "lane tdd eval\n")
	requireStaged(t, lane.worktree, lane.evalPath())
}

func TestRecoverBeforeReview_SourceLeakStillRelocatesBesideASibling(t *testing.T) {
	t.Parallel()
	repo, lane, sibling := twoLanesOnOneMainTree(t)
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseBuild)
	const leak = "go/internal/router/silent.go"
	writeTreeFile(t, repo, leak, "package router\n")
	writeTreeFile(t, repo, sibling.evalPath(), "sibling scout copy\n")

	if err := cr.recoverBeforeReview(PhaseBuild, dr); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	requireAbsent(t, repo, leak)
	requireTreeFile(t, lane.worktree, leak, "package router\n")
	requireStaged(t, lane.worktree, leak)
}

func TestRecoverBeforeReview_ScoutsOwnMainTreeEvalCopyLeavesTheMainTree(t *testing.T) {
	t.Parallel()
	repo, lane, _ := twoLanesOnOneMainTree(t)
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseScout)
	writeTreeFile(t, repo, lane.evalPath(), "scout belt-and-braces copy\n")

	if err := cr.recoverBeforeReview(PhaseScout, dr); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	requireAbsent(t, repo, lane.evalPath())
	requireTreeFile(t, lane.worktree, lane.evalPath(), "scout belt-and-braces copy\n")
}

func TestRecoverBeforeReview_SiblingEvalDuringScoutStaysForItsOwner(t *testing.T) {
	t.Parallel()
	repo, lane, sibling := twoLanesOnOneMainTree(t)
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseScout)
	writeTreeFile(t, repo, sibling.evalPath(), "sibling scout copy\n")

	if err := cr.recoverBeforeReview(PhaseScout, dr); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	requireTreeFile(t, repo, sibling.evalPath(), "sibling scout copy\n")
	requireAbsent(t, lane.worktree, sibling.evalPath())
}

func TestRecoverBeforeReview_RegisteredMintStaysInTheMainTree(t *testing.T) {
	t.Parallel()
	repo, lane, _ := twoLanesOnOneMainTree(t)
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseBuild)
	if err := mintregistry.Append(mintregistry.Path(repo), "xlane-mint", time.Now()); err != nil {
		t.Fatalf("registry append: %v", err)
	}
	writeMintSpec(t, repo, "xlane-mint", "")
	const mint = ".evolve/phases/xlane-mint/phase.json"

	if err := cr.recoverBeforeReview(PhaseBuild, dr); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	requireTreeFile(t, repo, mint, `{"name":"xlane-mint","optional":true}`)
	requireAbsent(t, lane.worktree, mint)
}

type phaseHookStorage struct {
	*fakeStorage
	phase Phase
	hook  func()
	fired bool
}

func (s *phaseHookStorage) WriteCycleState(ctx context.Context, cs CycleState) error {
	if !s.fired && cs.Phase == string(s.phase) {
		s.fired = true
		s.hook()
	}
	return s.fakeStorage.WriteCycleState(ctx, cs)
}

func TestRunCycle_SiblingEvalWrittenDuringTDDIsNeitherChargedNorClaimed(t *testing.T) {
	t.Parallel()
	root := initAuditLeakRepo(t)
	writeLanePin(t, RunWorkspacePath(root, 1), "router-silent-errors")
	sibling := RunWorkspacePath(root, 2)
	pinnedLaneWorkspace(t, sibling, "phasespec-roots-silent-policy-error")
	writeRunLease(t, sibling, time.Now())
	const siblingEval = ".evolve/evals/phasespec-roots-silent-policy-error.md"
	runners := buildRunners(nil)
	runners[PhaseTDD] = &tddLeakRunner{name: string(PhaseTDD), onRun: func() {
		writeTreeFile(t, root, siblingEval, "sibling scout copy\n")
	}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorktreeProvisioner(gitWorktree{}))

	res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})

	if err != nil {
		t.Fatalf("a sibling's eval must not abort this lane's cycle: %v", err)
	}
	if !slices.Contains(res.PhasesRun, PhaseShip) {
		t.Fatalf("the cycle must run to ship; phases=%v", res.PhasesRun)
	}
	requireTreeFile(t, root, siblingEval, "sibling scout copy\n")
}

func TestRunCycle_PhaseBaselineExcludesASiblingWriteBeforeThePhase(t *testing.T) {
	t.Parallel()
	root := initAuditLeakRepo(t)
	const siblingLeak = "go/internal/sibling/leak.go"
	st := &phaseHookStorage{fakeStorage: &fakeStorage{}, phase: PhaseTDD, hook: func() {
		writeTreeFile(t, root, siblingLeak, "package sibling\n")
	}}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil), WithWorktreeProvisioner(gitWorktree{}))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"}); err != nil {
		t.Fatalf("cycle failed: %v", err)
	}

	if !st.fired {
		t.Fatal("the sibling write never happened: the harness did not reach tdd")
	}
	requireTreeFile(t, root, siblingLeak, "package sibling\n")
}

func TestRunCycleFromPhase_PhaseBaselineExcludesASiblingWriteBeforeThePhase(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	const siblingLeak = "go/internal/sibling/leak.go"
	st := &phaseHookStorage{
		fakeStorage: &fakeStorage{
			state:      State{LastCycleNumber: 9},
			cycleState: CycleState{CycleID: 9, WorkspacePath: t.TempDir(), ActiveWorktree: wt},
		},
		phase: PhaseTDD,
		hook:  func() { writeTreeFile(t, repo, siblingLeak, "package sibling\n") },
	}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))

	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: repo}, &ResumePoint{Phase: string(PhaseTriage), CycleID: 9})

	if !st.fired {
		t.Fatalf("the sibling write never happened: the resume did not reach tdd (err=%v)", err)
	}
	requireTreeFile(t, repo, siblingLeak, "package sibling\n")
	requireAbsent(t, wt, siblingLeak)
}

func TestRunCycleFromPhase_ALeakDuringTheResumedPhaseIsStillRelocated(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	const leak = "go/internal/router/resumed.go"
	var leaked atomic.Bool
	runners := buildRunners(nil)
	runners[PhaseTDD] = &tddLeakRunner{name: string(PhaseTDD), onRun: func() {
		writeTreeFile(t, repo, leak, "package router\n")
		leaked.Store(true)
	}}
	st := &fakeStorage{
		state:      State{LastCycleNumber: 9},
		cycleState: CycleState{CycleID: 9, WorkspacePath: t.TempDir(), ActiveWorktree: wt},
	}
	o := NewOrchestrator(st, &fakeLedger{}, runners)

	_, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: repo}, &ResumePoint{Phase: string(PhaseTriage), CycleID: 9})

	if !leaked.Load() {
		t.Fatalf("the resumed tdd never ran (err=%v)", err)
	}
	requireAbsent(t, repo, leak)
	if err != nil && strings.Contains(err.Error(), "worktree-leak recovery failed") {
		t.Fatalf("the resumed phase's own leak must be recovered: %v", err)
	}
}

func TestMaybeRemediate_FixDispatchBaselineExcludesASiblingWriteDuringTheGate(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	workspace := t.TempDir()
	runners := buildRunners(nil)
	runners[PhaseBuild] = remediationNoopBuilder{}
	gate := &scriptedRunner{name: PhaseTDD, verdicts: []string{VerdictPASS}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners,
		WithWorkflowConfig(policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{string(PhaseTDD)}}))
	cr := &cycleRun{
		o: o, ctx: context.Background(), req: CycleRequest{ProjectRoot: repo}, cycle: 1811,
		cs:          CycleState{CycleID: 1811, WorkspacePath: workspace, ActiveWorktree: wt},
		retryConfig: o.retryConfig, workflowConfig: o.workflowConfig,
	}
	dr := dispatchResult{
		resp:          PhaseResponse{Phase: string(PhaseTDD), Verdict: VerdictFAIL, ArtifactsDir: workspace},
		attemptCount:  1,
		phaseWorktree: wt,
		runner:        gate,
		phaseReq:      PhaseRequest{Cycle: 1811, ProjectRoot: repo, Workspace: workspace, Worktree: wt},
	}
	takePhaseSnapshot(t, cr, &dr, PhaseTDD)
	const siblingLeak = "go/internal/sibling/leak.go"
	writeTreeFile(t, repo, siblingLeak, "package sibling\n")

	if _, err := cr.maybeRemediate(PhaseTDD, &dr); err != nil {
		t.Fatalf("remediation failed: %v", err)
	}

	if gate.calls != 1 {
		t.Fatalf("the gate must re-run after the fix dispatch; calls=%d", gate.calls)
	}
	requireTreeFile(t, repo, siblingLeak, "package sibling\n")
	requireAbsent(t, wt, siblingLeak)
}

func TestRecoverBeforeReview_SiblingEditToACommittedEvalIsNotStolen(t *testing.T) {
	t.Parallel()
	repo, lane, sibling := twoLanesOnOneMainTree(t)
	writeTreeFile(t, repo, sibling.evalPath(), "committed by an earlier cycle\n")
	gitInRepo(t, repo, "add", "-A")
	gitInRepo(t, repo, "commit", "-q", "-m", "earlier cycle eval")
	gitInRepo(t, lane.worktree, "checkout", "-q", "--detach", gitInRepo(t, repo, "rev-parse", "HEAD"))
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseTDD)
	writeTreeFile(t, repo, sibling.evalPath(), "sibling re-materialized copy\n")

	if err := cr.recoverBeforeReview(PhaseTDD, dr); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	requireTreeFile(t, repo, sibling.evalPath(), "sibling re-materialized copy\n")
	if st := gitInRepo(t, lane.worktree, "status", "--porcelain", "-uall"); st != "" {
		t.Fatalf("the lane worktree must not carry the sibling's edit; status=%q", st)
	}
}

func TestRecoverBeforeReview_OwnEvalLeakNeverOverwritesTheWorktreeCopy(t *testing.T) {
	t.Parallel()
	repo, lane, _ := twoLanesOnOneMainTree(t)
	cr, dr := lane.phaseStart(t, repo, 1811, PhaseBuild)
	writeTreeFile(t, lane.worktree, lane.evalPath(), "fuller TDD eval\n")
	writeTreeFile(t, repo, lane.evalPath(), "scout's shorter copy\n")

	if err := cr.recoverBeforeReview(PhaseBuild, dr); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	requireTreeFile(t, lane.worktree, lane.evalPath(), "fuller TDD eval\n")
	requireAbsent(t, repo, lane.evalPath())
}

func TestGuardChargesThisLanesOwnEvalWhileItsOwnLeaseIsLive(t *testing.T) {
	t.Parallel()
	root := initAuditLeakRepo(t)
	const ownSlug = "router-silent-errors"
	writeLanePin(t, RunWorkspacePath(root, 1), ownSlug)
	ownEval := evalsDir + ownSlug + ".md"
	runners := buildRunners(nil)
	runners[PhaseTriage] = &tddLeakRunner{name: string(PhaseTriage), onRun: func() {
		writeTreeFile(t, root, ownEval, "triage wrote the lane's own eval into the main tree\n")
	}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, runners, WithWorktreeProvisioner(gitWorktree{}))

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g"})

	if err == nil || !strings.Contains(err.Error(), "tree-diff") || !strings.Contains(err.Error(), ownEval) {
		t.Fatalf("this lane's own keyed leak must be charged even though its own run is live; got: %v", err)
	}
}

func TestDefaultGitDirtyPaths_ReportsAFailedGitStatus(t *testing.T) {
	t.Parallel()
	notARepo := t.TempDir()

	paths, err := defaultGitDirtyPaths(context.Background(), notARepo)

	if err == nil {
		t.Fatalf("a failed git status must be reported, not read as a clean tree; got paths %v", paths)
	}
}

func TestRecoverPhaseLeak_SkipsAPhaseWithNoPrePhaseSnapshot(t *testing.T) {
	t.Parallel()
	repo, wt := realWorktree(t)
	writeTreeFile(t, repo, "operator-notes.md", "dirty before the phase, unknown to recovery\n")
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))

	err := o.recoverPhaseLeak(context.Background(), phaseLeakScope{
		projectRoot: repo,
		cycleState:  CycleState{CycleID: 7, WorkspacePath: t.TempDir(), ActiveWorktree: wt},
		phase:       PhaseBuild,
	})

	if err != nil {
		t.Fatalf("a missing snapshot degrades recovery, it does not fail the phase: %v", err)
	}
	requireTreeFile(t, repo, "operator-notes.md", "dirty before the phase, unknown to recovery\n")
	requireAbsent(t, wt, "operator-notes.md")
}
