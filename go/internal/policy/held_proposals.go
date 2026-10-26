package policy

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

const defaultHeldProposalsStage = "shadow"

type HeldProposalsPolicy struct {
	Stage string `json:"stage,omitempty"`
}

func (h *HeldProposalsPolicy) UnmarshalJSON(raw []byte) error {
	type block HeldProposalsPolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("held_proposals: %w", err)
	}
	if _, ok := config.GateStage(decoded.Stage); decoded.Stage != "" && !ok {
		return fmt.Errorf("held_proposals: stage %q is not off, shadow or enforce", decoded.Stage)
	}
	*h = HeldProposalsPolicy(decoded)
	return nil
}

func (p Policy) HeldProposalsStage() config.Stage {
	stage, _ := config.GateStage(defaultHeldProposalsStage)
	if p.HeldProposals != nil && p.HeldProposals.Stage != "" {
		stage, _ = config.GateStage(p.HeldProposals.Stage)
	}
	return stage
}

func HeldProposalsStageFor(projectRoot string) (config.Stage, error) {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return config.StageEnforce, fmt.Errorf("policy: held_proposals: %w", err)
	}
	return pol.HeldProposalsStage(), nil
}
