// Package verdict is the phase runner's judge: it reconciles a bridge outcome
// against the contracted deliverable and classifies the phase's verdict.
// See docs/architecture/packages/internal-phases-runner-verdict.md.
package verdict

import (
	"context"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// The engine's Signal Center codes, one per WARN decision it reports.
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

// The settle ladder's bounds: a liveness ceiling (about 3 s) for a deliverable
// still flushing to disk, never the verdict-correctness mechanism.
const (
	SettleRetries  = 15
	SettleInterval = 200 * time.Millisecond
)

// Verify is the host's deliverable probe; Classify is the phase's verdict hook, bound per call.
type (
	Verify   func(id Identity, phase string, roots phasecontract.Roots) (deliverable.Result, error)
	Classify func(artifact string) (verdict string, diags []core.Diagnostic, nextPhase string)
)

// Dispatch is the engine's one input: the request fields it reads, the
// pre-dispatch snapshot and the bridge outcome, which is read even when BridgeErr is set.
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

// Engine judges one dispatch outcome. The probe is required; every other seam is an Option.
type Engine struct {
	verify       Verify
	sleep        func(time.Duration)
	stdoutFilter func(workspace, phase string) error
	optional     bool
	signals      func() *signalcenter.Center
}

// Option configures an Engine at construction.
type Option func(*Engine)

// New builds the engine over its required probe; a nil probe panics at first use.
func New(verify Verify, opts ...Option) *Engine {
	e := &Engine{verify: verify, sleep: time.Sleep}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// WithSleep installs the settle ladder's sleep; the default is time.Sleep.
func WithSleep(fn func(time.Duration)) Option {
	return func(e *Engine) { e.sleep = fn }
}

// WithStdoutFilter installs the clean-stdout companion writer; nil writes no companion.
func WithStdoutFilter(fn func(workspace, phase string) error) Option {
	return func(e *Engine) { e.stdoutFilter = fn }
}

// WithOptional marks the phase optional: a teardown with no trustworthy deliverable degrades to WARN.
func WithOptional(optional bool) Option {
	return func(e *Engine) { e.optional = optional }
}

// WithSignals installs the Signal Center accessor, read at every use; nil is the Null Object.
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

// warn is the engine's one producer. Empty field values are dropped so every
// code stays under the Center's field cap; a nil Center ignores the event.
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

// Judge reconciles the bridge outcome against the deliverable, then classifies.
// A FAIL arm also returns the bridge error wrapped, so errors.Is still finds the infra sentinel.
func (e *Engine) Judge(ctx context.Context, d Dispatch, classify Classify) (core.PhaseResponse, error) {
	r, early, err := e.reconcile(ctx, d)
	if early != nil {
		return *early, err
	}
	return e.classify(ctx, d, r, classify), nil
}

// Identity names the dispatch to the verifier, so its salvage reports carry the right cycle and run.
type Identity struct {
	Cycle int
	RunID string
	Phase string
}

func identityOf(d Dispatch) Identity {
	return Identity{Cycle: d.Cycle, RunID: d.RunID, Phase: d.Phase}
}
