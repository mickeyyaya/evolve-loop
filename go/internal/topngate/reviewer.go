package topngate

import (
	"context"
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// reviewer composes the top_n binding gate behind one core.DeliverableReviewer.
// It is stage-aware: at StageShadow every violation is logged but approved; at
// StageEnforce a CERTAIN violation aborts the cycle at the build->audit
// transition. StageOff is never constructed (the composition root skips
// WithReviewer entirely). Mirrors internal/evalgate.reviewer — stage-gating is
// the whole rollout control (no feature flag).
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
		gates: []gate{topNBindingGate{}},
		logf:  func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) },
	}
}

// Review runs each applicable gate. The first CERTAIN violation aborts at
// StageEnforce; everything else (any violation at shadow, a non-build phase) is
// logged and approved. A blocked review records a non-empty abort_reason.
func (r *reviewer) Review(_ context.Context, in core.ReviewInput) core.ReviewResult {
	for _, g := range r.gates {
		if !g.appliesTo(in.Phase) {
			continue
		}
		reason, block := g.check(in)
		if reason == "" {
			continue
		}
		r.logf("[topngate] %s: %s (stage=%s, blocking=%v)", g.name(), reason, r.stage, block && r.stage == config.StageEnforce)
		if block && r.stage == config.StageEnforce {
			return core.ReviewResult{Approve: false, Reason: reason}
		}
	}
	return core.ReviewResult{Approve: true}
}
