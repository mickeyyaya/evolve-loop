package gcpolicy_test

import (
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func TestX11_BranchesPolicyDefaultsAndOverrides(t *testing.T) {
	var def gcpolicy.BranchesPolicy
	if got := def.ClosedGrace(); got != 3*24*time.Hour {
		t.Errorf("default ClosedGrace = %v, want 72h", got)
	}
	prefixes := def.OwnedPrefixList()
	if !slices.Contains(prefixes, "console/") {
		t.Errorf("default OwnedPrefixList = %v, want console/", prefixes)
	}
	prefixes[0] = "mutated/"
	if slices.Contains(def.OwnedPrefixList(), "mutated/") {
		t.Errorf("OwnedPrefixList did not return a clone")
	}
	if !def.IsOwned("console/a") || def.IsOwned("cycle-1") || def.IsOwned("stray/x") {
		t.Errorf("default IsOwned failed on console/a, cycle-1, or stray/x")
	}
	custom := gcpolicy.BranchesPolicy{
		OwnedPrefixes:   []string{"stray/"},
		ClosedGraceDays: 1,
	}
	if !slices.Equal(custom.OwnedPrefixList(), []string{"stray/"}) {
		t.Errorf("custom OwnedPrefixList = %v, want [stray/]", custom.OwnedPrefixList())
	}
	if got := custom.ClosedGrace(); got != 24*time.Hour {
		t.Errorf("custom ClosedGrace = %v, want 24h", got)
	}
	if !custom.IsOwned("stray/x") || custom.IsOwned("console/a") {
		t.Errorf("custom IsOwned failed on stray/x or console/a")
	}
}
