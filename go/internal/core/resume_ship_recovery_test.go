package core

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	staleBaseCycle = 42
	staleBaseRunID = "run-42"
	laneFile       = "lane.txt"
)

type staleBaseLane struct {
	repo      *gittest.Repo
	root      string
	workspace string
	base      string
	laneTip   string
	mainTip   string
}

func newStaleBaseLane(t *testing.T, peerFile string) staleBaseLane {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Git("config", "commit.gpgsign", "false")
	repo.Write("shared.txt", "base\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	repo.Git("checkout", "-q", "-b", "cycle")
	repo.Write(laneFile, "lane change\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "ship cycle 42")
	laneTip := repo.Git("rev-parse", "HEAD")
	repo.Git("checkout", "-q", "main")
	repo.Write(peerFile, "peer change\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "a peer lands first")
	mainTip := repo.Git("rev-parse", "HEAD")
	repo.Git("checkout", "-q", "cycle")
	root := t.TempDir()
	workspace := RunWorkspacePath(root, staleBaseCycle)
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	return staleBaseLane{repo: repo, root: root, workspace: workspace, base: base, laneTip: laneTip, mainTip: mainTip}
}

type orderedRunner struct {
	PhaseRunner
	name    string
	order   *[]string
	observe func(context.Context)
}

func (r orderedRunner) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	*r.order = append(*r.order, r.name)
	if r.observe != nil {
		r.observe(ctx)
	}
	return r.PhaseRunner.Run(ctx, req)
}

type landingShip struct {
	*fakeRunner
	order   *[]string
	observe func(context.Context)
}

func (r landingShip) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	*r.order = append(*r.order, "ship")
	if r.observe != nil {
		r.observe(ctx)
	}
	resp, err := r.fakeRunner.Run(ctx, req)
	if err != nil {
		return resp, err
	}
	return resp, os.WriteFile(filepath.Join(req.Workspace, "ship-binding.json"), []byte("{}\n"), 0o644)
}

type staleBaseScenario struct {
	checkpointBase string
	noBase         bool
	shipErr        error
	shipFailures   int
	options        []Option
	observeShip    func(context.Context)
	observeAudit   func(context.Context)
}

type staleBaseOutcome struct {
	order    []string
	ship     *fakeRunner
	audit    *fakeRunner
	debugger *fakeRunner
	storage  *fakeStorage
	signals  *[]signalcenter.Event
	result   CycleResult
	err      error
}

func (l staleBaseLane) resumeShip(t *testing.T, sc staleBaseScenario) staleBaseOutcome {
	t.Helper()
	switch {
	case sc.noBase:
		sc.checkpointBase = ""
	case sc.checkpointBase == "":
		sc.checkpointBase = l.base
	}
	out := staleBaseOutcome{
		ship:     &fakeRunner{name: "ship", failErr: sc.shipErr, failUntil: sc.shipFailures},
		audit:    &fakeRunner{name: "audit"},
		debugger: &fakeRunner{name: "debugger"},
		storage: &fakeStorage{
			state: State{LastCycleNumber: staleBaseCycle},
			cycleState: CycleState{
				CycleID: staleBaseCycle, RunID: staleBaseRunID, WorkspacePath: l.workspace,
				ActiveWorktree: l.repo.Dir, WorktreeBaseSHA: sc.checkpointBase,
				CompletedPhases: []string{"scout", "triage", "build", "audit"},
			},
		},
	}
	runners := buildRunners(nil)
	runners[PhaseShip] = landingShip{fakeRunner: out.ship, order: &out.order, observe: sc.observeShip}
	runners[PhaseAudit] = orderedRunner{PhaseRunner: out.audit, name: "audit", order: &out.order, observe: sc.observeAudit}
	runners[PhaseDebugger] = orderedRunner{PhaseRunner: out.debugger, name: "debugger", order: &out.order}
	center, events := recordingCenter()
	out.signals = events
	options := append([]Option{WithWorktreeProvisioner(&fakeWorktree{path: l.repo.Dir}), WithSignalCenter(center)}, sc.options...)
	o := NewOrchestrator(out.storage, &fakeLedger{}, runners, options...)
	out.result, out.err = o.RunCycleFromPhase(context.Background(),
		CycleRequest{ProjectRoot: l.root, GoalHash: "stale-base"},
		&ResumePoint{Phase: string(PhaseShip), CycleID: staleBaseCycle})
	return out
}

func landingDiverged() *ShipError {
	return NewShipError(CodeGitFFMergeDiverged, ShipClassPrecondition, StageAtomicShip,
		"ship: ff-merge cycle into main failed (rc=1; divergent history)",
		shiperr.GitRCKey, "1", shiperr.CycleBranchKey, "cycle", shiperr.BranchKey, "main")
}

func colliderRefusal() *ShipError {
	return NewShipError(CodeGitFFMergeDiverged, ShipClassPrecondition, StageAtomicShip,
		"ship: untracked files in main working tree would be overwritten by merge: lane.txt",
		"colliders", laneFile)
}

func requireOrder(t *testing.T, got []string, want ...string) {
	t.Helper()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("phases dispatched = %v, want %v", got, want)
	}
}

