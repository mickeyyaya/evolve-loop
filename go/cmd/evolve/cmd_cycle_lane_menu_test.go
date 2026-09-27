package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestWireOrchestrator_TheNoWorkCheckAsksTheLaneMenu(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if d := wireOrchestratorDeps(root, evolveDir, io.Discard); !d.Orchestrator.LaneMenuWired() {
		t.Fatal("the production composition root must give the no-work check the lane menu triage offers, or a sequential cycle with only console-owned or waiting work seals a claim failure")
	}
}

func TestLaneMenuOf_WithholdsProtectedAndWaitingItems(t *testing.T) {
	inboxRoot := filepath.Join(t.TempDir(), ".evolve", "inbox")
	if err := os.MkdirAll(inboxRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inboxRoot, "blocker.json"), []byte(`{"id":"blocker"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	guarded := func(path string) bool { return path == "go/internal/guarded/x.go" }
	items := []inboxbatch.Item{
		{ID: "blocker"},
		{ID: "guarded", Files: []string{"go/internal/guarded/x.go"}},
		{ID: "waiting", Deps: []string{"blocker"}},
	}
	ready := laneMenuOf(guarded)(inboxRoot, items)
	if len(ready) != 1 || ready[0].ID != "blocker" {
		t.Errorf("the lane menu offers only the ready item: %+v", ready)
	}
}
