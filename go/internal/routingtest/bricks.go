package routingtest

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type Brick func(*ScenarioSpec)

func Scenario(name string, bricks ...Brick) ScenarioSpec {
	s := ScenarioSpec{Name: name}
	for _, b := range bricks {
		b(&s)
	}
	return s
}

func Pure() Brick { return func(s *ScenarioSpec) { s.Surface = PureKernel } }

func Cycle() Brick { return func(s *ScenarioSpec) { s.Surface = FullOrchestrator } }

func At(phase string) Brick { return func(s *ScenarioSpec) { s.Current = phase } }

func Done(phases ...string) Brick {
	return func(s *ScenarioSpec) { s.Completed = append([]string(nil), phases...) }
}

func CompletedVerdict(v string) Brick { return func(s *ScenarioSpec) { s.Verdict = v } }

func Off() Brick      { return func(s *ScenarioSpec) { s.Stage = config.StageOff } }
func Shadow() Brick   { return func(s *ScenarioSpec) { s.Stage = config.StageShadow } }
func Advisory() Brick { return func(s *ScenarioSpec) { s.Stage = config.StageAdvisory } }
func Enforce() Brick  { return func(s *ScenarioSpec) { s.Stage = config.StageEnforce } }

func MaxInserts(n int) Brick { return func(s *ScenarioSpec) { s.MaxInsertions = n } }

func Mandatory(p ...string) Brick {
	return func(s *ScenarioSpec) { s.Mandatory = append([]string(nil), p...) }
}

func SeverityTrigger() Brick {
	return func(s *ScenarioSpec) {
		ensureTriggers(s)
		s.Triggers["tester"] = config.RoutingBlock{
			InsertWhen: []config.Condition{{Field: "build.severity_max", Op: "gte", Value: "HIGH"}},
		}
	}
}

var testerOnARedBuild = config.Condition{Field: "build.acs_red", Op: "gt", Value: 0}

func AdvisorJudgedTester() Brick {
	return func(s *ScenarioSpec) {
		ensureTriggers(s)
		s.Triggers["tester"] = config.RoutingBlock{
			InsertWhen: []config.Condition{testerOnARedBuild},
			RubricHint: []string{"a risky green build also warrants tester"},
		}
	}
}

func TrivialCycle() Brick { return func(s *ScenarioSpec) { s.Signals.CycleSize = "trivial" } }
func SmallCycle() Brick   { return func(s *ScenarioSpec) { s.Signals.CycleSize = "small" } }
func MediumCycle() Brick  { return func(s *ScenarioSpec) { s.Signals.CycleSize = "medium" } }
func LargeCycle() Brick   { return func(s *ScenarioSpec) { s.Signals.CycleSize = "large" } }

func TriageSize(size string) Brick { return func(s *ScenarioSpec) { s.Signals.TriageSize = size } }

func RedBuild(n int) Brick {
	return func(s *ScenarioSpec) { s.Signals.ACSRed = n; s.Signals.BuildVerdict = orPASS(s.Signals.BuildVerdict) }
}

func GreenBuild() Brick {
	return func(s *ScenarioSpec) { s.Signals.ACSGreen = 3; s.Signals.BuildVerdict = orPASS(s.Signals.BuildVerdict) }
}

func Severity(sev string) Brick {
	return func(s *ScenarioSpec) {
		s.Signals.SeverityMax = sev
		s.Signals.BuildVerdict = orPASS(s.Signals.BuildVerdict)
	}
}

func Backlog(n int) Brick { return func(s *ScenarioSpec) { s.Signals.ScoutBacklog = n } }

func DiffLOC(n int) Brick {
	return func(s *ScenarioSpec) {
		s.Signals.DiffLOC = n
		s.Signals.BuildVerdict = orPASS(s.Signals.BuildVerdict)
	}
}

func AuditIs(v string) Brick { return func(s *ScenarioSpec) { s.Signals.AuditVerdict = v } }

func TriageOff() Brick {
	return func(s *ScenarioSpec) { ensureEnable(s); s.Enable["triage"] = config.EnableOff }
}
func PhaseEnabled(phase string, e config.Enable) Brick {
	return func(s *ScenarioSpec) { ensureEnable(s); s.Enable[phase] = e }
}
func IntentRequired() Brick {
	return func(s *ScenarioSpec) { ensureEnable(s); s.Enable["intent"] = config.EnableOn }
}
func StrictAudit() Brick {
	return func(s *ScenarioSpec) { s.Strict = true }
}

func SeedFailure(classification string, n int) Brick {
	return func(s *ScenarioSpec) {
		for i := 0; i < n; i++ {
			s.FailedAt = append(s.FailedAt, FailedRecordSpec{Classification: classification, Verdict: "FAIL"})
		}
	}
}

func PhaseVerdict(phase core.Phase, verdict string) Brick {
	return func(s *ScenarioSpec) {
		if s.Verdicts == nil {
			s.Verdicts = map[string]string{}
		}
		s.Verdicts[string(phase)] = verdict
	}
}

func Agent(atPhase, next string, inserts ...string) Brick {
	return func(s *ScenarioSpec) {
		if s.Agent.Proposals == nil {
			s.Agent.Proposals = map[string]*router.Proposal{}
		}
		s.Agent.Proposals[atPhase] = &router.Proposal{NextPhase: next, InsertPhases: inserts}
	}
}

func AgentJustified(atPhase, next, justification string, inserts ...string) Brick {
	return func(s *ScenarioSpec) {
		if s.Agent.Proposals == nil {
			s.Agent.Proposals = map[string]*router.Proposal{}
		}
		s.Agent.Proposals[atPhase] = &router.Proposal{NextPhase: next, InsertPhases: inserts, Justification: justification}
	}
}

func AgentError(atPhase string) Brick {
	return func(s *ScenarioSpec) {
		if s.Agent.ErrorsOn == nil {
			s.Agent.ErrorsOn = map[string]bool{}
		}
		s.Agent.ErrorsOn[atPhase] = true
	}
}

func PlanRun(phase string) router.PhasePlanEntry {
	return router.PhasePlanEntry{Phase: phase, Run: true, Justification: "scripted: run " + phase}
}

func PlanSkip(phase string) router.PhasePlanEntry {
	return router.PhasePlanEntry{Phase: phase, Run: false, Justification: "scripted: skip " + phase}
}

func AgentPlan(entries ...router.PhasePlanEntry) Brick {
	return func(s *ScenarioSpec) { s.Agent.Plan = entries }
}

func AgentPlanError() Brick {
	return func(s *ScenarioSpec) { s.Agent.PlanError = true }
}

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

func ExpectProposeAt(phases ...string) Brick {
	return func(s *ScenarioSpec) { s.Expect.ProposeAt = phases }
}
func ExpectInvariants(names ...string) Brick {
	return func(s *ScenarioSpec) { s.Expect.Invariants = append(s.Expect.Invariants, names...) }
}

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
func orPASS(v string) string {
	if v == "" {
		return "PASS"
	}
	return v
}
