package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func priorityClassAt(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var item struct {
		PriorityClass string `json:"priority_class"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatal(err)
	}
	return item.PriorityClass
}

func TestBuildGoalStallItem_IsStabilityClassedAndValidateRequiresAClass(t *testing.T) {
	item := buildGoalStallItem(goalStallKind, "abcd1234ef", &goalStallEscalation{streak: 3}, 0.9, 1, "2026-10-06T00:00:00Z")
	if item.PriorityClass != "stability" {
		t.Errorf("a goal stuck shipping nothing is a stability defect; priority_class = %q", item.PriorityClass)
	}
	item.PriorityClass = ""
	if err := item.validate(); err == nil {
		t.Error("validate must refuse an unclassed goal-stall item")
	}
}

func TestWritePipelineEscalation_FilesAStabilityClassedItem(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	sf := &cyclestate.SystemFailureSignal{Category: "verdict-incoherence", Level: "system", Evidence: "e", Halt: true}
	rec := writePipelineEscalation(evolveDir, root, 899, filepath.Join(evolveDir, "runs", "cycle-899"), sf, io.Discard)
	if got := priorityClassAt(t, rec.InboxItemPath); got != "stability" {
		t.Errorf("a halted loop is a stability defect; priority_class = %q", got)
	}
}

func TestFileUnexplainedOutcomeDefect_FilesADebuggabilityClassedItem(t *testing.T) {
	root := t.TempDir()
	fileUnexplainedOutcomeDefect(root, 77, "no outcome recorded")
	path := filepath.Join(root, ".evolve", "inbox", "auto-unexplained-outcome-cycle-77.json")
	if got := priorityClassAt(t, path); got != "debuggability" {
		t.Errorf("an outcome nobody can explain is a debuggability defect; priority_class = %q", got)
	}
}
