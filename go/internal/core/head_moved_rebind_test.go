package core

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

type headMovedOutcome struct {
	order   []string
	carries []CompositionVerdictInput
	storage *fakeStorage
	signals *[]signalcenter.Event
	result  CycleResult
	err     error
}

func pendedAuditedLane(t *testing.T) *shippedLane {
	t.Helper()
	fx := newCommittedLane(t, "lane.txt", addLaneFile, addPeerFile)
	fx.git("reset", "-q", "--soft", fx.base)
	return fx
}

func headMovedAuditRow(fx *shippedLane) LedgerEntry {
	row := fx.auditRow()
	row.ArtifactSHA256 = "audit-ref"
	row.GitHEAD = fx.base
	row.WorktreeBaseSHA = fx.base
	return row
}

func headMovedShipError(fx *shippedLane) *ShipError {
	return NewShipError(CodeAuditBindingHeadMoved, ShipClassPrecondition, StageVerifyClass,
		"git HEAD has moved since audit", "audited", fx.base, "current", fx.newBase)
}

func resumeShipAfterHeadMoved(t *testing.T, fx *shippedLane) headMovedOutcome {
	t.Helper()
	return resumeShipRaising(t, fx, headMovedShipError(fx), []LedgerEntry{headMovedAuditRow(fx)})
}

func fleetRebaseShipError(fx *shippedLane, message string) *ShipError {
	return NewShipError(CodeGitFleetRebaseNeeded, ShipClassTransient, StageAtomicShip, message, "current", fx.newBase)
}

func resumeShipRaising(t *testing.T, fx *shippedLane, raised *ShipError, rows []LedgerEntry) headMovedOutcome {
	t.Helper()
	out := headMovedOutcome{storage: &fakeStorage{
		state: State{LastCycleNumber: unwindCycle},
		cycleState: CycleState{
			CycleID: unwindCycle, RunID: unwindRunID, WorkspacePath: fx.workspace,
			ActiveWorktree: fx.worktree, WorktreeBaseSHA: fx.base,
			ExplanationDocumentationVersion: fx.cycleState().ExplanationDocumentationVersion,
			CompletedPhases:                 []string{"scout", "triage", "build", "audit"},
			AuditDispatches:                 1,
		},
	}}
	ship := &fakeRunner{name: "ship", failErr: raised, failUntil: 1}
	runners := buildRunners(nil)
	runners[PhaseShip] = orderedRunner{PhaseRunner: ship, name: "ship", order: &out.order}
	runners[PhaseAudit] = orderedRunner{PhaseRunner: &fakeRunner{name: "audit"}, name: "audit", order: &out.order}
	center, events := recordingCenter()
	out.signals = events
	o := NewOrchestrator(out.storage, &fakeLedger{entries: rows}, runners,
		WithWorktreeProvisioner(&fakeWorktree{path: fx.worktree}),
		WithSignalCenter(center),
		WithCompositionSnapshot(func(context.Context, string, string) (CompositionAuditSnapshot, error) {
			return CompositionAuditSnapshot{LaneAuditRef: "audit-ref"}, nil
		}),
		WithCompositionGateRunner(func(context.Context, string) map[string]ciparity.GateOutcome {
			return allComposedGatesPass()
		}),
		WithCompositionVerdictWriter(func(_ string, in CompositionVerdictInput) error {
			out.carries = append(out.carries, in)
			return nil
		}),
	)
	out.result, out.err = o.RunCycleFromPhase(context.Background(),
		CycleRequest{ProjectRoot: fx.root, GoalHash: "head-moved"},
		&ResumePoint{Phase: string(PhaseShip), CycleID: unwindCycle})
	return out
}

func recoveryPaths(events []signalcenter.Event) []map[string]string {
	var paths []map[string]string
	for _, e := range events {
		if e.Code == CodeShipRecoveryPath {
			paths = append(paths, e.Fields)
		}
	}
	return paths
}

func auditRoundsRead(t *testing.T, fx *shippedLane, cs CycleState) int {
	t.Helper()
	body, err := json.Marshal(cs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fx.workspace, cyclespend.StateFile), body, 0o644); err != nil {
		t.Fatal(err)
	}
	spend, err := cyclespend.Read(fx.workspace)
	if err != nil {
		t.Fatal(err)
	}
	return spend.AuditRounds
}

