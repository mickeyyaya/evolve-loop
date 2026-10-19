package exit

import (
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
)

type Outcome string

const (
	Pass  Outcome = Outcome(handoff.ExitPass)
	Retry Outcome = "retry"
	Cap   Outcome = Outcome(handoff.ExitCap)

	Blocked Outcome = Outcome(handoff.ExitBlocked)
)

type Claim struct {
	Defect  string
	Path    string
	Failing string
	Guard   string
}

type Proof struct {
	Blocked bool
	Proven  []Claim
}

type Prover func(defects []string) Proof

type Policy struct {
	MaxRedispatch int
	Prover        Prover
}

type Result struct {
	Ask      string
	Status   handoff.Status
	Resolved bool
	Claim    string
}

type Round struct {
	Met []string
}

type Decision struct {
	Outcome Outcome
	Unmet   []string
	Blocked []string
	Paths   []string
}

func Decide(req handoff.Request, results []Result, history []Round, policy Policy) Decision {
	met := metAsks(results)
	unmet := unmetRequired(req, met)
	if len(unmet) == 0 {
		return Decision{Outcome: Pass}
	}
	if blocked, ok := proveBlocked(req, results, unmet, policy); ok {
		return blocked
	}
	if len(history) >= policy.MaxRedispatch {
		return Decision{Outcome: Cap}
	}
	if len(history) > 0 && !hasGain(req, met, history) {
		return Decision{Outcome: Cap}
	}
	return Decision{Outcome: Retry, Unmet: unmet}
}

func proveBlocked(req handoff.Request, results []Result, unmet []string, policy Policy) (Decision, bool) {
	if policy.Prover == nil {
		return Decision{}, false
	}
	var defects []string
	for _, r := range results {
		if r.Status == handoff.StatusBlocked && r.Claim != "" {
			defects = append(defects, r.Claim)
		}
	}
	if len(defects) == 0 {
		return Decision{}, false
	}
	proof := policy.Prover(defects)
	if !proof.Blocked {
		return Decision{}, false
	}
	named := map[string]bool{}
	paths := map[string]bool{}
	for _, claim := range proof.Proven {
		matched := false
		for _, r := range results {
			if r.Status == handoff.StatusBlocked && r.Claim == claim.Defect {
				named[r.Ask] = true
				matched = true
			}
		}
		if matched {
			paths[claim.Path] = true
			named[claim.Failing] = true
			named[claim.Guard] = true
		}
	}
	ids := askIDsIn(req, named)
	for _, id := range unmet {
		if !named[id] {
			return Decision{}, false
		}
	}
	return Decision{Outcome: Blocked, Blocked: ids, Paths: sortedKeys(paths)}, true
}

func askIDsIn(req handoff.Request, set map[string]bool) []string {
	var ids []string
	for _, ask := range req.Asks {
		if set[ask.ID] {
			ids = append(ids, ask.ID)
		}
	}
	return ids
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
