package gatecontract

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type StageInput struct {
	Root   string
	Cycle  int
	Fields map[string]string
}

type Decision struct {
	Fields map[string]string
}

type EvidenceRecord struct {
	Key    string
	Cycle  int
	Seq    int
	Fields map[string]string
}

type StageVerdict struct {
	Ready      bool
	Decisions  int
	FalseStops int
	Escapes    int
	Conflicts  int
}

type EmitSite struct {
	Path string
	Func string
	Ref  string
}

type Promotion struct {
	MaxEscapes      int
	MaxFalseStops   int
	MinDecisions    int
	WindowCycles    int
	MaxFalseStopPct int
}

type Shadow struct {
	PolicyKey     string
	Signal        signalcenter.Code
	Decide        func(context.Context, StageInput) (Decision, error)
	Enforce       func(context.Context, StageInput, Decision) error
	Score         func(context.Context, []EvidenceRecord) (StageVerdict, error)
	Fields        []string
	EmitSites     []EmitSite
	Promotion     Promotion
	Closure       []string
	PolicySubtree string
}

func (e Entry) EffectiveStage(policyStage func(string) string) string {
	if e.Shadow != nil && e.Shadow.PolicyKey != "" {
		return policyStage(e.Shadow.PolicyKey)
	}
	return e.Stage
}