func TestRunCycleFromPhase_AMovedHeadOverAnUnchangedLaneRebindsWithoutAnAuditRound(t *testing.T) {
	t.Parallel()

	fx := pendedAuditedLane(t)

	out := resumeShipAfterHeadMoved(t, fx)

	if out.err != nil {
		t.Fatalf("a moved main under an unchanged lane must recover, got %v", out.err)
	}
	requireOrder(t, out.order, "ship", "ship")
	if len(out.carries) != 1 || out.carries[0].GitHead != fx.newBase || out.carries[0].AuditedBase != fx.base || out.carries[0].AuditedTreeSHA != fx.auditedTree {
		t.Fatalf("carries = %+v, want one identical-rebase record that re-binds the audit of %s on %s to the new main %s", out.carries, fx.auditedTree, fx.base, fx.newBase)
	}
	if head := fx.git("rev-parse", "HEAD"); head != fx.newBase {
		t.Fatalf("worktree HEAD = %s, want the lane pending on the new main %s", head, fx.newBase)
	}
	if got := auditRoundsRead(t, fx, out.storage.cycleState); got != 1 {
		t.Fatalf("the spend reader counts %d audit round(s), want 1: a re-bind is not an audit round", got)
	}
	paths := recoveryPaths(*out.signals)
	if len(paths) != 1 {
		t.Fatalf("recovery path signals = %v, want one %s path for the moved HEAD", paths, recoveryPathRebind)
	}
	if _, reason := paths[0]["reason"]; reason || paths[0]["path"] != recoveryPathRebind || paths[0]["code"] != string(CodeGitFleetRebaseNeeded) || paths[0]["original_code"] != string(CodeAuditBindingHeadMoved) {
		t.Fatalf("recovery path signals = %v, want one %s path for the moved HEAD", paths, recoveryPathRebind)
	}
}

func TestRunCycleFromPhase_AMovedHeadOverAChangedLaneStillGetsAnAuditRound(t *testing.T) {
	t.Parallel()

	fx := pendedAuditedLane(t)
	fx.write("lane.txt", "a change the audit never saw\n")

	out := resumeShipAfterHeadMoved(t, fx)

	if out.err != nil {
		t.Fatalf("the re-audit route must still recover, got %v", out.err)
	}
	requireOrder(t, out.order, "ship", "audit", "ship")
	if len(out.carries) != 0 {
		t.Fatalf("carries = %+v, want none: the lane diff changed", out.carries)
	}
	if head := fx.git("rev-parse", "HEAD"); head != fx.base {
		t.Fatalf("worktree HEAD = %s, want the changed lane left pending on its base %s", head, fx.base)
	}
	if got := auditRoundsRead(t, fx, out.storage.cycleState); got != 2 {
		t.Fatalf("the spend reader counts %d audit round(s), want 2: a changed lane pays its audit round", got)
	}
	paths := recoveryPaths(*out.signals)
	if len(paths) != 1 || paths[0]["path"] != recoveryPathReaudit || paths[0]["code"] != string(CodeAuditBindingHeadMoved) || paths[0]["reason"] == "" {
		t.Fatalf("recovery path signals = %v, want one %s path that names why", paths, recoveryPathReaudit)
	}
}

