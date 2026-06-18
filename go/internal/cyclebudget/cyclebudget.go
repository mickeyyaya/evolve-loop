// Package cyclebudget is the pure decision core for advisor-decided, completion-
// driven loop termination. Instead of the operator pre-guessing `--max-cycles`,
// the loop runs until the planned work (the backlog the scout/triage phases
// produced) is drained — or the advisor judges the request complete — bounded by
// a safety cap. This package holds ONLY the decision; the dispatcher supplies the
// inputs (cycles run, cap, remaining backlog, the advisor's goal-complete signal)
// and acts on the verdict. Pure + leaf (stdlib only) so it is trivially testable
// and importable by cmd/evolve and core alike.
package cyclebudget

import "strings"

// Stage is the rollout dial (EVOLVE_CYCLE_BUDGET), mirroring the project's other
// off→advisory→enforce axes so this lands default-off (no behavior change) and
// can soak in advisory before driving the loop.
type Stage int

const (
	// Off — byte-identical to today: the operator's --max-cycles for-loop governs
	// and the budget logic is a no-op.
	Off Stage = iota
	// Advisory — the decision is COMPUTED and reported (shadow-soak: log what it
	// WOULD do) but --max-cycles still governs; the loop is never stopped here.
	Advisory
	// Enforce — completion/cap drive the loop; --max-cycles becomes the safety
	// ceiling supplied as the cap.
	Enforce
)

// String renders the stage as its env-token.
func (s Stage) String() string {
	switch s {
	case Advisory:
		return "advisory"
	case Enforce:
		return "enforce"
	default:
		return "off"
	}
}

// ParseStage maps an EVOLVE_CYCLE_BUDGET value to a Stage; unknown/empty ⇒ Off
// (fail-safe to today's behavior).
func ParseStage(v string) Stage {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "advisory":
		return Advisory
	case "enforce":
		return Enforce
	default:
		return Off
	}
}

// Decision is the verdict for whether the batch should stop after a cycle.
// Stop drives the loop (Enforce only). Advisory is true when the stage is
// Advisory and the loop WOULD have stopped — the dispatcher logs Reason for the
// shadow-soak but keeps running. Reason is "goal_complete" or "cap".
type Decision struct {
	Stop     bool
	Advisory bool
	Reason   string
}

// Next decides whether the loop should stop after completing cyclesRun cycles.
// completion (the advisor judged the goal done, or the backlog is drained) takes
// precedence over the safety cap. In Off it is always a no-op; in Advisory it
// reports the would-stop reason without stopping; in Enforce it stops.
//
// cyclesRun is 1-based (Next is called after a cycle), so completion can fire
// only once at least the first planning cycle has run.
func Next(stage Stage, cyclesRun, cap, backlogRemaining int, advisorGoalComplete bool) Decision {
	if stage == Off {
		return Decision{}
	}
	reason := ""
	switch {
	case advisorGoalComplete || backlogRemaining <= 0:
		reason = "goal_complete"
	case cap > 0 && cyclesRun >= cap:
		reason = "cap"
	default:
		return Decision{}
	}
	if stage == Advisory {
		return Decision{Advisory: true, Reason: reason}
	}
	return Decision{Stop: true, Reason: reason}
}
