package routingtest

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestBricks_VariadicSlicesAreDefensivelyCopied pins the cycle-263 audit
// finding (C263_002): Mandatory() and Done() stored the caller's variadic
// backing array directly, so a caller mutating (or reusing) its slice after
// building the spec silently corrupted the scenario — shared-backing-storage
// aliasing. Immutability rule: return/store new objects, never alias caller
// memory.
func TestBricks_VariadicSlicesAreDefensivelyCopied(t *testing.T) {
	t.Parallel()
	args := []string{"scout", "build"}
	done := []string{"scout"}
	var s ScenarioSpec
	Mandatory(args...)(&s)
	Done(done...)(&s)
	args[0] = "MUTATED"
	done[0] = "MUTATED"
	if s.Mandatory[0] != "scout" {
		t.Fatalf("Mandatory must defensively copy its variadic args; spec saw caller mutation: %v", s.Mandatory)
	}
	if s.Completed[0] != "scout" {
		t.Fatalf("Done must defensively copy its variadic args; spec saw caller mutation: %v", s.Completed)
	}
}

func TestBrick_PositionAndVerdictSetters(t *testing.T) {
	var s ScenarioSpec
	At("build")(&s)
	Done("scout", "tdd", "build")(&s)
	CompletedVerdict("PASS")(&s)
	if s.Current != "build" || s.Verdict != "PASS" {
		t.Fatalf("position bricks did not set current/verdict: %+v", s)
	}
	if got := s.Completed; len(got) != 3 || got[0] != "scout" || got[2] != "build" {
		t.Fatalf("Done phases = %v, want [scout tdd build]", got)
	}
}

func TestBrick_StageAndInsertControls(t *testing.T) {
	var s ScenarioSpec
	Shadow()(&s)
	if s.Stage != config.StageShadow {
		t.Fatalf("Shadow stage = %v", s.Stage)
	}
	Advisory()(&s)
	Enforce()(&s)
	MaxInserts(2)(&s)
	if s.Stage != config.StageEnforce || s.MaxInsertions != 2 {
		t.Fatalf("stage/cap bricks produced %+v", s)
	}
	Off()(&s)
	if s.Stage != config.StageOff {
		t.Fatalf("Off stage = %v", s.Stage)
	}
}

func TestBrick_SignalFixturesMarkPresence(t *testing.T) {
	var s ScenarioSpec
	SmallCycle()(&s)
	TriageSize("large")(&s)
	RedBuild(2)(&s)
	Severity("HIGH")(&s)
	DiffLOC(42)(&s)
	AuditIs("WARN")(&s)
	sig := s.Signals.Signals()
	if sig.Scout.CycleSizeEstimate != "small" || !sig.Scout.Present {
		t.Fatalf("scout signal = %+v", sig.Scout)
	}
	if sig.Triage.CycleSize != "large" || !sig.Triage.Present {
		t.Fatalf("triage signal = %+v", sig.Triage)
	}
	if sig.Build.ACSRed != 2 || sig.Build.DiffLOC != 42 || !sig.Build.Present {
		t.Fatalf("build signal = %+v", sig.Build)
	}
	if sig.Audit.Verdict != "WARN" || !sig.Audit.Present {
		t.Fatalf("audit signal = %+v", sig.Audit)
	}
}

func TestBrick_AgentAndExpectationBricks(t *testing.T) {
	var s ScenarioSpec
	Agent("build", "ship", "tester")(&s)
	AgentJustified("audit", "ship", "audit passed")(&s)
	AgentError("retrospective")(&s)
	AgentPlan(PlanRun("build"), PlanSkip("audit"))(&s)
	AgentPlanError()(&s)
	ExpectNext("audit")(&s)
	ExpectInserts("tester")(&s)
	ExpectSkips("memo")(&s)
	ExpectClamp("proposal-weakened-next")(&s)
	ExpectReason("mandatory:audit")(&s)
	ExpectJustification("audit passed")(&s)
	ExpectPhases(core.PhaseScout, core.PhaseBuild)(&s)
	ExpectAbsent(core.PhaseRetro)(&s)
	ExpectDecisionInsert("tester")(&s)
	ExpectDecisionClamp("proposal-weakened-next")(&s)
	ExpectRoutingLedger(2)(&s)
	ExpectRetro("PROCEED")(&s)
	ExpectProposeAt("build")(&s)
	ExpectInvariants("determinism")(&s)

	if got := s.Agent.Proposals["build"]; got == nil || got.NextPhase != "ship" || len(got.InsertPhases) != 1 {
		t.Fatalf("Agent proposal = %+v", got)
	}
	if !s.Agent.ErrorsOn["retrospective"] || !s.Agent.PlanError || !s.Agent.active() || !s.Agent.hasPlan() {
		t.Fatalf("agent flags not populated: %+v", s.Agent)
	}
	if s.Expect.NextPhase != "audit" || s.Expect.RoutingLedgerMin != 2 || len(s.Expect.Invariants) != 1 {
		t.Fatalf("expectations not populated: %+v", s.Expect)
	}
}

func TestBrick_EnableEnvAndTriggerBricks(t *testing.T) {
	var s ScenarioSpec
	TriageOff()(&s)
	PhaseEnabled("memo", config.EnableOn)(&s)
	IntentRequired()(&s)
	StrictAudit()(&s)
	SeverityTrigger()(&s)
	if s.Enable["triage"] != config.EnableOff || s.Enable["memo"] != config.EnableOn {
		t.Fatalf("enable map = %+v", s.Enable)
	}
	if s.Env["EVOLVE_REQUIRE_INTENT"] != "1" || s.Env["EVOLVE_STRICT_AUDIT"] != "1" {
		t.Fatalf("env map = %+v", s.Env)
	}
	trigger := s.Triggers["tester"]
	if len(trigger.InsertWhen) != 1 || trigger.InsertWhen[0].Field != "build.severity_max" {
		t.Fatalf("severity trigger = %+v", trigger)
	}
}
