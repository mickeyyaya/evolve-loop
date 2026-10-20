package core

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
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
		PhaseAudit, probe, PhaseRequest{Attempt: 1}, PhaseResponse{Phase: "audit", Verdict: VerdictPASS}, nil, (&cycleRun{phaseDispatches: map[Phase]int{PhaseAudit: 1}}).countDispatch); err != nil {
		t.Fatalf("reviewResumedDeliverable: %v", err)
	}
	if !slices.Equal(probe.attempts, []int{2, 3}) {
		t.Fatalf("resumed correction attempts %v, want [2 3] after the resumed dispatch 1", probe.attempts)
	}
}

func TestDispatchAttempt_ARemediationRoundIsTheNextDispatchOfTheGateAndTheBuilder(t *testing.T) {
	t.Parallel()
	wf := policy.WorkflowConfig{RemediationRounds: 1, RemediablePhases: []string{"tdd"}}

	gate, build, _, err := remediationHarness(t, wf, PhaseTDD, []string{VerdictFAIL, VerdictPASS})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if !slices.Equal(gate.attempts, []int{1, 2}) || !slices.Equal(build.attempts, []int{1, 2}) {
		t.Fatalf("gate attempts %v build attempts %v, want [1 2] each: the gate re-run and the builder fix are the next dispatches", gate.attempts, build.attempts)
	}
}

func TestDispatchAttempt_AnEvaluateMembersInfraRelaunchIsItsNextAttempt(t *testing.T) {
	t.Parallel()
	member := &fakeRunner{name: string(PhaseAudit), failErr: wrapTimeout(), failUntil: 1}
	cr := newBatchCycleRun(t, map[Phase]PhaseRunner{PhaseAudit: member}, 1)

	if _, err := cr.dispatchEvaluateBatch([]Phase{PhaseAudit}); err != nil {
		t.Fatalf("dispatchEvaluateBatch: %v", err)
	}
	var got []int
	for _, r := range member.requests {
		got = append(got, r.Attempt)
	}
	if !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("evaluate member attempts %v, want [1 2]", got)
	}
}

func TestDispatchAttempt_AResumedRunCountsItsCorrectionsAfterTheResumedDispatch(t *testing.T) {
	t.Parallel()
	projectRoot, st := resumedCycleNine(t)
	probe := &attemptProbe{escalationProbe: &escalationProbe{phase: "audit", threshold: neverDemotingThreshold, approveAfter: 1}}
	runners := buildRunners(nil)
	runners[PhaseAudit] = probe
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithWorktreeProvisioner(&fakeWorktree{path: st.cycleState.ActiveWorktree}), WithReviewer(probe))

	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: projectRoot, GoalHash: "g"}, &ResumePoint{Phase: string(PhaseAudit), CycleID: 9}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if !slices.Equal(probe.attempts, []int{1, 2}) {
		t.Fatalf("resumed audit attempts %v, want [1 2]: the correction follows the resumed dispatch", probe.attempts)
	}
}

func TestDispatchAttempt_EveryPhaseOfAResumedRunSharesOneCount(t *testing.T) {
	t.Parallel()
	projectRoot, st := resumedCycleNine(t)
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil), WithWorktreeProvisioner(&fakeWorktree{path: st.cycleState.ActiveWorktree}))
	req, rp := CycleRequest{ProjectRoot: projectRoot, GoalHash: "g"}, &ResumePoint{Phase: string(PhaseAudit), CycleID: 9}
	var counted []int
	o.resumeDispatch = func(cr *cycleRun, next Phase, phaseReq PhaseRequest, _ retryOpts) (map[string]bool, PhaseResponse, int, error) {
		counted = append(counted, cr.countDispatch(PhaseBuild, phaseReq).Attempt)
		return nil, PhaseResponse{Phase: string(next), Verdict: VerdictPASS}, 1, nil
	}

	if _, err := o.RunCycleFromPhase(context.Background(), req, rp); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if !slices.Equal(counted, []int{1, 2}) {
		t.Fatalf("counts %v, want 1..n: one count lives across the iterations of a resumed run", counted)
	}
}

func resumedCycleNine(t *testing.T) (string, *fakeStorage) {
	t.Helper()
	projectRoot := t.TempDir()
	ws := filepath.Join(projectRoot, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	return projectRoot, &fakeStorage{state: State{LastCycleNumber: 9}, cycleState: CycleState{CycleID: 9, WorkspacePath: ws, ActiveWorktree: t.TempDir()}}
}
