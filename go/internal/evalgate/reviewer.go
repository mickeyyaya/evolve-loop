package evalgate

import (
	"context"
	"fmt"
	"os"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// gate is one structural inter-phase check. appliesTo selects the phase whose
// deliverable it inspects; check returns a non-empty reason on a violation and
// block=true only when the violation is CERTAIN (a stat'd-missing eval file, a
// definite tautology) and so should abort the cycle at enforce. Any ambiguity
// (parse failure, advisory WARN) returns block=false so enforce never
// false-blocks a healthy cycle.
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
		gates: []gate{materializationGate{}, qualityGate{}, floorBindingGate{}},
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