func requireShipErrorSurfaced(t *testing.T, out staleBaseOutcome, code ShipErrorCode) {
	t.Helper()
	if out.err == nil || !strings.Contains(out.err.Error(), string(code)) {
		t.Fatalf("err = %v, want the %s ship error to end the cycle", out.err, code)
	}
	if out.result.FinalVerdict != VerdictFAIL {
		t.Fatalf("FinalVerdict = %q, want FAIL", out.result.FinalVerdict)
	}
}

func TestRunCycleFromPhase_AShipRefusedOnAStaleBaseRebasesAndReAuditsBeforeShipping(t *testing.T) {
	t.Parallel()

	lane := newStaleBaseLane(t, "peer.txt")

	out := lane.resumeShip(t, staleBaseScenario{shipErr: landingDiverged(), shipFailures: 1})

	if out.err != nil {
		t.Fatalf("a resumed cycle whose base is an ancestor of main must recover, got %v", out.err)
	}
	requireOrder(t, out.order, "ship", "audit", "ship")
	if out.result.FinalVerdict != VerdictPASS {
		t.Fatalf("FinalVerdict = %q, want PASS", out.result.FinalVerdict)
	}
	if got := out.audit.requests[0].Context[CtxKeyShipErrorCode]; got != string(CodeGitFleetRebaseNeeded) {
		t.Fatalf("the re-audit saw ship_error_code %q, want %q so it audits the rebased tree knowing why", got, CodeGitFleetRebaseNeeded)
	}
	if got := reAuditRecoveryCode(out.storage); got != string(CodeGitFleetRebaseNeeded) {
		t.Fatalf("the cycle state written before the re-audit carries ShipRecoveryCode %q, want %q", got, CodeGitFleetRebaseNeeded)
	}
}

func reAuditRecoveryCode(st *fakeStorage) string {
	for _, cs := range st.cycleStateLog {
		if cs.Phase == string(PhaseAudit) {
			return cs.ShipRecoveryCode
		}
	}
	return ""
}

func TestRunCycleFromPhase_TheRecoveryRebasesTheLaneOntoMainAndKeepsItsChange(t *testing.T) {
	t.Parallel()

	lane := newStaleBaseLane(t, "peer.txt")

	out := lane.resumeShip(t, staleBaseScenario{shipErr: landingDiverged(), shipFailures: 1})

	if out.err != nil {
		t.Fatal(out.err)
	}
	if !strings.Contains(lane.repo.Git("log", "--format=%s", "-3"), "a peer lands first") {
		t.Fatalf("the lane does not sit on the peer's commit:\n%s", lane.repo.Git("log", "--oneline", "-5"))
	}
	if got := lane.repo.Git("show", "HEAD:"+laneFile); got != "lane change" {
		t.Fatalf("the rebased lane lost its change: %q", got)
	}
	if lane.repo.Git("merge-base", "main", "HEAD") != lane.repo.Git("rev-parse", "main") {
		t.Fatal("main is not an ancestor of the rebased lane, so the next ship cannot fast-forward")
	}
}

func TestRunCycleFromPhase_ARecoveryBudgetSpentEndsWithTheShipError(t *testing.T) {
	t.Parallel()

	lane := newStaleBaseLane(t, "peer.txt")

	out := lane.resumeShip(t, staleBaseScenario{shipErr: landingDiverged(), shipFailures: 99})

	requireOrder(t, out.order, "ship", "audit", "ship", "audit", "ship")
	requireShipErrorSurfaced(t, out, CodeGitFFMergeDiverged)
}

