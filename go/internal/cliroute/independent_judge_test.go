package cliroute_test

import (
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute/cliroutetest"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func seat(t *testing.T, f operatorFixture, agent string) cliroute.Decision {
	t.Helper()
	return mustResolve(t, f.router, cliroute.Request{Agent: agent, Phase: cliroutetest.PhaseOf(agent), DefaultModel: "balanced"})
}

func requireNoFamily(t *testing.T, judge cliroute.Decision, err error, family string) {
	t.Helper()
	if err != nil {
		t.Fatalf("IndependentOf(%s): %v", family, err)
	}
	if len(judge.Plan.Candidates) == 0 {
		t.Fatal("the T1 audit seat has no candidate")
	}
	for _, fam := range families(judge.Plan.Candidates) {
		if fam == family {
			t.Fatalf("the T1 audit seat %v reaches the family %s of the builder", judge.Plan.Candidates, family)
		}
	}
}

func TestT1AuditSeatIsAnotherFamily(t *testing.T) {
	t.Run("the operator table", func(t *testing.T) {
		f := newOperatorFixture(t)
		builder := llmroute.Family(seat(t, f, "builder").Plan.Candidates[0])
		judge, err := seat(t, f, "auditor").IndependentOf(builder)
		requireNoFamily(t, judge, err, builder)
	})
	t.Run("a seat that starts on the builder family", func(t *testing.T) {
		auditor := cliroute.Decision{Plan: llmroute.Plan{Candidates: []string{"claude", "agy", "claude-sonnet"}, Triggers: []int{1}}, Rule: "agents:auditor"}
		judge, err := auditor.IndependentOf("claude")
		requireNoFamily(t, judge, err, "claude")
		if judge.Plan.Candidates[0] != "agy" || judge.Rule != "agents:auditor" || len(judge.Plan.Triggers) != 1 {
			t.Errorf("judge = %+v, want the agy candidate with the rule and the triggers kept", judge)
		}
		if auditor.Plan.Candidates[0] != "claude" {
			t.Errorf("IndependentOf changed the input decision: %v", auditor.Plan.Candidates)
		}
	})
}

func TestIndependentOf_RefusesASeatWithOnlyTheBuilderFamily(t *testing.T) {
	f := newOperatorFixture(t)
	auditor := seat(t, f, "auditor")
	only := llmroute.Family(auditor.Plan.Candidates[0])

	_, err := auditor.IndependentOf(only)

	if !errors.Is(err, cliroute.ErrNoIndependentFamily) {
		t.Fatalf("err = %v, want ErrNoIndependentFamily for a seat that has only the family %s", err, only)
	}
}
