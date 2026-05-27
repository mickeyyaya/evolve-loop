package router

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
)

func fixedTime() time.Time { return time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC) }

func testCfg() config.RoutingConfig {
	return config.RoutingConfig{
		Stage:         config.StageEnforce,
		Mode:          config.ModeStaticPreset,
		Mandatory:     []string{"scout", "build", "audit", "ship"},
		Conditional:   map[string]config.CondRule{"tdd": {Field: "cycle_size", Op: "!=", Value: "trivial"}},
		MaxInsertions: 4,
		PhaseEnable:   map[string]config.Enable{},
		Triggers: map[string]config.RoutingBlock{
			"tester": {InsertWhen: []config.Condition{{Field: "build.acs_red", Op: "gt", Value: 0}}},
		},
	}
}

func base(cur string) RouteInput {
	return RouteInput{Current: cur, Cfg: testCfg(), BudgetRemaining: 10, Completed: []string{}, Now: fixedTime()}
}

func TestRoute_StartToScout(t *testing.T) {
	in := base("start")
	d := Route(in, nil)
	if d.NextPhase != "scout" {
		t.Errorf("start → %q, want scout", d.NextPhase)
	}
}

func TestRoute_StartToIntentWhenForced(t *testing.T) {
	in := base("start")
	in.Cfg.PhaseEnable["intent"] = config.EnableOn
	d := Route(in, nil)
	if d.NextPhase != "intent" {
		t.Errorf("start → %q, want intent (forced on)", d.NextPhase)
	}
}

func TestRoute_ScoutToTDDWhenNonTrivial(t *testing.T) {
	in := base("scout")
	in.Completed = []string{"scout"}
	in.Signals.Scout = ScoutSignals{CycleSizeEstimate: "medium", Present: true}
	d := Route(in, nil)
	if d.NextPhase != "tdd" {
		t.Errorf("scout(medium) → %q, want tdd (conditional-pin)", d.NextPhase)
	}
	if d.Reason != "conditional-pin:tdd" {
		t.Errorf("reason = %q, want conditional-pin:tdd", d.Reason)
	}
}

func TestRoute_ScoutSkipsTDDWhenTrivial(t *testing.T) {
	in := base("scout")
	in.Completed = []string{"scout"}
	in.Signals.Scout = ScoutSignals{CycleSizeEstimate: "trivial", Present: true}
	d := Route(in, nil)
	if d.NextPhase != "build" {
		t.Errorf("scout(trivial) → %q, want build (tdd skipped)", d.NextPhase)
	}
}

func TestRoute_BuildInsertsTesterOnRed(t *testing.T) {
	in := base("build")
	in.Completed = []string{"scout", "tdd", "build"}
	in.Signals.Build = BuildSignals{ACSRed: 2, Present: true}
	d := Route(in, nil)
	if d.NextPhase != "tester" {
		t.Errorf("build(red>0) → %q, want tester", d.NextPhase)
	}
	if len(d.InsertPhases) != 1 || d.InsertPhases[0] != "tester" {
		t.Errorf("InsertPhases = %v, want [tester]", d.InsertPhases)
	}
}

// TestRoute_UserPhaseInsertedViaCatalogOrder proves the runtime integration: a
// user-defined phase spliced into cfg.Order (after build) is proposed when its
// insert_when fires against the uniform signal plane (sig.Generic). This is the
// end of the chain — author → catalog → cfg.Order+Triggers → routed.
func TestRoute_UserPhaseInsertedViaCatalogOrder(t *testing.T) {
	in := base("build")
	in.Completed = []string{"scout", "tdd", "build"}
	in.Cfg.Order = []string{"scout", "tdd", "build", "security-scan", "audit", "ship"}
	in.Cfg.Triggers["security-scan"] = config.RoutingBlock{
		InsertWhen: []config.Condition{{Field: "security.cves", Op: "gt", Value: 0}},
	}
	in.Signals.Generic = map[string]any{"security.cves": float64(2)}

	d := Route(in, nil)
	if d.NextPhase != "security-scan" {
		t.Errorf("build → %q, want security-scan (user phase inserted via order)", d.NextPhase)
	}
	if len(d.InsertPhases) != 1 || d.InsertPhases[0] != "security-scan" {
		t.Errorf("InsertPhases = %v, want [security-scan]", d.InsertPhases)
	}
}

