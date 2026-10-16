package observerengine

import (
	"fmt"
	"strconv"
	"syscall"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

type incident struct {
	kind    string
	payload map[string]any
	event   recovery.StallEvent
}

const (
	actionLegacyEnforce      = "legacy_enforce"
	actionKillRetrySkippedNo = "kill_retry_skipped_no_pgid"
)

func (e *Engine) decide(in incident) (action, reason string, kill bool) {
	if e.d.Policy == nil {
		return actionLegacyEnforce, "", e.s.Enforce && e.s.PGID > 0
	}
	verdict, reason := e.d.Policy.Decide(in.event)
	willKill := verdict == recovery.StallKillRetry && e.s.PGID > 0
	action = string(verdict)
	if verdict == recovery.StallKillRetry && !willKill {
		action = actionKillRetrySkippedNo
	}
	return action, reason, willKill
}

func (e *Engine) respond(in incident) {
	action, reason, kill := e.decide(in)
	if e.d.Policy != nil {
		in.payload["action"] = action
		in.payload["action_reason"] = reason
	}
	e.emit(originTick, in.kind, "INCIDENT", in.payload)
	if !kill {
		return
	}
	e.reportFault(originTick, CodeStallKillSent, e.killReason(in.kind, action, reason), e.killFields(in, action, reason))
	if err := e.d.Kill(e.s.PGID, syscall.SIGTERM); err != nil {
		e.reportFault(originTick, CodeKillFailed, err.Error(), map[string]string{
			"step": "respond", "pgid": strconv.Itoa(e.s.PGID), "signal": "SIGTERM", "kind": in.kind,
		})
	}
}

func (e *Engine) killReason(kind, action, reason string) string {
	if action == actionLegacyEnforce {
		return fmt.Sprintf("ENFORCE: killing pgid %d due to %s", e.s.PGID, kind)
	}
	return fmt.Sprintf("stall-policy: killing pgid %d due to %s (%s)", e.s.PGID, kind, reason)
}

func (e *Engine) killFields(in incident, action, reason string) map[string]string {
	fields := map[string]string{
		"step": "respond", "kind": in.kind, "pgid": strconv.Itoa(e.s.PGID), "action": action, "action_reason": reason,
	}
	if in.event.ThresholdS > 0 {
		fields["idle_s"] = strconv.Itoa(in.event.IdleS)
		fields["threshold_s"] = strconv.Itoa(in.event.ThresholdS)
	}
	return fields
}