func TestCarryAuditedChangeOverMovedMain_KeepsTheOriginalErrorWhenNoCarrierIsCommitted(t *testing.T) {
	t.Parallel()

	fx := pendedAuditedLane(t)
	staleBase := fx.cycleState()
	staleBase.WorktreeBaseSHA = fx.newBase
	for name, tc := range map[string]struct {
		cs   CycleState
		rows []LedgerEntry
		want string
	}{
		"in-place cycle":          {cs: CycleState{RunID: unwindRunID, ActiveWorktree: fx.root, WorktreeBaseSHA: fx.base}, rows: []LedgerEntry{headMovedAuditRow(fx)}, want: "the cycle has no worktree of its own"},
		"no auditor row":          {cs: fx.cycleState(), want: "no auditor row names the audited tree"},
		"audit on another base":   {cs: fx.cycleState(), rows: []LedgerEntry{func() LedgerEntry { r := headMovedAuditRow(fx); r.WorktreeBaseSHA = fx.newBase; return r }()}, want: "the change is not pending"},
		"a stale cycle base":      {cs: staleBase, rows: []LedgerEntry{headMovedAuditRow(fx)}, want: "the change is not pending"},
		"a worktree that is gone": {cs: CycleState{RunID: unwindRunID, ActiveWorktree: t.TempDir() + "/gone", WorktreeBaseSHA: fx.base}, rows: []LedgerEntry{headMovedAuditRow(fx)}, want: "read the worktree HEAD"},
	} {
		t.Run(name, func(t *testing.T) {
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: tc.rows}, buildRunners(nil))
			cr := &cycleRun{ctx: context.Background(), o: o, cycle: unwindCycle, cs: tc.cs, req: CycleRequest{ProjectRoot: fx.root}}
			original := headMovedShipError(fx)

			got, declined := cr.carryAuditedChangeOverMovedMain(original)

			if got != original || !strings.HasPrefix(declined, tc.want) {
				t.Fatalf("carryAuditedChangeOverMovedMain = (%+v, %q), want the original error and a reason that starts %q", got, declined, tc.want)
			}
			if head := fx.git("rev-parse", "HEAD"); head != fx.base {
				t.Fatalf("HEAD = %s, want the lane untouched on %s", head, fx.base)
			}
		})
	}
}

func TestCarryAuditedChangeOverMovedMain_ACommittedCarrierBecomesAFleetRebase(t *testing.T) {
	t.Parallel()

	fx := pendedAuditedLane(t)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{headMovedAuditRow(fx)}}, buildRunners(nil))
	cr := &cycleRun{ctx: context.Background(), o: o, cycle: unwindCycle, cs: fx.cycleState(), req: CycleRequest{ProjectRoot: fx.root}}
	original := headMovedShipError(fx)

	got, declined := cr.carryAuditedChangeOverMovedMain(original)

	if declined != "" || got.Code != CodeGitFleetRebaseNeeded || got.Class != ShipClassTransient || got.Stage != original.Stage {
		t.Fatalf("carryAuditedChangeOverMovedMain = (%+v, %q), want a transient fleet rebase at %s", got, declined, original.Stage)
	}
	if got.Debug[shiperr.OriginalCodeKey] != string(CodeAuditBindingHeadMoved) || got.Debug["current"] != fx.newBase {
		t.Fatalf("Debug = %v, want the original code and keys kept", got.Debug)
	}
	if _, polluted := original.Debug[shiperr.OriginalCodeKey]; polluted {
		t.Fatal("the reclassification wrote into the original error's Debug map")
	}
	if parent, tree := fx.git("rev-parse", "HEAD^"), fx.git("rev-parse", "HEAD^{tree}"); parent != fx.base || tree != fx.auditedTree {
		t.Fatalf("carrier = parent %s tree %s, want the audited tree %s on %s", parent, tree, fx.auditedTree, fx.base)
	}
	if status := fx.git("status", "--porcelain"); status != "" {
		t.Fatalf("status = %q, want a clean carrier the rebase can replay", status)
	}
}

func TestCarryAuditedChangeOverMovedMain_LeavesOtherCodesAlone(t *testing.T) {
	t.Parallel()

	cr := &cycleRun{ctx: context.Background()}
	original := NewShipError(CodeGitFFMergeDiverged, ShipClassTransient, StageAtomicShip, "the fast-forward merge diverged")

	if got, declined := cr.carryAuditedChangeOverMovedMain(original); got != original || declined != "" {
		t.Fatalf("carryAuditedChangeOverMovedMain = (%+v, %q), want the error untouched", got, declined)
	}
}

func failingGitSubcommand(sub string) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, out, errw io.Writer) (int, error) {
		if slices.Contains(args, sub) {
			return 1, nil
		}
		return sysexec.DefaultRunner(ctx, name, dir, args, env, stdin, out, errw)
	}
}

