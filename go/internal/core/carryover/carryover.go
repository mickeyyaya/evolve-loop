// Package carryover is unit 03 of the component breakdown (ADR-0103): the
// lifecycle of state.json's carryoverTodos[] — the memory the next cycle's
// planner reads. One Lifecycle owns mint admission (fingerprint twin →
// refresh, id twin → skip, else append), the cycle-closeout readers (the memo
// and prescription documents, the triage-dropped retirement) in their
// load-bearing order, the ship-time retirement rule and the state.json persist
// behind two ports at the point of use. Every stamp arrives computed (the
// producers own their TTLs); the unit holds only the Signal Center accessor
// and reports its failure modes as carryover.warning under module carryover.
// Design: docs/architecture/decomposition/03-carryover-lifecycle.md.
package carryover

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// The unit's codes (commit 1: the three WARN conditions that replaced six
// hand-written stderr lines), registered with their reasons.
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

// The producers' priority vocabulary in ONE table (it was spelled five ways
// in four files); the consumer is core's carryoverPriorityRank, pinned there.
const (
	PriorityBlocking     = "P0"     // a failure that blocked a cycle
	PriorityLesson       = "P1"     // a judgment phase's reasoned objection
	PriorityPrescription = "high"   // an unenforced audit prescription
	PriorityMemoDefault  = "medium" // a memo todo naming no priority
	// PrescriptionPrefix tags the defect-ledger rows an audit's WARN
	// prescriptions carry — the LEDGER's belief, projected from its home
	// (internal/core/defectledger, ADR-0103 unit 09).
	PrescriptionPrefix = defectledger.PrescriptionPrefix
	// MaxActionRunes caps a todo's action (the former maxAdoptedDefectRunes).
	MaxActionRunes = 500
	// MaxSummaryRunes caps a failure summary's message (the former maxFailureLearningSummaryChars).
	MaxSummaryRunes = 500
)

// Lifecycle owns the in-process lifecycle of state.json's carryover todos. It
// holds only the Signal Center accessor: no clock (every stamp arrives
// computed or with the caller's now), no store (Persist takes the port per
// call, as the old seam read the orchestrator's storage at call time).
type Lifecycle struct {
	signals func() *signalcenter.Center
}

// Option configures a Lifecycle at construction (functional options).
type Option func(*Lifecycle)

// WithSignals installs the accessor of the Signal Center the unit reports
// through — an accessor read at every use, because the orchestrator's Center
// is itself an option tests apply after construction. A nil accessor, or one
// returning nil, is the Null Object (tests and the exported test-only facades;
// the production roots always wire a Center, and SignalsWired proves it).
func WithSignals(c func() *signalcenter.Center) Option {
	return func(l *Lifecycle) {
		if c != nil {
			l.signals = c
		}
	}
}

// New builds a Lifecycle.
func New(opts ...Option) *Lifecycle {
	l := &Lifecycle{}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// SignalsWired reports whether a Center is reachable now — the root's wiring proof.
func (l *Lifecycle) SignalsWired() bool { return l.center() != nil }

func (l *Lifecycle) center() *signalcenter.Center {
	if l.signals == nil {
		return nil
	}
	return l.signals()
}

// warn is the unit's one producer: a carryover.warning WARN under module
// carryover. A nil Center is the Null Object (Emit on nil is a no-op).
func (l *Lifecycle) warn(origin string, cycle int, code signalcenter.Code, reason string, fields map[string]string) {
	l.center().Emit(signalcenter.Event{
		Cycle: cycle, Module: signalcenter.ModuleCarryover, Origin: origin, Kind: signalcenter.KindCarryoverWarning,
		Severity: signalcenter.SeverityWarn, Code: code, Reason: reason, Fields: fields,
	})
}
