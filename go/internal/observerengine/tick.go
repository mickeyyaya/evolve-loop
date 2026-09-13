package observerengine

import (
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

// The origins the engine stamps: the exported entry point whose call produced
// the report (fields.step names the step inside it).
const (
	originStart       = "Engine.Start"
	originTick        = "Engine.Tick"
	originShutdown    = "Engine.Shutdown"
	originWriteReport = "Engine.WriteReport"
)

// Start emits the observer_started envelope — the lifecycle's first emit.
func (e *Engine) Start() {
	e.emit(originStart, "observer_started", "INFO", map[string]any{
		"scope":   e.s.Scope,
		"enforce": e.s.Enforce,
		"stall_s": e.s.StallS,
		"poll_s":  e.s.PollS,
	})
}

// Stop is what a Tick asks of the host's loop.
type Stop int

// StopNone keeps polling; StopEOFGrace means the stdout log stopped growing
// for the grace period after at least one event — the host emits the
// eof_grace shutdown and leaves its loop.
const (
	StopNone Stop = iota
	StopEOFGrace
)

// Tick is one poll in the fixed order the original tick body had: the
// process probe (before the tail, so a dead agent's residual lines cannot
// refresh the liveness clocks), the ingest, the stall rules, the heartbeat,
// the EOF check. No ticker, no sleep: the host's loop owns time.
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

// Shutdown emits the observer_shutdown envelope with the host's reason
// (sigusr1 | stop-timer | eof_grace).
func (e *Engine) Shutdown(reason string) {
	e.emit(originShutdown, "observer_shutdown", "INFO", map[string]any{"reason": reason})
}

// probeProcess fires the process_dead INCIDENT exactly once when the probe
// says the agent's group is gone; nil probe = off.
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

// ingest tails the stdout log and feeds each new line to the decoder; a quiet
// tick counts toward the EOF grace and leaves the offset where it was.
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

// scopeRunsRules is the preserved check: only the two known scopes run rules.
func (e *Engine) scopeRunsRules() bool {
	return e.s.Scope == ScopeCycle || e.s.Scope == ScopePhase
}

// runStallRules reads the idle clock ONCE, then the nudge-and-hard-stall
// pair, then the no-progress backstop (its own clock read).
func (e *Engine) runStallRules() {
	if !e.scopeRunsRules() {
		return
	}
	stall := e.d.Now().Sub(e.lastEventTS).Seconds()
	e.idleRules(int(stall))
	e.noProgressBackstop()
}

// idleRules is the soft-stall nudge (once, opt-in) THEN the hard stall.
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

// noProgressBackstop is WS-E1's babbling-agent rule: the progress clock only
// resets on tool_use / tool_result, so an agent streaming text forever trips
// it; opt-in via MaxNoProgressS > 0.
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

// heartbeat emits the counters every HeartbeatEvery polls (0 = never).
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

// eofReached is the pure EOF-grace predicate: the log stopped growing for the
// grace period AND at least one event was seen.
func (e *Engine) eofReached() bool {
	return e.eofQuietCount*e.s.PollS >= e.s.EOFGraceS && e.eventCount > 0
}
