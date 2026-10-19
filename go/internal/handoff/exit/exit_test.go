package exit_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/handoff"
	"github.com/mickeyyaya/evolve-loop/go/internal/handoff/exit"
)

var request = handoff.Request{
	Pair: "build-audit",
	Asks: []handoff.Ask{
		{ID: "a1", Required: true},
		{ID: "a2", Required: true},
		{ID: "a3", Required: true},
		{ID: "o1", Required: false},
	},
}

func met(ask string) exit.Result {
	return exit.Result{Ask: ask, Status: handoff.StatusMet, Resolved: true}
}

func unmet(ask string) exit.Result {
	return exit.Result{Ask: ask, Status: handoff.StatusUnmet, Resolved: true}
}

func round(asks ...string) exit.Round {
	return exit.Round{Met: asks}
}

func TestDecide(t *testing.T) {
	policy := exit.Policy{MaxRedispatch: 2}
	tests := []struct {
		name    string
		req     handoff.Request
		results []exit.Result
		history []exit.Round
		policy  exit.Policy
		want    exit.Outcome
		unmet   []string
	}{
		{"every required ask met passes", request, []exit.Result{met("a1"), met("a2"), met("a3"), unmet("o1")}, nil, policy, exit.Pass, nil},
		{"optional unmet ask does not block pass", request, []exit.Result{met("a1"), met("a2"), met("a3")}, nil, policy, exit.Pass, nil},
		{"request without asks passes", handoff.Request{Pair: "build-audit"}, nil, nil, policy, exit.Pass, nil},
		{"request with optional asks only passes", handoff.Request{Asks: []handoff.Ask{{ID: "o1"}}}, []exit.Result{unmet("o1")}, nil, policy, exit.Pass, nil},
		{"pass wins when the redispatch cap is spent", request, []exit.Result{met("a1"), met("a2"), met("a3")}, []exit.Round{round("a1"), round("a1", "a2")}, policy, exit.Pass, nil},
		{"pass wins when the round gained nothing", request, []exit.Result{met("a1"), met("a2"), met("a3")}, []exit.Round{round("a1", "a2", "a3")}, policy, exit.Pass, nil},
		{"met ask with unresolved evidence is unmet", request, []exit.Result{met("a1"), met("a2"), {Ask: "a3", Status: handoff.StatusMet}}, nil, policy, exit.Retry, []string{"a3"}},
		{"blocked ask is unmet", request, []exit.Result{met("a1"), met("a2"), {Ask: "a3", Status: handoff.StatusBlocked, Resolved: true}}, nil, policy, exit.Retry, []string{"a3"}},
		{"not applicable required ask is unmet", request, []exit.Result{met("a1"), met("a2"), {Ask: "a3", Status: handoff.StatusNotApplicable, Resolved: true}}, nil, policy, exit.Retry, []string{"a3"}},
		{"required ask without a result is unmet", request, []exit.Result{met("a1"), met("a2")}, nil, policy, exit.Retry, []string{"a3"}},
		{"conflicting results for one ask are unmet", request, []exit.Result{met("a1"), met("a2"), met("a3"), unmet("a3")}, nil, policy, exit.Retry, []string{"a3"}},
		{"retry carries only the unmet required ids in request order", request, []exit.Result{unmet("a3"), met("a2"), unmet("a1"), unmet("o1")}, nil, policy, exit.Retry, []string{"a1", "a3"}},
		{"first round that meets nothing retries", request, []exit.Result{unmet("a1"), unmet("a2"), unmet("a3")}, nil, policy, exit.Retry, []string{"a1", "a2", "a3"}},
		{"round with a new met ask retries", request, []exit.Result{met("a1"), met("a2"), unmet("a3")}, []exit.Round{round("a1")}, policy, exit.Retry, []string{"a3"}},
		{"new optional ask counts as gain", request, []exit.Result{met("a1"), unmet("a2"), met("o1")}, []exit.Round{round("a1")}, policy, exit.Retry, []string{"a2", "a3"}},
		{"gain is measured against every earlier round", request, []exit.Result{met("a2"), unmet("a1"), unmet("a3")}, []exit.Round{round("a1"), round("a2")}, exit.Policy{MaxRedispatch: 5}, exit.Cap, nil},
		{"round that meets no new ask caps before the round limit", request, []exit.Result{met("a1"), unmet("a2"), unmet("a3")}, []exit.Round{round("a1")}, exit.Policy{MaxRedispatch: 5}, exit.Cap, nil},
		{"round that meets nothing at all caps after a first round", request, []exit.Result{unmet("a1"), unmet("a2"), unmet("a3")}, []exit.Round{round()}, exit.Policy{MaxRedispatch: 5}, exit.Cap, nil},
		{"result for an ask outside the request is no gain", request, []exit.Result{met("a1"), unmet("a2"), unmet("a3"), met("ghost")}, []exit.Round{round("a1")}, exit.Policy{MaxRedispatch: 5}, exit.Cap, nil},
		{"met ask with unresolved evidence is no gain", request, []exit.Result{met("a1"), {Ask: "a2", Status: handoff.StatusMet}, unmet("a3")}, []exit.Round{round("a1")}, exit.Policy{MaxRedispatch: 5}, exit.Cap, nil},
		{"spent redispatch cap caps although the round gained", request, []exit.Result{met("a1"), met("a2"), unmet("a3")}, []exit.Round{round("a1"), round("a1")}, policy, exit.Cap, nil},
		{"unspent redispatch cap retries", request, []exit.Result{met("a1"), met("a2"), unmet("a3")}, []exit.Round{round("a1")}, policy, exit.Retry, []string{"a3"}},
		{"zero redispatch cap caps the first failed round", request, []exit.Result{met("a1"), unmet("a2"), unmet("a3")}, nil, exit.Policy{}, exit.Cap, nil},
		{"negative redispatch cap caps the first failed round", request, []exit.Result{met("a1"), unmet("a2"), unmet("a3")}, nil, exit.Policy{MaxRedispatch: -1}, exit.Cap, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := exit.Decide(tc.req, tc.results, tc.history, tc.policy)
			if got.Outcome != tc.want {
				t.Fatalf("Outcome = %q, want %q (unmet %v)", got.Outcome, tc.want, got.Unmet)
			}
			if tc.want == exit.Cap {
				return
			}
			if len(got.Unmet) != len(tc.unmet) || (len(tc.unmet) > 0 && !reflect.DeepEqual(got.Unmet, tc.unmet)) {
				t.Errorf("Unmet = %v, want %v", got.Unmet, tc.unmet)
			}
		})
	}
}

