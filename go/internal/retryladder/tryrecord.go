package retryladder

import (
	"cmp"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type Context string

const (
	ContextNew   Context = "new"
	ContextFresh Context = "fresh"
)

type Stage string

const (
	StageShadow  Stage = policy.RetryLadderShadow
	StageEnforce Stage = policy.RetryLadderEnforce
)

const SignalDecision = "LADDER_DECISION"

var climbingSeats = []string{"build", "tdd", "scout", "triage", "fault-localization", "bug-reproduction"}

func Climbs(phase string) bool { return slices.Contains(climbingSeats, phase) }

type Try struct {
	Rung         Rung
	FailureClass Class
	Fingerprint  string
	Context      Context
	Stage        Stage
	Would        Rung
	Tier         string
}

func (t *Try) Mark(rec *llmcalls.Record) {
	resolved := Try{}
	if t != nil {
		resolved = *t
	}
	rec.Rung = string(cmp.Or(resolved.Rung, R0))
	rec.Context = string(cmp.Or(resolved.Context, ContextNew))
	rec.LadderStage = string(cmp.Or(resolved.Stage, StageShadow))
	rec.LadderWould = string(resolved.Would)
	rec.FailureClass = string(resolved.FailureClass)
	rec.Fingerprint = resolved.Fingerprint
}

func (t *Try) LadderTier() string {
	if t == nil {
		return ""
	}
	return t.Tier
}
