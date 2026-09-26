package runner

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable/gatesignal"
	"github.com/mickeyyaya/evolve-loop/go/internal/logfilter"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner/verdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// signalSource is the optional Bridge capability exposing its Center.
type signalSource interface{ Signals() *signalcenter.Center }

// resolveSignals is the one derivation of the engine's Center: Options.Signals, else the Bridge's own, else nil.
func resolveSignals(opts Options) func() *signalcenter.Center {
	if opts.Signals != nil {
		return opts.Signals
	}
	if src, ok := opts.Bridge.(signalSource); ok {
		return src.Signals
	}
	return nil
}

// wiredVerdictEngine is the engine's one construction and the one resolution of its seams; the seams live nowhere else.
func wiredVerdictEngine(opts Options) *verdict.Engine {
	verify := func(id verdict.Identity, phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		switch {
		case opts.VerifyFn != nil:
			return opts.VerifyFn(phase, roots)
		case opts.ContractVerifier != nil:
			if v := opts.ContractVerifier(); v != nil {
				// The gate's own verifier, so the engine classifies the bytes the gate approves.
				return v.VerifyForClassification(gatesignal.Check{Cycle: id.Cycle, RunID: id.RunID, Phase: id.Phase}, phase, roots)
			}
		}
		// Catalog-aware so user and minted phases resolve as the host gate resolves them. It never
		// salvages: only the gate repairs, and the engine sees a repair only through the gate's verifier.
		return deliverable.VerifyCatalogAwareStage(phase, roots, opts.PhaseIO)
	}
	sleep := opts.SleepFn
	if sleep == nil {
		sleep = settleSleep
	}
	filter := opts.StdoutFilter
	if filter == nil {
		filter = logfilter.Process
	}
	if opts.DisableStdoutFilter {
		filter = nil
	}
	return verdict.New(verify, verdict.WithSleep(sleep), verdict.WithStdoutFilter(filter),
		verdict.WithOptional(opts.Optional), verdict.WithSignals(resolveSignals(opts)))
}

// dispatchOf is the one projection of the host's stage objects onto the engine's input; keyed, as go vet requires.
func dispatchOf(req core.PhaseRequest, prep phasePreparation, plan phaseDispatchPlan, d phaseDispatchResult) verdict.Dispatch {
	return verdict.Dispatch{
		Cycle: req.Cycle, RunID: req.RunID, Phase: prep.phase, Workspace: req.Workspace, Worktree: req.Worktree,
		ProjectRoot: req.ProjectRoot, ExplanationDocumentationVersion: req.ExplanationDocumentationVersion,
		ArtifactPath: prep.artifactPath, PreDispatch: prep.preDispatch, HadPreDispatch: prep.hadPreDispatch,
		Bridge: d.bridgeResponse, BridgeErr: d.bridgeErr, DurationMS: d.durationMS,
		ResolvedModel: d.resolvedModel, ModelSource: plan.modelSource, FenceDiagnostics: d.fenceDiagnostics,
	}
}

// classifyWith binds the phase hook to Run's request, WorktreeVerified already stamped, and the terminal bridge response.
func (b *BaseRunner) classifyWith(req core.PhaseRequest, bres core.BridgeResponse) verdict.Classify {
	return func(artifact string) (string, []core.Diagnostic, string) {
		return b.hooks.Classify(artifact, req, bres)
	}
}

// SignalsWired reports whether this runner's verdict engine reaches a Signal Center.
func (b *BaseRunner) SignalsWired() bool { return b.judge.SignalsWired() }

// ContractVerifierWired reports whether the composition root supplied the engine's verifier; it does not prove the gate live.
func (b *BaseRunner) ContractVerifierWired() bool {
	return !b.verifyInjected && b.contractVerifier != nil && b.contractVerifier() != nil
}

// The engine's settle bounds under the names the settle tests read.
const (
	reconcileSettleRetries  = verdict.SettleRetries
	reconcileSettleInterval = verdict.SettleInterval
)
