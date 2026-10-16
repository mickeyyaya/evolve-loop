package evalgate

import (
	"context"
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type remediator interface {
	remediation(in core.ReviewInput) string
}

type gate interface {
	name() string
	appliesTo(phase string) bool
	check(in core.ReviewInput) (reason string, block bool)
}

type reviewer struct {
	stage config.Stage
	gates []gate
	logf  func(format string, args ...any)
}

func NewReviewer(stage config.Stage) core.DeliverableReviewer {
	return &reviewer{
		stage: stage,
		gates: []gate{materializationGate{}, qualityGate{}, floorBindingGate{}, flakyShapeGate(), unsatisfiableShapeGate()},
		logf:  func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) },
	}
}

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
			if rm, ok := g.(remediator); ok {
				res.Remediation = rm.remediation(in)
			}
			return res
		}
	}
	return core.ReviewResult{Approve: true}
}
