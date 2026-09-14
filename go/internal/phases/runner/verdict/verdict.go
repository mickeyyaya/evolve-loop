// Package verdict is unit 11 of the component breakdown (ADR-0103): the phase
// runner's verdict engine — the fifth and sixth step of BaseRunner.Run's
// template. One Engine turns (what the bridge did, what preparation
// snapshotted, the deliverable probe) into the core.PhaseResponse core's
// dispatch loops route on: the bounded settle ladder, the teardown reconcile
// arms (stale-leftover refusal → well-formed → optional degrade → ACS
// deterministic floor → forensic FAIL), the substantive-error FAIL, the
// verdict-source rule (the contracted file is the sole verdict source, the
// pane only for an uncontracted phase), the clean-stdout companion, the ship
// guard and the violation trail. Prompt preparation, routing, the dispatch
// chain and the worktree fence stay in the host (unit 11b). The Engine holds
// the probe, a sleep, the stdout filter, the optional flag and the Signal
// Center accessor; it persists nothing, writes no stderr, reads no env, and
// reports its five decisions as runner.warning under module runner. Design:
// docs/architecture/decomposition/11-phaserunner.md.
package verdict

import (
	"context"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// The unit's codes — the five WARN decisions of the verdict engine, four of
// which replaced hand-written stderr lines — registered with their reasons.
const (
	CodeTeardownFail          signalcenter.Code = "RUNNER_TEARDOWN_FAIL"
	CodeOptionalPhaseDegraded signalcenter.Code = "RUNNER_OPTIONAL_PHASE_DEGRADED"
	CodeReconciled            signalcenter.Code = "RUNNER_RECONCILED"
	CodeDeliverableUnverified signalcenter.Code = "RUNNER_DELIVERABLE_UNVERIFIED"
	CodeStdoutFilterFailed    signalcenter.Code = "RUNNER_STDOUT_FILTER_FAILED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleRunner, CodeTeardownFail, "a bridge INFRA teardown (artifact-wait timeout or transient failure) ended the session and no trustworthy deliverable rescued it — a mandatory phase FAILs and core's retry loop classifies the wrapped sentinel; the reason is the FAIL diagnostic's own text; fields.teardown = timeout / transient, exit, cause = stale_leftover / unverifiable / malformed, codes, verr, roots (ws= wt= evolve=), report and acs (size=N tail=… or absent), stale_leftover, settle_attempts (re-probes: never flushed vs malformed), deliverable")
	signalcenter.RegisterCode(signalcenter.ModuleRunner, CodeOptionalPhaseDegraded, "an optional phase hit a bridge infra teardown with no trustworthy deliverable and degraded to WARN; the cycle continues — a stream and console ADDITION (the arm was a response diagnostic only); fields.teardown, exit, cause, verr, stale_leftover, settle_attempts, deliverable")
	signalcenter.RegisterCode(signalcenter.ModuleRunner, CodeReconciled, "a bridge infra teardown was overridden by a well-formed deliverable (fields.via=verify) or by the ACS deterministic floor (via=acs_floor, overridden_codes); the response carries Reconciled=true and core files the reconciled_timeout ledger disposition — ONE event where two INFO lines (ACS-FLOOR, RECONCILED) fired before; fields.verdict, deliverable, teardown, exit, settle_attempts")
	signalcenter.RegisterCode(signalcenter.ModuleRunner, CodeDeliverableUnverified, "a CONTRACTED deliverable was still not well-formed after the settle window on the clean-exit path AND the final verdict is FAIL — the ship guard downgraded a clean-ship verdict (fields.downgraded=true) or Classify itself returned FAIL on the malformed or absent bytes; fault-only, a legitimate WARN/SKIPPED pass-through emits nothing; fields.codes, verdict_before, verdict, settle_attempts, deliverable")
	signalcenter.RegisterCode(signalcenter.ModuleRunner, CodeStdoutFilterFailed, "the clean-stdout companion of the phase's raw log (the logfilter writer's <phase>-stdout.clean.txt) could not be written; the phase continues and the raw log stays the forensic source; fields.workspace (the phase is the event's own)")
}

// The settle ladder's bounds — a pure LIVENESS ceiling (≈ 3 s) for a
// contracted deliverable that has not finished flushing to disk, never the
// verdict-correctness mechanism (that is the file-authoritative rule): a
// clean-exit agent can return control before its `Write <phase>-report.md`
// lands (cycle-921, the ADR-0072 verdict incoherence). The host projects them
// under their old names for the settle tests it keeps.
const (
	SettleRetries  = 15
	SettleInterval = 200 * time.Millisecond
)

// Verify is the deliverable probe as the host resolves it (Options.VerifyFn or
// the catalog-aware default). Classify is the phase's own verdict hook, bound
// by the host PER CALL over its request and bridge response — the engine
// judges bytes it never reads itself and a verdict rule it never owns
// (Strategy), and never sees core.PhaseRequest or the host's Hooks.
type (
	Verify   func(phase string, roots phasecontract.Roots) (deliverable.Result, error)
	Classify func(artifact string) (verdict string, diags []core.Diagnostic, nextPhase string)
)

// Dispatch is the ONE input shape: the request fields the engine reads, what
// preparation snapshotted, and what the bridge did. The host projects it ONCE
// (a keyed literal); the leaf's own test constructs it POSITIONALLY so a new
// field fails to compile there instead of silently zeroing at the seam.
// Bridge is meaningful even when BridgeErr != nil: CostUSD/Tokens/BootMS/
// ExitCode are read on the error arms.
type Dispatch struct {
	Cycle                           int
	RunID                           string
	Phase                           string
	Workspace                       string
	Worktree                        string
	ProjectRoot                     string
	ExplanationDocumentationVersion int
	ArtifactPath                    string
	PreDispatch                     Snapshot
	HadPreDispatch                  bool
	Bridge                          core.BridgeResponse
	BridgeErr                       error
	DurationMS                      int64
	ResolvedModel                   string
	ModelSource                     string
	FenceDiagnostics                []core.Diagnostic
}

// Engine judges one dispatch outcome. The probe is required; the rest are
// options with the host's defaults. No clock, no store, no stderr.
type Engine struct {
	verify       Verify
	sleep        func(time.Duration)
	stdoutFilter func(workspace, phase string) error
	optional     bool
	signals      func() *signalcenter.Center
}

// Option configures an Engine at construction (functional options).
type Option func(*Engine)

// New builds the engine over its required probe; a nil probe is a programming
// error and panics at first use — no guard.
func New(verify Verify, opts ...Option) *Engine {
	e := &Engine{verify: verify, sleep: time.Sleep}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// WithSleep installs the settle ladder's clock (default time.Sleep); the host
// passes its resolved sleep seam so its tests count the intervals.
func WithSleep(fn func(time.Duration)) Option {
	return func(e *Engine) { e.sleep = fn }
}

// WithStdoutFilter installs the clean-stdout companion writer (the host's
// logfilter.Process by default); nil is the Null Object — no companion is
// written (the host omits it when the filter is disabled). Where the
// companion lives is the writer's belief; the engine never spells it.
func WithStdoutFilter(fn func(workspace, phase string) error) Option {
	return func(e *Engine) { e.stdoutFilter = fn }
}

// WithOptional marks the phase non-essential: a teardown with no trustworthy
// deliverable degrades to WARN and the cycle advances instead of aborting.
func WithOptional(optional bool) Option {
	return func(e *Engine) { e.optional = optional }
}

// WithSignals installs the accessor of the Signal Center the unit reports
// through — read at every use, never snapshotted. A nil accessor, or one
// returning nil, is the Null Object; SignalsWired proves a root wired one.
func WithSignals(c func() *signalcenter.Center) Option {
	return func(e *Engine) { e.signals = c }
}

// SignalsWired reports whether the engine currently reaches a Center.
func (e *Engine) SignalsWired() bool { return e.center() != nil }

func (e *Engine) center() *signalcenter.Center {
	if e.signals == nil {
		return nil
	}
	return e.signals()
}

// warn is the unit's one producer: a runner.warning WARN under module runner,
// stamped with the dispatch's cycle, phase and run id and the emitting method
// as origin. Empty field values are omitted so every code stays under the
// Center's field cap. A nil Center is the Null Object (Emit on nil is a no-op).
func (e *Engine) warn(origin string, d Dispatch, code signalcenter.Code, reason string, fields map[string]string) {
	for k, v := range fields {
		if v == "" {
			delete(fields, k)
		}
	}
	e.center().Emit(signalcenter.Event{
		Cycle: d.Cycle, RunID: d.RunID, Phase: d.Phase, Module: signalcenter.ModuleRunner, Origin: origin,
		Kind: signalcenter.KindRunnerWarning, Severity: signalcenter.SeverityWarn, Code: code, Reason: reason, Fields: fields,
	})
}

// Judge is the fifth and sixth step of the host's Run template: reconcile the
// bridge outcome against the deliverable, then classify. It returns exactly
// what the two stage functions returned before the extraction: a populated
// FAIL response AND fmt.Errorf("%s: bridge: %w", phase, bridgeErr) on the
// teardown-FAIL and substantive arms (the sentinel survives errors.Is for
// core's IsInfraTeardownError and bridgeExitCode), (WARN, nil) on the optional
// arm, (the classified response, nil) otherwise.
func (e *Engine) Judge(ctx context.Context, d Dispatch, classify Classify) (core.PhaseResponse, error) {
	r, early, err := e.reconcile(ctx, d)
	if early != nil {
		return *early, err
	}
	return e.classify(ctx, d, r, classify), nil
}
