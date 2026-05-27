package routingtest

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// Brick is a composable mutation of a ScenarioSpec (functional-options style).
type Brick func(*ScenarioSpec)

// Scenario assembles a named spec from bricks.
func Scenario(name string, bricks ...Brick) ScenarioSpec {
	s := ScenarioSpec{Name: name}
	for _, b := range bricks {
		b(&s)
	}
	return s
}

// --- surface + kernel position ---

// Pure marks the spec as a single-decision PureKernel scenario.
func Pure() Brick { return func(s *ScenarioSpec) { s.Surface = PureKernel } }

// Cycle marks the spec as an end-to-end FullOrchestrator scenario.
func Cycle() Brick { return func(s *ScenarioSpec) { s.Surface = FullOrchestrator } }

// At sets the just-completed phase (PureKernel RouteInput.Current).
func At(phase string) Brick { return func(s *ScenarioSpec) { s.Current = phase } }

// Done sets the already-completed phases (PureKernel RouteInput.Completed).
func Done(phases ...string) Brick { return func(s *ScenarioSpec) { s.Completed = phases } }

// CompletedVerdict sets the verdict of the just-completed phase (audit branch).
func CompletedVerdict(v string) Brick { return func(s *ScenarioSpec) { s.Verdict = v } }

// --- stage / mode / config ---

func Off() Brick      { return func(s *ScenarioSpec) { s.Stage = config.StageOff } }
func Shadow() Brick   { return func(s *ScenarioSpec) { s.Stage = config.StageShadow } }
func Advisory() Brick { return func(s *ScenarioSpec) { s.Stage = config.StageAdvisory } }
func Enforce() Brick  { return func(s *ScenarioSpec) { s.Stage = config.StageEnforce } }

func Budget(v float64) Brick      { return func(s *ScenarioSpec) { s.BudgetUSD = v } }
func MaxInserts(n int) Brick      { return func(s *ScenarioSpec) { s.MaxInsertions = n } }
func Mandatory(p ...string) Brick { return func(s *ScenarioSpec) { s.Mandatory = p } }

// SeverityTrigger swaps the tester insert trigger to build.severity_max>=HIGH
// (the default is build.acs_red>0).
func SeverityTrigger() Brick {
	return func(s *ScenarioSpec) {
		ensureTriggers(s)
		s.Triggers["tester"] = config.RoutingBlock{
			InsertWhen: []config.Condition{{Field: "build.severity_max", Op: "gte", Value: "HIGH"}},
		}
	}
}

// --- signal fixtures ---

func TrivialCycle() Brick { return func(s *ScenarioSpec) { s.Signals.CycleSize = "trivial" } }
func SmallCycle() Brick   { return func(s *ScenarioSpec) { s.Signals.CycleSize = "small" } }
func MediumCycle() Brick  { return func(s *ScenarioSpec) { s.Signals.CycleSize = "medium" } }
func LargeCycle() Brick   { return func(s *ScenarioSpec) { s.Signals.CycleSize = "large" } }

// TriageSize sets triage's authoritative cycle-size refinement.
func TriageSize(size string) Brick { return func(s *ScenarioSpec) { s.Signals.TriageSize = size } }

// RedBuild marks the build present with n failing predicates (acs_red).
func RedBuild(n int) Brick {
	return func(s *ScenarioSpec) { s.Signals.ACSRed = n; s.Signals.BuildVerdict = orPASS(s.Signals.BuildVerdict) }
}

// GreenBuild marks the build present with acs_red=0.
func GreenBuild() Brick {
	return func(s *ScenarioSpec) { s.Signals.ACSGreen = 3; s.Signals.BuildVerdict = orPASS(s.Signals.BuildVerdict) }
}

// Severity sets the build's max thrust severity (marks build present).
func Severity(sev string) Brick {
	return func(s *ScenarioSpec) {
		s.Signals.SeverityMax = sev
		s.Signals.BuildVerdict = orPASS(s.Signals.BuildVerdict)
	}
}

// AuditIs marks the audit present with the given verdict.
func AuditIs(v string) Brick { return func(s *ScenarioSpec) { s.Signals.AuditVerdict = v } }

// --- enables / env ---

func TriageOff() Brick {
	return func(s *ScenarioSpec) { ensureEnable(s); s.Enable["triage"] = config.EnableOff }
}
func PhaseEnabled(phase string, e config.Enable) Brick {
	return func(s *ScenarioSpec) { ensureEnable(s); s.Enable[phase] = e }
}
func IntentRequired() Brick {
	return func(s *ScenarioSpec) { ensureEnv(s); s.Env["EVOLVE_REQUIRE_INTENT"] = "1" }
}
func StrictAudit() Brick {
	return func(s *ScenarioSpec) { ensureEnv(s); s.Env["EVOLVE_STRICT_AUDIT"] = "1" }
}