func TestCommitPendingAuditedChange_ANamedGitFailureLeavesTheLaneOnItsBase(t *testing.T) {
	for name, tc := range map[string]struct {
		breakLane func(t *testing.T, fx *shippedLane)
		want      string
	}{
		"an unreadable lane file": {
			breakLane: func(t *testing.T, fx *shippedLane) {
				if os.Geteuid() == 0 {
					t.Skip("root reads a mode-000 file")
				}
				locked := filepath.Join(fx.worktree, "locked.txt")
				if err := os.WriteFile(locked, []byte("x"), 0o000); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(locked, 0o600) })
			},
			want: "read the worktree tree",
		},
		"commit-tree fails": {
			breakLane: func(t *testing.T, _ *shippedLane) { swapGitRunner(t, failingGitSubcommand("commit-tree")) },
			want:      "commit the audited change",
		},
		"reset fails": {
			breakLane: func(t *testing.T, _ *shippedLane) { swapGitRunner(t, failingGitSubcommand("reset")) },
			want:      "move the lane to its carrier",
		},
	} {
		t.Run(name, func(t *testing.T) {
			fx := pendedAuditedLane(t)
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{headMovedAuditRow(fx)}}, buildRunners(nil))
			tc.breakLane(t, fx)

			declined := o.commitPendingAuditedChange(context.Background(), fx.root, unwindCycle, fx.cycleState())

			if !strings.HasPrefix(declined, tc.want) {
				t.Fatalf("declined = %q, want a reason that starts %q", declined, tc.want)
			}
			if head := fx.git("rev-parse", "HEAD"); head != fx.base {
				t.Fatalf("HEAD = %s, want the lane left on %s", head, fx.base)
			}
		})
	}
}

func TestCommitPendingAuditedChange_AnUnstagedLaneWithANewFileBecomesACleanCarrier(t *testing.T) {
	t.Parallel()

	fx := pendedAuditedLane(t)
	fx.write("fresh.txt", "a file the lane added and never staged\n")
	fx.git("reset", "-q")
	held, err := treefence.Take(context.Background(), fx.worktree)
	if err != nil {
		t.Fatal(err)
	}
	row := headMovedAuditRow(fx)
	row.WorktreeTreeSHA = held.Tree
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{row}}, buildRunners(nil))

	if declined := o.commitPendingAuditedChange(context.Background(), fx.root, unwindCycle, fx.cycleState()); declined != "" {
		t.Fatalf("declined = %q, want the unchanged lane carried", declined)
	}

	if tree := fx.git("rev-parse", "HEAD^{tree}"); tree != held.Tree {
		t.Fatalf("carrier tree = %s, want the audited tree %s with the new file", tree, held.Tree)
	}
	if status := fx.git("status", "--porcelain"); status != "" {
		t.Fatalf("status = %q, want the index on the carrier so the rebase replays a clean lane", status)
	}
}

func TestRecoveryPathOf_NamesEachRoute(t *testing.T) {
	t.Parallel()

	for next, want := range map[Phase]string{
		"":            recoveryPathEnd,
		PhaseShip:     recoveryPathRebind,
		PhaseAudit:    recoveryPathReaudit,
		PhaseBuild:    recoveryPathRebuild,
		PhaseDebugger: recoveryPathDebugger,
		PhaseScout:    string(PhaseScout),
	} {
		if got := recoveryPathOf(next); got != want {
			t.Errorf("recoveryPathOf(%q) = %q, want %q", next, got, want)
		}
	}
}

func TestReportRecoveryPath_IgnoresAShipErrorThatNamesNoMovedMain(t *testing.T) {
	t.Parallel()

	for _, code := range []ShipErrorCode{CodeGitFFMergeDiverged, CodeAuditBindingTreeMismatch} {
		center, events := recordingCenter()
		cr := &cycleRun{ctx: context.Background(), o: NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(center))}
		other := NewShipError(code, ShipClassPrecondition, StageAtomicShip, "not a moved main")

		cr.reportRecoveryPath(shipRecovery{raised: other, routed: other, next: PhaseAudit})

		if paths := recoveryPaths(*events); len(paths) != 0 {
			t.Errorf("recovery path signals = %v, want none for %s", paths, code)
		}
	}
}