func TestRunCycleFromPhase_ADivergenceThatIsNotAStaleBaseEndsTheCycle(t *testing.T) {
	t.Parallel()

	lane := newStaleBaseLane(t, "peer.txt")

	out := lane.resumeShip(t, staleBaseScenario{checkpointBase: lane.laneTip, shipErr: landingDiverged(), shipFailures: 1})

	requireOrder(t, out.order, "ship")
	requireShipErrorSurfaced(t, out, CodeGitFFMergeDiverged)
}

func TestRunCycleFromPhase_ACheckpointWithNoBaseKeepsTheShipError(t *testing.T) {
	t.Parallel()

	lane := newStaleBaseLane(t, "peer.txt")

	out := lane.resumeShip(t, staleBaseScenario{noBase: true, shipErr: landingDiverged(), shipFailures: 1})

	requireOrder(t, out.order, "ship")
	requireShipErrorSurfaced(t, out, CodeGitFFMergeDiverged)
}

func TestRunCycleFromPhase_AColliderRefusalIsNotRebased(t *testing.T) {
	t.Parallel()

	lane := newStaleBaseLane(t, "peer.txt")

	out := lane.resumeShip(t, staleBaseScenario{shipErr: colliderRefusal(), shipFailures: 1})

	requireOrder(t, out.order, "ship")
	requireShipErrorSurfaced(t, out, CodeGitFFMergeDiverged)
}

func TestRunCycleFromPhase_AShipErrorOfAnotherCodeFollowsTheFleetRoute(t *testing.T) {
	t.Parallel()

	lane := newStaleBaseLane(t, "peer.txt")
	rebaseNeeded := NewShipError(CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip, "a peer moved main")

	out := lane.resumeShip(t, staleBaseScenario{shipErr: rebaseNeeded, shipFailures: 1})

	if out.err != nil {
		t.Fatalf("a resumed ship that already reports GIT_FLEET_REBASE_NEEDED must recover, got %v", out.err)
	}
	requireOrder(t, out.order, "ship", "audit", "ship")
}

func TestRunCycleFromPhase_AShipFailureThatIsNotAShipErrorEndsTheCycle(t *testing.T) {
	t.Parallel()

	lane := newStaleBaseLane(t, "peer.txt")

	out := lane.resumeShip(t, staleBaseScenario{shipErr: errors.New("ship runner crashed"), shipFailures: 1})

	requireOrder(t, out.order, "ship")
	if out.err == nil || !strings.Contains(out.err.Error(), "ship runner crashed") {
		t.Fatalf("err = %v, want the crash to end the cycle", out.err)
	}
}

func TestRunCycleFromPhase_ARebaseConflictEndsTheCycleInsteadOfDispatchingTheDebugger(t *testing.T) {
	t.Parallel()

	lane := newStaleBaseLane(t, laneFile)

	out := lane.resumeShip(t, staleBaseScenario{shipErr: landingDiverged(), shipFailures: 1})

	requireOrder(t, out.order, "ship")
	requireShipErrorSurfaced(t, out, CodeGitFFMergeDiverged)
	if out.debugger.calls != 0 {
		t.Fatalf("the resume loop dispatched the debugger %d time(s); it has no debugger re-entry", out.debugger.calls)
	}
	if _, err := os.Stat(filepath.Join(lane.repo.Dir, ".git", "rebase-merge")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the failed rebase was left open in the lane: %v", err)
	}
}

