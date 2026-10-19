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
