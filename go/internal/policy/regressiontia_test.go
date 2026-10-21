package policy

import (
	"strings"
	"testing"
)

func TestRegressionTIAConfig_DefaultsOff(t *testing.T) {
	if got := (Policy{}).RegressionTIAConfig().Stage; got != "off" {
		t.Errorf("zero-value Policy{}.RegressionTIAConfig().Stage = %q, want \"off\" — the checked-in policy.json has no regression_tia block, so absent MUST mean dormant", got)
	}
	p := Policy{RegressionTIA: nil}
	if got := p.RegressionTIAConfig().Stage; got != "off" {
		t.Errorf("nil RegressionTIA block ⇒ Stage = %q, want \"off\"", got)
	}
	p = Policy{RegressionTIA: &RegressionTIAPolicy{Stage: ""}}
	if got := p.RegressionTIAConfig().Stage; got != "off" {
		t.Errorf("present block with empty stage ⇒ Stage = %q, want \"off\"", got)
	}
}

func TestRegressionTIAConfig_HonorsClosedVocabulary(t *testing.T) {
	for _, stage := range []string{"off", "shadow", "enforce"} {
		p := Policy{RegressionTIA: &RegressionTIAPolicy{Stage: stage}}
		if got := p.RegressionTIAConfig().Stage; got != stage {
			t.Errorf("stage %q resolved to %q, want it honored verbatim", stage, got)
		}
	}
}

func TestRegressionTIAConfig_UnknownStageFallsBackToOff(t *testing.T) {
	for _, stage := range []string{"on", "true", "Shadow", "ENFORCE", "advsory", "1", " shadow"} {
		p := Policy{RegressionTIA: &RegressionTIAPolicy{Stage: stage}}
		if got := p.RegressionTIAConfig().Stage; got != "off" {
			t.Errorf("unknown stage %q resolved to %q, want \"off\" — a typo must not silently arm regression selection", stage, got)
		}
	}
}

func TestRegressionTIAPolicy_TheBlockIsStrictDecoded(t *testing.T) {
	for name, block := range map[string]string{
		"a mistyped stage": `{"regression_tia":{"stage":"shadwo"}}`,
		"an unknown key":   `{"regression_tia":{"stage":"shadow","bogus":1}}`,
		"a wrong type":     `{"regression_tia":{"stage":1}}`,
		"a padded stage":   `{"regression_tia":{"stage":" shadow"}}`,
	} {
		if _, err := Parse([]byte(block)); err == nil || !strings.Contains(err.Error(), "regression_tia") {
			t.Errorf("%s: Parse = %v, want a refusal that names regression_tia", name, err)
		}
	}
	for _, stage := range []string{"off", "shadow", "enforce"} {
		p, err := Parse([]byte(`{"regression_tia":{"stage":"` + stage + `"}}`))
		if err != nil || p.RegressionTIAConfig().Stage != stage {
			t.Errorf("stage %q = (%q, %v), want it honored", stage, p.RegressionTIAConfig().Stage, err)
		}
	}
}
