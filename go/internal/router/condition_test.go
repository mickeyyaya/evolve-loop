package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func sigFixture() RoutingSignals {
	return RoutingSignals{
		Scout:  ScoutSignals{CycleSizeEstimate: "large", ItemCount: 5, CarryoverCount: 2, BacklogSize: 9, Present: true},
		Triage: TriageSignals{CycleSize: "medium", Present: true},
		Build:  BuildSignals{Verdict: "PASS", ACSRed: 3, ACSGreen: 30, ACSRegression: 4, FilesTouched: 7, DiffLOC: 540, SeverityMax: SevHigh, Present: true},
		Audit:  AuditSignals{Verdict: "WARN", Confidence: 0.62, RedCount: 1, Present: true},
	}
}

func TestEvalCondition_NumericOps(t *testing.T) {
	sig := sigFixture()
	cases := []struct {
		field, op string
		val       interface{}
		want      bool
	}{
		{"build.acs_red", "gt", 0, true},
		{"build.acs_red", "gt", 5, false},
		{"build.acs_red", "gte", 3, true},
		{"build.acs_red", "lt", 5, true},
		{"build.acs_red", "lte", 3, true},
		{"build.acs_red", "eq", 3, true},
		{"build.acs_red", "ne", 9, true},
		{"audit.confidence", "lt", 0.7, true},
		{"audit.red_count", "gt", 0, true},
		{"scout.item_count", "gte", 5, true},
		{"scout.carryover_count", "gt", 1, true},
		{"scout.backlog_size", "gte", 9, true},
		{"scout.backlog_size", "gt", 9, false},
		{"build.acs_regression", "gt", 3, true},
		{"build.files_touched", "gte", 7, true},
		{"build.diff_loc", "gte", 500, true},
		{"build.diff_loc", "lt", 500, false},
		{"build.severity_max", "gte", "HIGH", true}, // severity coercion
		{"build.severity_max", "gte", "CRITICAL", false},
	}
	for _, c := range cases {
		got := evalCondition(sig, config.Condition{Field: c.field, Op: c.op, Value: c.val})
		if got != c.want {
			t.Errorf("evalCondition(%s %s %v) = %v, want %v", c.field, c.op, c.val, got, c.want)
		}
	}
}

func TestEvalCondition_StringOps(t *testing.T) {
	sig := sigFixture()
	cases := []struct {
		field, op string
		val       interface{}
		want      bool
	}{
		{"cycle_size", "eq", "medium", true}, // triage precedence
		{"cycle_size", "ne", "trivial", true},
		{"scout.cycle_size", "eq", "large", true},
		{"build.verdict", "eq", "PASS", true},
		{"audit.verdict", "ne", "PASS", true},
		{"unknown.field", "eq", "x", false}, // unknown → false (fail-safe)
		{"build.acs_red", "bogusop", 1, false},
	}
	for _, c := range cases {
		got := evalCondition(sig, config.Condition{Field: c.field, Op: c.op, Value: c.val})
		if got != c.want {
			t.Errorf("evalCondition(%s %s %v) = %v, want %v", c.field, c.op, c.val, got, c.want)
		}
	}
}

func TestEvalCondition_GenericSignals(t *testing.T) {
	sig := RoutingSignals{Generic: map[string]any{
		"security.cves":         float64(2), // JSON number
		"security.severity_max": "HIGH",
		"deploy.ready":          true,
	}}
	cases := []struct {
		field, op string
		val       interface{}
		want      bool
	}{
		{"security.cves", "gt", 0, true},
		{"security.cves", "eq", 2, true},
		{"security.severity_max", "eq", "HIGH", true},
		{"security.severity_max", "ne", "LOW", true},
		{"deploy.ready", "eq", "true", true},
		{"deploy.ready", "ne", "false", true},  // bool renders "true"/"false"
		{"security.missing", "eq", "x", false}, // absent generic → fail-safe false
	}
	for _, c := range cases {
		got := evalCondition(sig, config.Condition{Field: c.field, Op: c.op, Value: c.val})
		if got != c.want {
			t.Errorf("evalCondition(%s %s %v) = %v, want %v", c.field, c.op, c.val, got, c.want)
		}
	}
}

func TestEvalCondition_AbsentFieldIsAlwaysFalse(t *testing.T) {
	// The bus exists but lacks the queried fields; unknown.field covers the default branch.
	sig := RoutingSignals{Generic: map[string]any{"scout.other": "present"}}
	cases := []struct {
		field, op string
		val       interface{}
	}{
		{"scout.goal_type", "ne", "growth"},
		{"scout.goal_type", "!=", "growth"},
		{"scout.goal_type", "eq", ""}, // "" == "" fail-open variant
		{"scout.goal_type", "ne", ""},
		{"scout.goal_type", "eq", "growth"},
		{"scout.goal_type", "gt", 0},
		{"scout.goal_type", "gte", 0},
		{"scout.goal_type", "lt", 5},
		{"scout.goal_type", "lte", 5},
		{"unknown.field", "ne", "x"},
		{"unknown.field", "eq", ""},
	}
	for _, c := range cases {
		if evalCondition(sig, config.Condition{Field: c.field, Op: c.op, Value: c.val}) {
			t.Errorf("evalCondition(absent %s %s %v) = true, want false (absent field must fail closed for every operator)", c.field, c.op, c.val)
		}
	}
}

