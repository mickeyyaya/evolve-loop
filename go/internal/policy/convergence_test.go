package policy_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func compiledConvergence() policy.ConvergenceConfig {
	return policy.ConvergenceConfig{
		Stage:                    "shadow",
		MaxFixRounds:             3,
		BaseBlockingBar:          "MEDIUM",
		RaisedBlockingBar:        "HIGH",
		ConcentrationThreshold:   0.6,
		ConcentrationWindow:      2,
		ConcentrationMinFindings: 5,
		MaxBackwardEdges:         3,
	}
}

func TestConvergenceConfig_AnAbsentBlockIsTheCompiledDefault(t *testing.T) {
	for name, p := range map[string]policy.Policy{
		"no workflow block":    {},
		"no convergence block": {Workflow: &policy.WorkflowPolicy{}},
		"an empty block":       {Workflow: &policy.WorkflowPolicy{Convergence: &policy.ConvergencePolicy{}}},
	} {
		t.Run(name, func(t *testing.T) {
			got := p.ConvergenceConfig()

			if !reflect.DeepEqual(got, compiledConvergence()) {
				t.Fatalf("ConvergenceConfig() = %+v, want %+v", got, compiledConvergence())
			}
		})
	}
}

func TestConvergenceConfig_EveryNamedValueOverridesTheDefault(t *testing.T) {
	p, err := loadPolicyText(t, `{"workflow": {"convergence": {
		"stage": "enforce",
		"max_fix_rounds": 4,
		"base_blocking_bar": "HIGH",
		"raised_blocking_bar": "CRITICAL",
		"concentration_threshold": 0.75,
		"concentration_window": 3,
		"concentration_min_findings": 7,
		"max_backward_edges": 5
	}}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := p.ConvergenceConfig()

	want := policy.ConvergenceConfig{
		Stage: "enforce", MaxFixRounds: 4, BaseBlockingBar: "HIGH", RaisedBlockingBar: "CRITICAL",
		ConcentrationThreshold: 0.75, ConcentrationWindow: 3, ConcentrationMinFindings: 7, MaxBackwardEdges: 5,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ConvergenceConfig() = %+v, want %+v", got, want)
	}
}

func TestConvergencePolicy_AnUnknownKeyIsRefused(t *testing.T) {
	_, err := loadPolicyText(t, `{"workflow": {"convergence": {"max_rounds": 4}}}`)

	if err == nil || !strings.Contains(err.Error(), "convergence") || !strings.Contains(err.Error(), "max_rounds") {
		t.Fatalf("Load error = %v, want a convergence error naming the unknown key max_rounds", err)
	}
}

func TestConvergencePolicy_AnOutOfRangeNumberIsRefusedUnderItsKey(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"max_fix_rounds", "0"},
		{"concentration_threshold", "0"},
		{"concentration_threshold", "1.01"},
		{"concentration_window", "0"},
		{"concentration_min_findings", "0"},
		{"max_backward_edges", "0"},
	} {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			_, err := loadPolicyText(t, `{"workflow": {"convergence": {"`+tc.key+`": `+tc.value+`}}}`)

			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("Load error = %v, want one naming %s", err, tc.key)
			}
		})
	}
}

func TestConvergencePolicy_TheBoundaryValuesAreAccepted(t *testing.T) {
	p, err := loadPolicyText(t, `{"workflow": {"convergence": {
		"max_fix_rounds": 1, "concentration_threshold": 1, "concentration_window": 1,
		"concentration_min_findings": 1, "max_backward_edges": 1
	}}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := p.ConvergenceConfig()

	if got.MaxFixRounds != 1 || got.ConcentrationThreshold != 1 || got.ConcentrationWindow != 1 ||
		got.ConcentrationMinFindings != 1 || got.MaxBackwardEdges != 1 {
		t.Fatalf("ConvergenceConfig() = %+v, want every boundary value kept", got)
	}
}

func TestConvergenceConfig_AnUnknownEnumWordWarnsAndResolvesToTheDefault(t *testing.T) {
	p, err := loadPolicyText(t, `{"workflow": {"convergence": {
		"stage": "enforcing", "base_blocking_bar": "medium", "raised_blocking_bar": "SEVERE"
	}}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := p.ConvergenceConfig()

	if got.Stage != "shadow" || got.BaseBlockingBar != "MEDIUM" || got.RaisedBlockingBar != "HIGH" {
		t.Fatalf("ConvergenceConfig() = %+v, want each unknown word resolved to its default", got)
	}
	want := []string{
		`workflow.convergence.stage: unknown value "enforcing", falling back to "shadow"`,
		`workflow.convergence.base_blocking_bar: unknown value "medium", falling back to "MEDIUM"`,
		`workflow.convergence.raised_blocking_bar: unknown value "SEVERE", falling back to "HIGH"`,
	}
	if !reflect.DeepEqual(got.Warnings, want) {
		t.Fatalf("Warnings = %q, want %q", got.Warnings, want)
	}
}

func TestConvergenceConfig_ARaisedBarBelowTheBaseWarnsAndHoldsTheBase(t *testing.T) {
	p, err := loadPolicyText(t, `{"workflow": {"convergence": {"base_blocking_bar": "HIGH", "raised_blocking_bar": "MEDIUM"}}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := p.ConvergenceConfig()

	if got.BaseBlockingBar != "HIGH" || got.RaisedBlockingBar != "HIGH" {
		t.Fatalf("bars = %s/%s, want HIGH/HIGH: the raised bar never lowers the base", got.BaseBlockingBar, got.RaisedBlockingBar)
	}
	want := []string{`workflow.convergence.raised_blocking_bar: "MEDIUM" is below base_blocking_bar "HIGH", holding "HIGH"`}
	if !reflect.DeepEqual(got.Warnings, want) {
		t.Fatalf("Warnings = %q, want %q", got.Warnings, want)
	}
}

func TestConvergenceConfig_ARaisedBarEqualToTheBaseIsKeptWithoutAWarning(t *testing.T) {
	p, err := loadPolicyText(t, `{"workflow": {"convergence": {"base_blocking_bar": "HIGH", "raised_blocking_bar": "HIGH"}}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := p.ConvergenceConfig()

	if got.RaisedBlockingBar != "HIGH" || len(got.Warnings) != 0 {
		t.Fatalf("ConvergenceConfig() = %+v, want HIGH kept with no warning", got)
	}
}

func TestConvergenceConfig_ResolvingReturnsAFreshWarningList(t *testing.T) {
	p, err := loadPolicyText(t, `{"workflow": {"convergence": {"stage": "loud"}}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	first := p.ConvergenceConfig()
	second := p.ConvergenceConfig()

	if len(first.Warnings) != 1 || len(second.Warnings) != 1 {
		t.Fatalf("warnings = %q then %q, want exactly one each time", first.Warnings, second.Warnings)
	}
}

func TestConvergenceConfig_ValidateRefusesAnUnresolvedConfigUnderItsKey(t *testing.T) {
	for _, tc := range []struct {
		key  string
		edit func(*policy.ConvergenceConfig)
	}{
		{"max_fix_rounds", func(c *policy.ConvergenceConfig) { c.MaxFixRounds = 0 }},
		{"concentration_window", func(c *policy.ConvergenceConfig) { c.ConcentrationWindow = 0 }},
		{"concentration_min_findings", func(c *policy.ConvergenceConfig) { c.ConcentrationMinFindings = 0 }},
		{"max_backward_edges", func(c *policy.ConvergenceConfig) { c.MaxBackwardEdges = 0 }},
		{"concentration_threshold", func(c *policy.ConvergenceConfig) { c.ConcentrationThreshold = 0 }},
		{"concentration_threshold", func(c *policy.ConvergenceConfig) { c.ConcentrationThreshold = 1.01 }},
		{"stage", func(c *policy.ConvergenceConfig) { c.Stage = "" }},
		{"base_blocking_bar", func(c *policy.ConvergenceConfig) { c.BaseBlockingBar = "LOW" }},
		{"raised_blocking_bar", func(c *policy.ConvergenceConfig) { c.RaisedBlockingBar = "LOUD" }},
	} {
		t.Run(tc.key, func(t *testing.T) {
			c := compiledConvergence()
			tc.edit(&c)

			err := c.Validate()

			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("Validate = %v, want an error naming %s", err, tc.key)
			}
		})
	}
	if err := compiledConvergence().Validate(); err != nil {
		t.Fatalf("the compiled default: Validate = %v", err)
	}
}