func TestRecoverShipError_AnErrorWithNoCarrierNeverPendsOne(t *testing.T) {
	t.Parallel()

	var sink strings.Builder
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithStderr(&sink))
	cs := CycleState{RunID: unwindRunID, ActiveWorktree: filepath.Join(t.TempDir(), "gone")}
	cr := &cycleRun{ctx: context.Background(), o: o, cycle: unwindCycle, cs: cs, req: CycleRequest{ProjectRoot: t.TempDir()}, recoveryDepth: 1 << 10}
	diverged := NewShipError(CodeGitFFMergeDiverged, ShipClassPrecondition, StageAtomicShip, "the branch diverged")

	if cr.recoverShipError(PhaseShip, diverged, PhaseResponse{}, 1) {
		t.Fatal("recoverShipError = true, want false: the recovery budget is spent")
	}

	if strings.Contains(sink.String(), "stays at the lane HEAD") {
		t.Fatalf("stderr = %q, want no carrier read: this error committed no carrier", sink.String())
	}
}

func TestRecoverShipError_AnExhaustedBudgetPendsTheCarrierBackOnItsBase(t *testing.T) {
	t.Parallel()

	fx := pendedAuditedLane(t)
	center, events := recordingCenter()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{headMovedAuditRow(fx)}}, buildRunners(nil), WithSignalCenter(center), WithStderr(fx.stderr))
	cr := &cycleRun{ctx: context.Background(), o: o, cycle: unwindCycle, cs: fx.cycleState(), req: CycleRequest{ProjectRoot: fx.root}, recoveryDepth: 1 << 10}

	if cr.recoverShipError(PhaseShip, headMovedShipError(fx), PhaseResponse{}, 1) {
		t.Fatal("recoverShipError = true, want false: the recovery budget is spent")
	}

	if head := fx.git("rev-parse", "HEAD"); head != fx.base {
		t.Fatalf("HEAD = %s, want the carrier pended back on %s", head, fx.base)
	}
	if tree := fx.git("write-tree"); tree != fx.auditedTree {
		t.Fatalf("index tree = %s, want the audited change %s pending", tree, fx.auditedTree)
	}
	if paths := recoveryPaths(*events); len(paths) != 1 || paths[0]["path"] != recoveryPathEnd {
		t.Fatalf("recovery path signals = %v, want one %s path", paths, recoveryPathEnd)
	}
}

func TestPendUnusedCarrier_LeavesALaneWithoutItsCarrierAlone(t *testing.T) {
	t.Parallel()

	fx := pendedAuditedLane(t)
	var sink strings.Builder
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithStderr(&sink))

	(&cycleRun{ctx: context.Background(), o: o, cycle: unwindCycle, cs: fx.cycleState()}).pendUnusedCarrier()

	if head := fx.git("rev-parse", "HEAD"); head != fx.base || sink.Len() != 0 {
		t.Fatalf("HEAD = %s, stderr = %q, want the lane untouched on %s with no warning", head, sink.String(), fx.base)
	}
}

func TestPendUnusedCarrier_WarnsWhenTheLaneCannotBeRead(t *testing.T) {
	t.Parallel()

	var sink strings.Builder
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithStderr(&sink))
	cs := CycleState{RunID: unwindRunID, ActiveWorktree: filepath.Join(t.TempDir(), "gone")}

	(&cycleRun{ctx: context.Background(), o: o, cycle: unwindCycle, cs: cs}).pendUnusedCarrier()

	if !strings.Contains(sink.String(), "WARN cycle 42 ship recovery ended; the carrier cycle-42/") {
		t.Fatalf("stderr = %q, want a warning that names the carrier left at HEAD", sink.String())
	}
}

func TestAsFleetRebase_KeepsTheOriginalCodeOfAnErrorWithNoDebug(t *testing.T) {
	t.Parallel()

	original := &ShipError{Code: CodeAuditBindingHeadMoved, Class: ShipClassPrecondition, Stage: StageVerifyClass, Message: "moved"}

	got := asFleetRebase(original, "why")

	if got.Message != "why: moved" || got.Debug[shiperr.OriginalCodeKey] != string(CodeAuditBindingHeadMoved) || original.Debug != nil {
		t.Fatalf("asFleetRebase = %+v, want the reason prefixed and the original code in a new Debug map", got)
	}
}

