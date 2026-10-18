package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var loopProcStartFn = proctree.StartOf

const (
	CodeLoopSystemFailureHalt   signalcenter.Code = "LOOP_SYSTEM_FAILURE_HALT"
	CodeLoopPipelineBlockerHalt signalcenter.Code = "LOOP_PIPELINE_BLOCKER_HALT"
	CodeLoopFleetLaneHalt       signalcenter.Code = "LOOP_FLEET_LANE_HALT"
	CodeLoopHalt                signalcenter.Code = "LOOP_HALT"
	CodeLoopEscalationBoundary  signalcenter.Code = "LOOP_ESCALATION_BOUNDARY"
	CodeLoopMinWidthRepair      signalcenter.Code = loopwave.CodeMinWidthRepair
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopSystemFailureHalt, "the batch halted on an ADR-0072 system failure the cycle itself signalled (the pipeline, not the task, is the cause); fields.category names the floor, fields.next is the escalation dossier's next_action, fields.escalation and fields.inbox_item the dossier and the P0 item the halt wrote")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopPipelineBlockerHalt, "the pipeline-blocker breaker halted the batch (identical fingerprints, unexplained failures or consecutive failures over the ceiling, or consecutive fleet lanes deferred for want of a worktree: rule lane-deferrals, category lane-provisioning, the git cause in the reason); fields.rule and fields.fingerprint name the rule, the rest are the system-failure halt's own fields (next, escalation, inbox_item)")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopFleetLaneHalt, "a fleet lane exited with the system-failure halt code; the lane's own LOOP_SYSTEM_FAILURE_HALT names the failure and the escalation it filed")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopHalt, "the batch halted at a boundary (plane diverged, sync refused, main CI red, disk below the floor, or a boundary CLI update whose smoke test failed: fields.families and fields.versions name it); the reason is the halt error")
	signalcenter.RegisterCode(signalcenter.ModuleLoop, CodeLoopEscalationBoundary, "the escalation boundary staged inbox items (bumped/filed/planned) or refused a staged autofile intent that names no priority_class after a cycle; fields carry the counts (refused among them, each refused id also printed as a WARN line) and the stage")
}

type loopHaltRule struct {
	code   signalcenter.Code
	fields map[string]string
}

var systemFailureRule = loopHaltRule{code: CodeLoopSystemFailureHalt}

func emitLoopHalt(signals *signalcenter.Center, cycle int, origin string, code signalcenter.Code, reason string, fields map[string]string) {
	signals.Emit(signalcenter.Event{
		Cycle: cycle, Module: signalcenter.ModuleLoop, Origin: origin, Kind: signalcenter.KindLoopHalt,
		Severity: signalcenter.SeverityIncident, Code: code, Reason: reason, Fields: fields,
	})
}

type loopAnnouncer func(signals *signalcenter.Center, stderr io.Writer) func(exit *int)

func silentLoop(*signalcenter.Center, io.Writer) func(*int) {
	return func(*int) {}
}

func (c loopConfig) announcer() loopAnnouncer {
	if c.announce == nil {
		return announceLoop
	}
	return c.announce
}

func (c loopConfig) retiring(closeSignals func()) loopConfig {
	outer := c.retire
	c.retire = func() {
		closeSignals()
		if outer != nil {
			outer()
		}
	}
	return c
}

func (c loopConfig) beforeReExec(flush func()) func() {
	if c.retire == nil {
		return flush
	}
	return c.retire
}

func announceLoop(signals *signalcenter.Center, stderr io.Writer) func(exit *int) {
	pid := os.Getpid()
	procStart, err := loopProcStartFn(pid)
	if err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: no loop.started event: the start time of pid %d is unreadable (%v), so a watcher cannot tell that this loop died\n", pid, err)
		return func(*int) {}
	}
	signals.Emit(signalcenter.Event{
		Module: signalcenter.ModuleLoop, Origin: "announceLoop", Kind: signalcenter.KindLoopStarted,
		Severity: signalcenter.SeverityInfo, Reason: "the loop process started", Fields: map[string]string{"proc_start": procStart},
	})
	return func(exit *int) {
		if crash := recover(); crash != nil {
			panic(crash)
		}
		signals.Emit(signalcenter.Event{
			Module: signalcenter.ModuleLoop, Origin: "announceLoop", Kind: signalcenter.KindLoopExit,
			Severity: signalcenter.SeverityInfo, Reason: "the loop process exits with code " + strconv.Itoa(*exit),
			Fields: map[string]string{"exit": strconv.Itoa(*exit)},
		})
	}
}

func emitLoopWave(signals *signalcenter.Center, wave int, origin string, code signalcenter.Code, reason string, fields map[string]string) {
	loopwave.EmitWave(signals, wave, origin, code, reason, fields)
}

func emitLoopEscalation(signals *signalcenter.Center, cycle int, origin, reason string, fields map[string]string) {
	signals.Emit(signalcenter.Event{
		Cycle: cycle, Module: signalcenter.ModuleLoop, Origin: origin, Kind: signalcenter.KindLoopEscalation,
		Severity: signalcenter.SeverityWarn, Code: CodeLoopEscalationBoundary, Reason: reason, Fields: fields,
	})
}

func formatSignalReport(cycle int, s signalcenter.Summary) string {
	if s.Total == 0 {
		return ""
	}
	line := fmt.Sprintf("[loop] cycle %d signals: %d total, %d WARN, %d INCIDENT", cycle, s.Total,
		s.BySeverity[signalcenter.SeverityWarn], s.BySeverity[signalcenter.SeverityIncident])
	if s.LastIncident != nil {
		line += fmt.Sprintf(" (last INCIDENT %s — %s)", s.LastIncident.Code, s.LastIncident.Reason)
	}
	if p, r, c := s.ByKind[signalcenter.KindGatePassed], s.ByKind[signalcenter.KindGateRejected], s.ByKind[signalcenter.KindGateCorrected]; p+r+c > 0 {
		line += fmt.Sprintf(" · gates: %d passed, %d rejected, %d corrected", p, r, c)
	}
	return line + "\n"
}
