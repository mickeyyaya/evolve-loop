package core

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func preBuildEvaluateOrchestrator(t *testing.T) *Orchestrator {
	t.Helper()
	return &Orchestrator{
		cfg:     config.RoutingConfig{Order: []string{"scout", "fault-localization", "bug-reproduction", "build", "audit", "ship"}},
		catalog: catalogOf(t, specWith("bug-reproduction", "evaluate", ""), specWith("fault-localization", "evaluate", "")),
	}
}

func TestResolveResumeNext_ANoPlanResumeOfAPreBuildEvaluatePhaseGoesToBuild(t *testing.T) {
	t.Parallel()
	o := preBuildEvaluateOrchestrator(t)
	next, err := o.resolveResumeNext(CycleState{WorkspacePath: t.TempDir()}, "bug-reproduction", VerdictPASS)
	if err != nil || next != PhaseBuild {
		t.Fatalf("a no-plan resume of bug-reproduction must follow the routing order to build, never degrade by archetype to audit: next=%q err=%v", next, err)
	}
}

func TestResolveResumeNext_TheRoutingOrderOutranksThePlanArrayOrder(t *testing.T) {
	t.Parallel()
	o := preBuildEvaluateOrchestrator(t)
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "routing-plan.json"),
		`[{"phase":"scout","run":true},{"phase":"bug-reproduction","run":true},{"phase":"audit","run":true},{"phase":"build","run":true}]`)
	if next, err := o.resolveResumeNext(CycleState{WorkspacePath: ws}, "bug-reproduction", VerdictPASS); err != nil || next != PhaseBuild {
		t.Fatalf("the live walk takes the successor from the routing order, so the resume must too: next=%q err=%v", next, err)
	}
}

func TestResolveResumeNext_TheOrderWalkSkipsWhatThePlanDoesNotRun(t *testing.T) {
	t.Parallel()
	o := preBuildEvaluateOrchestrator(t)
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "routing-plan.json"),
		`[{"phase":"fault-localization","run":true},{"phase":"bug-reproduction","run":false},{"phase":"build","run":true}]`)
	if next, err := o.resolveResumeNext(CycleState{WorkspacePath: ws}, "fault-localization", VerdictPASS); err != nil || next != PhaseBuild {
		t.Fatalf("a phase the plan marks run:false is not the successor: next=%q err=%v", next, err)
	}
	if next, err := o.resolveResumeNext(CycleState{WorkspacePath: ws}, "unlisted-planner", VerdictPASS); err != nil || next != PhaseBuild {
		t.Fatalf("a phase outside the order and the plan still degrades by archetype (plan, the default, goes to build): next=%q err=%v", next, err)
	}
}

func TestResolveResumeNext_APlanThatRunsNoLaterOptionalPhaseEndsTheCycle(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{cfg: config.RoutingConfig{Order: []string{"build", "audit", "closing-note", "digest", "recap", "digest"}}}
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "routing-plan.json"),
		`[{"phase":"closing-note","run":true},{"phase":"digest","run":false},{"phase":"recap","run":false}]`)
	if next, err := o.resolveResumeNext(CycleState{WorkspacePath: ws}, "closing-note", VerdictPASS); err != nil || next != PhaseEnd {
		t.Fatalf("with every later optional phase marked run:false, and a duplicate name in the order, the walk ends the cycle: next=%q err=%v", next, err)
	}
}

func TestResolveResumeNext_ThePlanCannotSkipTheIntegrityFloor(t *testing.T) {
	t.Parallel()
	o := preBuildEvaluateOrchestrator(t)
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "routing-plan.json"),
		`[{"phase":"bug-reproduction","run":true},{"phase":"build","run":false},{"phase":"audit","run":false},{"phase":"ship","run":true}]`)
	if next, err := o.resolveResumeNext(CycleState{WorkspacePath: ws}, "bug-reproduction", VerdictPASS); err != nil || next != PhaseBuild {
		t.Fatalf("routing-plan.json is the raw plan before the floor clamp; a resume must never skip build: next=%q err=%v", next, err)
	}
	writeFileAt(t, filepath.Join(ws, "routing-plan.json"),
		`[{"phase":"minted-check","run":true},{"phase":"build","run":false},{"phase":"audit","run":false},{"phase":"ship","run":true}]`)
	if next, err := o.resolveResumeNext(CycleState{WorkspacePath: ws}, "minted-check", VerdictPASS); err != nil || next != PhaseBuild {
		t.Fatalf("the plan-array fallback must not skip build either: next=%q err=%v", next, err)
	}
}

