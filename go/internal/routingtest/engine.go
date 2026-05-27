package routingtest

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// fixedNow is the deterministic clock for pure-kernel decisions.
var fixedNow = time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)

// RunAll executes a set of specs, each as its own subtest. This is the entry
// point a catalog calls: "send a set of test configurations."
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

// Run executes one spec against its Surface and asserts the embedded
// expectations + invariants.
func Run(t *testing.T, s ScenarioSpec) {
	t.Helper()
	switch s.Surface {
	case FullOrchestrator:
		runCycle(t, s)
	default:
		runPure(t, s)
	}
}

// buildConfig resolves a ScenarioSpec's knobs into a RoutingConfig, filling the
// same safe defaults config.Load would (so a zero spec behaves like production).
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
		cfg.Conditional = map[string]config.CondRule{"tdd": {Field: "cycle_size", Op: "!=", Value: "trivial"}}
	}
	if cfg.PhaseEnable == nil {
		cfg.PhaseEnable = map[string]config.Enable{}
	}
	if cfg.Triggers == nil {
		cfg.Triggers = map[string]config.RoutingBlock{
			"tester": {InsertWhen: []config.Condition{{Field: "build.acs_red", Op: "gt", Value: 0}}},
		}
	}
	if cfg.MaxInsertions == 0 {
		cfg.MaxInsertions = 4
	}
	return cfg
}

func budgetOf(s ScenarioSpec) float64 {
	if s.BudgetUSD == 0 {
		return 100
	}
	return s.BudgetUSD
}

// --- PureKernel ---

func runPure(t *testing.T, s ScenarioSpec) {
	in := router.RouteInput{
		Current:         s.Current,
		Verdict:         s.Verdict,
		Signals:         s.Signals.Signals(),
		Cfg:             buildConfig(s),
		BudgetRemaining: budgetOf(s),
		Completed:       s.Completed,
		Strict:          s.Env["EVOLVE_STRICT_AUDIT"] == "1",
		Now:             fixedNow,
	}
	proposal := s.Agent.Proposals[s.Current] // nil if none scripted at Current
	got := router.Route(in, proposal)

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
	assertInvariants(t, s, in, proposal, got)
}

// --- FullOrchestrator ---

func runCycle(t *testing.T, s ScenarioSpec) {
	cfg := buildConfig(s)
	var strat router.RoutingStrategy = router.StaticPreset{}
	if s.Agent.active() {
		cfg.Mode = config.ModeDynamicLLM
		strat = router.LLMProposal{Proposer: &scriptedProposer{spec: s.Agent}}
	}

	projectRoot := t.TempDir()
	cycle := s.LastCycle + 1
	ws := seedWorkspace(t, projectRoot, cycle, s.Signals.HandoffFiles())

	st := &FakeStorage{state: core.State{LastCycleNumber: s.LastCycle, FailedAt: failedRecords(s.FailedAt)}}
	led := &FakeLedger{}
	runners := buildRunners(s.Verdicts)
	o := core.NewOrchestrator(st, led, runners, core.WithRouting(cfg, strat))

	env := map[string]string{"EVOLVE_DISABLE_WORKSPACE_GUARD": "1"}
	for k, v := range s.Env {
		env[k] = v
	}

	res, err := o.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: projectRoot,
		GoalHash:    "g",
		Budget:      core.BudgetEnvelope{MaxUSD: budgetOf(s)},
		Env:         env,
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	decisions := readRoutingDecisions(t, ws)

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
		if n := countLedgerKind(led.entries, "routing_decision"); n < s.Expect.RoutingLedgerMin {
			t.Errorf("routing_decision ledger entries=%d, want >=%d", n, s.Expect.RoutingLedgerMin)
		}
	}
	if s.Expect.RetroPrefix != "" && !strings.HasPrefix(res.RetroDecision, s.Expect.RetroPrefix) {
		t.Errorf("RetroDecision=%q, want prefix %q", res.RetroDecision, s.Expect.RetroPrefix)
	}
}

// --- assertion helpers ---

func assertPhaseSeq(t *testing.T, got, want []core.Phase) {
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

// sameSet reports order-insensitive equality of two string slices.
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

// subset reports whether every element of want is in have.
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
