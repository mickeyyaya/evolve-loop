package core

import (
	"fmt"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeCycleSpendLimit signalcenter.Code = cyclespend.SignalCode

	spendLimitReason = "spend_limit"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleCycle, CodeCycleSpendLimit, "the spend of the cycle passed a limit of workflow.cycle_spend at a phase boundary (ADR-0158 decisions 14 to 16); fields.decision is would_stop (shadow, once for each cycle, the flow of today runs), over (enforce, the first boundary over the limit where no stop is legal) or stop (enforce, at the failed audit where the cycle goes to retro and seals handed-off; a passing audit still ships); fields.stage is shadow or enforce, fields.partial is true when the ledger did not load whole, fields.limit names the limit, fields.value and fields.max give the spend and the limit, and fields.phase the boundary")
}

func (cr *cycleRun) checkCycleSpend() {
	cfg := cr.o.workflowConfig.CycleSpend
	if cfg.Stage == config.StageOff || cr.spendLimitStopped || cr.current == PhaseRetro || cr.current == PhaseShip {
		return
	}
	spend, err := cyclespend.Read(cr.cs.WorkspacePath)
	if err != nil {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN cycle spend: %v; the limits read the part that loaded\n", err)
	}
	spend.AuditRounds = cr.cs.AuditDispatches
	onOver := cyclespend.ActionWouldStop
	if cfg.Stage == config.StageEnforce {
		onOver = cyclespend.ActionStop
	}
	d := cyclespend.Judge(spend, cfg.Limits, onOver)
	switch {
	case d.Action == cyclespend.ActionNone:
	case d.Action == cyclespend.ActionStop && cr.lastVerdict == VerdictFAIL && cr.o.sm.CanTransition(cr.current, PhaseRetro):
		cr.spendLimitStopped, cr.scheduledNext = true, PhaseRetro
		cr.emitSpendLimit(cfg.Stage, d, err != nil)
		if cr.result.TerminationReason != CycleTerminationHandedOff {
			cr.sealHandedOff(&cr.result, cr.phaseTimings, repairHandoffReasonPrefix+spendLimitReason+": "+d.Summary()+"; salvage "+cr.cs.ActiveWorktree)
		}
	case !cr.spendLimitSeen:
		cr.spendLimitSeen = true
		if d.Action == cyclespend.ActionStop {
			d.Action = cyclespend.ActionOver
		}
		cr.emitSpendLimit(cfg.Stage, d, err != nil)
	}
}

func (cr *cycleRun) emitSpendLimit(stage config.Stage, d cyclespend.Decision, partial bool) {
	cr.o.signals.Emit(signalcenter.Event{
		Cycle: cr.cycle, RunID: cr.o.signalRunID(), Phase: string(cr.current),
		Module: signalcenter.ModuleCycle, Origin: "cycleRun.checkCycleSpend", Kind: signalcenter.KindPhaseOutcome,
		Severity: signalcenter.SeverityWarn, Code: CodeCycleSpendLimit,
		Reason: "cycle spend (" + stage.String() + "): " + d.Action + " on " + d.Summary(),
		Fields: map[string]string{
			"stage": stage.String(), cyclespend.FieldDecision: d.Action, "limit": d.Limit, "value": d.Value, "max": d.Max,
			"phase": string(cr.current), "partial": strconv.FormatBool(partial),
		},
	})
}
