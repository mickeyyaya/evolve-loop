package policy_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestFleetConfig_BudgetAbsent(t *testing.T) {
	for _, tc := range []struct {
		name string
		pol  policy.Policy
	}{
		{"no-fleet-block", policy.Policy{}},
		{"fleet-without-budget", policy.Policy{Fleet: &policy.FleetPolicy{Count: 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pol.FleetConfig(); got.Budget != nil {
				t.Errorf("FleetConfig().Budget = %+v, want nil (absent block ⇒ no budget)", got.Budget)
			}
		})
	}
}

func TestFleetConfig_BudgetResolution(t *testing.T) {
	cases := []struct {
		name           string
		budget         policy.FleetBudgetPolicy
		wantStage      string
		wantCapacity   float64
		wantSafety     float64
		wantHistoryPos bool
		wantHistory    int
	}{
		{
			name:           "empty-block-defaults-shadow",
			budget:         policy.FleetBudgetPolicy{},
			wantStage:      "shadow",
			wantHistoryPos: true,
		},
		{
			name:           "full-tunables-shadow-default",
			budget:         policy.FleetBudgetPolicy{CapacityCycles: 40, SafetyFraction: 0.8},
			wantStage:      "shadow",
			wantCapacity:   40,
			wantSafety:     0.8,
			wantHistoryPos: true,
		},
		{
			name:           "explicit-enforce",
			budget:         policy.FleetBudgetPolicy{Stage: "enforce", CapacityCycles: 40, SafetyFraction: 0.8},
			wantStage:      "enforce",
			wantCapacity:   40,
			wantSafety:     0.8,
			wantHistoryPos: true,
		},
		{
			name:        "history-window-override",
			budget:      policy.FleetBudgetPolicy{Stage: "shadow", HistoryWindow: 25},
			wantStage:   "shadow",
			wantHistory: 25,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			budget := tc.budget
			// Declared by type name: apicover does not count the .Budget field access.
			var b *policy.FleetBudgetConfig = policy.Policy{Fleet: &policy.FleetPolicy{Budget: &budget}}.FleetConfig().Budget
			if b == nil {
				t.Fatalf("FleetConfig().Budget = nil, want non-nil for a present block")
			}
			if b.Stage != tc.wantStage {
				t.Errorf("Stage = %q, want %q", b.Stage, tc.wantStage)
			}
			if b.CapacityCycles != tc.wantCapacity {
				t.Errorf("CapacityCycles = %v, want %v", b.CapacityCycles, tc.wantCapacity)
			}
			if b.Safety != tc.wantSafety {
				t.Errorf("Safety = %v, want %v", b.Safety, tc.wantSafety)
			}
			if tc.wantHistory != 0 && b.HistoryWindow != tc.wantHistory {
				t.Errorf("HistoryWindow = %d, want %d", b.HistoryWindow, tc.wantHistory)
			}
			if tc.wantHistoryPos && b.HistoryWindow <= 0 {
				t.Errorf("HistoryWindow = %d, want > 0 (defaulted)", b.HistoryWindow)
			}
		})
	}
}

func TestFleetConfig_BudgetStageUnknown(t *testing.T) {
	got := policy.Policy{Fleet: &policy.FleetPolicy{Budget: &policy.FleetBudgetPolicy{Stage: "enfroce"}}}.FleetConfig()
	if got.Budget == nil || got.Budget.Stage != "shadow" {
		t.Fatalf("unknown Stage must fail safe to shadow, got %+v", got.Budget)
	}
	found := false
	for _, w := range got.Warnings {
		if strings.Contains(w, "enfroce") && strings.Contains(w, "shadow") {
			found = true
		}
	}
	if !found {
		t.Errorf("Warnings = %v, want one naming the rejected %q and the fallback \"shadow\"", got.Warnings, "enfroce")
	}
}
