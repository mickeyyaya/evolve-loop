package runner

// verdict_engine.go — unit 11 (ADR-0103, design decomposition/11-phaserunner.md):
// the runner's seam onto the verdict engine. Every old caller keeps its
// spelling: runner.New(Options{…}) at all fifteen production construction
// sites, the ten embedders, the swarmrunner Decorator and phaseregistrar never
// learn the unit exists; the three settle tests keep reading the bound under
// its old names; the engine's ONE construction (which is also the ONE
// resolution of its seams), the ONE projection onto its input and the Center
// derivation live here.

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/logfilter"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner/verdict"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// signalSource is the optional capability a core.Bridge may expose: the Center
// it was built with. The production Adapter satisfies it (Signals() beside
// SignalsWired()); fakeBridge and the registry-factory roots
// (bridge.NewDefault(root, nil)) do not. Declared where it is consumed (the
// core.PersonaProber / swarmrunner optional-interface idiom); unexported so the
// host's apicover row sees no new type.
type signalSource interface{ Signals() *signalcenter.Center }

// resolveSignals is the ONE derivation of the engine's Center accessor:
// Options.Signals (tests, the --simulate twin, any foreign root) wins; else
// the injected Bridge's when it is a signalSource; else nil (the Null Object).
// The bridge's accessor is a method value read live at every use.
func resolveSignals(opts Options) func() *signalcenter.Center {
	if opts.Signals != nil {
		return opts.Signals
	}
	if src, ok := opts.Bridge.(signalSource); ok {
		return src.Signals
	}
	return nil
}

// wiredVerdictEngine is the ONE construction (TestVerdictEngine_OneConstructionSite)
// and the ONE resolution of the engine's seams — Options to defaults, handed
// to the engine and kept nowhere else (the engine is their only home; New
// stores the result once, eagerly, and nothing builds one lazily). The probe:
// Options.VerifyFn or the catalog-aware default over Options.PhaseIO; the
// clock: Options.SleepFn or settleSleep; the stdout filter: Options.StdoutFilter
// or logfilter.Process, nil (the engine's Null Object) when disabled; the
// optional flag; the live Center accessor.
func wiredVerdictEngine(opts Options) *verdict.Engine {
	verify := opts.VerifyFn
	if verify == nil {
		// Catalog-aware so the reconcile check resolves user/minted phases
		// under the SAME policy as the host gate and the agent self-check —
		// a builtin-only default left an inserted phase's surviving artifact
		// unresolvable on timeout, synthesizing FAIL. Stage-threaded (3.10
		// Slice 1) so the rung also reaches the host gate's verdict at enforce;
		// opts.PhaseIO's zero value (StageOff) is byte-identical to the prior
		// VerifyCatalogAware default.
		stage := opts.PhaseIO
		verify = func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
			return deliverable.VerifyCatalogAwareStage(phase, roots, stage)
		}
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

// dispatchOf is the ONE projection from the host's three stage objects onto
// the engine's input — keyed (go vet's composites check rejects a positional
// literal of an imported struct; the leaf's own test constructs it
// positionally so a new field breaks the build there).
func dispatchOf(req core.PhaseRequest, prep phasePreparation, plan phaseDispatchPlan, d phaseDispatchResult) verdict.Dispatch {
	return verdict.Dispatch{
		Cycle: req.Cycle, RunID: req.RunID, Phase: prep.phase, Workspace: req.Workspace, Worktree: req.Worktree,
		ProjectRoot: req.ProjectRoot, ExplanationDocumentationVersion: req.ExplanationDocumentationVersion,
		ArtifactPath: prep.artifactPath, PreDispatch: prep.preDispatch, HadPreDispatch: prep.hadPreDispatch,
		Bridge: d.bridgeResponse, BridgeErr: d.bridgeErr, DurationMS: d.durationMS,
		ResolvedModel: d.resolvedModel, ModelSource: plan.modelSource, FenceDiagnostics: d.fenceDiagnostics,
	}
}

// classifyWith binds the phase hook over Run's request (WorktreeVerified
// already stamped) and the terminal bridge response — the Strategy the engine
// calls without importing the runner or seeing core.PhaseRequest.
func (b *BaseRunner) classifyWith(req core.PhaseRequest, bres core.BridgeResponse) verdict.Classify {
	return func(artifact string) (string, []core.Diagnostic, string) {
		return b.hooks.Classify(artifact, req, bres)
	}
}

// SignalsWired reports whether this runner's verdict engine reaches a Signal
// Center — the root-wiring proof (the swarmrunner Decorator forwards it).
func (b *BaseRunner) SignalsWired() bool { return b.judge.SignalsWired() }

// The settle bounds under their old names — Strangler projections for the
// settle tests that read them (the values are the engine's).
const (
	reconcileSettleRetries  = verdict.SettleRetries
	reconcileSettleInterval = verdict.SettleInterval
)
