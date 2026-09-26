package phasespec

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestRootsWithPolicy(t *testing.T) {
	if got := RootsWithPolicy("/proj", policy.PathsConfig{}); len(got) == 0 {
		t.Fatal("RootsWithPolicy with empty cfg returned no roots; want the default root")
	}
	got := RootsWithPolicy("/proj", policy.PathsConfig{PhaseRoots: "/abs/root:rel/root"})
	want := []string{"/abs/root", filepath.Join("/proj", "rel/root")}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("RootsWithPolicy = %v, want %v", got, want)
	}
}

func TestRoots(t *testing.T) {
	if got := Roots(t.TempDir()); len(got) == 0 {
		t.Fatal("Roots returned no roots; want the default discovery root")
	}
}
