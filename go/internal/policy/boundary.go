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

func (b *BoundaryRatchetMergePolicy) UnmarshalJSON(raw []byte) error {
	type block BoundaryRatchetMergePolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("boundary.ratchet_merge: %w", err)
	}
	if _, ok := config.GateStage(decoded.Stage); decoded.Stage != "" && !ok {
		return fmt.Errorf("boundary.ratchet_merge: stage %q is not off, shadow or enforce", decoded.Stage)
	}
	*b = BoundaryRatchetMergePolicy(decoded)
	return nil
}

func (p Policy) BoundaryRatchetMergeStage() config.Stage {
	raw := defaultBoundaryRatchetMergeStage
	if p.Boundary != nil && p.Boundary.RatchetMerge != nil && p.Boundary.RatchetMerge.Stage != "" {
		raw = p.Boundary.RatchetMerge.Stage
	}
	stage, _ := config.GateStage(raw)
	return stage
}
