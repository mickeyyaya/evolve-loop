package topngate

import (
	"context"
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// reviewer runs the gates behind one core.DeliverableReviewer: shadow logs
// every finding, enforce also rejects the first certain violation.
type reviewer struct {
	stage config.Stage
	gates []gate
	logf  func(format string, args ...any)
}

// NewReviewer returns the topngate reviewer for stage; callers skip it at StageOff.
func NewReviewer(stage config.Stage) core.DeliverableReviewer {
	return &reviewer{
		stage: stage,
		gates: []gate{topNBindingGate{}, tddScopeGate{}},
		logf:  func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) },
	}
}

// Review logs every gate finding and rejects only a certain violation at StageEnforce.
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
