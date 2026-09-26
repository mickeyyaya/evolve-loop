package core

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// CodeHostEffectFailed marks a declared effect the host could not perform.
const CodeHostEffectFailed signalcenter.Code = "ORCHESTRATOR_HOST_EFFECT_FAILED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeHostEffectFailed, "the host could not perform a declared effect before the phase's review; the effects gate still judges it")
}

// HostEffects performs the effects a phase declares, given exactly the input
// its review receives.
type HostEffects interface {
	Perform(ctx context.Context, in ReviewInput) error
}

// WithHostEffects binds the performer; unbound, declared effects stay the agent's.
func WithHostEffects(h HostEffects) Option {
	return func(o *Orchestrator) { o.hostEffects = h }
}

// HostEffectsWired reports whether a performer is bound.
func (o *Orchestrator) HostEffectsWired() bool { return o.hostEffects != nil }

func (o *Orchestrator) performEffectsAndReview(ctx context.Context, in ReviewInput) ReviewResult {
	if o.hostEffects != nil {
		if err := o.hostEffects.Perform(ctx, in); err != nil {
			o.signals.Emit(signalcenter.Event{
				Cycle: in.Cycle, RunID: o.signalRunID(), Phase: in.Phase,
				Module: signalcenter.ModuleOrchestrator, Origin: "Orchestrator.review", Kind: signalcenter.KindInboxWarning,
				Severity: signalcenter.SeverityWarn, Code: CodeHostEffectFailed, Reason: err.Error(),
			})
		}
	}
	return o.reviewer.Review(ctx, in)
}
