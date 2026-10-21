package qualitypanel

import "fmt"

type Stage string

const (
	StageShadow Stage = "shadow"
	StageReport Stage = "report"
	StageGate   Stage = "gate"
)

func ParseStage(s string) (Stage, error) {
	switch st := Stage(s); st {
	case StageShadow, StageReport, StageGate:
		return st, nil
	}
	return "", fmt.Errorf("stage %q is not shadow, report or gate", s)
}

type Direction int

const (
	HigherIsBetter Direction = iota
	LowerIsBetter
)

type Index struct {
	ID           string
	Name         string
	Unit         string
	Direction    Direction
	Threshold    float64
	HasThreshold bool
	Breachable   bool
	NMin         int
	Baseline     string
}

func (ix Index) DefaultStage() Stage {
	if ix.Baseline != "" {
		return StageReport
	}
	return StageShadow
}

func Indexes() []Index { return append([]Index(nil), registry...) }

func Lookup(id string) (Index, bool) {
	for _, ix := range registry {
		if ix.ID == id {
			return ix, true
		}
	}
	return Index{}, false
}

func StageFor(id string, overrides map[string]Stage) Stage {
	if st, ok := overrides[id]; ok {
		return st
	}
	ix, _ := Lookup(id)
	return ix.DefaultStage()
}
