package router_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// countingProposer records each Current it sees; its proposal agrees with Current, so it never clamps.
type countingProposer struct{ seen []string }

func (c *countingProposer) Propose(in router.RouteInput) (*router.Proposal, error) {
	c.seen = append(c.seen, in.Current)
	return &router.Proposal{NextPhase: in.Current}, nil
}

func TestHybridCadence_ProposeGatedByPlanAndBranch(t *testing.T) {
	t.Parallel()
	// Only the plan's non-nil-ness gates shouldPropose.
	plan := &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "tdd", Run: true}, {Phase: "build", Run: true},
		{Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
	cfg := config.RoutingConfig{
		Stage:     config.StageAdvisory,
		Mode:      config.ModeDynamicLLM,
		Mandatory: []string{"scout", "build", "audit", "ship"},
	}
	cases := []struct {
		name     string
		current  string
		plan     *router.PhasePlan
		wantCall bool
	}{
		{"plan: non-branch scout → no propose", "scout", plan, false},
		{"plan: non-branch tdd → no propose", "tdd", plan, false},
		{"plan: branch build → propose", "build", plan, true},
		{"plan: branch audit → propose", "audit", plan, true},
		{"no plan: scout → propose (legacy cadence)", "scout", nil, true},
		{"no plan: tdd → propose (legacy cadence)", "tdd", nil, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cp := &countingProposer{}
			router.LLMProposal{Proposer: cp}.Decide(router.RouteInput{
				Current:   tc.current,
				Plan:      tc.plan,
				Cfg:       cfg,
				Completed: []string{"scout"},
			})
			if gotCall := len(cp.seen) == 1; gotCall != tc.wantCall {
				t.Errorf("Propose called=%v (seen=%v), want %v", gotCall, cp.seen, tc.wantCall)
			}
		})
	}
}
