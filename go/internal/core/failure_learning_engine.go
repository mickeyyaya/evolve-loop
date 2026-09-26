package core

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurelearning"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// failureLearning lazily builds and caches the live engine for an
// Orchestrator assembled as a literal. No nil-orchestrator branch: every
// caller is an Orchestrator method.
func (o *Orchestrator) failureLearning() *failurelearning.Engine {
	if o.learn == nil {
		o.learn = o.wiredFailureLearning()
	}
	return o.learn
}

// wiredFailureLearning is the one construction of the engine: the clock
// through a closure and the Center through an accessor, since tests swap
// o.now and apply WithSignalCenter after construction.
func (o *Orchestrator) wiredFailureLearning() *failurelearning.Engine {
	return failurelearning.New(func() time.Time { return o.now() }, o.carryover(),
		failurelearning.WithSignals(func() *signalcenter.Center { return o.signals }))
}

func failureOf(fl failureLearningRequest) failurelearning.Failure {
	return failurelearning.Failure{Cycle: fl.Cycle, Phase: fl.Failed, Err: fl.Err,
		ProjectRoot: fl.CycleRequest.ProjectRoot, Workspace: fl.CycleState.WorkspacePath}
}

// recordFailedApproachState records over the request's State and returns
// what it derived. Callers guarantee fl.State/CycleState are non-nil.
func (o *Orchestrator) recordFailedApproachState(fl failureLearningRequest) (summary, todoID string, structured *phasecontract.FailureBlock) {
	l := o.failureLearning().RecordFailedApproach(fl.State, failureOf(fl))
	return l.Summary, l.TodoID, l.Structured
}

// writeDeterministicLearning renders the artifacts and records the recurrence
// closure, best-effort.
func (o *Orchestrator) writeDeterministicLearning(fl failureLearningRequest, summary string, structured *phasecontract.FailureBlock) {
	o.failureLearning().WriteFloor(failureOf(fl), failurelearning.Learned{Summary: summary, Structured: structured})
}
