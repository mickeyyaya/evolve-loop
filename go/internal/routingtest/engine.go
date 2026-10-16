package routingtest

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

var fixedNow = time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)

func RunAll(t *testing.T, specs []ScenarioSpec) {
	t.Helper()
	if len(specs) == 0 {
		t.Fatal("routingtest: empty spec set")
	}
	for _, s := range specs {
		s := s
		t.Run(s.Name, func(t *testing.T) { Run(t, s) })
	}
}

func Run(t *testing.T, s ScenarioSpec) {
	t.Helper()
	switch s.Surface {
	case FullOrchestrator:
		runCycle(t, s)
	default:
		runPure(t, s)
	}
}

func buildConfig(s ScenarioSpec) config.RoutingConfig {
	cfg := config.RoutingConfig{
		Stage:         s.Stage,
		Mode:          s.Mode,
		Mandatory:     s.Mandatory,
		Conditional:   s.Conditional,
		PhaseEnable:   s.Enable,
		Triggers:      s.Triggers,
		MaxInsertions: s.MaxInsertions,
	}
	if cfg.Mandatory == nil {
		cfg.Mandatory = []string{"scout", "build", "audit", "ship"}
	}
	if cfg.Conditional == nil {
		cfg.Conditional = map[string]config.CondRule{"tdd": config.DefaultTddRule()}
	}
	if cfg.PhaseEnable == nil {
		cfg.PhaseEnable = map[string]config.Enable{}
	}
	if cfg.Triggers == nil {
		cfg.Triggers = map[string]config.RoutingBlock{
			"tester": {InsertWhen: []config.Condition{testerOnARedBuild}},
		}
	}
	if cfg.MaxInsertions == 0 {
		cfg.MaxInsertions = 4
	}
	return cfg
}

func runPure(t *testing.T, s ScenarioSpec) {
	in := router.RouteInput{
		Current:   s.Current,
		Verdict:   s.Verdict,
		Signals:   s.Signals.Signals(),
		Cfg:       buildConfig(s),
		Completed: s.Completed,
		Strict:    s.Strict,
		Now:       fixedNow,
	}
	if len(s.Agent.Plan) > 0 {
		raw := &router.PhasePlan{Entries: append([]router.PhasePlanEntry(nil), s.Agent.Plan...)}
		plan, _ := router.ClampPlanToFloor(in, raw)
		in.Plan = plan
	}
	proposal := s.Agent.Proposals[s.Current]
	got := router.Route(in, proposal)

	checkPureExpectations(t, s, got)
	assertInvariants(t, s, in, proposal, got)
}

type errorReporter interface {
	Errorf(format string, args ...any)
}

func checkPureExpectations(t errorReporter, s ScenarioSpec, got router.RouterDecision) {
	if s.Expect.NextPhase != "" && got.NextPhase != s.Expect.NextPhase {
		t.Errorf("NextPhase=%q, want %q (decision=%+v)", got.NextPhase, s.Expect.NextPhase, got)
	}
	if s.Expect.Inserts != nil && !sameSet(got.InsertPhases, s.Expect.Inserts) {
		t.Errorf("InsertPhases=%v, want %v", got.InsertPhases, s.Expect.Inserts)
	}
	if s.Expect.Skips != nil && !subset(s.Expect.Skips, got.SkipPhases) {
		t.Errorf("SkipPhases=%v must contain %v", got.SkipPhases, s.Expect.Skips)
	}
	for _, rule := range s.Expect.Clamps {
		if !hasClampRule(got, rule) {
			t.Errorf("missing clamp %q; clamps=%+v", rule, got.Clamps)
		}
	}
	if s.Expect.Reason != "" && got.Reason != s.Expect.Reason {
		t.Errorf("Reason=%q, want %q", got.Reason, s.Expect.Reason)
	}
	if s.Expect.Justification != "" && !strings.Contains(got.Justification, s.Expect.Justification) {
		t.Errorf("Justification=%q, want substring %q", got.Justification, s.Expect.Justification)
	}
}

var routingtestRunCycle = func(o *core.Orchestrator, req core.CycleRequest) (core.CycleResult, error) {
	return o.RunCycle(context.Background(), req)
}

