package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

func failMarshal(t *testing.T) {
	t.Helper()
	orig := marshalIndentJSON
	marshalIndentJSON = func(any, string, string) ([]byte, error) { return nil, errors.New("marshal exploded") }
	t.Cleanup(func() { marshalIndentJSON = orig })
}

func TestMarshalFailures_AreReportedNotFatal(t *testing.T) {
	failMarshal(t)
	if err := AppendResolvedFingerprint(t.TempDir(), "a|b|c", "me", time.Unix(0, 0)); err == nil {
		t.Error("AppendResolvedFingerprint must surface a marshal failure")
	}
	if err := writeFailureDossier(t.TempDir(), &failureDossier{}); err == nil {
		t.Error("writeFailureDossier must surface a marshal failure")
	}
	if err := writePhaseIOShadowFile(t.TempDir(), "build", emptyHandoffs(), 1, nil); err == nil {
		t.Error("writePhaseIOShadowFile must surface a marshal failure")
	}
	selfCheckWT := t.TempDir()
	writeBuildSelfCheckArtifact(selfCheckWT, nil)
	if _, err := os.Stat(filepath.Join(selfCheckWT, ".evolve", "build-selfcheck.json")); err == nil {
		t.Error("no build-selfcheck artifact may be written when encoding fails")
	}
	ws := t.TempDir()
	persistFloorFailReasons(&CycleState{WorkspacePath: ws}, PhaseShip,
		[]cyclestate.Diagnostic{{Severity: cyclestate.SeverityError, Message: "m"}})
	if _, err := os.Stat(floorFailReasonPath(ws, PhaseShip)); err == nil {
		t.Error("no floor fail-reason file may be written when encoding fails")
	}
	o := orchestratorWithFailingLedger()
	o.recordShipError(context.Background(), 1, CycleState{WorkspacePath: ws}, &shiperr.ShipError{Code: "c"})
	o.recordPlanRejectionsKind(context.Background(), 1, CycleState{WorkspacePath: ws}, nil, "plan")
	if _, err := os.Stat(filepath.Join(ws, "ship-error.json")); err == nil {
		t.Error("ship-error.json must not be written when encoding fails")
	}
}

func TestEscalateRetroReason_NilLedgerOrNonProceedKeepsReason(t *testing.T) {
	t.Parallel()
	if got := escalateRetroReason("proceed: x", "p", nil); got != "proceed: x" {
		t.Fatalf("nil ledger: %q", got)
	}
	if got := escalateRetroReason("adapt: x", "p", nil); got != "adapt: x" {
		t.Fatalf("non-proceed: %q", got)
	}
}

func TestParseAdjudication_BlankActionIsNil(t *testing.T) {
	t.Parallel()
	if parseAdjudication(`{"action":"  "}`, filepath.Join(t.TempDir(), "absent")) != nil {
		t.Fatal("blank action must not parse")
	}
}

func TestEmitPhaseOutputsSignal_UnwritableWorkspaceStillReturns(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.Chmod(ws, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ws, 0o755) })
	emitPhaseOutputsSignal(ws, 1, []string{"audit"}, nil)
}

func TestAppendPhaseIOShadowMismatch_LedgerFailureIsNonFatal(t *testing.T) {
	t.Parallel()
	appendPhaseIOShadowMismatch(context.Background(), &fakeLedger{failOnAppend: true}, "ts", 1, "r", PhaseBuild,
		[]phaseIOMismatch{{Field: "f", Want: "a", Got: "b"}})
}

func TestEmitPhaseIOShadowWithSig_WriteFailureAndMismatchesAreReported(t *testing.T) {
	t.Parallel()
	led := &fakeLedger{}
	cr := &cycleRun{
		o:     NewOrchestrator(&fakeStorage{}, led, nil),
		cs:    CycleState{WorkspacePath: filepath.Join(t.TempDir(), "absent")},
		cycle: 1,
		ctx:   context.Background(),
	}
	var sig router.RoutingSignals
	sig.Audit.Present = true
	sig.Audit.DefectsBySeverity = map[router.Severity]int{router.Severity(98): 1, router.Severity(99): 2}
	cr.emitPhaseIOShadowWithSig(PhaseBuild, nil, sig)
	found := false
	for _, e := range led.entries {
		if e.Kind == "phaseio_shadow_mismatch" {
			found = true
		}
	}
	if !found {
		t.Fatal("oversized inputs must surface as a recorded shadow mismatch")
	}
}

func TestAssemblePhaseIO_DigestFailureFallsBackByStage(t *testing.T) {
	t.Parallel()
	cr := &cycleRun{
		o:  NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil),
		cs: CycleState{WorkspacePath: filepath.Join(t.TempDir(), "absent")},
	}
	cr.o.cfg.PhaseIO = config.StageShadow
	_ = cr.assemblePhaseIO(PhaseBuild, "", nil)
	cr.o.cfg.PhaseIO = config.StageEnforce
	if in := cr.assemblePhaseIO(PhaseBuild, "/wt", map[string]string{"goal": "g"}); in.Phase != string(PhaseBuild) {
		t.Fatalf("enforced fallback must still build the envelope, got phase %q", in.Phase)
	}
}

