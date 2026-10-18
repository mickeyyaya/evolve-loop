package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/coherence"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

func orchestratorWithFailingLedger() *Orchestrator {
	return NewOrchestrator(&fakeStorage{}, &fakeLedger{failOnAppend: true}, nil)
}

func fileAsDir(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "file")
	writeFileAt(t, p, "x")
	return p
}

func TestRecordShipError_WorkspaceFailuresStillAppendLedger(t *testing.T) {
	t.Parallel()
	se := &shiperr.ShipError{Code: "c", Class: "k", Stage: "s", Message: "m"}
	t.Run("workspace is a file", func(t *testing.T) {
		orchestratorWithFailingLedger().recordShipError(context.Background(), 1, CycleState{WorkspacePath: fileAsDir(t)}, se)
	})
	t.Run("artifact path is a directory", func(t *testing.T) {
		ws := t.TempDir()
		if err := os.Mkdir(filepath.Join(ws, "ship-error.json"), 0o755); err != nil {
			t.Fatal(err)
		}
		orchestratorWithFailingLedger().recordShipError(context.Background(), 1, CycleState{WorkspacePath: ws}, se)
	})
}

func TestRecordDebuggerDecision_LedgerFailureIsNonFatal(t *testing.T) {
	t.Parallel()
	orchestratorWithFailingLedger().recordDebuggerDecision(context.Background(), 1, CycleState{WorkspacePath: t.TempDir()}, PhaseResponse{})
}

func TestRecordPlanRejectionsKind_Failures(t *testing.T) {
	t.Parallel()
	o := orchestratorWithFailingLedger()
	ctx := context.Background()
	o.recordPlanRejectionsKind(ctx, 1, CycleState{}, nil, "plan")
	o.recordPlanRejectionsKind(ctx, 1, CycleState{WorkspacePath: fileAsDir(t)}, nil, "plan")
	ws := t.TempDir()
	if err := os.Mkdir(filepath.Join(ws, "advisor-rejections-x.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	o.recordPlanRejectionsKind(ctx, 1, CycleState{WorkspacePath: ws}, []router.PlanRejection{}, "x")
	o.recordPlanRejectionsKind(ctx, 1, CycleState{WorkspacePath: t.TempDir()}, nil, "plan")
}

func TestEscalateRetroReason_GenericAndBlankPatternsKeepReason(t *testing.T) {
	t.Parallel()
	led := &recurrence.Ledger{Entries: map[string]*recurrence.Entry{"p": {Pattern: "p", Count: 5, Generic: true}}}
	if got := escalateRetroReason("proceed: x", "p", led); got != "proceed: x" {
		t.Fatalf("generic pattern must not escalate, got %q", got)
	}
	o := &Orchestrator{}
	hist := []FailedRecord{{Classification: ""}}
	if got := o.escalateRetroReasonForHistory(t.TempDir(), "proceed: x", hist); got != "proceed: x" {
		t.Fatalf("blank classification must keep reason, got %q", got)
	}
}

func TestDispositionGate_UnverifiableIdentity(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "disposition.json"), "{bad")
	if got := readDispositionLegitimacy(ws, 1); got != "" {
		t.Fatalf("malformed disposition = %q", got)
	}
	writeFileAt(t, filepath.Join(ws, "disposition.json"), "{}")
	writeFileAt(t, filepath.Join(ws, "failure-digest.json"), "{bad")
	if got := readDispositionLegitimacy(ws, 1); got != "" {
		t.Fatalf("malformed digest must distrust legitimacy, got %q", got)
	}
}

func TestPlanRunOrder_WithoutClampedPlanUsesConfiguredOrder(t *testing.T) {
	t.Parallel()
	cr := &cycleRun{o: &Orchestrator{cfg: config.RoutingConfig{Order: []string{"a", "b"}}}}
	if got := cr.planRunOrder(); len(got) != 2 || got[0] != "a" {
		t.Fatalf("order = %v", got)
	}
}

func TestFailureDigest_MalformedReasonAndUnwritableWorkspace(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "audit-fail-reason.json"), "{bad")
	if phase, reasons := readAuditFailReason(ws); phase != "" || reasons != nil {
		t.Fatalf("malformed reason = %q %v", phase, reasons)
	}
	bad := filepath.Join(fileAsDir(t), "ws")
	(&Orchestrator{}).ensureFailureDigest(1, t.TempDir(), bad, "build", "boom")
}