func TestCarryAuditedChangeOverMovedMain_AFleetRebaseOnAPendingLaneCarriesTheChange(t *testing.T) {
	t.Parallel()

	fx := pendedAuditedLane(t)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{headMovedAuditRow(fx)}}, buildRunners(nil))
	cr := &cycleRun{ctx: context.Background(), o: o, cycle: unwindCycle, cs: fx.cycleState(), req: CycleRequest{ProjectRoot: fx.root}}
	original := fleetRebaseShipError(fx, "main moved before the repo-contract pack")

	got, declined := cr.carryAuditedChangeOverMovedMain(original)

	if declined != "" || got.Code != CodeGitFleetRebaseNeeded || got.Debug[shiperr.OriginalCodeKey] != string(CodeGitFleetRebaseNeeded) {
		t.Fatalf("carryAuditedChangeOverMovedMain = (%+v, %q), want a fleet rebase that holds the carried change", got, declined)
	}
	if parent, tree := fx.git("rev-parse", "HEAD^"), fx.git("rev-parse", "HEAD^{tree}"); parent != fx.base || tree != fx.auditedTree {
		t.Fatalf("carrier = parent %s tree %s, want the audited tree %s on %s", parent, tree, fx.auditedTree, fx.base)
	}
}

func TestCarryAuditedChangeOverMovedMain_AFleetRebaseOnACommittedLaneLeavesItAlone(t *testing.T) {
	t.Parallel()

	fx := newCommittedLane(t, "lane.txt", addLaneFile, addPeerFile)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{headMovedAuditRow(fx)}}, buildRunners(nil))
	cr := &cycleRun{ctx: context.Background(), o: o, cycle: unwindCycle, cs: fx.cycleState(), req: CycleRequest{ProjectRoot: fx.root}}
	original := fleetRebaseShipError(fx, "peer moved main at land time")

	got, declined := cr.carryAuditedChangeOverMovedMain(original)

	if got != original || declined != "" {
		t.Fatalf("carryAuditedChangeOverMovedMain = (%+v, %q), want the error untouched", got, declined)
	}
	if head := fx.git("rev-parse", "HEAD"); head != fx.shipCommit {
		t.Fatalf("HEAD = %s, want the committed lane left on %s", head, fx.shipCommit)
	}
}

