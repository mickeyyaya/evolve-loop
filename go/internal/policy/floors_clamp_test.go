package policy_test

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestFloorsClamp_ACeilinglessFloorAgentGetsTheFloorFamilyAsItsCeiling(t *testing.T) {
	floors := policy.Floors{"claude": {"auditor"}}
	got := floors.Clamp("auditor", profiles.Profile{Name: "auditor", CLI: "claude-tmux"})
	if !slices.Equal(got.AllowedCLIs, []string{"claude"}) {
		t.Fatalf("AllowedCLIs = %v, want [claude]", got.AllowedCLIs)
	}
}

func TestFloorsClamp_AnAgentOutsideTheFloorKeepsItsProfile(t *testing.T) {
	floors := policy.Floors{"claude": {"auditor"}}
	in := profiles.Profile{Name: "scout", CLI: "codex-tmux"}
	if got := floors.Clamp("scout", in); got.AllowedCLIs != nil {
		t.Fatalf("an agent off the floor gained a ceiling: %v", got.AllowedCLIs)
	}
}

func TestFloorsClamp_AnEmptyFloorClampsNothing(t *testing.T) {
	if got := (policy.Floors{}).Clamp("auditor", profiles.Profile{Name: "auditor"}); got.AllowedCLIs != nil {
		t.Fatalf("floors {} must leave the ceiling empty, got %v", got.AllowedCLIs)
	}
}

func TestFloorsClamp_APrivateCeilingIsKept(t *testing.T) {
	floors := policy.Floors{"claude": {"auditor"}}
	got := floors.Clamp("auditor", profiles.Profile{Name: "auditor", AllowedCLIs: []string{"codex"}})
	if !slices.Equal(got.AllowedCLIs, []string{"codex"}) {
		t.Fatalf("a private ceiling must stay so RuleEmpty can report it: %v", got.AllowedCLIs)
	}
}

func TestFloorsClamp_DoesNotMutateItsInput(t *testing.T) {
	floors := policy.Floors{"claude": {"auditor"}}
	in := profiles.Profile{Name: "auditor"}
	floors.Clamp("auditor", in)
	if in.AllowedCLIs != nil {
		t.Fatalf("Clamp mutated its argument: %v", in.AllowedCLIs)
	}
}
