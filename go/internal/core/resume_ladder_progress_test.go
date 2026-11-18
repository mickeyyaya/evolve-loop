package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

type phaseReviewers map[string]DeliverableReviewer

func (p phaseReviewers) Review(ctx context.Context, in ReviewInput) ReviewResult {
	if r, ok := p[in.Phase]; ok {
		return r.Review(ctx, in)
	}
	return ReviewResult{Approve: true}
}

func newPhaseLadderProbe(phase Phase, approveAfter, wallAt int) *wallAtCallRunner {
	probe := &wallAtCallRunner{ladderProbe: newLadderProbe(approveAfter), wallAt: wallAt}
	probe.phase = string(phase)
	return probe
}

func rungOrder(r retryladder.Rung) int {
	return int(r[len(r)-1])
}

func TestResumedRun_LadderProgressAfterTheResumedPhaseSurvivesTheNextDispatch(t *testing.T) {
	t.Parallel()
	scout := newPhaseLadderProbe(PhaseScout, 2, 0)
	tdd := newPhaseLadderProbe(PhaseTDD, 2, 0)
	reviewers := phaseReviewers{string(PhaseScout): scout, string(PhaseTDD): tdd}
	cycle := newResumedLadderCycleWith(t, policy.RetryLadderEnforce, CycleState{},
		map[Phase]PhaseRunner{PhaseScout: scout, PhaseTDD: tdd}, reviewers)
	if err := cycle.resumeAt(PhaseScout); err != nil {
		t.Fatalf("resume: %v", err)
	}
	resumed := scout.requests[len(scout.requests)-1]
	later := tdd.requests[len(tdd.requests)-1]
	if resumed.Ladder == nil || later.Ladder == nil || rungOrder(later.Ladder.Rung) <= rungOrder(resumed.Ladder.Rung) {
		t.Fatalf("the tdd corrections must raise the ladder above the resumed rung: resumed %+v, tdd %+v", resumed.Ladder, later.Ladder)
	}
	if len(cycle.build.requests) == 0 {
		t.Fatal("the build phase was never dispatched")
	}
	next := cycle.build.requests[0]
	if next.Ladder == nil || next.Ladder.Rung != later.Ladder.Rung || next.Ladder.Tier != later.Ladder.Tier || next.Assignment.Effort != later.Assignment.Effort {
		t.Fatalf("a later raise of the resumed run was reverted to the resumed rung at the next dispatch: build %+v effort %q, later raise %+v effort %q",
			next.Ladder, next.Assignment.Effort, later.Ladder, later.Assignment.Effort)
	}
}

func TestResumedRun_AQuotaWallAfterLaterLadderProgressCheckpointsTheLaterRung(t *testing.T) {
	var checkpoints []CycleState
	previous := QuotaBoundaryCheckpointer
	t.Cleanup(func() { QuotaBoundaryCheckpointer = previous })
	QuotaBoundaryCheckpointer = func(cs CycleState, _ string, _ time.Time) error {
		checkpoints = append(checkpoints, cs)
		return nil
	}

	tdd := newPhaseLadderProbe(PhaseTDD, 2, 0)
	build := newPhaseLadderProbe(PhaseBuild, 0, 2)
	reviewers := phaseReviewers{string(PhaseTDD): tdd, string(PhaseBuild): build}
	cycle := newResumedLadderCycleWith(t, policy.RetryLadderEnforce, CycleState{},
		map[Phase]PhaseRunner{PhaseTDD: tdd, PhaseBuild: build}, reviewers)
	if err := cycle.resumeAt(PhaseTDD); !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("the wall must pause the resumed run: %v", err)
	}
	resumedRung := tdd.requests[len(tdd.requests)-1].Ladder
	laterTry := build.requests[len(build.requests)-1]
	if resumedRung == nil || laterTry.Ladder == nil {
		t.Fatalf("the resumed run and its later build try must both carry the ladder: resumed %+v, build %+v", resumedRung, laterTry.Ladder)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("the pause wrote %d quota checkpoints, want 1", len(checkpoints))
	}
	saved := checkpoints[0].Ladder
	if saved == nil || saved.Rung != string(laterTry.Ladder.Rung) || saved.Tier != laterTry.Ladder.Tier || saved.Effort != laterTry.Assignment.Effort {
		t.Fatalf("the quota checkpoint kept the resumed rung and lost the later raise: saved %+v, later try %+v effort %q", saved, laterTry.Ladder, laterTry.Assignment.Effort)
	}
}

func newAdoptingRun(t *testing.T, cycle int, kept *LadderState) *cycleRun {
	t.Helper()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithWorkflowConfig(ladderWorkflow(policy.RetryLadderEnforce)))
	keepResumedLadder(o, cycle, kept)
	return &cycleRun{o: o, cycle: cycle, ctx: context.Background(), req: CycleRequest{ProjectRoot: t.TempDir()}, cs: CycleState{WorkspacePath: t.TempDir()}}
}

func laterProgress() *LadderState {
	return &LadderState{Rung: string(retryladder.R3), Tier: "deep", Effort: "high", AuditFix: "mark", RepairRound: 2}
}

func TestAdoptedResumedLadder_ProgressMadeAfterTheAdoptionSurvivesTheNextDispatch(t *testing.T) {
	run := newAdoptingRun(t, 77, &LadderState{Rung: string(retryladder.R1), Tier: "balanced", Effort: "medium"})

	first := PhaseRequest{}
	run.applyDispatchPolicy(PhaseTDD, &first)
	if first.Ladder == nil || first.Ladder.Rung != retryladder.R1 {
		t.Fatalf("the first dispatch must adopt the recorded rung: %+v", first.Ladder)
	}

	run.cs.Ladder = laterProgress()
	second := PhaseRequest{}
	run.applyDispatchPolicy(PhaseTDD, &second)
	if second.Ladder == nil || second.Ladder.Rung != retryladder.R3 || second.Assignment.Effort != "high" {
		t.Fatalf("the next dispatch reverted the later raise to the adopted snapshot: %+v effort %q", second.Ladder, second.Assignment.Effort)
	}
	if got := run.cs.Ladder; got == nil || got.AuditFix != "mark" || got.RepairRound != 2 || got.Rung != string(retryladder.R3) {
		t.Fatalf("the next dispatch overwrote the ladder state of the run: %+v", got)
	}
}

func TestAdoptedResumedLadder_ProgressMadeAfterTheAdoptionSurvivesAQuotaPause(t *testing.T) {
	var checkpoints []CycleState
	previous := QuotaBoundaryCheckpointer
	t.Cleanup(func() { QuotaBoundaryCheckpointer = previous })
	QuotaBoundaryCheckpointer = func(cs CycleState, _ string, _ time.Time) error {
		checkpoints = append(checkpoints, cs)
		return nil
	}
	run := newAdoptingRun(t, 78, &LadderState{Rung: string(retryladder.R1), Tier: "balanced", Effort: "medium"})

	dispatch := PhaseRequest{}
	run.applyDispatchPolicy(PhaseTDD, &dispatch)
	run.cs.Ladder = laterProgress()
	if err := run.pauseForQuota(PhaseTDD, PhaseResponse{}, 1, wrapTransient(85)); !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("the wall must pause the run: %v", err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("the pause wrote %d quota checkpoints, want 1", len(checkpoints))
	}
	if got := checkpoints[0].Ladder; got == nil || got.Rung != string(retryladder.R3) || got.AuditFix != "mark" || got.RepairRound != 2 {
		t.Fatalf("the quota checkpoint kept the adopted snapshot and lost the later raise: %+v", got)
	}
}
