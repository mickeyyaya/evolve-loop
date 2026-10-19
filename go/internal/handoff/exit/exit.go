package exit

import "github.com/mickeyyaya/evolve-loop/go/internal/handoff"

type Outcome string

const (
	Pass  Outcome = Outcome(handoff.ExitPass)
	Retry Outcome = "retry"
	Cap   Outcome = Outcome(handoff.ExitCap)
)

type Policy struct {
	MaxRedispatch int
}

type Result struct {
	Ask      string
	Status   handoff.Status
	Resolved bool
}

type Round struct {
	Met []string
}

type Decision struct {
	Outcome Outcome
	Unmet   []string
}

func Decide(req handoff.Request, results []Result, history []Round, policy Policy) Decision {
	met := metAsks(results)
	unmet := unmetRequired(req, met)
	if len(unmet) == 0 {
		return Decision{Outcome: Pass}
	}
	if len(history) >= policy.MaxRedispatch {
		return Decision{Outcome: Cap}
	}
	if len(history) > 0 && !hasGain(req, met, history) {
		return Decision{Outcome: Cap}
	}
	return Decision{Outcome: Retry, Unmet: unmet}
}

func metAsks(results []Result) map[string]bool {
	met := map[string]bool{}
	for _, r := range results {
		ok := r.Status == handoff.StatusMet && r.Resolved
		prior, seen := met[r.Ask]
		met[r.Ask] = ok && (!seen || prior)
	}
	return met
}

func unmetRequired(req handoff.Request, met map[string]bool) []string {
	var unmet []string
	for _, ask := range req.Asks {
		if ask.Required && !met[ask.ID] {
			unmet = append(unmet, ask.ID)
		}
	}
	return unmet
}

func hasGain(req handoff.Request, met map[string]bool, history []Round) bool {
	earlier := map[string]bool{}
	for _, round := range history {
		for _, id := range round.Met {
			earlier[id] = true
		}
	}
	for _, ask := range req.Asks {
		if met[ask.ID] && !earlier[ask.ID] {
			return true
		}
	}
	return false
}
