package evalgate

import (
	"context"
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// gate is one structural inter-phase check. appliesTo selects the phase whose
// deliverable it inspects; check returns block=true only when a violation is
// CERTAIN (a stat'd-missing eval file, a definite tautology) and so should abort
// the cycle at enforce. Any ambiguity (parse failure, advisory WARN) returns
// block=false so enforce never false-blocks a healthy cycle.
//
// `block` IS THE VIOLATION SIGNAL — `reason` is not. An ADVISORY gate returns a
// non-empty reason on a perfectly healthy deliverable, because that reason is
// how its observation reaches the phase log at all (flakyShapeGate emits exactly
// one line per tdd phase on EVERY path, including the clean one). A future
// simplification of Review that keys rejection off `reason != ""` would
// therefore fail every tdd phase — including cycles with no Go predicate package
// (review MEDIUM). Key on block, always.
type gate interface {
	name() string
	appliesTo(phase string) bool
	check(in core.ReviewInput) (reason string, block bool)
}

// reviewer composes the structural gates behind one core.DeliverableReviewer.
// It is stage-aware: at StageShadow every violation is logged but approved; at
// StageEnforce a CERTAIN violation aborts the cycle. StageOff is never
// constructed (the composition root skips WithReviewer entirely).
type reviewer struct {
	stage config.Stage
	gates []gate
	logf  func(format string, args ...any)
}

// NewReviewer builds the composite gate reviewer for the given stage. Callers
// wire it via core.WithReviewer only when stage != StageOff.
func NewReviewer(stage config.Stage) core.DeliverableReviewer {
	return &reviewer{
		stage: stage,
		gates: []gate{materializationGate{}, qualityGate{}, floorBindingGate{}, flakyShapeGate{}},
		logf:  func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) },
	}
}

// Review runs each applicable gate. The first CERTAIN violation aborts at
// StageEnforce; everything else (advisory violations, any violation at shadow)
// is logged and approved.
func (r *reviewer) Review(_ context.Context, in core.ReviewInput) core.ReviewResult {
	for _, g := range r.gates {
		if !g.appliesTo(in.Phase) {
			continue
		}
		reason, block := g.check(in)
		if reason == "" {
			continue
		}
		r.logf("[evalgate] %s: %s (stage=%s, blocking=%v)", g.name(), reason, r.stage, block && r.stage == config.StageEnforce)
		if block && r.stage == config.StageEnforce {
			return core.ReviewResult{Approve: false, Reason: reason}
		}
	}
	return core.ReviewResult{Approve: true}
}
