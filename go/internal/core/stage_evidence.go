package core

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

const (
	StageKeyRetryLadder                      = "workflow.retry_ladder.stage"
	CodeStageEvidenceFault signalcenter.Code = "ORCHESTRATOR_STAGE_EVIDENCE_FAULT"

	stageExitCompleted = "completed"
	stageExitFailed    = "failed"
	stageExitParked    = "parked"
	stageExitDeadline  = "deadline"
	stageExitUnended   = "unended"
	stageSealBudget    = 30 * time.Second
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodeStageEvidenceFault, "the shadow-stage evidence of a scope could not be sealed or written, so the scope reads as incomplete and its decisions do not count; fields.cycle names the cycle and fields.error the cause (shadow-contract K2)")
}

type StageRecorder interface {
	Record(key string, fields map[string]string)
	Close()
}

type StageSealer interface {
	SealStageEvidence(ctx context.Context, scope stageevidence.ScopeKey, in stageevidence.SealInput) (stageevidence.Seal, error)
}

type StageEvidence struct {
	Open   func(cycle int) StageRecorder
	Sealer StageSealer
}

func WithStageEvidence(e StageEvidence) Option {
	return func(o *Orchestrator) {
		if e.Open != nil && e.Sealer != nil {
			o.stageEvidence = e
		}
	}
}

func (o *Orchestrator) StageEvidenceWired() bool { return o.stageEvidence.Open != nil }

type stageSession struct {
	o     *Orchestrator
	cycle int
	rec   StageRecorder
}

func (o *Orchestrator) beginStageSession(cycle int) *stageSession {
	if o.stageEvidence.Open == nil {
		return nil
	}
	s := &stageSession{o: o, cycle: cycle, rec: o.stageEvidence.Open(cycle)}
	o.stageSessions.Store(cycle, s)
	return s
}

func (o *Orchestrator) recordStage(cycle int, key string, fields map[string]string) {
	if s, live := o.stageSessions.Load(cycle); live {
		s.(*stageSession).rec.Record(key, fields)
	}
}

func (s *stageSession) finish(ctx context.Context, retErr error, completed bool, verdict string) {
	if s == nil {
		return
	}
	s.o.stageSessions.CompareAndDelete(s.cycle, s)
	s.rec.Close()
	exit := stageExitOf(ctx, retErr, completed)
	sealCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stageSealBudget)
	defer cancel()
	in := stageevidence.SealInput{Shipped: exit == stageExitCompleted && IsShippingVerdict(verdict), Exit: exit}
	if _, err := s.o.stageEvidence.Sealer.SealStageEvidence(sealCtx, stageevidence.CycleScope(s.cycle), in); err != nil && !errors.Is(err, stageevidence.ErrEmptyChain) {
		s.o.emitStageFault(s.cycle, err)
	}
}

func (o *Orchestrator) releaseSeal(seal func()) {
	defer seal()
	o.releaseShipWindow()
}

func (c *cycleRun) releaseRun(cleanup func(preserveWorktree, completedNormally bool), retErr error) {
	defer func() { c.stage.finish(c.ctx, retErr, c.cycleCompletedNormally, c.result.FinalVerdict) }()
	cleanup(c.preserveWorktree, c.cycleCompletedNormally)
}

func stageExitOf(ctx context.Context, retErr error, completed bool) string {
	switch {
	case isParked(retErr):
		return stageExitParked
	case completed && retErr == nil:
		return stageExitCompleted
	case ctx.Err() != nil:
		return stageExitDeadline
	case retErr != nil:
		return stageExitFailed
	}
	return stageExitUnended
}

func StageFaultEvent(cycle int, origin, action string, err error) signalcenter.Event {
	return signalcenter.Event{
		Cycle: cycle, Module: signalcenter.ModuleOrchestrator, Origin: origin,
		Kind: signalcenter.KindSystemFailure, Severity: signalcenter.SeverityWarn, Code: CodeStageEvidenceFault,
		Reason: "stage evidence of cycle " + strconv.Itoa(cycle) + " could not be " + action + ": " + err.Error(),
		Fields: map[string]string{"cycle": strconv.Itoa(cycle), "error": err.Error()},
	}
}

func (o *Orchestrator) emitStageFault(cycle int, err error) {
	ev := StageFaultEvent(cycle, "stageSession.finish", "sealed", err)
	ev.RunID = o.signalRunID()
	o.signals.Emit(ev)
}
