package config

import (
	"path/filepath"
	"testing"
)

func TestEnumStringsFull(t *testing.T) {
	cases := []struct {
		got, want string
	}{
		{StageOff.String(), "0"},
		{StageShadow.String(), "shadow"},
		{StageAdvisory.String(), "advisory"},
		{StageEnforce.String(), "enforce"},
		{ModeStaticPreset.String(), "static"},
		{ModeDynamicLLM.String(), "llm"},
		{EnableOn.String(), "on"},
		{EnableOff.String(), "off"},
		{EnableContent.String(), "content"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("String() = %q, want %q", c.got, c.want)
		}
	}
}

func TestParseCondRule_AllOpsAndError(t *testing.T) {
	cases := []struct {
		in, op string
	}{
		{"a>=1", ">="},
		{"a<=1", "<="},
		{"a>1", ">"},
		{"a<1", "<"},
	}
	for _, c := range cases {
		got, err := parseCondRule(c.in)
		if err != nil || got.Op != c.op {
			t.Errorf("parseCondRule(%q) = %+v, %v; want op %q", c.in, got, err, c.op)
		}
	}
	if _, err := parseCondRule("nofieldoperator"); err == nil {
		t.Errorf("parseCondRule with no operator should error")
	}
}

func TestApplyEnv_ConditionalAndBadInts(t *testing.T) {
	cfg, ws := Load(filepath.Join(t.TempDir(), "absent.json"), map[string]string{
		"EVOLVE_CONDITIONAL_MANDATORY":   "audit:verdict==PASS",
		"EVOLVE_MAX_OPTIONAL_INSERTIONS": "notanint",
		"EVOLVE_ROUTING_MODE":            "preset",
	})
	rule, ok := cfg.Conditional["audit"]
	if !ok || rule.Field != "verdict" || rule.Op != "==" || rule.Value != "PASS" {
		t.Errorf("conditional audit = %+v (ok=%v), want {verdict == PASS}", rule, ok)
	}
	if cfg.Mode != ModeStaticPreset {
		t.Errorf("Mode = %v, want ModeStaticPreset (alias 'preset')", cfg.Mode)
	}
	if cfg.MaxInsertions != 4 {
		t.Errorf("MaxInsertions = %d, want default 4 (bad int ignored)", cfg.MaxInsertions)
	}
	if !hasWarning(ws, "unknown-value") {
		t.Errorf("expected unknown-value warning for non-int max insertions")
	}
}

func TestApplyRegistry_BadConditionalAndUnknownEnabled(t *testing.T) {
	reg := writeRegistry(t, `{
      "schema_version": 3,
      "config": { "conditional_mandatory": {"tdd": "garbage"} },
      "phases": [ {"name":"tester","enabled":"maybe"} ]
    }`)
	cfg, ws := Load(reg, map[string]string{})
	if !hasWarning(ws, "unknown-value") {
		t.Errorf("expected unknown-value warning for bad conditional + bad enabled")
	}
	if cfg.PhaseEnable["tester"] != EnableContent {
		t.Errorf("unknown enabled should default to EnableContent, got %v", cfg.PhaseEnable["tester"])
	}
}

func TestSkipWhenParsed(t *testing.T) {
	reg := writeRegistry(t, `{
      "schema_version": 3, "config": {},
      "phases": [ {"name":"retrospective","optional":true,
        "routing": {"skip_when": [ {"field":"triage.cycle_size","op":"eq","value":"trivial"} ] } } ]
    }`)
	cfg, _ := Load(reg, map[string]string{})
	tb := cfg.Triggers["retrospective"]
	if len(tb.SkipWhen) != 1 || tb.SkipWhen[0].Field != "triage.cycle_size" {
		t.Errorf("skip_when not parsed: %+v", tb)
	}
}
