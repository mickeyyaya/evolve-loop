package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPhaseRoots_DefaultWhenUnset(t *testing.T) {
	// No policy.json in tempdir → phaseRoots falls back to .evolve/phases default.
	root := t.TempDir()
	got := phaseRoots(root)
	want := filepath.Join(root, ".evolve", "phases")
	if len(got) != 1 || got[0] != want {
		t.Errorf("phaseRoots = %v, want [%s]", got, want)
	}
}

func TestPhaseRoots_ColonSplitRelativeAndAbsolute(t *testing.T) {
	root := t.TempDir()
	// Write policy.json with custom phase roots to test full stack via phaseRoots().
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	polJSON := `{"paths":{"phase_roots":".evolve/phases:/abs/plugin/phases: vendor/phases :"}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(polJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	got := phaseRoots(root)
	want := []string{
		filepath.Join(root, ".evolve/phases"),
		"/abs/plugin/phases",
		filepath.Join(root, "vendor/phases"),
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
