package runner

import (
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable/gatesignal"
	"github.com/mickeyyaya/evolve-loop/go/internal/logfilter"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner/verdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type signalSource interface{ Signals() *signalcenter.Center }

func resolveSignals(opts Options) func() *signalcenter.Center {
	if opts.Signals != nil {
		return opts.Signals
	}
	if src, ok := opts.Bridge.(signalSource); ok {
		return src.Signals
	}
	return nil
}

func wiredVerdictEngine(opts Options) *verdict.Engine {
	verify := func(id verdict.Identity, phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		switch {
		case opts.VerifyFn != nil:
			return opts.VerifyFn(phase, roots)
		case opts.ContractVerifier != nil:
			if v := opts.ContractVerifier(); v != nil {
				return v.VerifyForClassification(gatesignal.Check{Cycle: id.Cycle, RunID: id.RunID, Phase: id.Phase}, phase, roots)
			}
		}
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

func dispatchOf(req core.PhaseRequest, prep phasePreparation, plan phaseDispatchPlan, d phaseDispatchResult) verdict.Dispatch {
	return verdict.Dispatch{
		Cycle: req.Cycle, RunID: req.RunID, Phase: prep.phase, Workspace: req.Workspace, Worktree: req.Worktree,
		ProjectRoot: req.ProjectRoot, ExplanationDocumentationVersion: req.ExplanationDocumentationVersion,
		ArtifactPath: prep.artifactPath, PreDispatch: prep.preDispatch, HadPreDispatch: prep.hadPreDispatch,
		Bridge: d.bridgeResponse, BridgeErr: d.bridgeErr, DurationMS: d.durationMS,
		ResolvedModel: d.resolvedModel, ModelSource: plan.modelSource, FenceDiagnostics: d.fenceDiagnostics,
	}
}

func (b *BaseRunner) classifyWith(req core.PhaseRequest, d verdict.Dispatch, skills []string) verdict.Classify {
	owesSelfReview := req.WritesSource() && slices.Contains(skills, policy.SelfReviewSkill)
	return func(artifact string) (string, []core.Diagnostic, string) {
		v, diags, next := b.hooks.Classify(artifact, req, d.Bridge)
		if owesSelfReview && v != core.VerdictFAIL && !phasecontract.SelfReviewRecorded(artifact) {
			b.warnSelfReviewMissing(d)
		}
		return v, diags, next
	}
}

func (b *BaseRunner) SignalsWired() bool { return b.judge.SignalsWired() }

func (b *BaseRunner) ContractVerifierWired() bool {
	return !b.verifyInjected && b.contractVerifier != nil && b.contractVerifier() != nil
}

const (
	reconcileSettleRetries  = verdict.SettleRetries
	reconcileSettleInterval = verdict.SettleInterval
)
