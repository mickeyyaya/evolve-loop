package core

// carryover_lifecycle.go — unit 03 (ADR-0103, design decomposition/03-carryover-lifecycle.md):
// the orchestrator's seam onto the carryover unit. Every old caller keeps its
// spelling: the persist and mint facades are Orchestrator methods (so the
// lifecycle reaches the orchestrator's Signal Center), the exported
// package-level facades survive for ship, the ACS-named tests and the fifteen
// direct test call sites on the Null Object, and the unexported facades keep
// alloc.go's union, the adoption cap and the failure summary in place.

import (
	"context"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// carryover returns the unit-03 lifecycle: eager from NewOrchestrator, lazily
// built and cached for an Orchestrator assembled as a literal. No
// nil-orchestrator branch: every caller is an Orchestrator method whose
// receiver is already dereferenced before the unit is reached.
func (o *Orchestrator) carryover() *carryover.Lifecycle {
	if o.carry == nil {
		o.carry = o.wiredCarryover()
	}
	return o.carry
}

// wiredCarryover is the ONE wired construction (TestCarryoverLifecycle_OneConstructionSite):
// the Center is read live, because WithSignalCenter is an option tests apply
// after construction.
func (o *Orchestrator) wiredCarryover() *carryover.Lifecycle {
	return carryover.New(carryover.WithSignals(func() *signalcenter.Center { return o.signals }))
}

// nullCarryover is the Null-Object lifecycle the exported package-level
// facades run on: they have no orchestrator and, after unit 03, no production
// caller (tests and ACS predicates only) — a future production caller would
// lose the WARN silently, which the nil-root pin and the construction guard
// defend against.
func nullCarryover() *carryover.Lifecycle { return carryover.New() }

// appendCarryoverTodoDeduped admits one todo through the unit's ONE rule.
func (o *Orchestrator) appendCarryoverTodoDeduped(state *State, todo CarryoverTodo) {
	o.carryover().Append(state, todo)
}

// writeFailureLearningState persists this run's failure-learning arrays; the
// store is read at call time (tests swap o.storage after construction).
func (o *Orchestrator) writeFailureLearningState(ctx context.Context, state *State) {
	o.carryover().Persist(ctx, o.storage, state)
}

// RetireCarryoverTodos keeps its exported signature for the ship phase and the
// ACS-named tests: the todos a ship committed, and their fingerprint twins,
// removed; the input is never mutated.
func RetireCarryoverTodos(todos []CarryoverTodo, committedIDs []string) []CarryoverTodo {
	return carryover.Retire(todos, committedIDs)
}

// ApplyDefectsAsCarryoverTodos keeps its exported signature for the ACS
// predicates that call it (no production caller — the unwired D2 contract).
//
// Deprecated: test/ACS facade on the unwired lifecycle — production goes
// through (*Orchestrator).carryover(); TestCarryoverLifecycle_OneConstructionSite
// rejects a non-test caller.
func ApplyDefectsAsCarryoverTodos(state *State, record FailedRecord) {
	nullCarryover().ApplyDefects(state, record)
}

// MergeWorkspaceCarryover and MergeWorkspacePrescriptionCarryover keep their
// exported signatures for the tests and ACS predicates that call them; the
// production closeout goes through the wired lifecycle's Closeout.
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

func capRunes(s string, maxRunes int) string { return carryover.CapRunes(s, maxRunes) }

func failureLearningSummary(cycle int, failed Phase, err error) string {
	return carryover.Summary(cycle, failed, err)
}

const (
	// carryoverPriorityBlocking is the priority of a todo minted from a failure
	// that blocked a cycle — the unit's vocabulary, projected.
	carryoverPriorityBlocking      = carryover.PriorityBlocking
	maxFailureLearningSummaryChars = carryover.MaxSummaryRunes
)
