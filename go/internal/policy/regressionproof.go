package policy

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

type RegressionProofPolicy struct {
	Stage string `json:"stage,omitempty"`
}

func (r *RegressionProofPolicy) UnmarshalJSON(raw []byte) error {
	type block RegressionProofPolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("regression_proof: %w", err)
	}
	if stage, ok := config.GateStage(decoded.Stage); decoded.Stage != "" && (!ok || stage == config.StageEnforce) {
		return fmt.Errorf("regression_proof: stage %q is not off or shadow", decoded.Stage)
	}
	*r = RegressionProofPolicy(decoded)
	return nil
}

func (p Policy) RegressionProofStage() config.Stage {
	if p.RegressionProof == nil || p.RegressionProof.Stage == "" {
		return config.StageShadow
	}
	stage, _ := config.GateStage(p.RegressionProof.Stage)
	return stage
}

func RegressionProofStageFor(projectRoot string) (config.Stage, error) {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return config.StageShadow, fmt.Errorf("policy: regression_proof: %w", err)
	}
	return pol.RegressionProofStage(), nil
}