func runCycle(t *testing.T, s ScenarioSpec) {
	cfg := buildConfig(s)
	var strat router.RoutingStrategy = router.StaticPreset{}
	var agent *scriptedProposer
	if s.Agent.active() {
		cfg.Mode = config.ModeDynamicLLM
		agent = &scriptedProposer{spec: s.Agent}
		strat = router.LLMProposal{Proposer: agent}
	}
	opts := []core.Option{core.WithRouting(cfg, strat)}
	if agent != nil && s.Agent.hasPlan() {
		opts = append(opts, core.WithPlanner(agent))
	}
	if len(s.Enable) > 0 {
		opts = append(opts, core.WithWorkflowConfig(policy.WorkflowConfig{PhaseEnables: phaseEnablesOf(s.Enable)}))
	}

	projectRoot := initScenarioRepo(t)
	cycle := s.LastCycle + 1
	ws := seedWorkspace(t, projectRoot, cycle, s.Signals.HandoffFiles())

	st := &fixtures.FakeStorage{State: core.State{LastCycleNumber: s.LastCycle, FailedAt: failedRecords(s.FailedAt)}}
	led := &fixtures.FakeLedger{}
	runners := fixtures.BuildRunners(verdictsByPhase(s.Verdicts))
	o := core.NewOrchestrator(st, led, runners, opts...)

	envCopy := make(map[string]string)
	for k, v := range s.Env {
		envCopy[k] = v
	}

	res, err := routingtestRunCycle(o, core.CycleRequest{
		ProjectRoot:           projectRoot,
		GoalHash:              "g",
		Env:                   envCopy,
		DisableWorkspaceGuard: true,
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	decisions := readRoutingDecisions(t, ws)

	assertCycleExpectations(t, s, res, decisions, led, agent)
}

func assertCycleExpectations(t testing.TB, s ScenarioSpec, res core.CycleResult, decisions []router.RouterDecision, led *fixtures.FakeLedger, agent *scriptedProposer) {
	t.Helper()
	if s.Expect.PhaseSequence != nil {
		assertPhaseSeq(t, res.PhasesRun, s.Expect.PhaseSequence)
	}
	for _, p := range s.Expect.PhasesAbsent {
		if containsPhase(res.PhasesRun, p) {
			t.Errorf("phase %s ran but should be absent; phases=%v", p, res.PhasesRun)
		}
	}
	for _, p := range s.Expect.DecisionInserts {
		if !anyDecisionInsert(decisions, p) {
			t.Errorf("no routing decision proposed inserting %q; decisions=%+v", p, decisions)
		}
	}
	for _, rule := range s.Expect.DecisionClamps {
		if !anyDecisionClamp(decisions, rule) {
			t.Errorf("no routing decision carried clamp %q; decisions=%+v", rule, decisions)
		}
	}
	if s.Expect.RoutingLedgerMin > 0 {
		if n := countLedgerKind(led.Entries, "routing_decision"); n < s.Expect.RoutingLedgerMin {
			t.Errorf("routing_decision ledger entries=%d, want >=%d", n, s.Expect.RoutingLedgerMin)
		}
	}
	if s.Expect.RetroPrefix != "" && !strings.Contains(res.RetroDecision, s.Expect.RetroPrefix) {
		t.Errorf("RetroDecision=%q, want marker %q", res.RetroDecision, s.Expect.RetroPrefix)
	}
	if s.Expect.ProposeAt != nil {
		if agent == nil {
			t.Fatalf("ProposeAt expects a scripted agent but none was configured")
		}
		if !sameSet(agent.seen, s.Expect.ProposeAt) {
			t.Errorf("Proposer invoked at %v, want exactly %v (hybrid cadence)", agent.seen, s.Expect.ProposeAt)
		}
	}
}

func phaseEnablesOf(enable map[string]config.Enable) map[string]string {
	phaseEnables := make(map[string]string, len(enable))
	for phase, e := range enable {
		switch e {
		case config.EnableOn:
			phaseEnables[phase] = "on"
		case config.EnableOff:
			phaseEnables[phase] = "off"
		}
	}
	return phaseEnables
}

func initScenarioRepo(t *testing.T) string {
	t.Helper()
	return gittest.FromTemplate(t, "routingtest-scenario-base", func(r *gittest.Repo) error {
		r.Write(".gitignore", ".evolve/\ngo/bin/\n")
		r.Git("add", ".gitignore")
		r.Git("commit", "-q", "-m", "scenario base")
		return nil
	}).Dir
}

func assertPhaseSeq(t testing.TB, got, want []core.Phase) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("PhasesRun=%v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("phase[%d]=%s, want %s; full=%v", i, got[i], want[i], got)
		}
	}
}

func hasClampRule(d router.RouterDecision, rule string) bool {
	for _, c := range d.Clamps {
		if c.Rule == rule {
			return true
		}
	}
	return false
}

func anyDecisionInsert(ds []router.RouterDecision, phase string) bool {
	for _, d := range ds {
		if subset([]string{phase}, d.InsertPhases) {
			return true
		}
	}
	return false
}

func anyDecisionClamp(ds []router.RouterDecision, rule string) bool {
	for _, d := range ds {
		if hasClampRule(d, rule) {
			return true
		}
	}
	return false
}

func countLedgerKind(entries []core.LedgerEntry, kind string) int {
	n := 0
	for _, e := range entries {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

func containsPhase(xs []core.Phase, want core.Phase) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string{}, a...)
	bc := append([]string{}, b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}

func subset(want, have []string) bool {
	set := map[string]bool{}
	for _, h := range have {
		set[h] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}