func TestSolutionViolations_DocumentCycleWithoutTasksFallsBackToProjectRoot(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "scout-report.md"),
		"<!-- challenge-token: x -->\ngoal_type: partnership-deal\ndeliverable_kind: document\n\n## Selected Tasks\n")
	if !DocumentCycle(ws) {
		t.Skip("fixture does not read as a document cycle")
	}
	out := SolutionViolations(ws, "", t.TempDir(), config.DeliverableKindSpec{})
	if len(out) != 1 || !strings.Contains(out[0], "binds no task") {
		t.Fatalf("violations = %v", out)
	}
}

func TestClearActiveWorktree_StorageFailures(t *testing.T) {
	t.Parallel()
	read := &Orchestrator{storage: cycleStateReadFailStorage{&fakeStorage{}}}
	read.clearActiveWorktree("/wt")
	other := &Orchestrator{storage: &fakeStorage{cycleState: CycleState{ActiveWorktree: "/elsewhere"}}}
	other.clearActiveWorktree("/wt")
	if other.storage.(*fakeStorage).cycleState.ActiveWorktree != "/elsewhere" {
		t.Fatal("a different worktree must be left named")
	}
	write := &Orchestrator{storage: &fakeStorage{cycleState: CycleState{ActiveWorktree: "/wt"}, failOnWriteCS: true}}
	write.clearActiveWorktree("/wt")
}

func TestPlaneFailures_ConsoleLeaseAndFreshSource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFileAt(t, filepath.Join(root, ".git"), "gitdir: "+filepath.Join(root, "gd"))
	if err := os.Mkdir(filepath.Join(root, "gd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := consoleLeasePath(root); got != "" {
		t.Fatalf("linked worktree without commondir = %q", got)
	}

	src := t.TempDir()
	acs := filepath.Join(src, "acs")
	if err := os.MkdirAll(acs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(acs, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(acs, 0o755) })
	if err := verifyFreshCycleSource(src, 7); err == nil {
		t.Fatal("unreadable acs directory must surface")
	}
}

func TestWorktreeBaseSHA_SkipsMalformedLogLines(t *testing.T) {
	useScriptedGit(t, func(_ int, args []string) scriptedReply {
		if args[0] == "rev-parse" {
			return scriptedReply{stdout: "head\n"}
		}
		return scriptedReply{stdout: "no-separator\n\nabc\x1freal subject\n"}
	})
	sha, err := resolveWorktreeBaseSHA(context.Background(), "/wt")
	if err != nil || sha != "abc" {
		t.Fatalf("sha=%q err=%v", sha, err)
	}
}

func TestEnsureCleanWorktree_QuarantineFailures(t *testing.T) {
	dirtyGit := func(resetExit int) func(int, []string) scriptedReply {
		return func(_ int, args []string) scriptedReply {
			switch args[0] {
			case "status":
				return scriptedReply{stdout: " M gone\n M kept\n"}
			case "reset":
				return scriptedReply{exit: resetExit, stdout: "reset out"}
			}
			return scriptedReply{}
		}
	}
	setup := func(t *testing.T) (wt, root string) {
		wt, root = t.TempDir(), t.TempDir()
		writeFileAt(t, filepath.Join(wt, "kept"), "k")
		return wt, root
	}
	t.Run("reset fails", func(t *testing.T) {
		wt, root := setup(t)
		useScriptedGit(t, dirtyGit(1))
		if _, err := ensureCleanWorktree(context.Background(), wt, root, 1); err == nil || !strings.Contains(err.Error(), "clean-provision") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("quarantine mkdir fails", func(t *testing.T) {
		wt := t.TempDir()
		writeFileAt(t, filepath.Join(wt, "kept"), "k")
		useScriptedGit(t, dirtyGit(0))
		if _, err := ensureCleanWorktree(context.Background(), wt, fileAsDir(t), 1); err == nil || !strings.Contains(err.Error(), "mkdir") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("quarantine move fails", func(t *testing.T) {
		wt, root := setup(t)
		q := quarantineDir(root, 1)
		if err := os.MkdirAll(q, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(q, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(q, 0o755) })
		useScriptedGit(t, dirtyGit(0))
		if _, err := ensureCleanWorktree(context.Background(), wt, root, 1); err == nil || !strings.Contains(err.Error(), "move") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestGitWorktree_CreateFailureModes(t *testing.T) {
	t.Parallel()
	blocked := filepath.Join(fileAsDir(t), "base")
	if _, err := (gitWorktree{baseOverride: blocked}).Create(t.TempDir(), 1); err == nil {
		t.Error("Create must refuse an uncreatable base")
	}
	if _, err := (gitWorktree{baseOverride: blocked}).CreateFrom(t.TempDir(), 1, "main"); err == nil {
		t.Error("CreateFrom must refuse an uncreatable base")
	}
	if _, err := (gitWorktree{baseOverride: "relative/base"}).CreateFrom(t.TempDir(), 1, "main"); err == nil {
		t.Error("CreateFrom must refuse a relative base")
	}
}

func TestOriginDefaultBranch_EmptyRemoteHeadFallsBackToMain(t *testing.T) {
	useScriptedGit(t, func(int, []string) scriptedReply { return scriptedReply{stdout: "origin/\n"} })
	if got := originDefaultBranch(context.Background(), gitexecAt("/p")); got != "main" {
		t.Fatalf("branch = %q", got)
	}
}

func emptyHandoffs() phaseio.Handoffs { return phaseio.Handoffs{} }

func gitexecAt(dir string) gitexec.Git { return gitexec.Git{Dir: dir, Exec: gitRunner} }
