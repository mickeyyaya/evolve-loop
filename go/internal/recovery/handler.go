package recovery

import "fmt"

// Action is the recovery chain's verdict.
type Action string

const (
	// ActionExtend keeps waiting; the agent is plausibly still working.
	ActionExtend Action = "extend"
	// ActionKillRetry terminates and re-dispatches fresh; the runner's fallback chain picks the CLI.
	ActionKillRetry Action = "kill_retry"
	// ActionEscalate surfaces to the operator and takes no automatic action.
	ActionEscalate Action = "escalate"
	// ActionAdvise hands the evidence to the LLM failure-advisor and promotes its verdict via PromoteAdvice.
	ActionAdvise Action = "advise"
)

// RecoverInput is the evidence one terminal state presents to the chain.
type RecoverInput struct {
	// Kind is "fatal_pane", "process_dead", "stuck_no_output" or "stuck_no_progress".
	Kind  string
	Cause TerminalCause
	// Busy reports whether the pane visibly shows the agent mid-turn.
	Busy bool
	// Attempts and MaxAttempts are the caller's stall extension budget.
	Attempts    int
	MaxAttempts int
	// Integrity marks an integrity-adjacent state, which is never auto-recovered.
	Integrity bool
}

// Decision is the chain's verdict, the handler that claimed the input, and a justification.
type Decision struct {
	Action  Action
	Handler string
	Reason  string
}

type chainHandler struct {
	name  string
	match func(in RecoverInput) (Decision, bool)
}

// chain is ordered: each handler outranks every handler below it. See ADR-0044.
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
		// Above busy-extend: a dead process can render busy-looking chrome forever.
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

// Recover returns the first claiming handler's decision, stamped with its name.
func Recover(in RecoverInput) Decision {
	for _, h := range chain {
		if d, matched := h.match(in); matched {
			d.Handler = h.name
			return d
		}
	}
	// Unreachable: the last handler always claims.
	return Decision{Action: ActionEscalate, Handler: "unreachable", Reason: "chain fell through"}
}

type chainStallPolicy struct {
	maxExtends int
}

// NewChainStallPolicy builds the chain-backed StallPolicy; maxExtends <= 0 means 6.
func NewChainStallPolicy(maxExtends int) StallPolicy {
	if maxExtends <= 0 {
		maxExtends = 6
	}
	return chainStallPolicy{maxExtends: maxExtends}
}

// Decide counts elapsed idle thresholds as attempts, because the observer calls it every tick.
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
	default: // advise degrades to escalate: the observer cannot dispatch an advisor
		return StallEscalate, d.Reason
	}
}
