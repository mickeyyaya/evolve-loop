package core

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// stubAdjudicator records whether it was consulted, so the no-deep-tier-without-a-choice guarantee is pinned.
type stubAdjudicator struct {
	give     *adjudication
	consults int
}

func (s *stubAdjudicator) Adjudicate(_ CycleState, _ retryEnvelope) *adjudication {
	s.consults++
	return s.give
}

func auditFailFixture(t *testing.T, class string, defects ...string) string {
	t.Helper()
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", class, defects...)
	return dir
}

func TestDecideAfterAuditFail(t *testing.T) {
	fp := policy.DefaultSystemFailurePolicy()

	tests := []struct {
		name         string
		class        string
		attempts     int
		adj          *adjudication
		wantNext     Phase
		wantHalt     bool
		wantConsults int
	}{
		{
			name:         "task-level audit fail re-enters the dev cycle",
			class:        policy.CategoryCodeAuditFail,
			adj:          &adjudication{Action: retryActionRetryTDD, Justification: "encode the defects as tests first"},
			wantNext:     PhaseTDD,
			wantConsults: 1,
		},
		{
			name:         "the adjudicator may choose the cheaper re-entry",
			class:        policy.CategoryCodeAuditFail,
			adj:          &adjudication{Action: retryActionRetryBuild, Justification: "tests are right; the change is wrong"},
			wantNext:     PhaseBuild,
			wantConsults: 1,
		},
		{
			name:         "an absent adjudication still retries at the policy default",
			class:        policy.CategoryCodeAuditFail,
			adj:          nil,
			wantNext:     PhaseTDD,
			wantConsults: 1,
		},
		{
			name:         "at the policy cap the cycle goes to retro",
			class:        policy.CategoryCodeAuditFail,
			attempts:     2,
			wantNext:     PhaseRetro,
			wantConsults: 0, // one legal action ⇒ no deep-tier dispatch
		},
		{
			name:         "a system-level class halts at audit, before any retry",
			class:        policy.CategoryInfraSystemic,
			wantNext:     PhaseRetro,
			wantHalt:     true,
			wantConsults: 0,
		},
		{
			name:         "an unrecognised class declines to retro",
			class:        "nobody-declared-this",
			wantNext:     PhaseRetro,
			wantConsults: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := floorOrchestrator(fixedNextStrategy{next: "end"})
			o.failurePolicy = fp
			stub := &stubAdjudicator{give: tc.adj}
			o.retryAdjudicator = stub
			cs := CycleState{
				CycleID:             1577,
				WorkspacePath:       auditFailFixture(t, tc.class, "H1 the auditor rejected this build"),
				AuditRepairAttempts: tc.attempts,
			}

			next, reason, sig := o.decideAfterAuditFail(cs)

			if next != tc.wantNext {
				t.Errorf("next = %s, want %s (reason %q)", next, tc.wantNext, reason)
			}
			if (sig != nil) != tc.wantHalt {
				t.Errorf("halt signal present = %v, want %v", sig != nil, tc.wantHalt)
			}
			if stub.consults != tc.wantConsults {
				t.Errorf("adjudicator consulted %d times, want %d — deep-tier cost must be paid only where a real choice exists",
					stub.consults, tc.wantConsults)
			}
			if reason == "" {
				t.Error("every audit-fail disposition must explain itself")
			}
		})
	}
}

func TestDecideAfterAuditFail_AdjudicatorCannotOverturnTheFloor(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "end"})
	o.failurePolicy = policy.DefaultSystemFailurePolicy()
	o.retryAdjudicator = &stubAdjudicator{give: &adjudication{
		Action:        retryActionRetryTDD,
		Justification: "I reviewed it and it looks recoverable to me",
	}}
	cs := CycleState{CycleID: 1001, WorkspacePath: auditFailFixture(t, policy.CategoryInfraSystemic, "all CLI families exhausted")}

	next, _, sig := o.decideAfterAuditFail(cs)

	if sig == nil || !sig.Halt {
		t.Fatalf("a system-level class must halt regardless of adjudication; sig=%+v", sig)
	}
	if next == PhaseTDD || next == PhaseBuild {
		t.Errorf("next = %s; an agent overturned the ADR-0072 floor", next)
	}
}

func TestDecideAfterAuditFail_NilAdjudicatorIsSafe(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "end"})
	o.failurePolicy = policy.DefaultSystemFailurePolicy()
	o.retryAdjudicator = nil
	cs := CycleState{CycleID: 1577, WorkspacePath: auditFailFixture(t, policy.CategoryCodeAuditFail, "H1")}

	next, reason, _ := o.decideAfterAuditFail(cs)

	if next != PhaseTDD {
		t.Errorf("next = %s, want tdd — policy grants the retry with no adjudicator present (reason %q)", next, reason)
	}
}

func TestAuditFailReentryEdgesAreLegal(t *testing.T) {
	sm := NewStateMachine()
	for _, target := range []Phase{PhaseTDD, PhaseBuild, PhaseRetro, PhaseShip} {
		if !sm.CanTransition(PhaseAudit, target) {
			t.Errorf("audit→%s is illegal; the audit-fail decision can produce it", target)
		}
	}
}