func TestDecide_DoesNotMutateItsInputs(t *testing.T) {
	results := []exit.Result{unmet("a3"), met("a2"), unmet("a1")}
	history := []exit.Round{round("a2")}
	wantResults := append([]exit.Result(nil), results...)
	wantHistory := []exit.Round{round("a2")}
	exit.Decide(request, results, history, exit.Policy{MaxRedispatch: 3})
	if !reflect.DeepEqual(results, wantResults) {
		t.Errorf("results mutated: %v", results)
	}
	if !reflect.DeepEqual(history, wantHistory) {
		t.Errorf("history mutated: %v", history)
	}
}

func TestDecide_IsDeterministic(t *testing.T) {
	results := []exit.Result{unmet("a3"), met("a2"), unmet("a1")}
	first := exit.Decide(request, results, nil, exit.Policy{MaxRedispatch: 3})
	for i := 0; i < 20; i++ {
		if got := exit.Decide(request, results, nil, exit.Policy{MaxRedispatch: 3}); !reflect.DeepEqual(got, first) {
			t.Fatalf("run %d = %v, want %v", i, got, first)
		}
	}
}

func TestOutcomeWords_MatchTheSpecAndTheHandoffExits(t *testing.T) {
	if string(exit.Pass) != string(handoff.ExitPass) {
		t.Errorf("Pass = %q, want %q", exit.Pass, handoff.ExitPass)
	}
	if string(exit.Cap) != string(handoff.ExitCap) {
		t.Errorf("Cap = %q, want %q", exit.Cap, handoff.ExitCap)
	}
	if exit.Retry != "retry" {
		t.Errorf("Retry = %q, want retry", exit.Retry)
	}
}