func TestEvalCondition_PresentEmptyString(t *testing.T) {
	sig := RoutingSignals{Generic: map[string]any{"scout.goal_type": ""}}
	cases := []struct {
		field, op string
		val       interface{}
		want      bool
	}{
		{"scout.goal_type", "eq", "", true},
		{"scout.goal_type", "ne", "growth", true},
		{"scout.goal_type", "ne", "", false},
		{"scout.goal_type", "eq", "growth", false},
	}
	for _, c := range cases {
		got := evalCondition(sig, config.Condition{Field: c.field, Op: c.op, Value: c.val})
		if got != c.want {
			t.Errorf("evalCondition(present-empty %s %s %v) = %v, want %v", c.field, c.op, c.val, got, c.want)
		}
	}
}

func TestEvalCondition_TypedFieldAbsentKeepsLegacySemantics(t *testing.T) {
	var sig RoutingSignals // no handoffs digested at all
	if !evalCondition(sig, config.Condition{Field: "cycle_size", Op: "ne", Value: "trivial"}) {
		t.Errorf("cycle_size ne trivial with zero signals = false; the tdd conditional-mandatory pin must stay true (conservative side) pre-handoff")
	}
}

func TestTriggerFires_AbsentFieldFailsClosed(t *testing.T) {
	absentNe := config.Condition{Field: "scout.goal_type", Op: "ne", Value: "growth"}

	if triggerFires(RoutingSignals{}, config.RoutingBlock{InsertWhen: []config.Condition{absentNe}}) {
		t.Errorf("insert_when(absent ne) fired; want quiet (fail-closed)")
	}

	sig := RoutingSignals{Build: BuildSignals{ACSRed: 2, Present: true}}
	block := config.RoutingBlock{
		InsertWhen: []config.Condition{{Field: "build.acs_red", Op: "gt", Value: 0}},
		SkipWhen:   []config.Condition{absentNe},
	}
	if !triggerFires(sig, block) {
		t.Errorf("skip_when(absent ne) suppressed a firing insert; absent skip conditions must be false")
	}
}

func TestCoerceNum_StringNumber(t *testing.T) {
	sig := sigFixture()
	if !evalCondition(sig, config.Condition{Field: "build.acs_red", Op: "eq", Value: "3"}) {
		t.Errorf("string '3' should coerce to numeric 3")
	}
	if evalCondition(sig, config.Condition{Field: "build.acs_red", Op: "gt", Value: "abc"}) {
		t.Errorf("non-numeric string should not satisfy numeric gt")
	}
}

func TestSeverityString(t *testing.T) {
	cases := map[Severity]string{SevCritical: "CRITICAL", SevHigh: "HIGH", SevMedium: "MEDIUM", SevLow: "LOW", SevNone: "NONE"}
	for sev, want := range cases {
		if sev.String() != want {
			t.Errorf("Severity(%d).String() = %q, want %q", sev, sev.String(), want)
		}
	}
	if ParseSeverity("med") != SevMedium {
		t.Errorf("ParseSeverity('med') should be SevMedium")
	}
}

func TestSelect_Strategy(t *testing.T) {
	cfgLLM := config.RoutingConfig{Mode: config.ModeDynamicLLM}
	cfgStatic := config.RoutingConfig{Mode: config.ModeStaticPreset}

	if _, ok := Select(cfgLLM, fakeProposer{}).(LLMProposal); !ok {
		t.Errorf("DynamicLLM + proposer should select LLMProposal")
	}
	if _, ok := Select(cfgLLM, nil).(StaticPreset); !ok {
		t.Errorf("DynamicLLM + nil proposer should fall back to StaticPreset")
	}
	if _, ok := Select(cfgStatic, fakeProposer{}).(StaticPreset); !ok {
		t.Errorf("StaticPreset mode should select StaticPreset")
	}
}

func TestLLMProposal_FailedProposerDegradesToStatic(t *testing.T) {
	in := base("build")
	in.Completed = []string{"scout", "tdd", "build"}
	in.Signals.Build = BuildSignals{ACSRed: 0, Present: true}
	got := LLMProposal{Proposer: errProposer{}}.Decide(in)
	want := StaticPreset{}.Decide(in)
	if got.NextPhase != want.NextPhase {
		t.Errorf("failed proposer should degrade to static next %q, got %q", want.NextPhase, got.NextPhase)
	}
}

type errProposer struct{}

func (errProposer) Propose(in RouteInput) (*Proposal, error) {
	return nil, errProposerErr
}

var errProposerErr = &proposerError{}

type proposerError struct{}

func (*proposerError) Error() string { return "boom" }
