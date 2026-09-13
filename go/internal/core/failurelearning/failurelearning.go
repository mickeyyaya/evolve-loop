// Package failurelearning is unit 03b of the component breakdown (ADR-0103):
// the deterministic half of learning from a failed phase. One Engine owns the
// failed-approach recorder (the FailedRecord and the P0 todo minted into the
// State through the unit-03 Lifecycle), the deterministic floor (the
// retrospective report, the lesson YAML and the inbox remediation items
// faillearn renders when no LLM retrospective can), and the recurrence closure.
// The orchestration around it — the retro runner, the observer, the cycle-state
// writes, the ledger, the checkpoint, the disposition gate and the persist —
// stays in core (unit 05's RunCycle engine). The Engine holds a clock, the
// Lifecycle and the Signal Center accessor; it never persists, never writes
// stderr, and reports its four failure modes as failurelearning.warning under
// module failurelearning. Design: docs/architecture/decomposition/03b-failure-learning-engine.md.
package failurelearning

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// The unit's codes — the four WARN conditions that replaced seven hand-written
// stderr lines — registered with their reasons.
const (
	CodePolicyLoadFailed       signalcenter.Code = "FAILURELEARNING_POLICY_LOAD_FAILED"
	CodeFloorWriteFailed       signalcenter.Code = "FAILURELEARNING_FLOOR_WRITE_FAILED"
	CodeRemediationTruncated   signalcenter.Code = "FAILURELEARNING_REMEDIATION_TRUNCATED"
	CodeRecurrenceLedgerFailed signalcenter.Code = "FAILURELEARNING_RECURRENCE_LEDGER_FAILED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleFailureLearning, CodePolicyLoadFailed, ".evolve/policy.json exists but could not be read or parsed while writing the deterministic floor (absence is silent); the novelty threshold and the remediation weight fall back to the compiled defaults and the floor proceeds — ONE read where two reads of one file fired two lines before; fields.step=floor, path")
	signalcenter.RegisterCode(signalcenter.ModuleFailureLearning, CodeFloorWriteFailed, "the deterministic floor could not write retrospective-report.md, the lesson YAML or the inbox remediation (a faillearn error, including the id-collision refusal); the FailedRecord and the P0 todo already stand in memory, the recurrence closure still runs, the phase failure is never masked; fields.step=floor, lessons_dir, workspace")
	signalcenter.RegisterCode(signalcenter.ModuleFailureLearning, CodeRemediationTruncated, "a failed phase self-reported more defects than the remediation cap; the first ones are filed as inbox items and the rest dropped — fix the emitter or raise the cap; fields.step=floor, reported, filed")
	signalcenter.RegisterCode(signalcenter.ModuleFailureLearning, CodeRecurrenceLedgerFailed, "the recurrence ledger keyed by the failing pattern could not be loaded, updated or saved (fields.op = load | record | save); Count() stays stale for this pattern, the phase failure is never masked; fields.step=recurrence, path")
}

// Failure is the ONE input shape: what failed. RecordFailedApproach reads
// Cycle, Phase, Err and Workspace; WriteFloor reads Cycle, Phase, ProjectRoot
// and Workspace. Core projects it from its request ONCE (failureOf).
type Failure struct {
	Cycle       int
	Phase       cyclestate.Phase
	Err         error
	ProjectRoot string
	Workspace   string
}

// Learned is what RecordFailedApproach DERIVED and WriteFloor consumes —
// outputs, never inputs: the summary is computed from Err, the block adopted
// from the failed phase's own report.
type Learned struct {
	Summary    string
	TodoID     string
	Structured *phasecontract.FailureBlock
}

// Engine owns the deterministic half of failure learning. Its collaborators
// are explicit at construction: the clock as a closure (tests swap the
// orchestrator's clock after construction), the unit-03 Lifecycle the P0 todo
// is minted through (so its admission events keep flowing), the Center through
// an accessor read at every use. No store (it never persists), no context.
type Engine struct {
	now     func() time.Time
	todos   *carryover.Lifecycle
	signals func() *signalcenter.Center
}

// Option configures an Engine at construction (functional options).
type Option func(*Engine)

// New builds the engine over its two required collaborators; a nil clock or
// lifecycle is a programming error and panics at first use — no guard.
func New(now func() time.Time, todos *carryover.Lifecycle, opts ...Option) *Engine {
	e := &Engine{now: now, todos: todos}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// WithSignals installs the accessor of the Signal Center the unit reports
// through — read at every use, because the orchestrator's Center is itself an
// option applied after construction. A nil accessor, or one returning nil, is
// the Null Object; SignalsWired proves the production roots wired one.
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

// warn is the unit's one producer: a failurelearning.warning WARN under module
// failurelearning, stamped with the failed cycle and phase. A nil Center is the
// Null Object (Emit on nil is a no-op).
func (e *Engine) warn(f Failure, code signalcenter.Code, reason string, fields map[string]string) {
	e.center().Emit(signalcenter.Event{
		Cycle: f.Cycle, Phase: string(f.Phase), Module: signalcenter.ModuleFailureLearning, Origin: "Engine.WriteFloor",
		Kind: signalcenter.KindFailureLearningWarning, Severity: signalcenter.SeverityWarn, Code: code, Reason: reason, Fields: fields,
	})
}
