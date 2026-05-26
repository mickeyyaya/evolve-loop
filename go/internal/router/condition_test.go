package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func sigFixture() RoutingSignals {
	return RoutingSignals{
		Scout:  ScoutSignals{CycleSizeEstimate: "large", ItemCount: 5, CarryoverCount: 2, Present: true},
		Triage: TriageSignals{CycleSize: "medium", Present: true},
		Build:  BuildSignals{Verdict: "PASS", ACSRed: 3, ACSGreen: 30, ACSRegression: 4, FilesTouched: 7, SeverityMax: SevHigh, Present: true},
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
		{"build.acs_regression", "gt", 3, true},
		{"build.files_touched", "gte", 7, true},
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
