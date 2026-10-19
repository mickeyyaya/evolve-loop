package exit_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/exit"
)

func TestExportsAreNamed(t *testing.T) {
	req := handoff.Request{Asks: []handoff.Ask{{ID: "a1", Required: true}}}
	results := []exit.Result{{Ask: "a1", Status: handoff.StatusUnmet, Resolved: true}}
	var decision exit.Decision = exit.Decide(req, results, []exit.Round{{Met: nil}}, exit.Policy{MaxRedispatch: 3})
	if decision.Outcome != exit.Cap {
		t.Fatalf("no-gain round = %q, want %q", decision.Outcome, exit.Cap)
	}
	var outcome exit.Outcome = exit.Decide(req, results, nil, exit.Policy{MaxRedispatch: 3}).Outcome
	if outcome != exit.Retry {
		t.Fatalf("first failed round = %q, want %q", outcome, exit.Retry)
	}
	met := []exit.Result{{Ask: "a1", Status: handoff.StatusMet, Resolved: true}}
	if got := exit.Decide(req, met, nil, exit.Policy{}).Outcome; got != exit.Pass {
		t.Fatalf("met round = %q, want %q", got, exit.Pass)
	}
}

func TestBlockedExportsAreNamed(t *testing.T) {
	req := handoff.Request{Asks: []handoff.Ask{{ID: "a1", Required: true}}}
	results := []exit.Result{{Ask: "a1", Status: handoff.StatusBlocked, Resolved: true, Claim: "BLOCKER protected-path x.go why"}}
	var prover exit.Prover = func(defects []string) exit.Proof {
		return exit.Proof{Blocked: true, Proven: []exit.Claim{{Defect: defects[0], Path: "x.go", Failing: "a1", Guard: "a1"}}}
	}
	var decision exit.Decision = exit.Decide(req, results, nil, exit.Policy{Prover: prover})
	if decision.Outcome != exit.Blocked {
		t.Fatalf("proven claim = %q, want %q", decision.Outcome, exit.Blocked)
	}
	if len(decision.Blocked) != 1 || decision.Blocked[0] != "a1" || len(decision.Paths) != 1 || decision.Paths[0] != "x.go" {
		t.Fatalf("decision = %+v, want a1 blocked on x.go", decision)
	}
}