func TestSelectNext_AuditFailUsesTheDecision(t *testing.T) {
	sm := NewStateMachine()
	staticNext, err := sm.Next(PhaseAudit, VerdictFAIL)
	if err != nil {
		t.Fatalf("static transition: %v", err)
	}
	if staticNext != PhaseRetro {
		t.Fatalf("precondition changed: static audit-FAIL successor = %s, want retro", staticNext)
	}
	// A static successor of "end" proves the decision, not the strategy, picks the next phase.
	o := floorOrchestrator(fixedNextStrategy{next: "end"})
	o.failurePolicy = policy.DefaultSystemFailurePolicy()
	cs := CycleState{CycleID: 1577, WorkspacePath: auditFailFixture(t, policy.CategoryCodeAuditFail, "H1")}

	next, _, _ := o.decideAfterAuditFail(cs)

	if next == staticNext {
		t.Errorf("decision returned the static successor %s; the chokepoint move is inert", next)
	}
}

func TestOrchestrator_AuditFailRetriesTheDevCycle(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	// Audit always FAILs, so the cycle exhausts its retry budget before reaching retro.
	runners := buildRunners(map[Phase]string{
		PhaseAudit: VerdictFAIL,
		PhaseRetro: VerdictFAIL,
	})
	// The plain fakeRunner declares no failure class, so the disposition would decline for want of one.
	runners[PhaseAudit] = &classDeclaringAuditRunner{t: t}
	o := NewOrchestrator(st, led, runners)

	res, _ := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})

	tdds, audits := 0, 0
	for _, p := range res.PhasesRun {
		switch p {
		case PhaseTDD:
			tdds++
		case PhaseAudit:
			audits++
		}
	}
	if audits < 2 {
		t.Errorf("audit ran %d time(s); a task-level FAIL must be re-audited after a repair (phases=%v)", audits, res.PhasesRun)
	}
	if tdds < 2 {
		t.Errorf("tdd ran %d time(s); the dev cycle was never re-entered (phases=%v)", tdds, res.PhasesRun)
	}
	// MaxRetries is 2 in the policy table.
	if audits > 3 {
		t.Errorf("audit ran %d times; the policy retry budget did not bind (phases=%v)", audits, res.PhasesRun)
	}
}

// classDeclaringAuditRunner emits a FAIL verdict with the failure block a real auditor writes.
type classDeclaringAuditRunner struct{ t *testing.T }

func (r *classDeclaringAuditRunner) Name() string { return string(PhaseAudit) }

func (r *classDeclaringAuditRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	writeAuditWithFailure(r.t, req.Workspace, "FAIL", "code-audit-fail", "H1 the auditor rejected this build")
	return PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictFAIL, ArtifactsDir: req.Workspace}, nil
}

func TestRouterCannotProposeTheAuditReentryEdges(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "end"})

	for _, to := range []Phase{PhaseTDD, PhaseBuild} {
		if !o.sm.CanTransition(PhaseAudit, to) {
			t.Errorf("audit→%s must remain schedulable by decideAfterAuditFail", to)
		}
		if o.transitionLegal(PhaseAudit, to) {
			t.Errorf("the routing advisor can propose audit→%s; it would bypass the retry budget entirely", to)
		}
	}

	for _, to := range []Phase{PhaseShip, PhaseRetro} {
		if !o.transitionLegal(PhaseAudit, to) {
			t.Errorf("audit→%s must stay router-proposable; only the re-entry edges are decision-only", to)
		}
	}
}

func TestDecideAfterAuditFail_SurfacesTheAdjudicatorsReasoning(t *testing.T) {
	const why = "the tests assert the wrong contract; rebuilding without re-deriving them re-earns this"

	o := floorOrchestrator(fixedNextStrategy{next: "end"})
	o.failurePolicy = policy.DefaultSystemFailurePolicy()
	o.retryAdjudicator = &stubAdjudicator{give: &adjudication{Action: retryActionRetryTDD, Justification: why}}
	cs := CycleState{CycleID: 1577, WorkspacePath: auditFailFixture(t, policy.CategoryCodeAuditFail, "H1")}

	_, reason, _ := o.decideAfterAuditFail(cs)

	if !strings.Contains(reason, why) {
		t.Errorf("the adjudicator's justification never reached the reason:\n%s", reason)
	}
}

func TestDecideAfterAuditFail_RecordsAClamp(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "end"})
	o.failurePolicy = policy.DefaultSystemFailurePolicy()
	o.retryAdjudicator = &stubAdjudicator{give: &adjudication{Action: retryAction("ship-it"), Justification: "trust me"}}
	cs := CycleState{CycleID: 1577, WorkspacePath: auditFailFixture(t, policy.CategoryCodeAuditFail, "H1")}

	_, reason, _ := o.decideAfterAuditFail(cs)

	if !strings.Contains(reason, "clamped") {
		t.Errorf("an out-of-envelope proposal was silently overridden:\n%s", reason)
	}
}
