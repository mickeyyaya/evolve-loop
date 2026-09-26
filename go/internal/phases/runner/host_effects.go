package runner

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner/verdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// CodeHostEffectFailed marks a declared effect the runner could not perform
// before the verdict engine judged the phase.
const CodeHostEffectFailed signalcenter.Code = "RUNNER_HOST_EFFECT_FAILED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleRunner, CodeHostEffectFailed, "the runner could not perform a declared host effect before judging the phase; the engine and the gate still judge the effect; the reason is the performer's error")
}

// HostEffectsWired reports whether the composition root bound host effects.
func (b *BaseRunner) HostEffectsWired() bool {
	return b.hostEffects != nil && b.hostEffects() != nil
}

// performHostEffects runs before the engine's first verification. It ignores
// cancellation, because a teardown is often what cancelled the context.
func (b *BaseRunner) performHostEffects(ctx context.Context, d verdict.Dispatch) {
	if !b.HostEffectsWired() {
		return
	}
	err := b.hostEffects().Perform(context.WithoutCancel(ctx), core.ReviewInput{
		Cycle: d.Cycle, RunID: d.RunID, Phase: d.Phase, Workspace: d.Workspace, Worktree: d.Worktree,
		ProjectRoot: d.ProjectRoot, ExplanationDocumentationVersion: d.ExplanationDocumentationVersion,
	})
	if err == nil || b.signals == nil {
		return
	}
	b.signals().Emit(signalcenter.Event{
		Cycle: d.Cycle, RunID: d.RunID, Phase: d.Phase,
		Module: signalcenter.ModuleRunner, Origin: "BaseRunner.performHostEffects", Kind: signalcenter.KindRunnerWarning,
		Severity: signalcenter.SeverityWarn, Code: CodeHostEffectFailed, Reason: err.Error(),
	})
}