func TestCarryAuditedChangeOverMovedMain_AFleetRebaseOnACommittedLaneWithAStrayFileLeavesItAlone(t *testing.T) {
	t.Parallel()

	fx := newCommittedLane(t, "lane.txt", addLaneFile, addPeerFile)
	if err := os.WriteFile(filepath.Join(fx.worktree, "stray.txt"), []byte("stray\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{headMovedAuditRow(fx)}}, buildRunners(nil))
	cr := &cycleRun{ctx: context.Background(), o: o, cycle: unwindCycle, cs: fx.cycleState(), req: CycleRequest{ProjectRoot: fx.root}}
	original := fleetRebaseShipError(fx, "peer moved main at land time")

	got, declined := cr.carryAuditedChangeOverMovedMain(original)

	if got != original || declined != "" {
		t.Fatalf("carryAuditedChangeOverMovedMain = (%+v, %q), want the error untouched: HEAD is not the cycle base", got, declined)
	}
	if head := fx.git("rev-parse", "HEAD"); head != fx.shipCommit {
		t.Fatalf("HEAD = %s, want the committed lane left on %s", head, fx.shipCommit)
	}
}

func TestCarryAuditedChangeOverMovedMain_ADeclinedFleetRebaseOnAPendingLaneNamesAMovedHead(t *testing.T) {
	t.Parallel()

	fx := pendedAuditedLane(t)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	cr := &cycleRun{ctx: context.Background(), o: o, cycle: unwindCycle, cs: fx.cycleState(), req: CycleRequest{ProjectRoot: fx.root}}
	original := fleetRebaseShipError(fx, "main moved before the repo-contract pack")

	got, declined := cr.carryAuditedChangeOverMovedMain(original)

	if got.Code != CodeAuditBindingHeadMoved || got.Debug[shiperr.OriginalCodeKey] != string(CodeGitFleetRebaseNeeded) || !strings.HasPrefix(declined, "no auditor row names the audited tree") {
		t.Fatalf("carryAuditedChangeOverMovedMain = (%+v, %q), want a moved-HEAD error that keeps the fleet-rebase code and the decline reason", got, declined)
	}
	if got.Stage != original.Stage || got.Message != original.Message {
		t.Fatalf("got stage %s message %q, want the stage and message of the raised error", got.Stage, got.Message)
	}
}

func TestRunCycleFromPhase_AFleetRebaseOnAPendingLaneCarriesTheChangeInsteadOfEndingAsLanded(t *testing.T) {
	t.Parallel()

	for name, message := range map[string]string{
		"early fast-forward check": "main moved after the lane base",
		"base-red rebase":          "the red scanner pack tests are red on the lane base too and green on main; rebase the lane onto main and verify again",
		"main-red stop":            "main is red and a stop holds, and the lane is not on the main tip; rebase the lane onto main and verify again",
	} {
		t.Run(name, func(t *testing.T) {
			fx := pendedAuditedLane(t)

			out := resumeShipRaising(t, fx, fleetRebaseShipError(fx, message), []LedgerEntry{headMovedAuditRow(fx)})

			if out.err != nil {
				t.Fatalf("a pending lane under a moved main must recover, got %v", out.err)
			}
			requireOrder(t, out.order, "ship", "ship")
			if head := fx.git("rev-parse", "HEAD"); head != fx.newBase {
				t.Fatalf("worktree HEAD = %s, want the audited change rebased onto the new main %s", head, fx.newBase)
			}
			paths := recoveryPaths(*out.signals)
			if len(paths) != 1 || paths[0]["path"] != recoveryPathRebind {
				t.Fatalf("recovery path signals = %v, want one %s path", paths, recoveryPathRebind)
			}
			if _, reason := paths[0]["reason"]; reason {
				t.Fatalf("recovery path signals = %v, want no reason", paths)
			}
		})
	}
}

func TestRunCycleFromPhase_ALandTimeFleetRebaseHasNoReasonField(t *testing.T) {
	t.Parallel()

	fx := newCommittedLane(t, "lane.txt", addLaneFile, addPeerFile)

	out := resumeShipRaising(t, fx, fleetRebaseShipError(fx, "peer moved main at land time"), []LedgerEntry{headMovedAuditRow(fx)})

	paths := recoveryPaths(*out.signals)
	if len(paths) != 1 {
		t.Fatalf("recovery path signals = %v, want one", paths)
	}
	if _, reason := paths[0]["reason"]; reason {
		t.Fatalf("recovery path signals = %v, want no reason on a land-time fleet rebase", paths)
	}
}

func TestRunCycleFromPhase_ADeclinedCarryOnAPendingFleetRebaseGoesToReaudit(t *testing.T) {
	t.Parallel()

	fx := pendedAuditedLane(t)

	out := resumeShipRaising(t, fx, fleetRebaseShipError(fx, "main moved after the lane base"), nil)

	if out.err != nil {
		t.Fatalf("the re-audit route must recover, got %v", out.err)
	}
	requireOrder(t, out.order, "ship", "audit", "ship")
	paths := recoveryPaths(*out.signals)
	if len(paths) != 1 || paths[0]["path"] != recoveryPathReaudit || paths[0]["code"] != string(CodeAuditBindingHeadMoved) || paths[0]["original_code"] != string(CodeGitFleetRebaseNeeded) || paths[0]["reason"] == "" {
		t.Fatalf("recovery path signals = %v, want one %s path for %s that names the fleet-rebase origin and why", paths, recoveryPathReaudit, CodeAuditBindingHeadMoved)
	}
}

func strayFileLaneWithNoAuditedChange(t *testing.T) (*shippedLane, LedgerEntry) {
	t.Helper()
	fx := pendedAuditedLane(t)
	fx.git("reset", "-q", "--hard")
	fx.write("stray.txt", "an untracked file the audit never selected\n")
	row := headMovedAuditRow(fx)
	row.WorktreeTreeSHA = fx.git("rev-parse", fx.base+"^{tree}")
	return fx, row
}

func TestCarryAuditedChangeOverMovedMain_AFleetRebaseOnALaneTheAuditJudgedUnchangedLeavesItAlone(t *testing.T) {
	t.Parallel()

	fx, row := strayFileLaneWithNoAuditedChange(t)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{row}}, buildRunners(nil))
	cr := &cycleRun{ctx: context.Background(), o: o, cycle: unwindCycle, cs: fx.cycleState(), req: CycleRequest{ProjectRoot: fx.root}}
	original := fleetRebaseShipError(fx, "main moved")

	if got, declined := cr.carryAuditedChangeOverMovedMain(original); got != original || declined != "" {
		t.Fatalf("carryAuditedChangeOverMovedMain = (%+v, %q), want the error untouched: the audit judged no change", got, declined)
	}
}

