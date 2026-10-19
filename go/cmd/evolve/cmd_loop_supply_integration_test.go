//go:build integration

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestModuleGraph_TheDefaultLoaderReadsTheGoModuleOfTheProjectRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	goDir := filepath.Join(root, "go")
	if err := os.MkdirAll(filepath.Join(goDir, "internal", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module github.com/mickeyyaya/evolve-loop/go\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "internal", "x", "x.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, _, _ := supplyCoordinator(t, root)
	b.loadGraph = nil

	graph, err := b.moduleGraph(policy.OrchestratorConfig{HubMinImporters: 120}, 1)

	if err != nil || graph == nil || len(graph.Packages([]string{"go/internal/x/x.go"})) != 1 {
		t.Errorf("moduleGraph = %v, %v, want the graph of the module under go/", graph, err)
	}
}
