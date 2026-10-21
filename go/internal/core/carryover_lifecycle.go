package core

import (
	"context"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func (o *Orchestrator) carryover() *carryover.Lifecycle {
	if o.carry == nil {
		o.carry = o.wiredCarryover()
	}
	return o.carry
}

func (o *Orchestrator) wiredCarryover() *carryover.Lifecycle {
	return carryover.New(carryover.WithSignals(func() *signalcenter.Center { return o.signals }))
}

func nullCarryover() *carryover.Lifecycle { return carryover.New() }

func (o *Orchestrator) appendCarryoverTodoDeduped(state *State, todo CarryoverTodo) {
	o.carryover().Append(state, todo)
}

func (o *Orchestrator) writeFailureLearningState(ctx context.Context, state *State) {
	o.carryover().Persist(ctx, o.storage, state)
}

func RetireCarryoverTodos(todos []CarryoverTodo, committedIDs []string) []CarryoverTodo {
	return carryover.Retire(todos, committedIDs)
}

// Deprecated: test/ACS facade on the unwired lifecycle — production goes
// through (*Orchestrator).carryover(); TestCarryoverLifecycle_OneConstructionSite
// rejects a non-test caller.
func ApplyDefectsAsCarryoverTodos(state *State, record FailedRecord) {
	nullCarryover().ApplyDefects(state, record)
}

// Deprecated: test/ACS facade on the unwired lifecycle — production goes
// through (*Orchestrator).carryover().Closeout; the construction-site guard
// rejects a non-test caller.
func MergeWorkspaceCarryover(state *State, workspacePath string, cycle int, now time.Time) {
	nullCarryover().MergeMemo(state, workspacePath, cycle, now)
}

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
	carryoverPriorityBlocking      = carryover.PriorityBlocking
	maxFailureLearningSummaryChars = carryover.MaxSummaryRunes
)

func truncateRunes(s string, max int) string { return carryover.TruncateRunes(s, max) }

func (o *Orchestrator) closeoutCarryover(state *State, workspace, projectRoot string, cycle int, shipped bool, now time.Time) {
	if shipped {
		deferrals := o.resolvedDeferrals(workspace, func(id string) string { return o.itemPathFor(projectRoot, id) })
		o.carryover().MergeAcceptanceDeferrals(state, deferrals, cycle, now)
	}
	o.carryover().Closeout(state, workspace, cycle, now)
}
