package policy

import (
	"fmt"
	"path/filepath"
)

type RegressionTIAPolicy struct {
	Stage string `json:"stage,omitempty"`
}

func (r *RegressionTIAPolicy) UnmarshalJSON(raw []byte) error {
	type block RegressionTIAPolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("regression_tia: %w", err)
	}
	switch decoded.Stage {
	case "", "off", "shadow", "enforce":
	default:
		return fmt.Errorf("regression_tia: stage %q is not off, shadow or enforce", decoded.Stage)
	}
	*r = RegressionTIAPolicy(decoded)
	return nil
}

type RegressionTIAConfig struct {
	Stage string
}

func (p Policy) RegressionTIAConfig() RegressionTIAConfig {
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

func RegressionTIAStageFor(projectRoot string) string {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return "off"
	}
	return pol.RegressionTIAConfig().Stage
}