func TestResolveResumeNext_ThePlanCannotSkipAuditOrShip(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{cfg: config.RoutingConfig{Order: []string{"build", "test-amplification", "audit", "post-audit-note", "ship"}}}
	ws := t.TempDir()
	cs := CycleState{WorkspacePath: ws, CompletedPhases: []string{"build", "test-amplification"}}
	writeFileAt(t, filepath.Join(ws, "routing-plan.json"),
		`[{"phase":"test-amplification","run":true},{"phase":"audit","run":false},{"phase":"post-audit-note","run":true},{"phase":"ship","run":false}]`)
	if next, err := o.resolveResumeNext(cs, "test-amplification", VerdictPASS); err != nil || next != PhaseAudit {
		t.Fatalf("a resume must never skip audit: next=%q err=%v", next, err)
	}
	afterAudit := CycleState{WorkspacePath: ws, CompletedPhases: []string{"build", "test-amplification", "audit", "post-audit-note"}}
	if next, err := o.resolveResumeNext(afterAudit, "post-audit-note", VerdictPASS); err != nil || next != PhaseShip {
		t.Fatalf("a resume must never skip ship: next=%q err=%v", next, err)
	}
}

func TestResolveResumeNext_ThePlanCannotSkipAPinnedTDD(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{cfg: config.RoutingConfig{Order: []string{"scout", "bug-reproduction", "tdd", "build", "audit", "ship"}}}
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "routing-plan.json"),
		`[{"phase":"bug-reproduction","run":true},{"phase":"tdd","run":false},{"phase":"build","run":true}]`)
	if next, err := o.resolveResumeNext(CycleState{WorkspacePath: ws}, "bug-reproduction", VerdictPASS); err != nil || next != PhaseTDD {
		t.Fatalf("tdd is on the ship floor unless a conditional rule unpins it, so a resume must not skip it: next=%q err=%v", next, err)
	}
}

func TestResolveResumeNext_ATrivialCycleUnpinsTDDAsTheFloorClampDoes(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{cfg: config.RoutingConfig{
		Order:       []string{"scout", "bug-reproduction", "tdd", "build", "audit", "ship"},
		Conditional: map[string]config.CondRule{"tdd": {Field: "cycle_size", Op: "!=", Value: "trivial"}},
	}}
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "handoff-scout.json"), `{"cycle_size_estimate":"trivial"}`)
	writeFileAt(t, filepath.Join(ws, "routing-plan.json"),
		`[{"phase":"bug-reproduction","run":true},{"phase":"tdd","run":false},{"phase":"build","run":true}]`)
	cs := CycleState{WorkspacePath: ws, CompletedPhases: []string{"scout", "bug-reproduction"}}
	if next, err := o.resolveResumeNext(cs, "bug-reproduction", VerdictPASS); err != nil || next != PhaseBuild {
		t.Fatalf("a trivial cycle does not pin tdd, so the plan's tdd run:false holds on resume: next=%q err=%v", next, err)
	}
	if next, err := o.resolveResumeNext(CycleState{WorkspacePath: filepath.Join(ws, "absent")}, "bug-reproduction", VerdictPASS); err != nil || next != PhaseTDD {
		t.Fatalf("when the signals cannot be read, tdd stays on the floor (fail closed): next=%q err=%v", next, err)
	}
}

