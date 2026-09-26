package evalgate

import (
	"context"
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// remediator is an optional gate capability: it states what would satisfy the
// gate's violation. Gates without it keep the default correction directive.
type remediator interface {
	remediation(in core.ReviewInput) string
}

// gate is one inter-phase check. block, not reason, signals a violation: an
// advisory gate returns a reason on a healthy deliverable so it reaches the log.
type gate interface {
	name() string
	appliesTo(phase string) bool
	check(in core.ReviewInput) (reason string, block bool)
}

// reviewer composes the gates behind one core.DeliverableReviewer. It is never
// built for StageOff, because the composition root then skips WithReviewer.
type reviewer struct {
	stage config.Stage
	gates []gate
	logf  func(format string, args ...any)
}

// NewReviewer builds the composite gate reviewer for the given stage.
func NewReviewer(stage config.Stage) core.DeliverableReviewer {
	return &reviewer{
		stage: stage,
		gates: []gate{materializationGate{}, qualityGate{}, floorBindingGate{}, flakyShapeGate{}},
		logf:  func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) },
	}
}

// Review logs every gate reason and, at StageEnforce, rejects on the first blocking violation.
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
			res := core.ReviewResult{Approve: false, Reason: reason}
			// The generic correction forbids creating files, so a gate whose remedy
			// is a missing file must supply its own.
			if rm, ok := g.(remediator); ok {
				res.Remediation = rm.remediation(in)
			}
			return res
		}
	}
	return core.ReviewResult{Approve: true}
}
