package phasespec

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// TestRootsWithPolicy pins the EVOLVE_PHASE_ROOTS → policy.PathsConfig conversion:
// an empty config falls back to the default root (joined under projectRoot); a
// configured colon-list is honored, with absolute entries kept and relative ones
// joined under projectRoot.
func TestRootsWithPolicy(t *testing.T) {
	// Empty config → default root under projectRoot (non-empty result).
	if got := RootsWithPolicy("/proj", policy.PathsConfig{}); len(got) == 0 {
		t.Fatal("RootsWithPolicy with empty cfg returned no roots; want the default root")
	}
	// Configured: absolute kept as-is, relative joined under projectRoot.
	got := RootsWithPolicy("/proj", policy.PathsConfig{PhaseRoots: "/abs/root:rel/root"})
	want := []string{"/abs/root", filepath.Join("/proj", "rel/root")}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("RootsWithPolicy = %v, want %v", got, want)
	}
}

// TestRoots covers the policy-loading wrapper Roots (cycle-17 made it delegate to
// RootsWithPolicy). With no policy.json under the temp root it falls back to the
// default root joined under projectRoot — a non-empty result.
func TestRoots(t *testing.T) {
	if got := Roots(t.TempDir()); len(got) == 0 {
		t.Fatal("Roots returned no roots; want the default discovery root")
	}
}
