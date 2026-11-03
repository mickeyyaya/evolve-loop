package policy

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

const defaultBoundaryRatchetMergeStage = "shadow"

type BoundaryPolicy struct {
	RatchetMerge *BoundaryRatchetMergePolicy `json:"ratchet_merge,omitempty"`
}

type BoundaryRatchetMergePolicy struct {
	Stage string `json:"stage,omitempty"`
}

func (p Policy) BoundaryRatchetMergeStage() (config.Stage, error) {
	raw := defaultBoundaryRatchetMergeStage
	if p.Boundary != nil && p.Boundary.RatchetMerge != nil && p.Boundary.RatchetMerge.Stage != "" {
		raw = p.Boundary.RatchetMerge.Stage
	}
	stage, ok := config.GateStage(raw)
	if !ok {
		return config.StageOff, fmt.Errorf("policy: boundary.ratchet_merge.stage must be off, shadow or enforce, got %q", raw)
	}
	return stage, nil
}