func TestRunCycleFromPhase_AFleetRebaseOnALaneTheAuditJudgedUnchangedEndsWithoutAReaudit(t *testing.T) {
	t.Parallel()

	fx, row := strayFileLaneWithNoAuditedChange(t)

	out := resumeShipRaising(t, fx, fleetRebaseShipError(fx, "the red scanner pack tests are red on the lane base too and green on main; rebase the lane onto main and verify again"), []LedgerEntry{row})

	requireOrder(t, out.order, "ship")
	paths := recoveryPaths(*out.signals)
	if len(paths) != 1 || paths[0]["path"] != recoveryPathEnd {
		t.Fatalf("recovery path signals = %v, want one %s path", paths, recoveryPathEnd)
	}
	if _, reason := paths[0]["reason"]; reason {
		t.Fatalf("recovery path signals = %v, want no reason", paths)
	}
}

func TestCarryAuditedChangeOverMovedMain_AFleetRebaseOnACleanLaneThatTheAuditSawChangedDeclines(t *testing.T) {
	t.Parallel()

	fx := pendedAuditedLane(t)
	fx.git("reset", "-q", "--hard")
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{headMovedAuditRow(fx)}}, buildRunners(nil))
	cr := &cycleRun{ctx: context.Background(), o: o, cycle: unwindCycle, cs: fx.cycleState(), req: CycleRequest{ProjectRoot: fx.root}}
	original := fleetRebaseShipError(fx, "main moved")

	got, declined := cr.carryAuditedChangeOverMovedMain(original)

	if got.Code != CodeAuditBindingHeadMoved || !strings.HasPrefix(declined, "the lane diff changed since the audit") {
		t.Fatalf("carryAuditedChangeOverMovedMain = (%+v, %q), want a moved-HEAD error: the audited change drifted", got, declined)
	}
}

func TestPendingAuditedChange_NamesTheAuditedChangeOfAPendingLane(t *testing.T) {
	t.Parallel()

	fx := pendedAuditedLane(t)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{headMovedAuditRow(fx)}}, buildRunners(nil))

	change, pending, declined := o.pendingAuditedChange(context.Background(), fx.root, fx.cycleState())

	if !pending || declined != "" || change.Base != fx.base || change.Tree != fx.auditedTree {
		t.Fatalf("pendingAuditedChange = (%+v, %v, %q), want the audited tree %s on %s", change, pending, declined, fx.auditedTree, fx.base)
	}
}

func TestPendingAuditedChange_AMissingOrInPlaceWorktreeIsNotPending(t *testing.T) {
	t.Parallel()

	fx := pendedAuditedLane(t)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{entries: []LedgerEntry{headMovedAuditRow(fx)}}, buildRunners(nil))
	for name, worktree := range map[string]string{"no worktree": "", "the project root": fx.root} {
		cs := fx.cycleState()
		cs.ActiveWorktree = worktree
		if _, pending, _ := o.pendingAuditedChange(context.Background(), fx.root, cs); pending {
			t.Errorf("%s: pending, want not pending", name)
		}
	}
}

func TestPendingAuditedChange_ALaneWithNoAuditorRowIsPendingOnlyWhenItHoldsFiles(t *testing.T) {
	t.Parallel()

	fx := pendedAuditedLane(t)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if _, pending, declined := o.pendingAuditedChange(context.Background(), fx.root, fx.cycleState()); !pending || !strings.HasPrefix(declined, "no auditor row") {
		t.Fatalf("pendingAuditedChange = (%v, %q), want an unaudited lane with files pending and declined", pending, declined)
	}
	fx.git("reset", "-q", "--hard")
	if _, pending, _ := o.pendingAuditedChange(context.Background(), fx.root, fx.cycleState()); pending {
		t.Fatal("a clean lane with no auditor row holds nothing to carry")
	}
}
