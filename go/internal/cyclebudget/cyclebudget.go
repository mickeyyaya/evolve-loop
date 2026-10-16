// Package cyclebudget is the pure decision core for advisor-decided,
// completion-driven loop termination.
package cyclebudget

import "strings"

type Stage int

const (
	Off Stage = iota
	Advisory
	Enforce
)

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

type Decision struct {
	Stop     bool
	Advisory bool
	Reason   string
}

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