// TestRoute_UserPhaseSkippedWhenTriggerQuiet confirms the user phase is skipped
// (→ audit) when its signal trigger does not fire — no spurious insertion.
func TestRoute_UserPhaseSkippedWhenTriggerQuiet(t *testing.T) {
	in := base("build")
	in.Completed = []string{"scout", "tdd", "build"}
	in.Cfg.Order = []string{"scout", "tdd", "build", "security-scan", "audit", "ship"}
	in.Cfg.Triggers["security-scan"] = config.RoutingBlock{
		InsertWhen: []config.Condition{{Field: "security.cves", Op: "gt", Value: 0}},
	}
	in.Signals.Generic = map[string]any{"security.cves": float64(0)}

	d := Route(in, nil)
	if d.NextPhase != "audit" {
		t.Errorf("build → %q, want audit (security-scan trigger quiet)", d.NextPhase)
	}
}

func TestRoute_BuildToAuditWhenNoRed(t *testing.T) {
	in := base("build")
	in.Completed = []string{"scout", "tdd", "build"}
	in.Signals.Build = BuildSignals{ACSRed: 0, Present: true}
	d := Route(in, nil)
	if d.NextPhase != "audit" {
		t.Errorf("build(red=0) → %q, want audit (tester skipped)", d.NextPhase)
	}
	if len(d.SkipPhases) != 1 || d.SkipPhases[0] != "tester" {
		t.Errorf("SkipPhases = %v, want [tester]", d.SkipPhases)
	}
}

func TestRoute_AuditPassToShip(t *testing.T) {
	in := base("audit")
	in.Verdict = "PASS"
	in.Completed = []string{"scout", "build", "audit"}
	d := Route(in, nil)
	if d.NextPhase != "ship" {
		t.Errorf("audit(PASS) → %q, want ship", d.NextPhase)
	}
}

func TestRoute_AuditFailToRetrospective(t *testing.T) {
	in := base("audit")
	in.Verdict = "FAIL"
	in.Completed = []string{"scout", "build", "audit"}
	d := Route(in, nil)
	if d.NextPhase != "retrospective" {
		t.Errorf("audit(FAIL) → %q, want retrospective", d.NextPhase)
	}
}

func TestRoute_Retro_DelegatesToFailureAdapter(t *testing.T) {
	// Two distinct code-audit-fail cycles (non-expired) → BLOCK-CODE → end.
	// BLOCK actions are strict-gated in failureadapter (fluent → AWARE/PROCEED).
	in := base("retro")
	in.Strict = true
	in.History = []failureadapter.Entry{
		{Cycle: 1, Classification: failureadapter.CodeAuditFail, ExpiresAt: "2099-01-01T00:00:00Z"},
		{Cycle: 2, Classification: failureadapter.CodeAuditFail, ExpiresAt: "2099-01-01T00:00:00Z"},
	}
	d := Route(in, nil)

	// Parity: Route's retro branch must equal failureadapter.Decide's mapping.
	dec := failureadapter.Decide(in.History, failureadapter.Options{Now: fixedTime(), Strict: true})
	if dec.Action != failureadapter.ActionBlockCode {
		t.Fatalf("precondition: expected BLOCK-CODE, got %s", dec.Action)
	}
	if d.NextPhase != PhaseEnd {
		t.Errorf("retro(block) → %q, want end", d.NextPhase)
	}
}

func TestRoute_Retro_RetryToTDD(t *testing.T) {
	in := base("retro")
	in.Strict = true // RETRY is strict-gated in failureadapter
	in.History = []failureadapter.Entry{
		{Cycle: 1, Classification: failureadapter.InfraTransient, ExpiresAt: "2099-01-01T00:00:00Z"},
	}
	d := Route(in, nil)
	if d.NextPhase != "tdd" {
		t.Errorf("retro(retry) → %q, want tdd", d.NextPhase)
	}
}

