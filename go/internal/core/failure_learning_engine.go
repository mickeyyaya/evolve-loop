package core

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurelearning"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// failureLearning returns the unit-03b engine (ADR-0103): NewOrchestrator
// builds it eagerly with the root's Center; an Orchestrator assembled as a
// literal — the remediation and floor-verdict tests build one — lazily builds
// and caches the same live engine on first use. No nil-orchestrator branch:
// every caller is an Orchestrator method.
func (o *Orchestrator) failureLearning() *failurelearning.Engine {
	if o.learn == nil {
		o.learn = o.wiredFailureLearning()
	}
	return o.learn
}

// wiredFailureLearning is the ONE construction of the engine
// (TestFailureLearningEngine_OneConstructionSite): the clock through a closure
// (tests swap o.now after construction), the orchestrator's ONE cached
// lifecycle (eager after o.carry, lazily built for a literal), the Center
// through an accessor (WithSignalCenter is an option applied after construction).
func (o *Orchestrator) wiredFailureLearning() *failurelearning.Engine {
	return failurelearning.New(func() time.Time { return o.now() }, o.carryover(),
		failurelearning.WithSignals(func() *signalcenter.Center { return o.signals }))
}

// failureOf is the ONE projection from the eleven-field request onto the
// engine's input: what failed, where.
func failureOf(fl failureLearningRequest) failurelearning.Failure {
	return failurelearning.Failure{Cycle: fl.Cycle, Phase: fl.Failed, Err: fl.Err,
		ProjectRoot: fl.CycleRequest.ProjectRoot, Workspace: fl.CycleState.WorkspacePath}
}

// recordFailedApproachState is the Strangler Fig facade the spine, the
// floor-verdict producer (recordFloorVerdictFailure) and the by-name tests
// keep: the engine records over the request's State and returns what it
// derived. Callers guarantee fl.State/CycleState are non-nil.
func (o *Orchestrator) recordFailedApproachState(fl failureLearningRequest) (summary, todoID string, structured *phasecontract.FailureBlock) {
	l := o.failureLearning().RecordFailedApproach(fl.State, failureOf(fl))
	return l.Summary, l.TodoID, l.Structured
}

// writeDeterministicLearning is the floor's facade (the three fallback tails
// and the by-name tests keep the spelling): the engine renders the artifacts
// and records the recurrence closure, best-effort.
func (o *Orchestrator) writeDeterministicLearning(fl failureLearningRequest, summary string, structured *phasecontract.FailureBlock) {
	o.failureLearning().WriteFloor(failureOf(fl), failurelearning.Learned{Summary: summary, Structured: structured})
}
