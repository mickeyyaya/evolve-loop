package recovery

import "fmt"

type Action string

const (
	ActionExtend    Action = "extend"
	ActionKillRetry Action = "kill_retry"
	ActionEscalate  Action = "escalate"
	ActionAdvise    Action = "advise"
)

type RecoverInput struct {
	Kind        string
	Cause       TerminalCause
	Busy        bool
	Attempts    int
	MaxAttempts int
	Integrity   bool
}

type Decision struct {
	Action  Action
	Handler string
	Reason  string
}

type chainHandler struct {
	name  string
	match func(in RecoverInput) (Decision, bool)
}

var chain = []chainHandler{
	{
		name: "integrity-escalate",
		match: func(in RecoverInput) (Decision, bool) {
			if in.Integrity {
				return Decision{Action: ActionEscalate, Reason: "integrity-adjacent state — never auto-recovered (ADR-0044 locked decision)"}, true
			}
			return Decision{}, false
		},
	},
	{
		name: "process-dead-kill",
		match: func(in RecoverInput) (Decision, bool) {
			if in.Kind == "process_dead" {
				return Decision{Action: ActionKillRetry, Reason: "agent process group is gone — no output can ever arrive; fast-fail so the fallback chain owns the fresh dispatch"}, true
			}
			return Decision{}, false
		},
	},
	{
		name: "busy-extend",
		match: func(in RecoverInput) (Decision, bool) {
			if in.Busy {
				return Decision{Action: ActionExtend, Reason: "agent visibly mid-turn — never kill a working agent (stop-review prime directive)"}, true
			}
			return Decision{}, false
		},
	},
	{
		name: "known-fatal-kill",
		match: func(in RecoverInput) (Decision, bool) {
			if _, known := validCauses[in.Cause]; known {
				return Decision{Action: ActionKillRetry, Reason: fmt.Sprintf("deterministic registry classified the pane as %s — fast-fail; the fallback chain owns the fresh dispatch", in.Cause)}, true
			}
			return Decision{}, false
		},
	},
	{
		name: "stall-budget-extend",
		match: func(in RecoverInput) (Decision, bool) {
			if (in.Kind == "stuck_no_output" || in.Kind == "stuck_no_progress") && in.Attempts < in.MaxAttempts {
				return Decision{Action: ActionExtend, Reason: fmt.Sprintf("unclassified stall within budget (%d/%d extensions) — the agent may be deep in thought", in.Attempts, in.MaxAttempts)}, true
			}
			return Decision{}, false
		},
	},
	{
		name: "unknown-advise",
		match: func(in RecoverInput) (Decision, bool) {
			return Decision{Action: ActionAdvise, Reason: "no deterministic classification — escalate to the LLM failure-advisor; its verdict will be promoted into the registry"}, true
		},
	},
}

func Recover(in RecoverInput) Decision {
	for _, h := range chain {
		if d, matched := h.match(in); matched {
			d.Handler = h.name
			return d
		}
	}
	return Decision{Action: ActionEscalate, Handler: "unreachable", Reason: "chain fell through"}
}

type chainStallPolicy struct {
	maxExtends int
}

func NewChainStallPolicy(maxExtends int) StallPolicy {
	if maxExtends <= 0 {
		maxExtends = 6
	}
	return chainStallPolicy{maxExtends: maxExtends}
}

func (p chainStallPolicy) Decide(ev StallEvent) (StallAction, string) {
	attempts := 0
	if ev.ThresholdS > 0 {
		attempts = ev.IdleS / ev.ThresholdS
	}
	d := Recover(RecoverInput{
		Kind:        ev.Kind,
		Cause:       CauseUnknown,
		Attempts:    attempts,
		MaxAttempts: p.maxExtends,
	})
	switch d.Action {
	case ActionExtend:
		return StallExtend, d.Reason
	case ActionKillRetry:
		return StallKillRetry, d.Reason
	default:
		return StallEscalate, d.Reason
	}
}
