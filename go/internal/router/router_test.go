package router

import (
	"reflect"
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
	return RouteInput{Current: cur, Cfg: testCfg(), Completed: []string{}, Now: fixedTime()}
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
	// Two unexpired code-audit-fail entries give BLOCK-CODE, which failureadapter returns only under Strict.
	in := base("retro")
	in.Strict = true
	in.History = []failureadapter.Entry{
		{Cycle: 1, Classification: failureadapter.CodeAuditFail, ExpiresAt: "2099-01-01T00:00:00Z"},
		{Cycle: 2, Classification: failureadapter.CodeAuditFail, ExpiresAt: "2099-01-01T00:00:00Z"},
	}
	d := Route(in, nil)

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
	in.Cfg.PhaseEnable["ship"] = config.EnableOff
	d := Route(in, nil)
	if d.NextPhase != "ship" {
		t.Errorf("ship=off → %q, want ship (mandatory clamp)", d.NextPhase)
	}
	if len(d.Clamps) == 0 || d.Clamps[0].Rule != "mandatory-never-skipped" {
		t.Errorf("expected mandatory-never-skipped clamp, got %+v", d.Clamps)
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
	in.Signals.Build = BuildSignals{ACSRed: 0, Present: true}
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
	llm := LLMProposal{Proposer: fakeProposer{p: &Proposal{NextPhase: "ship"}}}.Decide(in)

	if static.NextPhase != llm.NextPhase {
		t.Errorf("static next %q != llm next %q — clamp floor must be identical", static.NextPhase, llm.NextPhase)
	}
}

func advisoryCfg() config.RoutingConfig {
	c := testCfg()
	c.Stage = config.StageAdvisory
	return c
}

// spinePlan is the floor-clamped shape the orchestrator always threads, plus extra entries.
func spinePlan(extra ...PhasePlanEntry) *PhasePlan {
	entries := []PhasePlanEntry{
		pe("scout", true), pe("tdd", true), pe("build", true),
		pe("audit", true), pe("ship", true),
	}
	return &PhasePlan{Entries: append(entries, extra...)}
}

func TestRoute_AdvisoryTriggerCapEnforced(t *testing.T) {
	in := base("build")
	in.Cfg = advisoryCfg()
	in.Cfg.MaxInsertions = 1
	in.Completed = []string{"scout", "tdd", "build", "plan-review"} // 1 optional already spent
	in.Signals.Build = BuildSignals{ACSRed: 2, Present: true}       // tester trigger would fire too
	in.Plan = spinePlan(pe("tester", true))

	d := Route(in, nil)
	if d.NextPhase != "audit" {
		t.Errorf("cap hit in advisory → %q, want audit (tester insert dropped)", d.NextPhase)
	}
	if !hasClamp(d, "max-insertions-cap") {
		t.Errorf("expected max-insertions-cap clamp, got %+v", d.Clamps)
	}
	if contains(d.InsertPhases, "tester") {
		t.Errorf("tester inserted past the cap: InsertPhases=%v", d.InsertPhases)
	}
}

func TestRoute_AdvisoryTriggerWithinCap(t *testing.T) {
	in := base("build")
	in.Cfg = advisoryCfg() // MaxInsertions: 4
	in.Completed = []string{"scout", "tdd", "build", "plan-review"}
	in.Signals.Build = BuildSignals{ACSRed: 2, Present: true}
	in.Plan = spinePlan(pe("tester", true))

	d := Route(in, nil)
	if d.NextPhase != "tester" {
		t.Errorf("within cap → %q, want tester (plan run:true phase fires)", d.NextPhase)
	}
	if hasClamp(d, "max-insertions-cap") {
		t.Errorf("unexpected max-insertions-cap clamp within cap: %+v", d.Clamps)
	}
}

func TestRoute_AdvisoryPlanPhaseExemptsFromCap(t *testing.T) {
	in := base("build")
	in.Cfg = advisoryCfg()
	in.Cfg.MaxInsertions = 1
	in.Cfg.PhaseEnable["tester"] = config.EnableOn
	in.Completed = []string{"scout", "tdd", "build", "plan-review"} // cap spent
	in.Plan = spinePlan(pe("tester", true))

	d := Route(in, nil)
	if d.NextPhase != "tester" {
		t.Errorf("EnableOn plan-phase at cap → %q, want tester (plan-scheduled, cap-exempt)", d.NextPhase)
	}
}

func TestRoute_AdvisoryFloorPhaseNotCapped(t *testing.T) {
	in := base("build")
	in.Cfg = advisoryCfg()
	in.Cfg.Mandatory = []string{"scout", "build", "ship"} // audit NOT mandatory here
	in.Cfg.MaxInsertions = 1
	in.Completed = []string{"scout", "tdd", "build", "plan-review"} // cap spent
	in.Plan = spinePlan()                                           // audit Run:true via the spine plan (floor-forced shape)

	d := Route(in, nil)
	if d.NextPhase != "audit" {
		t.Errorf("floor phase at cap → %q, want audit (ship-floor phases are never cap-skipped)", d.NextPhase)
	}
}

func TestRoute_AdvisoryPlanRunFalse(t *testing.T) {
	in := base("build")
	in.Cfg = advisoryCfg()
	in.Completed = []string{"scout", "tdd", "build"}
	in.Signals.Build = BuildSignals{ACSRed: 2, Present: true} // tester trigger fires
	in.Plan = spinePlan(pe("tester", false))                  // advisor explicitly vetoed tester

	d := Route(in, nil)
	if d.NextPhase != "audit" {
		t.Errorf("plan run:false + firing trigger → %q, want audit (veto wins)", d.NextPhase)
	}
	if !contains(d.SkipPhases, "tester") {
		t.Errorf("tester not recorded as skipped: SkipPhases=%v", d.SkipPhases)
	}
	if contains(d.InsertPhases, "tester") {
		t.Errorf("vetoed tester inserted: InsertPhases=%v", d.InsertPhases)
	}
}

func TestRoute_AdvisoryPlanVetoUserPhaseAbsentSignal(t *testing.T) {
	in := base("build")
	in.Cfg = advisoryCfg()
	in.Cfg.Order = []string{"scout", "tdd", "build", "growth-loop", "audit", "ship"}
	in.Cfg.Triggers["growth-loop"] = config.RoutingBlock{
		InsertWhen: []config.Condition{{Field: "scout.goal_type", Op: "ne", Value: "growth"}},
	}
	in.Completed = []string{"scout", "tdd", "build"}
	// No generic signals: scout.goal_type is never emitted.
	in.Plan = spinePlan(pe("growth-loop", false))

	d := Route(in, nil)
	if d.NextPhase != "audit" {
		t.Errorf("vetoed catalog phase with absent-signal trigger → %q, want audit", d.NextPhase)
	}
	if contains(d.InsertPhases, "growth-loop") {
		t.Errorf("growth-loop inserted despite plan veto: InsertPhases=%v", d.InsertPhases)
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
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

func TestWalk_MandatoryPhaseWithRubricOnlyRoutingBlockUnchanged(t *testing.T) {
	run := func(in RouteInput) RouterDecision { return Route(in, nil) }

	plain := base("scout")
	plain.Completed = []string{"scout"}
	plain.Signals.Scout = ScoutSignals{CycleSizeEstimate: "medium", Present: true}

	hinted := base("scout")
	hinted.Completed = []string{"scout"}
	hinted.Signals.Scout = ScoutSignals{CycleSizeEstimate: "medium", Present: true}
	for _, p := range []string{"scout", "build", "audit", "tdd"} {
		blk := hinted.Cfg.Triggers[p]
		blk.RubricHint = []string{"some advisory hint for " + p}
		hinted.Cfg.Triggers[p] = blk
	}

	d1, d2 := run(plain), run(hinted)
	if d1.NextPhase != d2.NextPhase || d1.Reason != d2.Reason {
		t.Errorf("rubric-only blocks changed the walk: (%q,%q) → (%q,%q)",
			d1.NextPhase, d1.Reason, d2.NextPhase, d2.Reason)
	}
	if !reflect.DeepEqual(d1.Clamps, d2.Clamps) {
		t.Errorf("rubric-only blocks changed clamps: %+v → %+v", d1.Clamps, d2.Clamps)
	}
}