func TestRunCycleFromPhase_AnExplanationCycleOnAStaleBaseRebindsAndReAuditsOnTheNewBase(t *testing.T) {
	t.Parallel()

	fx := newShippedLane(t, "lane.txt", addLaneFile, addPeerFile)
	var order []string
	ship := &fakeRunner{name: "ship", failErr: landingDiverged(), failUntil: 1}
	audit := &fakeRunner{name: "audit"}
	runners := buildRunners(nil)
	runners[PhaseShip] = landingShip{fakeRunner: ship, order: &order}
	runners[PhaseAudit] = orderedRunner{PhaseRunner: audit, name: "audit", order: &order}
	storage := &fakeStorage{state: State{LastCycleNumber: unwindCycle}, cycleState: fx.cycleState(), enforceExplanationContract: true}
	o := NewOrchestrator(storage, &fakeLedger{entries: []LedgerEntry{fx.auditRow()}}, runners,
		WithWorktreeProvisioner(&fakeWorktree{path: fx.worktree}), WithStderr(io.Discard))

	result, err := o.RunCycleFromPhase(context.Background(),
		CycleRequest{ProjectRoot: fx.root, GoalHash: "stale-base"},
		&ResumePoint{Phase: string(PhaseShip), CycleID: unwindCycle})

	if err != nil {
		t.Fatalf("an explanation cycle resumed on a stale base must recover, got %v", err)
	}
	requireOrder(t, order, "ship", "audit", "ship")
	if result.FinalVerdict != VerdictPASS {
		t.Fatalf("FinalVerdict = %q, want PASS", result.FinalVerdict)
	}
	if got := audit.requests[0].WorktreeBaseSHA; got != fx.newBase {
		t.Fatalf("the re-audit was dispatched on base %s, want the rebased base %s", got, fx.newBase)
	}
	fx.requirePendingLaneChangeOn(fx.newBase)
}

