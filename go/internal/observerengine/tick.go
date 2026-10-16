package observerengine

import (
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

const (
	originStart       = "Engine.Start"
	originTick        = "Engine.Tick"
	originShutdown    = "Engine.Shutdown"
	originWriteReport = "Engine.WriteReport"
)

func (e *Engine) Start() {
	e.emit(originStart, "observer_started", "INFO", map[string]any{
		"scope":   e.s.Scope,
		"enforce": e.s.Enforce,
		"stall_s": e.s.StallS,
		"poll_s":  e.s.PollS,
	})
}

type Stop int

const (
	StopNone Stop = iota
	StopEOFGrace
)

func (e *Engine) Tick() Stop {
	e.pollCounter++
	e.probeProcess()
	e.ingest()
	e.runStallRules()
	e.heartbeat()
	if e.eofReached() {
		return StopEOFGrace
	}
	return StopNone
}

func (e *Engine) Shutdown(reason string) {
	e.emit(originShutdown, "observer_shutdown", "INFO", map[string]any{"reason": reason})
}

func (e *Engine) probeProcess() {
	if e.d.ProcessAlive == nil || e.processDeadFired || e.s.PGID <= 0 || e.d.ProcessAlive(e.s.PGID) {
		return
	}
	e.processDeadFired = true
	e.respond(incident{
		kind:    "process_dead",
		payload: map[string]any{"pgid": e.s.PGID},
		event:   recovery.StallEvent{Kind: "process_dead", Phase: e.s.Phase},
	})
}

func (e *Engine) ingest() {
	lines, offset := e.tail()
	if len(lines) == 0 {
		e.eofQuietCount++
		return
	}
	e.eofQuietCount = 0
	e.lastByteOff = offset
	for _, line := range lines {
		e.processLine(line)
	}
}

func (e *Engine) scopeRunsRules() bool {
	return e.s.Scope == ScopeCycle || e.s.Scope == ScopePhase
}

func (e *Engine) runStallRules() {
	if !e.scopeRunsRules() {
		return
	}
	stall := e.d.Now().Sub(e.lastEventTS).Seconds()
	e.idleRules(int(stall))
	e.noProgressBackstop()
}

func (e *Engine) idleRules(idle int) {
	if e.s.NudgeS > 0 && idle >= e.s.NudgeS && !e.nudged {
		e.nudged = true
		if err := e.d.Nudge(e.s.NudgeBody); err != nil {
			e.reportFault(originTick, CodeNudgeAppendFailed, err.Error(), map[string]string{
				"step": "nudge", "idle_s": strconv.Itoa(idle), "threshold_s": strconv.Itoa(e.s.NudgeS), "agent": e.s.Agent,
			})
		}
		e.emit(originTick, "soft_stall_nudge", "WARN", map[string]any{"idle_s": idle, "threshold_s": e.s.NudgeS})
	}
	if idle >= e.s.StallS {
		e.respond(incident{
			kind:    "stuck_no_output",
			payload: map[string]any{"idle_s": idle, "threshold_s": e.s.StallS},
			event:   recovery.StallEvent{Kind: "stuck_no_output", Phase: e.s.Phase, IdleS: idle, ThresholdS: e.s.StallS},
		})
	}
}

func (e *Engine) noProgressBackstop() {
	if e.s.MaxNoProgressS <= 0 {
		return
	}
	noProgress := int(e.d.Now().Sub(e.lastProgressTS).Seconds())
	if noProgress < e.s.MaxNoProgressS {
		return
	}
	e.respond(incident{
		kind: "stuck_no_progress",
		payload: map[string]any{
			"no_progress_s": noProgress, "threshold_s": e.s.MaxNoProgressS,
			"tool_calls": e.toolCallCount, "tool_results": e.toolResultCnt,
		},
		event: recovery.StallEvent{
			Kind: "stuck_no_progress", Phase: e.s.Phase, IdleS: noProgress, ThresholdS: e.s.MaxNoProgressS,
			ToolCalls: e.toolCallCount, ToolResults: e.toolResultCnt,
		},
	})
}

func (e *Engine) heartbeat() {
	if e.s.HeartbeatEvery <= 0 || e.pollCounter%e.s.HeartbeatEvery != 0 {
		return
	}
	e.emit(originTick, "heartbeat", "INFO", map[string]any{
		"event_count":     e.eventCount,
		"tool_call_count": e.toolCallCount,
		"error_count":     e.errorCount,
		"cumulative_cost": e.cumulativeCost,
	})
}

func (e *Engine) eofReached() bool {
	return e.eofQuietCount*e.s.PollS >= e.s.EOFGraceS && e.eventCount > 0
}
