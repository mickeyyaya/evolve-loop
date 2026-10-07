package cliroute_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func benchedFamilies(families ...string) func(string) bool {
	return func(cli string) bool {
		for _, f := range families {
			if llmroute.Family(cli) == f {
				return true
			}
		}
		return false
	}
}

func TestDecision_WalkAt_LeadsWithTheFirstUnbenchedPermittedCandidate(t *testing.T) {
	d := cliroute.Decision{Plan: llmroute.Plan{
		Candidates:  []string{"agy-tmux", "agy-claude-tmux", "claude-tmux"},
		Triggers:    []int{85},
		TierCeiling: map[string][]string{"deep": {"agy-claude", "claude"}},
	}}
	cases := []struct {
		name        string
		tier        string
		benched     []string
		want        []string
		wantHealthy bool
	}{
		{"the ceiling drops agy at deep", "deep", nil, []string{"agy-claude-tmux", "claude-tmux"}, true},
		{"an opus tier reads as deep", "opus", nil, []string{"agy-claude-tmux", "claude-tmux"}, true},
		{"a benched first CLI walks after the healthy lead", "deep", []string{"agy-claude"}, []string{"claude-tmux", "agy-claude-tmux"}, true},
		{"every permitted CLI benched keeps the order and reports it", "deep", []string{"agy-claude", "claude"}, []string{"agy-claude-tmux", "claude-tmux"}, false},
		{"no ceiling at balanced keeps agy first", "balanced", nil, []string{"agy-tmux", "agy-claude-tmux", "claude-tmux"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			walk, healthy, err := d.WalkAt(tc.tier, benchedFamilies(tc.benched...))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(walk.Candidates, tc.want) || healthy != tc.wantHealthy {
				t.Errorf("walk = %v healthy = %v, want %v %v", walk.Candidates, healthy, tc.want, tc.wantHealthy)
			}
			if !reflect.DeepEqual(walk.Triggers, []int{85}) {
				t.Errorf("triggers = %v, want the decision's", walk.Triggers)
			}
		})
	}
}

func TestDecision_WalkAt_NothingPermittedAtTheTierIsARefusal(t *testing.T) {
	d := cliroute.Decision{Plan: llmroute.Plan{
		Candidates:  []string{"agy-tmux"},
		TierCeiling: map[string][]string{"deep": {"claude"}},
	}}

	_, _, err := d.WalkAt("deep", benchedFamilies())

	if !errors.Is(err, cliroute.ErrRefused) {
		t.Fatalf("err = %v, want a refusal: the ceiling leaves the advisor no CLI at its tier", err)
	}
}
