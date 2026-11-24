package policy

import "fmt"

type RatchetMergeStage string

const (
	RatchetMergeOff     RatchetMergeStage = "off"
	RatchetMergeShadow  RatchetMergeStage = "shadow"
	RatchetMergeEnforce RatchetMergeStage = "enforce"
)

var ratchetMergeStages = []RatchetMergeStage{RatchetMergeOff, RatchetMergeShadow, RatchetMergeEnforce}

type BoundaryPolicy struct {
	RatchetMerge *RatchetMergePolicy `json:"ratchet_merge,omitempty"`
}

type RatchetMergePolicy struct {
	Stage RatchetMergeStage `json:"stage,omitempty"`
}

func (b *BoundaryPolicy) validate() error {
	if b == nil || b.RatchetMerge == nil || b.RatchetMerge.Stage == "" {
		return nil
	}
	for _, stage := range ratchetMergeStages {
		if b.RatchetMerge.Stage == stage {
			return nil
		}
	}
	return fmt.Errorf("boundary.ratchet_merge.stage: %q is not one of %v", b.RatchetMerge.Stage, ratchetMergeStages)
}

func (p Policy) RatchetMergeStage() RatchetMergeStage {
	if p.Boundary == nil || p.Boundary.RatchetMerge == nil || p.Boundary.RatchetMerge.Stage == "" {
		return RatchetMergeShadow
	}
	return p.Boundary.RatchetMerge.Stage
}
