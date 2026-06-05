package main

import (
	"path/filepath"
	"testing"
)

func TestPhaseRoots_DefaultWhenUnset(t *testing.T) {
	t.Setenv("EVOLVE_PHASE_ROOTS", "")
	got := phaseRoots("/proj")
	want := filepath.Join("/proj", ".evolve", "phases")
	if len(got) != 1 || got[0] != want {
		t.Errorf("phaseRoots = %v, want [%s]", got, want)
	}
}

func TestPhaseRoots_ColonSplitRelativeAndAbsolute(t *testing.T) {
	t.Setenv("EVOLVE_PHASE_ROOTS", ".evolve/phases:/abs/plugin/phases: vendor/phases :")
	got := phaseRoots("/proj")
	want := []string{
		filepath.Join("/proj", ".evolve/phases"),
		"/abs/plugin/phases",
		filepath.Join("/proj", "vendor/phases"),
	}
	if len(got) != len(want) {
		t.Fatalf("phaseRoots = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("phaseRoots[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