func TestBaseBehindMain_OnlyAPinnedAncestorOfMainCounts(t *testing.T) {
	t.Parallel()

	lane := newStaleBaseLane(t, "peer.txt")
	for _, tc := range []struct {
		name     string
		base     string
		worktree string
		want     bool
	}{
		{"a pinned ancestor of main", lane.base, lane.repo.Dir, true},
		{"the lane tip, which main does not hold", lane.laneTip, lane.repo.Dir, false},
		{"the tip of main, which main has not moved past", lane.mainTip, lane.repo.Dir, false},
		{"a branch name, which is not a pinned base", "main", lane.repo.Dir, false},
		{"no base", "", lane.repo.Dir, false},
		{"a directory that is not a repository", lane.base, t.TempDir(), false},
		{"a worktree that is gone", lane.base, filepath.Join(t.TempDir(), "gone"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cr := &cycleRun{ctx: context.Background(), cs: CycleState{WorktreeBaseSHA: tc.base, ActiveWorktree: tc.worktree}}

			if got := cr.baseBehindMain(); got != tc.want {
				t.Fatalf("baseBehindMain() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStaleBaseAsRebase_KeepsTheEvidenceOfTheRefusalItReplaces(t *testing.T) {
	t.Parallel()

	lane := newStaleBaseLane(t, "peer.txt")
	cr := &cycleRun{ctx: context.Background(), cs: CycleState{WorktreeBaseSHA: lane.base, ActiveWorktree: lane.repo.Dir}}
	original := landingDiverged()

	got, ok := AsShipError(cr.staleBaseAsRebase(original))

	if !ok || got.Code != CodeGitFleetRebaseNeeded || got.Class != ShipClassTransient || got.Stage != original.Stage {
		t.Fatalf("reclassified error = %+v, want a transient %s at stage %s", got, CodeGitFleetRebaseNeeded, original.Stage)
	}
	if got.Debug[shiperr.OriginalCodeKey] != string(CodeGitFFMergeDiverged) || got.Debug[shiperr.CycleBranchKey] != "cycle" {
		t.Fatalf("Debug = %v, want the original code and the landing's own keys kept", got.Debug)
	}
	if _, polluted := original.Debug[shiperr.OriginalCodeKey]; polluted {
		t.Fatal("the reclassification wrote into the original error's Debug map")
	}
	if !strings.Contains(got.Message, lane.base) || !strings.Contains(got.Message, original.Message) {
		t.Fatalf("Message = %q, want the base and the original message", got.Message)
	}
}

func TestBaseBehindMain_WithNoWorktreeNeverRunsGit(t *testing.T) {
	git := &seamGit{}
	useSeamGit(t, git)
	cr := &cycleRun{ctx: context.Background(), cs: CycleState{WorktreeBaseSHA: strings.Repeat("a", 40)}}

	if cr.baseBehindMain() {
		t.Fatal("a cycle with no worktree has no base to be behind main")
	}
	if len(git.calls) != 0 {
		t.Fatalf("git ran %d time(s) with no worktree, so it would have run in the process directory: %v", len(git.calls), git.calls)
	}
}

func TestStaleBaseAsRebase_KeepsADivergenceOfAnotherBranch(t *testing.T) {
	t.Parallel()

	lane := newStaleBaseLane(t, "peer.txt")
	cr := &cycleRun{ctx: context.Background(), cs: CycleState{WorktreeBaseSHA: lane.base, ActiveWorktree: lane.repo.Dir}}
	release := NewShipError(CodeGitFFMergeDiverged, ShipClassPrecondition, StageAtomicShip, "ship: ff-merge cycle into release failed",
		shiperr.GitRCKey, "1", shiperr.CycleBranchKey, "cycle", shiperr.BranchKey, "release")

	if got := cr.staleBaseAsRebase(release); got != error(release) {
		t.Fatalf("a divergence from release was rewritten to %v; the base test and the rebase both use main", got)
	}
}

func TestRunCycleFromPhase_ABaseEqualToTheTipOfMainKeepsTheShipError(t *testing.T) {
	t.Parallel()

	lane := newStaleBaseLane(t, "peer.txt")

	out := lane.resumeShip(t, staleBaseScenario{checkpointBase: lane.mainTip, shipErr: landingDiverged(), shipFailures: 1})

	requireOrder(t, out.order, "ship")
	requireShipErrorSurfaced(t, out, CodeGitFFMergeDiverged)
}

func TestRunCycleFromPhase_TheShipErrorSignalNamesTheCodeTheLandingReturned(t *testing.T) {
	t.Parallel()

	lane := newStaleBaseLane(t, "peer.txt")

	out := lane.resumeShip(t, staleBaseScenario{shipErr: landingDiverged(), shipFailures: 1})

	shipErrors := eventsOfKind(*out.signals, signalcenter.KindShipError)
	if len(shipErrors) != 1 {
		t.Fatalf("ship.error events = %d, want 1: %+v", len(shipErrors), *out.signals)
	}
	if got := shipErrors[0].Code; got != shiperr.SignalCode(CodeGitFleetRebaseNeeded) {
		t.Fatalf("signal code = %q, want %q", got, shiperr.SignalCode(CodeGitFleetRebaseNeeded))
	}
	if got := shipErrors[0].Fields[shiperr.OriginalCodeKey]; got != string(CodeGitFFMergeDiverged) {
		t.Fatalf("signal fields = %v, want %s=%s so the triage line keeps the code the landing returned", shipErrors[0].Fields, shiperr.OriginalCodeKey, CodeGitFFMergeDiverged)
	}
}

func TestRunCycleFromPhase_ARebaseThatProvesTheAuditedChangeShipsWithoutAReAudit(t *testing.T) {
	t.Parallel()

	lane := newStaleBaseLane(t, "peer.txt")
	audited, exit, err := gitCapture(context.Background(), lane.repo.Dir, "diff", lane.base, lane.laneTip)
	if err != nil || exit != 0 {
		t.Fatalf("audited diff: exit=%d err=%v", exit, err)
	}
	patchID, err := compositionPatchID([]byte(audited))
	if err != nil {
		t.Fatal(err)
	}
	var verdicts []CompositionVerdictInput
	out := lane.resumeShip(t, staleBaseScenario{shipErr: landingDiverged(), shipFailures: 1, options: []Option{
		WithCompositionSnapshot(snapshotFromFixture([]byte(audited), patchID)),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome { return allComposedGatesPass() }),
		WithCompositionVerdictWriter(func(_ string, in CompositionVerdictInput) error {
			verdicts = append(verdicts, in)
			return nil
		}),
	}})

	if out.err != nil {
		t.Fatalf("a rebase that the composition proof covers must ship, got %v", out.err)
	}
	requireOrder(t, out.order, "ship", "ship")
	if len(verdicts) != 1 || verdicts[0].Cycle != staleBaseCycle || verdicts[0].PatchID != patchID {
		t.Fatalf("composition verdicts = %+v, want one for cycle %d with patch id %s", verdicts, staleBaseCycle, patchID)
	}
	if out.audit.calls != 0 {
		t.Fatalf("the audit ran %d time(s); the proof replaces the re-audit", out.audit.calls)
	}
}