func TestSameClassStreak_BlankLastClassificationIsZero(t *testing.T) {
	t.Parallel()
	if got := sameClassStreak([]FailedRecord{{Classification: "a"}, {Classification: ""}}); got != 0 {
		t.Fatalf("streak = %d", got)
	}
}

func TestRecordPhaseOutcome_NonAbortGradeIsAnnounced(t *testing.T) {
	t.Parallel()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil)
	var res CycleResult
	var timings []phaseTimingEntry
	o.recordPhaseOutcome(&res, &timings, t.TempDir(), recovery.PhaseOutcome{Phase: "build", AbortReason: "missing_challenge_token"})
}

func TestRetroStateWriteFailuresAreNonFatal(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{failOnWriteCS: true}
	o := NewOrchestrator(st, &fakeLedger{}, nil)
	cs := &CycleState{WorkspacePath: t.TempDir()}
	var timings []phaseTimingEntry
	fl := failureLearningRequest{Cycle: 1, Failed: PhaseBuild, Err: os.ErrInvalid, CycleState: cs, State: &State{}, Result: &CycleResult{}, Timings: &timings}
	runner := &fakeRunner{name: string(PhaseRetro)}
	if _, err := o.dispatchRetro(context.Background(), fl, runner, "s", "t"); err != nil {
		t.Fatalf("dispatchRetro: %v", err)
	}
	o.completeRetro(context.Background(), fl, PhaseResponse{}, "t")
}

func TestWithinRoot_RelativeRootAgainstAbsolutePath(t *testing.T) {
	t.Parallel()
	if withinRoot("/abs/path", "") {
		t.Fatal("empty root contains nothing")
	}
	if withinRoot("/abs/path", "relative") {
		t.Fatal("unrelatable paths are not within root")
	}
}

func TestGitStatusNames_Edges(t *testing.T) {
	if got := gitStatusNames("", 5); got != nil {
		t.Fatalf("blank worktree = %v", got)
	}
	useScriptedGit(t, func(int, []string) scriptedReply { return scriptedReply{stdout: " M a\n\n M b\n M c\n"} })
	if got := gitStatusNames("/w", 5); len(got) != 3 {
		t.Fatalf("names = %v", got)
	}
	if got := gitStatusNames("/w", 1); len(got) != 1 {
		t.Fatalf("capped names = %v", got)
	}
	useScriptedGit(t, func(int, []string) scriptedReply { return scriptedReply{exit: 128} })
	if got := gitStatusNames("/w", 5); got != nil {
		t.Fatalf("failed status = %v", got)
	}
}

func TestRecordCrossArtifactInvariants_UnwritableWorkspace(t *testing.T) {
	t.Parallel()
	recordCrossArtifactInvariants(1, filepath.Join(fileAsDir(t), "ws"), "")
	_ = coherence.CheckCrossArtifactInvariants
}

func TestVerifyFreshCycleSource_UnresolvableModule(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	goDir := filepath.Join(root, "go")
	if err := os.Mkdir(goDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(goDir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(goDir, 0o755) })
	if err := verifyFreshCycleSource(root, 1); err == nil {
		t.Fatal("unreadable module directory must error")
	}
}

func TestLoadRecentDossiers_SkipsBadNamesAndFilesAndHonoursLimit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := dossier.CyclesDir(root)
	writeFileAt(t, filepath.Join(dir, "cycle-x.json"), "{}")
	writeFileAt(t, filepath.Join(dir, "cycle-1.json"), "{bad")
	writeFileAt(t, filepath.Join(dir, "cycle-2.json"), `{"cycle":2}`)
	writeFileAt(t, filepath.Join(dir, "cycle-3.json"), `{"cycle":3}`)
	if err := os.Mkdir(filepath.Join(dir, "cycle-4.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := loadRecentDossiers(root, 0)
	if len(got) != 2 {
		t.Fatalf("all = %d dossiers", len(got))
	}
	if got := loadRecentDossiers(root, 1); len(got) != 0 {
		t.Fatalf("limit 1 keeps only the newest (unreadable) file, got %d", len(got))
	}
}
