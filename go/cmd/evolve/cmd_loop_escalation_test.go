package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

// ADR-0072 S6: the halt writes a diagnostic dossier AND auto-files a P0
// pipeline-repair inbox item — so the QUEUE is injected (never_stop honored)
// even though the loop halts. On resume the pipeline fix is worked first.

func TestWritePipelineEscalation_WritesDossierAndInboxItem(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sf := &cyclestate.SystemFailureSignal{
		Category: "verdict-incoherence",
		Level:    "system",
		Evidence: "recorded=FAIL but audit=PASS and acs=PASS",
		Halt:     true,
	}
	writePipelineEscalation(evolveDir, root, 899, filepath.Join(root, ".evolve/runs/cycle-899"), sf, os.Stderr)

	// 1) escalation dossier
	escB, err := os.ReadFile(filepath.Join(evolveDir, "pipeline-escalation.json"))
	if err != nil {
		t.Fatalf("pipeline-escalation.json not written: %v", err)
	}
	var esc map[string]any
	if err := json.Unmarshal(escB, &esc); err != nil {
		t.Fatalf("escalation not valid JSON: %v", err)
	}
	if esc["category"] != "verdict-incoherence" {
		t.Errorf("escalation category = %v, want verdict-incoherence", esc["category"])
	}
	if esc["cycle"].(float64) != 899 {
		t.Errorf("escalation cycle = %v, want 899", esc["cycle"])
	}

	// 2) P0 pipeline-repair inbox item
	itemB, err := os.ReadFile(filepath.Join(root, ".evolve/inbox/pipeline-defect-verdict-incoherence.json"))
	if err != nil {
		t.Fatalf("pipeline-repair inbox item not filed: %v", err)
	}
	var item map[string]any
	if err := json.Unmarshal(itemB, &item); err != nil {
		t.Fatalf("inbox item not valid JSON: %v", err)
	}
	if item["kind"] != "pipeline-repair" {
		t.Errorf("inbox kind = %v, want pipeline-repair", item["kind"])
	}
	if item["priority"] != "P0" {
		t.Errorf("inbox priority = %v, want P0", item["priority"])
	}
	if w, ok := item["weight"].(float64); !ok || w < 0.9 {
		t.Errorf("inbox weight = %v, want >= 0.9 (P0)", item["weight"])
	}
}
