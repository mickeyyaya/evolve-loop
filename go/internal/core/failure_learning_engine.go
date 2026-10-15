package core

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/failurelearning"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func (o *Orchestrator) failureLearning() *failurelearning.Engine {
	if o.learn == nil {
		o.learn = o.wiredFailureLearning()
	}
	return o.learn
}

func (o *Orchestrator) wiredFailureLearning() *failurelearning.Engine {
	return failurelearning.New(func() time.Time { return o.now() }, o.carryover(),
		failurelearning.WithSignals(func() *signalcenter.Center { return o.signals }))
}

func failureOf(fl failureLearningRequest) failurelearning.Failure {
	return failurelearning.Failure{Cycle: fl.Cycle, Phase: fl.Failed, Err: fl.Err,
		ProjectRoot: fl.CycleRequest.ProjectRoot, Workspace: fl.CycleState.WorkspacePath}
}

func (o *Orchestrator) recordFailedApproachState(fl failureLearningRequest) (summary, todoID string, structured *phasecontract.FailureBlock) {
	l := o.failureLearning().RecordFailedApproach(fl.State, failureOf(fl))
	return l.Summary, l.TodoID, l.Structured
}

func (o *Orchestrator) writeDeterministicLearning(fl failureLearningRequest, summary string, structured *phasecontract.FailureBlock) {
	o.failureLearning().WriteFloor(failureOf(fl), failurelearning.Learned{Summary: summary, Structured: structured})
}
