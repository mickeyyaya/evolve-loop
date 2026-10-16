package policy

import "path/filepath"

type RegressionTIAPolicy struct {
	Stage string `json:"stage,omitempty"`
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
