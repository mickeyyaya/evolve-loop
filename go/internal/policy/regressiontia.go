package policy

import "path/filepath"

// RegressionTIAPolicy is the "regression_tia" block for EGPS regression test-impact selection.
// See ADR-0082.
type RegressionTIAPolicy struct {
	// Stage is "off" (default: nothing computed), "shadow" (emit the decision,
	// skip nothing) or "enforce"; any other value is "off".
	Stage string `json:"stage,omitempty"`
}

// RegressionTIAConfig is the resolved regression-TIA configuration with defaults applied.
type RegressionTIAConfig struct {
	Stage string
}

// RegressionTIAConfig returns the regression-TIA stage; only off, shadow and enforce are honored.
func (p Policy) RegressionTIAConfig() RegressionTIAConfig {
	// Anything unknown is "off": selection that skips a predicate can hide a regression.
	c := RegressionTIAConfig{Stage: "off"}
	if p.RegressionTIA == nil {
		return c
	}
	switch p.RegressionTIA.Stage {
	case "off", "shadow", "enforce":
		c.Stage = p.RegressionTIA.Stage
	}
	return c
}

// RegressionTIAStageFor loads projectRoot's policy and returns the stage, or "off" if the file is unreadable.
func RegressionTIAStageFor(projectRoot string) string {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return "off"
	}
	return pol.RegressionTIAConfig().Stage
}
