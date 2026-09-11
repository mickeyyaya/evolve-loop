package profiles_test

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestRouterProfileAllowsEveryRouterContractArtifact(t *testing.T) {
	loader, _ := profiles.RealTreeProfiles(t)
	profile, err := loader.Get("router")
	if err != nil {
		t.Fatalf("load router profile: %v", err)
	}

	for _, contractID := range []string{"router", "router-replan", "router-proposal"} {
		contract, ok := phasecontract.For(contractID)
		if !ok {
			t.Fatalf("contract %q is not registered", contractID)
		}
		for _, operation := range []string{"Write", "Edit"} {
			permission := operation + "(.evolve/runs/cycle-*/" + contract.ArtifactName + ")"
			if !slices.Contains(profile.AllowedTools, permission) {
				t.Errorf("router profile must allow %s for contract %q", permission, contractID)
			}
		}
	}
}