// SeedFailure appends n non-expired failedApproaches records (retro arc input).
func SeedFailure(classification string, n int) Brick {
	return func(s *ScenarioSpec) {
		for i := 0; i < n; i++ {
			s.FailedAt = append(s.FailedAt, FailedRecordSpec{Classification: classification, Verdict: "FAIL"})
		}
	}
}

// PhaseVerdict sets the fakeRunner verdict for a phase (orchestrator surface).
func PhaseVerdict(phase core.Phase, verdict string) Brick {
	return func(s *ScenarioSpec) {
		if s.Verdicts == nil {
			s.Verdicts = map[string]string{}
		}
		s.Verdicts[string(phase)] = verdict
	}
}

// --- the simulated dynamic agent ---

// Agent scripts the proposer to return {next, inserts} when Current==atPhase.
func Agent(atPhase, next string, inserts ...string) Brick {
	return func(s *ScenarioSpec) {
		if s.Agent.Proposals == nil {
			s.Agent.Proposals = map[string]*router.Proposal{}
		}
		s.Agent.Proposals[atPhase] = &router.Proposal{NextPhase: next, InsertPhases: inserts}
	}
}

// AgentJustified scripts the proposer like Agent, but also attaches a
// justification string — exercises the advisor-rationale capture path.
func AgentJustified(atPhase, next, justification string, inserts ...string) Brick {
	return func(s *ScenarioSpec) {
		if s.Agent.Proposals == nil {
			s.Agent.Proposals = map[string]*router.Proposal{}
		}
		s.Agent.Proposals[atPhase] = &router.Proposal{NextPhase: next, InsertPhases: inserts, Justification: justification}
	}
}

// AgentError makes the proposer fail at atPhase (exercises degrade-to-static).
func AgentError(atPhase string) Brick {
	return func(s *ScenarioSpec) {
		if s.Agent.ErrorsOn == nil {
			s.Agent.ErrorsOn = map[string]bool{}
		}
		s.Agent.ErrorsOn[atPhase] = true
	}
}

// --- expectations ---

func ExpectNext(p string) Brick       { return func(s *ScenarioSpec) { s.Expect.NextPhase = p } }
func ExpectInserts(p ...string) Brick { return func(s *ScenarioSpec) { s.Expect.Inserts = p } }
func ExpectSkips(p ...string) Brick   { return func(s *ScenarioSpec) { s.Expect.Skips = p } }
func ExpectClamp(rules ...string) Brick {
	return func(s *ScenarioSpec) { s.Expect.Clamps = append(s.Expect.Clamps, rules...) }
}
func ExpectReason(r string) Brick { return func(s *ScenarioSpec) { s.Expect.Reason = r } }
func ExpectJustification(substr string) Brick {
	return func(s *ScenarioSpec) { s.Expect.Justification = substr }
}

func ExpectPhases(p ...core.Phase) Brick { return func(s *ScenarioSpec) { s.Expect.PhaseSequence = p } }
func ExpectAbsent(p ...core.Phase) Brick {
	return func(s *ScenarioSpec) { s.Expect.PhasesAbsent = append(s.Expect.PhasesAbsent, p...) }
}
func ExpectDecisionInsert(p ...string) Brick {
	return func(s *ScenarioSpec) { s.Expect.DecisionInserts = append(s.Expect.DecisionInserts, p...) }
}
func ExpectDecisionClamp(rule ...string) Brick {
	return func(s *ScenarioSpec) { s.Expect.DecisionClamps = append(s.Expect.DecisionClamps, rule...) }
}
func ExpectRoutingLedger(min int) Brick {
	return func(s *ScenarioSpec) { s.Expect.RoutingLedgerMin = min }
}
func ExpectRetro(prefix string) Brick { return func(s *ScenarioSpec) { s.Expect.RetroPrefix = prefix } }
func ExpectInvariants(names ...string) Brick {
	return func(s *ScenarioSpec) { s.Expect.Invariants = append(s.Expect.Invariants, names...) }
}

// --- helpers ---

func ensureEnable(s *ScenarioSpec) {
	if s.Enable == nil {
		s.Enable = map[string]config.Enable{}
	}
}
func ensureTriggers(s *ScenarioSpec) {
	if s.Triggers == nil {
		s.Triggers = map[string]config.RoutingBlock{}
	}
}
func ensureEnv(s *ScenarioSpec) {
	if s.Env == nil {
		s.Env = map[string]string{}
	}
}
func orPASS(v string) string {
	if v == "" {
		return "PASS"
	}
	return v
}