func TestResolveResumeNext_ThePlanFallbackCannotSkipAConfiguredMandatoryPhase(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{cfg: config.RoutingConfig{Order: []string{"build", "audit", "ship"}, Mandatory: []string{"security-scan"}}}
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "routing-plan.json"),
		`[{"phase":"minted-check","run":true},{"phase":"security-scan","run":false},{"phase":"build","run":true}]`)
	if next, err := o.resolveResumeNext(CycleState{WorkspacePath: ws}, "minted-check", VerdictPASS); err != nil || next != "security-scan" {
		t.Fatalf("the plan-array fallback must run a cfg.Mandatory phase: next=%q err=%v", next, err)
	}
}

func TestHoldShipFloor_ShipWaitsForBuildAndAudit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		next      Phase
		completed []string
		want      Phase
	}{
		{PhaseShip, nil, PhaseBuild},
		{PhaseShip, []string{"build"}, PhaseAudit},
		{PhaseShip, []string{"build", "audit"}, PhaseShip},
		{PhaseEnd, nil, PhaseEnd},
	} {
		if got := holdShipFloor(CycleState{CompletedPhases: tc.completed}, tc.next); got != tc.want {
			t.Errorf("holdShipFloor(%s after %v) = %s, want %s", tc.next, tc.completed, got, tc.want)
		}
	}
}

func TestResolveResumeNext_TheDegradeNeverReachesShipBeforeAudit(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{catalog: catalogOf(t, specWith("c1", "control", ""))}
	if next, err := o.resolveResumeNext(CycleState{WorkspacePath: t.TempDir(), CompletedPhases: []string{"build"}}, "c1", VerdictPASS); err != nil || next != PhaseAudit {
		t.Fatalf("a control phase degrades toward ship, and the floor holds it at audit: next=%q err=%v", next, err)
	}
}

func TestDegradeInsertedSuccessor_GoesToBuildUntilBuildIsComplete(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{catalog: catalogOf(t, specWith("c1", "control", ""), specWith("e1", "evaluate", ""))}
	for _, tc := range []struct {
		phase     Phase
		completed []string
		want      Phase
	}{
		{"c1", nil, PhaseBuild},
		{"e1", nil, PhaseBuild},
		{"c1", []string{"build"}, PhaseShip},
		{"c1", []string{"build", "audit"}, PhaseShip},
	} {
		if got := o.degradeInsertedSuccessor(CycleState{CompletedPhases: tc.completed}, tc.phase); got != tc.want {
			t.Errorf("%s after %v -> %s, want %s (build first, then the archetype)", tc.phase, tc.completed, got, tc.want)
		}
	}
}

func TestResolveResumeNext_ThePlanCannotSkipAConfiguredMandatoryPhase(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{cfg: config.RoutingConfig{
		Order:     []string{"scout", "bug-reproduction", "security-scan", "build", "audit", "ship"},
		Mandatory: []string{"security-scan"},
	}}
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "routing-plan.json"),
		`[{"phase":"bug-reproduction","run":true},{"phase":"security-scan","run":false},{"phase":"build","run":true}]`)
	if next, err := o.resolveResumeNext(CycleState{WorkspacePath: ws}, "bug-reproduction", VerdictPASS); err != nil || next != "security-scan" {
		t.Fatalf("a phase in cfg.Mandatory runs on resume whatever the raw plan says: next=%q err=%v", next, err)
	}
}

func TestResolveResumeNext_APhaseOutsideTheOrderFallsBackToThePlan(t *testing.T) {
	t.Parallel()
	o := preBuildEvaluateOrchestrator(t)
	ws := t.TempDir()
	writeFileAt(t, filepath.Join(ws, "routing-plan.json"),
		`[{"phase":"minted-check","run":true},{"phase":"ship","run":true}]`)
	done := CycleState{WorkspacePath: ws, CompletedPhases: []string{"build", "audit"}}
	if next, err := o.resolveResumeNext(done, "minted-check", VerdictPASS); err != nil || next != PhaseShip {
		t.Fatalf("a minted phase that is not in the order resumes by the plan: next=%q err=%v", next, err)
	}
	if next, err := o.resolveResumeNext(CycleState{WorkspacePath: ws}, "minted-check", VerdictPASS); err != nil || next != PhaseBuild {
		t.Fatalf("a plan with no build entry must not reach ship before build: next=%q err=%v", next, err)
	}
	if next, err := o.resolveResumeNext(CycleState{WorkspacePath: ws, CompletedPhases: []string{"build"}}, "minted-check", VerdictPASS); err != nil || next != PhaseAudit {
		t.Fatalf("a plan with no audit entry must not reach ship before audit: next=%q err=%v", next, err)
	}
}