func TestRoute_Retro_EmptyHistoryProceedsToEnd(t *testing.T) {
	d := Route(base("retro"), nil)
	if d.NextPhase != PhaseEnd {
		t.Errorf("retro(empty) → %q, want end", d.NextPhase)
	}
}

func TestRoute_Clamp_MandatoryNeverSkipped(t *testing.T) {
	in := base("audit")
	in.Verdict = "PASS"
	in.Completed = []string{"scout", "build", "audit"}
	in.Cfg.PhaseEnable["ship"] = config.EnableOff // try to disable a mandatory phase
	d := Route(in, nil)
	if d.NextPhase != "ship" {
		t.Errorf("ship=off → %q, want ship (mandatory clamp)", d.NextPhase)
	}
	if len(d.Clamps) == 0 || d.Clamps[0].Rule != "mandatory-never-skipped" {
		t.Errorf("expected mandatory-never-skipped clamp, got %+v", d.Clamps)
	}
}

func TestRoute_Clamp_BudgetExhaustedDropsInsert(t *testing.T) {
	in := base("build")
	in.Completed = []string{"scout", "tdd", "build"}
	in.Signals.Build = BuildSignals{ACSRed: 5, Present: true}
	in.BudgetRemaining = 0 // exhausted
	d := Route(in, nil)
	if d.NextPhase != "audit" {
		t.Errorf("budget=0 → %q, want audit (tester insert dropped)", d.NextPhase)
	}
	if !hasClamp(d, "budget-exhausted") {
		t.Errorf("expected budget-exhausted clamp, got %+v", d.Clamps)
	}
}

func TestRoute_Clamp_MaxInsertionsCap(t *testing.T) {
	in := base("build")
	in.Completed = []string{"scout", "tdd", "build", "plan-review"} // 1 optional insert already
	in.Cfg.MaxInsertions = 1
	in.Signals.Build = BuildSignals{ACSRed: 5, Present: true}
	d := Route(in, nil)
	if d.NextPhase != "audit" {
		t.Errorf("cap hit → %q, want audit (tester dropped)", d.NextPhase)
	}
	if !hasClamp(d, "max-insertions-cap") {
		t.Errorf("expected max-insertions-cap clamp, got %+v", d.Clamps)
	}
}

func TestRoute_Clamp_LLMProposalClampedToKernelNext(t *testing.T) {
	in := base("build")
	in.Completed = []string{"scout", "tdd", "build"}
	in.Signals.Build = BuildSignals{ACSRed: 0, Present: true} // kernel → audit
	prop := &Proposal{NextPhase: "ship", Justification: "skip audit, looks fine"}
	d := Route(in, prop)
	if d.NextPhase != "audit" {
		t.Errorf("proposal=ship → %q, want audit (kernel wins)", d.NextPhase)
	}
	if !hasClamp(d, "llm-proposal-clamped") {
		t.Errorf("expected llm-proposal-clamped clamp, got %+v", d.Clamps)
	}
}

func TestStrategy_StaticVsLLM_SameClampFloor(t *testing.T) {
	in := base("build")
	in.Completed = []string{"scout", "tdd", "build"}
	in.Signals.Build = BuildSignals{ACSRed: 0, Present: true}

	static := StaticPreset{}.Decide(in)
	// LLM proposer that tries to jump to ship; must be clamped to the same next.
	llm := LLMProposal{Proposer: fakeProposer{p: &Proposal{NextPhase: "ship"}}}.Decide(in)

	if static.NextPhase != llm.NextPhase {
		t.Errorf("static next %q != llm next %q — clamp floor must be identical", static.NextPhase, llm.NextPhase)
	}
}

type fakeProposer struct{ p *Proposal }

func (f fakeProposer) Propose(in RouteInput) (*Proposal, error) { return f.p, nil }

func hasClamp(d RouterDecision, rule string) bool {
	for _, c := range d.Clamps {
		if c.Rule == rule {
			return true
		}
	}
	return false
}
