package core

import (
	"context"
	"slices"
	"testing"
)

type attemptProbe struct {
	*escalationProbe
	attempts []int
}

func (p *attemptProbe) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	p.attempts = append(p.attempts, req.Attempt)
	return p.escalationProbe.Run(ctx, req)
}

func TestDispatchAttempt_TwoCorrectionRoundsAreAttemptsOneTwoAndThree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "agy-tmux", []string{"codex-tmux"})
	probe := &attemptProbe{escalationProbe: &escalationProbe{phase: "build", threshold: neverDemotingThreshold, approveAfter: 2}}
	runners := buildRunners(nil)
	runners[PhaseBuild] = probe
	o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 0}}, &fakeLedger{}, runners, WithReviewer(probe))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "g", DisableWorkspaceGuard: true}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if !slices.Equal(probe.attempts, []int{1, 2, 3}) {
		t.Fatalf("build attempts %v, want [1 2 3]: every correction round is the next dispatch of the phase", probe.attempts)
	}
	audit := runners[PhaseAudit].(*fakeRunner)
	if len(audit.requests) == 0 || audit.requests[0].Attempt != 1 {
		t.Fatalf("audit requests %+v, want its first dispatch at attempt 1: each phase counts its own dispatches", audit.requests)
	}
}

func TestDispatchAttempt_AnInfraRelaunchIsTheNextAttempt(t *testing.T) {
	t.Parallel()
	scout := &fakeRunner{name: string(PhaseScout), failErr: wrapTimeout(), failUntil: 1}
	runners := buildRunners(nil)
	runners[PhaseScout] = scout
	o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 0}}, &fakeLedger{}, runners)

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	var got []int
	for _, r := range scout.requests {
		got = append(got, r.Attempt)
	}
	if !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("scout attempts %v, want [1 2]", got)
	}
}

func TestDispatchAttempt_ParallelEvaluateMembersCountWithoutARace(t *testing.T) {
	t.Parallel()
	cr := &cycleRun{}
	const n = 16
	got := make(chan int, n)
	for range n {
		go func() { got <- cr.countDispatch(PhaseAudit, PhaseRequest{}).Attempt }()
	}
	var attempts []int
	for range n {
		attempts = append(attempts, <-got)
	}
	slices.Sort(attempts)
	for i, a := range attempts {
		if a != i+1 {
			t.Fatalf("attempts %v, want each of 1..%d once", attempts, n)
		}
	}
}

func TestDispatchAttempt_AResumedCorrectionContinuesTheCount(t *testing.T) {
	t.Parallel()
	probe := &attemptProbe{escalationProbe: &escalationProbe{phase: "audit", threshold: neverDemotingThreshold, approveAfter: 2}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithReviewer(probe))

	if _, err := o.reviewResumedDeliverable(context.Background(), t.TempDir(), 9, CycleState{CycleID: 9, WorkspacePath: t.TempDir()},
		PhaseAudit, probe, PhaseRequest{Attempt: 1}, PhaseResponse{Phase: "audit", Verdict: VerdictPASS}, nil); err != nil {
		t.Fatalf("reviewResumedDeliverable: %v", err)
	}
	if !slices.Equal(probe.attempts, []int{2, 3}) {
		t.Fatalf("resumed correction attempts %v, want [2 3] after the resumed dispatch 1", probe.attempts)
	}
}
