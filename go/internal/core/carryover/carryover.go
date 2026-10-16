// Package carryover owns the lifecycle of the carryoverTodos array in state.json.
// See docs/architecture/packages/internal-core-carryover.md.
package carryover

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeWorkspaceReadFailed signalcenter.Code = "CARRYOVER_WORKSPACE_READ_FAILED"
	CodeWorkspaceMalformed  signalcenter.Code = "CARRYOVER_WORKSPACE_MALFORMED"
	CodePersistFailed       signalcenter.Code = "CARRYOVER_PERSIST_FAILED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleCarryover, CodeWorkspaceReadFailed, "a cycle-workspace carryover document (carryover-todos.json or defect-ledger.json) exists but could not be read (not absence: permissions, a directory at the path); nothing is merged, the closeout proceeds; fields name the document and path")
	signalcenter.RegisterCode(signalcenter.ModuleCarryover, CodeWorkspaceMalformed, "a cycle-workspace carryover document is not its documented JSON shape; the file is skipped whole, never aborting the cycle — a persona defect for the memo or audit owner; the reason carries the decode error")
	signalcenter.RegisterCode(signalcenter.ModuleCarryover, CodePersistFailed, "state.json could not take this run's failure-learning arrays (fields.step: write = the legacy whole-state write, update = the serialized read-modify-write); the in-memory state stands and the caller proceeds — the FailedRecord and the P0 todo may be lost from disk")
}

const (
	PriorityBlocking     = "P0"
	PriorityLesson       = "P1"
	PriorityPrescription = "high"
	PriorityMemoDefault  = "medium"
	PrescriptionPrefix   = defectledger.PrescriptionPrefix
	MaxActionRunes       = 500
	MaxSummaryRunes      = 500
)

type Lifecycle struct {
	signals func() *signalcenter.Center
}

type Option func(*Lifecycle)

func WithSignals(c func() *signalcenter.Center) Option {
	return func(l *Lifecycle) {
		if c != nil {
			l.signals = c
		}
	}
}

func New(opts ...Option) *Lifecycle {
	l := &Lifecycle{}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

func (l *Lifecycle) SignalsWired() bool { return l.center() != nil }

func (l *Lifecycle) center() *signalcenter.Center {
	if l.signals == nil {
		return nil
	}
	return l.signals()
}

func (l *Lifecycle) warn(origin string, cycle int, code signalcenter.Code, reason string, fields map[string]string) {
	l.center().Emit(signalcenter.Event{
		Cycle: cycle, Module: signalcenter.ModuleCarryover, Origin: origin, Kind: signalcenter.KindCarryoverWarning,
		Severity: signalcenter.SeverityWarn, Code: code, Reason: reason, Fields: fields,
	})
}