func TestResolveResumeNext_TheLastPhaseOfTheOrderEndsTheCycle(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{cfg: config.RoutingConfig{Order: []string{"build", "audit", "closing-note"}}}
	if next, err := o.resolveResumeNext(CycleState{WorkspacePath: t.TempDir()}, "closing-note", VerdictPASS); err != nil || next != PhaseEnd {
		t.Fatalf("the order's last phase ends the cycle: next=%q err=%v", next, err)
	}
}

func TestRegisterMintedPhases_ABatchMateAnchorResolvesThroughTheFixpoint(t *testing.T) {
	t.Parallel()
	signals := signalcenter.New()
	o := mintOrchestrator(t, fakeMinter{})
	o.signals = signals
	plan := &router.PhasePlan{MintPhases: []phaseconfig.PhaseConfig{
		{PhaseSpec: phasespec.PhaseSpec{Name: "late", Optional: true, After: "early"}, Prompt: "inline persona for late"},
		{PhaseSpec: phasespec.PhaseSpec{Name: "early", Optional: true, After: "scout"}, Prompt: "inline persona for early"},
	}}
	o.registerMintedPhases(plan)
	if got := strings.Join(o.cfg.Order, ","); got != "scout,early,late,build,audit,ship" {
		t.Fatalf("a minted phase anchored to a later batch-mate must slot after it: order=%s", got)
	}
	signals.Flush()
	for _, e := range signals.Recent() {
		if e.Code == phasespec.CodeRoutingDefect {
			t.Fatalf("a batch whose anchors resolve has no routing defect: %+v", e)
		}
	}
}

func TestRegisterMintedPhases_AnUnresolvableAnchorIsACodedSignal(t *testing.T) {
	t.Parallel()
	signals := signalcenter.New()
	o := mintOrchestrator(t, fakeMinter{})
	o.signals = signals
	o.registerMintedPhases(&router.PhasePlan{MintPhases: []phaseconfig.PhaseConfig{
		{PhaseSpec: phasespec.PhaseSpec{Name: "orphan", Optional: true, After: "nowhere"}, Prompt: "inline persona for orphan"},
	}})
	signals.Flush()
	recent := signals.Recent()
	if len(recent) != 1 || recent[0].Code != phasespec.CodeRoutingDefect || !strings.Contains(recent[0].Reason, "anchor nowhere") {
		t.Fatalf("an unresolvable anchor must reach the Signal Center as the registered routing-defect code: %+v", recent)
	}
}

func TestRegisterMintedPhases_ARejectedOrClashingMintIsACodedSignal(t *testing.T) {
	t.Parallel()
	signals := signalcenter.New()
	o := mintOrchestrator(t, fakeMinter{reject: map[string]bool{"refused": true}})
	o.signals = signals
	o.registerMintedPhases(mintPlan("refused", "build"))
	signals.Flush()
	var reasons []string
	for _, e := range signals.Recent() {
		if e.Code == phasespec.CodeRoutingDefect {
			reasons = append(reasons, e.Reason)
		}
	}
	joined := strings.Join(reasons, "\n")
	if len(reasons) != 2 || !strings.Contains(joined, `minted phase "refused" rejected`) || !strings.Contains(joined, `minted phase "build" clashes`) {
		t.Fatalf("a rejected mint and a clashing mint must each be a coded routing defect, got %q", joined)
	}
}
