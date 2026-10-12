package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards/treediff"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

type kindFailingLedgerE4 struct {
	fakeLedger
	failKinds map[string]bool
}

func (l *kindFailingLedgerE4) Append(ctx context.Context, e LedgerEntry) error {
	if l.failKinds[e.Kind] {
		return errors.New("forced " + e.Kind + " append fail")
	}
	return l.fakeLedger.Append(ctx, e)
}

func (l *kindFailingLedgerE4) kinds() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, 0, len(l.entries))
	for _, e := range l.entries {
		out = append(out, e.Kind)
	}
	return out
}

type countingPassRunnerE4 struct {
	name  string
	calls int
}

func (r *countingPassRunnerE4) Name() string { return r.name }
func (r *countingPassRunnerE4) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.calls++
	return PhaseResponse{Phase: r.name, Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func correctionRunE4(t *testing.T, rev DeliverableReviewer, ledger Ledger, root string, runner PhaseRunner) (*cycleRun, *dispatchResult) {
	t.Helper()
	o := NewOrchestrator(&fakeStorage{}, ledger, buildRunners(nil), WithReviewer(rev))
	cr := &cycleRun{
		o: o, ctx: context.Background(), cycle: 3, req: CycleRequest{ProjectRoot: root},
		cs:          CycleState{CycleID: 3, WorkspacePath: t.TempDir(), RunID: "run-3"},
		retryConfig: o.retryConfig, envSnap: map[string]string{}, ctxSnap: map[string]string{},
	}
	dr := &dispatchResult{resp: PhaseResponse{Phase: string(PhaseBuild), Verdict: VerdictPASS}, runner: runner, attemptCount: 1}
	return cr, dr
}

func TestReviewWithCorrections_ALostCorrectionLedgerRowWarnsAndTheLadderContinues(t *testing.T) {
	ledger := &kindFailingLedgerE4{failKinds: map[string]bool{"contract_correction": true}}
	rev := &sequencedReviewer{phase: "build", results: []ReviewResult{{Approve: false, Reason: "missing header"}, {Approve: true}}}
	runner := &countingPassRunnerE4{name: "build"}
	cr, dr := correctionRunE4(t, rev, ledger, t.TempDir(), runner)

	var act loopAction
	var err error
	log := captureStderr(t, func() { act, err = cr.reviewWithCorrections(PhaseBuild, dr) })

	if act != loopNext || err != nil {
		t.Fatalf("act=%v err=%v, want the approved correction to advance", act, err)
	}
	if runner.calls != 1 || rev.calls != 2 {
		t.Fatalf("runner=%d reviews=%d, want one re-dispatch and its re-review", runner.calls, rev.calls)
	}
	if !strings.Contains(log, "WARN contract_correction ledger append: forced contract_correction append fail") {
		t.Fatalf("stderr must surface the lost correction row:\n%s", log)
	}
	if dr.phaseReq.CorrectionDirective != "" {
		t.Fatalf("the correction directive outlived the ladder: %q", dr.phaseReq.CorrectionDirective)
	}
}

func TestReviewWithCorrections_ALeakAfterACorrectionAbortsBeforeTheReReview(t *testing.T) {
	root := gittest.Fixture(t).Dir
	blocked := filepath.Join(root, "blocked-worktree")
	writeFile(t, blocked, "not a directory\n")
	writeFile(t, filepath.Join(root, "leak.txt"), "escaped build output\n")
	rev := &sequencedReviewer{phase: "build", results: []ReviewResult{{Approve: false, Reason: "missing header"}, {Approve: true}}}
	runner := &countingPassRunnerE4{name: "build"}
	cr, dr := correctionRunE4(t, rev, &fakeLedger{}, root, runner)
	cr.cs.ActiveWorktree = blocked
	cr.checkout = &checkoutDecision{decided: true, checkout: true}
	dr.treeGuard = &treediff.Guard{}
	dr.beforeDirty = []string{"blocked-worktree"}

	var act loopAction
	var err error
	captureStderr(t, func() { act, err = cr.reviewWithCorrections(PhaseBuild, dr) })

	if act != loopAbort || err == nil || !strings.Contains(err.Error(), "worktree-leak recovery failed") {
		t.Fatalf("act=%v err=%v, want an abort on the failed leak recovery", act, err)
	}
	if runner.calls != 1 || rev.calls != 1 {
		t.Fatalf("runner=%d reviews=%d: the corrected deliverable must never be re-reviewed over an unsafe tree", runner.calls, rev.calls)
	}
	if len(cr.result.PhasesRun) == 0 || cr.result.PhasesRun[0] != PhaseBuild {
		t.Fatalf("the failed correction must be recorded as the phase outcome: %v", cr.result.PhasesRun)
	}
	if _, statErr := os.Stat(filepath.Join(root, "leak.txt")); statErr != nil {
		t.Fatalf("the leaked file must stay where it is: %v", statErr)
	}
}

func TestReviewWithCorrections_ALiveFixRungSpendsItsBudgetWithoutRedispatching(t *testing.T) {
	var inputs []interaction.CorrectionInput
	orig := nextCorrection
	nextCorrection = func(in interaction.CorrectionInput) interaction.CorrectionAction {
		copied := in
		copied.RungBudget = map[string]int{}
		for k, v := range in.RungBudget {
			copied.RungBudget[k] = v
		}
		inputs = append(inputs, copied)
		if len(inputs) == 1 {
			return interaction.CorrectionAction{Rung: interaction.RungLiveFix, Reason: "named session"}
		}
		return interaction.CorrectionAction{Reason: "exhausted"}
	}
	t.Cleanup(func() { nextCorrection = orig })
	rev := &sequencedReviewer{phase: "build", results: []ReviewResult{{Approve: false, Reason: "still missing header"}}}
	runner := &countingPassRunnerE4{name: "build"}
	cr, dr := correctionRunE4(t, rev, &fakeLedger{}, t.TempDir(), runner)

	var act loopAction
	var err error
	captureStderr(t, func() { act, err = cr.reviewWithCorrections(PhaseBuild, dr) })

	if act != loopAbort || err == nil || !strings.Contains(err.Error(), "still missing header") {
		t.Fatalf("act=%v err=%v, want the exhausted ladder to abort with the reviewer's reason", act, err)
	}
	if runner.calls != 0 || rev.calls != 1 {
		t.Fatalf("runner=%d reviews=%d: a live-fix rung re-dispatches nothing at v1", runner.calls, rev.calls)
	}
	if len(inputs) != 2 || inputs[0].RungBudget[interaction.RungLiveFix] != 1 || inputs[1].RungBudget[interaction.RungLiveFix] != 0 {
		t.Fatalf("ladder inputs = %+v, want the live-fix budget spent between the two decisions", inputs)
	}
	if inputs[1].RungBudget[interaction.RungRedispatch] != inputs[0].RungBudget[interaction.RungRedispatch] {
		t.Fatalf("the live-fix rung spent re-dispatch budget: %+v", inputs)
	}
}
