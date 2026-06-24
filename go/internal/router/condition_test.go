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

// TestEvalCondition_GenericSignals verifies routing conditions resolve against
// the uniform signal plane (sig.Generic) — the path that makes a user-defined
// phase's emitted signal routable. JSON numbers arrive as float64.
func TestEvalCondition_GenericSignals(t *testing.T) {
	sig := RoutingSignals{Generic: map[string]any{
		"security.cves":         float64(2), // JSON number
		"security.severity_max": "HIGH",     // string
		"deploy.ready":          true,       // bool
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

// TestEvalCondition_AbsentFieldIsAlwaysFalse encodes the cycle-238 defect D2
// (missing-signal fail-open): an insert_when condition on a generic field that
// was NEVER EMITTED must evaluate false for EVERY operator. At the defective
// baseline, `ne` on an absent field returned true ("" != value) and `eq ""`
// returned true ("" == ""), so catalog phases with `goal_type != <other-goal>`
// triggers fired when scout.goal_type was simply not emitted.
func TestEvalCondition_AbsentFieldIsAlwaysFalse(t *testing.T) {
	// Generic bus exists but the queried fields are absent from it; also covers
	// the nil-bus case via "unknown.field" (resolveField default branch).
	sig := RoutingSignals{Generic: map[string]any{"scout.other": "present"}}
	cases := []struct {
		field, op string
		val       interface{}
	}{
		{"scout.goal_type", "ne", "growth"}, // the cycle-238 fail-open shape
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

// TestEvalCondition_PresentEmptyString is the over-fix guard for D2: a generic
// field that IS emitted with an empty-string value is PRESENT and must keep
// normal string-comparison semantics — fail-closed applies to absence, not to
// empty values.
func TestEvalCondition_PresentEmptyString(t *testing.T) {
	sig := RoutingSignals{Generic: map[string]any{"scout.goal_type": ""}}
	cases := []struct {
		field, op string
		val       interface{}
		want      bool
	}{
		{"scout.goal_type", "eq", "", true},        // present "" == "" matches
		{"scout.goal_type", "ne", "growth", true},  // present "" != "growth" fires
		{"scout.goal_type", "ne", "", false},       // present "" != "" does not
		{"scout.goal_type", "eq", "growth", false}, // present "" == "growth" does not
	}
	for _, c := range cases {
		got := evalCondition(sig, config.Condition{Field: c.field, Op: c.op, Value: c.val})
		if got != c.want {
			t.Errorf("evalCondition(present-empty %s %s %v) = %v, want %v", c.field, c.op, c.val, got, c.want)
		}
	}
}

// TestEvalCondition_TypedFieldAbsentKeepsLegacySemantics scopes D2 to the
// GENERIC signal plane: typed-struct fields keep their legacy semantics even
// when no handoff has been digested yet. The TDD-pin (`cycle_size != trivial`)
// DEPENDS on this — with zero signals it must evaluate true so tdd stays
// pinned on the conservative side (see floor.go tddPinned + shouldRun). A
// blanket absent⇒false over typed fields would silently unpin tdd at cycle
// start, weakening the integrity floor.
func TestEvalCondition_TypedFieldAbsentKeepsLegacySemantics(t *testing.T) {
	var sig RoutingSignals // no handoffs digested at all
	if !evalCondition(sig, config.Condition{Field: "cycle_size", Op: "ne", Value: "trivial"}) {
		t.Errorf("cycle_size ne trivial with zero signals = false; the tdd conditional-mandatory pin must stay true (conservative side) pre-handoff")
	}
}

// TestTriggerFires_AbsentFieldFailsClosed proves D2 at the trigger level — the
// exact cycle-238 mechanism end-to-end through triggerFires:
//  1. insert_when `ne` on an absent signal must NOT fire the phase, and
//  2. skip_when `ne` on an absent signal must NOT suppress an otherwise-firing
//     insert (fail-closed means absent CONDITIONS are false, in both polarities).
func TestTriggerFires_AbsentFieldFailsClosed(t *testing.T) {
	absentNe := config.Condition{Field: "scout.goal_type", Op: "ne", Value: "growth"}

	// (1) insert_when on absent field: must not fire.
	if triggerFires(RoutingSignals{}, config.RoutingBlock{InsertWhen: []config.Condition{absentNe}}) {
		t.Errorf("insert_when(absent ne) fired; want quiet (fail-closed)")
	}

	// (2) skip_when on absent field must not veto a genuinely firing insert.
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
	// String numeric value coerces for a numeric field.
	sig := sigFixture()
	if !evalCondition(sig, config.Condition{Field: "build.acs_red", Op: "eq", Value: "3"}) {
		t.Errorf("string '3' should coerce to numeric 3")
	}
	// Non-numeric string on numeric field → no match.
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
