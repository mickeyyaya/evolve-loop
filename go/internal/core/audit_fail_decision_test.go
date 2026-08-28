package core

// audit_fail_decision_test.go — RED contract for the disposition of an audit FAIL,
// decided AT THE AUDIT CHOKEPOINT instead of after a full retrospective.
//
// This is the integration of the envelope (what policy makes legal) with the clamp
// (what an adjudicator may choose inside it). It is the seam that takes retro off
// the retry path: retro is now reached only when the disposition is DECLINE.
//
// The adjudicator is injected as a Strategy so this decision is testable without a
// bridge dispatch, and defaults to nil — the Null Object path, which must yield a
// working decision rather than no decision.

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// stubAdjudicator is a Strategy double. It also records whether it was consulted,
// so the cost guarantee ("deep tier only where there is a real choice") is pinned
// rather than assumed.
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
			// The wave-3/4 shape: a task-level rejection now retries, with no
			// dependency on retro having written anything.
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
			// NULL OBJECT: no adjudicator output at all still retries, because
			// POLICY — not the agent — is what grants the retry.
			name:         "an absent adjudication still retries at the policy default",
			class:        policy.CategoryCodeAuditFail,
			adj:          nil,
			wantNext:     PhaseTDD,
			wantConsults: 1,
		},
		{
			// Retro is now REACHED, not passed through: it is the terminal
			// learning step once the budget is spent.
			name:         "at the policy cap the cycle goes to retro",
			class:        policy.CategoryCodeAuditFail,
			attempts:     2,
			wantNext:     PhaseRetro,
			wantConsults: 0, // one legal action ⇒ no deep-tier dispatch
		},
		{
			// The floor still binds, and binds BEFORE any retry — the whole point
			// of moving the chokepoint without weakening ADR-0072.
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

// The safety property at the integration level: an adjudicator cannot talk the
// cycle out of a floor halt.
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

// A nil adjudicator (the production default until the persona is wired) must not
// panic and must not block retries — policy alone is sufficient authority.
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

// The graph must permit what the decision produces, or the disposition is computed
// and then rejected at dispatch — wired and inert, the failure shape that has
// recurred repeatedly in this subsystem.
func TestAuditFailReentryEdgesAreLegal(t *testing.T) {
	sm := NewStateMachine()
	for _, target := range []Phase{PhaseTDD, PhaseBuild, PhaseRetro, PhaseShip} {
		if !sm.CanTransition(PhaseAudit, target) {
			t.Errorf("audit→%s is illegal; the audit-fail decision can produce it", target)
		}
	}
}

// And the live selector must actually TAKE the decision on an audit FAIL, rather
// than falling through to the static successor.
func TestSelectNext_AuditFailUsesTheDecision(t *testing.T) {
	sm := NewStateMachine()
	staticNext, err := sm.Next(PhaseAudit, VerdictFAIL)
	if err != nil {
		t.Fatalf("static transition: %v", err)
	}
	if staticNext != PhaseRetro {
		t.Fatalf("precondition changed: static audit-FAIL successor = %s, want retro", staticNext)
	}
	// The decision must be able to disagree with the static successor — that is
	// the whole point of the chokepoint move.
	o := floorOrchestrator(fixedNextStrategy{next: "end"})
	o.failurePolicy = policy.DefaultSystemFailurePolicy()
	cs := CycleState{CycleID: 1577, WorkspacePath: auditFailFixture(t, policy.CategoryCodeAuditFail, "H1")}

	next, _, _ := o.decideAfterAuditFail(cs)

	if next == staticNext {
		t.Errorf("decision returned the static successor %s; the chokepoint move is inert", next)
	}
}

// THE LIVE PATH. Everything above tests the decision in isolation; this drives a
// real cycle and asserts the retry actually happens. A decision computed and never
// consumed is the failure shape this subsystem keeps producing — a router eating a
// grant, a config knob reaching no composition root, a seeder called with the wrong
// phase. Each was green in isolation and inert in production.
func TestOrchestrator_AuditFailRetriesTheDevCycle(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	// Audit always FAILs, so the cycle must exhaust its policy retry budget and
	// only then reach retro.
	runners := buildRunners(map[Phase]string{
		PhaseAudit: VerdictFAIL,
		PhaseRetro: VerdictFAIL,
	})
	// A real auditor writes a report declaring its failure CLASS — verified
	// against cycles 1572/1574/1576/1577, all of which declare "code-audit-fail".
	// The plain fakeRunner writes no artifact, so the disposition would correctly
	// (but unrealistically) decline for want of a class.
	runners[PhaseAudit] = &classDeclaringAuditRunner{t: t}
	o := NewOrchestrator(st, led, runners)

	res, _ := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})

	// The dev cycle must be re-entered, not torn down on the first rejection.
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
	// And the budget must BIND: MaxRetries is 2 in the policy table.
	if audits > 3 {
		t.Errorf("audit ran %d times; the policy retry budget did not bind (phases=%v)", audits, res.PhasesRun)
	}
}

// classDeclaringAuditRunner emits a FAIL verdict AND the machine-readable failure
// block a real auditor emits, so the disposition has the class it keys on.
type classDeclaringAuditRunner struct{ t *testing.T }

func (r *classDeclaringAuditRunner) Name() string { return string(PhaseAudit) }

func (r *classDeclaringAuditRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	writeAuditWithFailure(r.t, req.Workspace, "FAIL", "code-audit-fail", "H1 the auditor rejected this build")
	return PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictFAIL, ArtifactsDir: req.Workspace}, nil
}

// C1 (architect review, CRITICAL). Adding audit→tdd/build to the legality graph
// made them legal for EVERYONE — including the routing advisor, which validates
// its proposals through the same graph. With routing at `advisory` (the live
// default) the advisor could therefore:
//
//   - grant a retry the envelope REFUSED, on a path that never calls
//     consumeAuditRepairGrant, so the budget is bypassed and the only remaining
//     bound is defaultMaxPhaseIterations (32), not MaxRetries (2);
//   - route backwards after a halt was signalled;
//   - override audit→ship on a PASSING audit, discarding the pass.
//
// The comment on the new edges claimed they were "legal ONLY through
// decideAfterAuditFail". This makes that true: the edges stay in the ONE legality
// graph (so the deterministic decision can schedule them and the SSOT is not
// forked), but the router may not PROPOSE them.
func TestRouterCannotProposeTheAuditReentryEdges(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "end"})

	for _, to := range []Phase{PhaseTDD, PhaseBuild} {
		// The deterministic decision may schedule it...
		if !o.sm.CanTransition(PhaseAudit, to) {
			t.Errorf("audit→%s must remain schedulable by decideAfterAuditFail", to)
		}
		// ...but the routing advisor may not propose it.
		if o.transitionLegal(PhaseAudit, to) {
			t.Errorf("the routing advisor can propose audit→%s; it would bypass the retry budget entirely", to)
		}
	}

	// Every other audit edge is unchanged for the router.
	for _, to := range []Phase{PhaseShip, PhaseRetro} {
		if !o.transitionLegal(PhaseAudit, to) {
			t.Errorf("audit→%s must stay router-proposable; only the re-entry edges are decision-only", to)
		}
	}
}

// The adjudicator's REASONING must reach the operator-visible reason. The clamp
// rejects an unjustified proposal because the justification is this phase's whole
// deliverable — computing it, requiring it, and discarding it is the defect shape
// ADR-0092's Incoherent flag had (review finding #6).
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

// A clamped proposal must SAY it was clamped, so an operator can tell a followed
// recommendation from an overridden one.
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
