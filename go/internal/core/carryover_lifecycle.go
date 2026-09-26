package core

import (
	"context"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// carryover is built lazily for an Orchestrator assembled as a literal.
func (o *Orchestrator) carryover() *carryover.Lifecycle {
	if o.carry == nil {
		o.carry = o.wiredCarryover()
	}
	return o.carry
}

// wiredCarryover is the one wired construction. It reads the Center live,
// because tests apply WithSignalCenter after construction.
func (o *Orchestrator) wiredCarryover() *carryover.Lifecycle {
	return carryover.New(carryover.WithSignals(func() *signalcenter.Center { return o.signals }))
}

// nullCarryover backs the test-only exported facades; a production caller
// would drop the unit's WARNs silently.
func nullCarryover() *carryover.Lifecycle { return carryover.New() }

func (o *Orchestrator) appendCarryoverTodoDeduped(state *State, todo CarryoverTodo) {
	o.carryover().Append(state, todo)
}

// writeFailureLearningState reads o.storage at call time, because tests swap it after construction.
func (o *Orchestrator) writeFailureLearningState(ctx context.Context, state *State) {
	o.carryover().Persist(ctx, o.storage, state)
}

// RetireCarryoverTodos drops the committed todos and their fingerprint twins without mutating its input.
func RetireCarryoverTodos(todos []CarryoverTodo, committedIDs []string) []CarryoverTodo {
	return carryover.Retire(todos, committedIDs)
}

// ApplyDefectsAsCarryoverTodos mints one todo per defect in record.
//
// Deprecated: test/ACS facade on the unwired lifecycle — production goes
// through (*Orchestrator).carryover(); TestCarryoverLifecycle_OneConstructionSite
// rejects a non-test caller.
func ApplyDefectsAsCarryoverTodos(state *State, record FailedRecord) {
	nullCarryover().ApplyDefects(state, record)
}

// MergeWorkspaceCarryover merges the workspace memo's carryover todos into state.
//
// Deprecated: test/ACS facade on the unwired lifecycle — production goes
// through (*Orchestrator).carryover().Closeout; the construction-site guard
// rejects a non-test caller.
func MergeWorkspaceCarryover(state *State, workspacePath string, cycle int, now time.Time) {
	nullCarryover().MergeMemo(state, workspacePath, cycle, now)
}

// MergeWorkspacePrescriptionCarryover is the prescription half of the same facade.
//
// Deprecated: test/ACS facade on the unwired lifecycle — see MergeWorkspaceCarryover.
func MergeWorkspacePrescriptionCarryover(state *State, workspacePath string, cycle int, now time.Time) {
	nullCarryover().MergePrescriptions(state, workspacePath, cycle, now)
}

func mergeFailedRecords(disk, incoming []FailedRecord) []FailedRecord {
	return carryover.MergeFailedRecords(disk, incoming)
}

func mergeCarryoverTodos(disk, incoming []CarryoverTodo) []CarryoverTodo {
	return carryover.MergeTodos(disk, incoming)
}

func failureLearningSummary(cycle int, failed Phase, err error) string {
	return carryover.Summary(cycle, failed, err)
}

const (
	// carryoverPriorityBlocking is the priority of a todo minted from a cycle-blocking failure.
	carryoverPriorityBlocking      = carryover.PriorityBlocking
	maxFailureLearningSummaryChars = carryover.MaxSummaryRunes
)

func truncateRunes(s string, max int) string { return carryover.TruncateRunes(s, max) }
