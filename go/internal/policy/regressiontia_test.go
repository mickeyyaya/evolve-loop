package policy

import "testing"

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
