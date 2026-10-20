package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func copyRealRegistry(t *testing.T, root string) {
	t.Helper()
	src := filepath.Join("..", "..", "..", "docs", "architecture", "phase-registry.json")
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Skipf("phase registry not present: %v", err)
	}
	dst := filepath.Join(root, "docs", "architecture", "phase-registry.json")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWireOrchestrator_APreBuildPhaseAnchoredAfterBuildReachesTheSignalCenter(t *testing.T) {
	root := t.TempDir()
	copyRealRegistry(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	writeUserPhase(t, root, "late-repro", map[string]any{
		"name": "late-repro", "optional": true, "archetype": "evaluate",
		"stage": "pre-build", "after": "build"})
	d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{})
	defer d.CloseSignals()
	d.Signals.Flush()
	reasons := strings.Join(routingDefects(d.Signals.Recent()), "\n")
	if !strings.Contains(reasons, "late-repro") || !strings.Contains(reasons, "pre-build") {
		t.Fatalf("a pre-build phase placed after build must reach the cycle's Signal Center as a routing defect; got %q", reasons)
	}
}

func TestWireOrchestrator_APostBuildPhaseAnchoredAfterBuildRaisesNoRoutingDefect(t *testing.T) {
	root := t.TempDir()
	copyRealRegistry(t, root)
	evolveDir := filepath.Join(root, ".evolve")
	writeUserPhase(t, root, "late-note", map[string]any{
		"name": "late-note", "optional": true, "archetype": "evaluate",
		"stage": "post-build", "after": "build"})
	d := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{})
	defer d.CloseSignals()
	d.Signals.Flush()
	for _, reason := range routingDefects(d.Signals.Recent()) {
		if strings.Contains(reason, "late-note") && strings.Contains(reason, "stage") {
			t.Fatalf("a post-build phase after build is legal, got defect %q", reason)
		}
	}
}
