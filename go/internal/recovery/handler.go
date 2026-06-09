package recovery

// handler.go — ADR-0044 C3: the recovery Chain of Responsibility, the single
// owner that turns a classified terminal state into a typed, justified
// recovery action. Modeled on internal/router/recovery.go (the repo's proven
// CoR shape: ordered handlers, first match wins, terminal catch-all).
//
// Order is LOAD-BEARING:
//
//	integrity-escalate   — an integrity breach never auto-recovers, period
//	busy-extend          — a visibly-working agent is never killed, even on
//	                       a fatal-looking pane (the stop-review prime
//	                       directive, cycle-254/255)
//	known-fatal-kill     — a typed cause from the deterministic registry
//	                       kills+retries immediately: zero LLM tokens, the
//	                       runner's exit-81 fallback chain owns the fresh
//	                       dispatch (cycle-262's rescue, minus the 20-min wait)
//	stall-budget-extend  — an unclassified stall within the extension budget
//	                       waits (the agent may be deep in thought)
//	unknown-advise       — the terminal catch-all: only the unknown residue
//	                       reaches the LLM failure-advisor tail (Rule 5),
//	                       whose verdict is then promoted so next time the
//	                       known-fatal-kill link catches it deterministically
//
// Pure decision logic: Recover never acts. The caller (observer policy,
// orchestrator hook) executes the action under its own stage gate.

import "fmt"

// Action is the typed recovery verdict vocabulary of the chain.
type Action string

const (
	// ActionExtend: keep waiting — the agent is plausibly still working.
	ActionExtend Action = "extend"
	// ActionKillRetry: terminate and re-dispatch FRESH (never a reused
	// REPL/context); the runner's fallback chain picks the next CLI.
	ActionKillRetry Action = "kill_retry"
	// ActionEscalate: surface to the operator; take no automatic action.
	ActionEscalate Action = "escalate"
	// ActionAdvise: hand the evidence to the LLM failure-advisor
	// (core.FailureAdvisor) and promote its verdict (recovery.PromoteAdvice).
	// Callers that cannot dispatch an advisor (the observer subprocess)
	// degrade this to escalate.
	ActionAdvise Action = "advise"
)

// RecoverInput is the evidence envelope one terminal state presents to the
// chain. String/int/bool-only (leaf discipline).
type RecoverInput struct {
	// Kind names the incident: "fatal_pane", "stuck_no_output",
	// "stuck_no_progress".
	Kind string
	// Cause is the deterministic registry's classification; CauseUnknown
	// (or zero) when nothing matched.
	Cause TerminalCause
	// Busy reports the per-CLI visible mid-turn affordance.
	Busy bool
	// Attempts / MaxAttempts carry the caller's extension budget (the
	// maxExtends backstop for stalls).
	Attempts    int
	MaxAttempts int
	// Integrity marks an integrity-adjacent state — never auto-recovered.
	Integrity bool
}

// Decision is the chain's verdict: the action, the handler that claimed the
// input (forensics), and a human-readable justification (every recovery
// decision is justified — ADR-0044).
type Decision struct {
	Action  Action
	Handler string
	Reason  string
}

// chainHandler is one link: it returns a Decision and whether it claimed the
// input (mirrors router.recoveryHandler).
type chainHandler struct {
	name  string
	match func(in RecoverInput) (Decision, bool)
}

// chain is the ordered Chain of Responsibility. See the file header for why
// the order is load-bearing.
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
		// Terminal catch-all: the unknown residue is the LLM tail's job.
		name: "unknown-advise",
		match: func(in RecoverInput) (Decision, bool) {
			return Decision{Action: ActionAdvise, Reason: "no deterministic classification — escalate to the LLM failure-advisor; its verdict will be promoted into the registry"}, true
		},
	},
}

// Recover walks the chain and returns the first claiming handler's decision,
// stamped with the handler name. Deterministic, side-effect-free.
func Recover(in RecoverInput) Decision {
	for _, h := range chain {
		if d, matched := h.match(in); matched {
			d.Handler = h.name
			return d
		}
	}
	// Unreachable: the terminal handler always matches.
	return Decision{Action: ActionEscalate, Handler: "unreachable", Reason: "chain fell through"}
}

// chainStallPolicy adapts the chain to the observer's StallPolicy port
// (C4 → C3 composition). The observer subprocess cannot dispatch an LLM
// advisor, so ActionAdvise degrades to StallEscalate (surface for the
// orchestrator/operator); ActionKillRetry maps directly.
type chainStallPolicy struct {
	maxExtends int
}

// NewChainStallPolicy builds the chain-backed stall policy with the given
// extension budget (typically the same maxExtends the stop-reviewer uses).
func NewChainStallPolicy(maxExtends int) StallPolicy {
	if maxExtends <= 0 {
		maxExtends = 6
	}
	return chainStallPolicy{maxExtends: maxExtends}
}

// Decide implements StallPolicy by translating the stall event into chain
// vocabulary. Attempts is derived from how many threshold-multiples have
// elapsed idle (the observer fires every tick, not once per onset).
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
	default: // escalate, advise — the observer surfaces; it never advises
		return StallEscalate, d.Reason
	}
}
