// Package failurelearning is unit 03b of the component breakdown (ADR-0103):
// the deterministic half of learning from a failed phase.
package failurelearning

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

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

type Failure struct {
	Cycle       int
	Phase       cyclestate.Phase
	Err         error
	ProjectRoot string
	Workspace   string
}

type Learned struct {
	Summary    string
	TodoID     string
	Structured *phasecontract.FailureBlock
}

type Engine struct {
	now     func() time.Time
	todos   *carryover.Lifecycle
	signals func() *signalcenter.Center
}

type Option func(*Engine)

func New(now func() time.Time, todos *carryover.Lifecycle, opts ...Option) *Engine {
	e := &Engine{now: now, todos: todos}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func WithSignals(c func() *signalcenter.Center) Option {
	return func(e *Engine) { e.signals = c }
}

func (e *Engine) SignalsWired() bool { return e.center() != nil }

func (e *Engine) center() *signalcenter.Center {
	if e.signals == nil {
		return nil
	}
	return e.signals()
}

func (e *Engine) warn(f Failure, code signalcenter.Code, reason string, fields map[string]string) {
	e.center().Emit(signalcenter.Event{
		Cycle: f.Cycle, Phase: string(f.Phase), Module: signalcenter.ModuleFailureLearning, Origin: "Engine.WriteFloor",
		Kind: signalcenter.KindFailureLearningWarning, Severity: signalcenter.SeverityWarn, Code: code, Reason: reason, Fields: fields,
	})
}
