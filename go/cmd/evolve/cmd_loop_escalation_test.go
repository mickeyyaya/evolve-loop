package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestWritePipelineEscalation_WritesDossierAndInboxItem(t *testing.T) {
	t.Parallel()
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

	itemB, err := os.ReadFile(filepath.Join(root, ".evolve/inbox/pipeline-defect-verdict-incoherence-cycle899.json"))
	if err != nil {
		t.Fatalf("pipeline-repair inbox item not filed: %v", err)
	}
	var item map[string]any
	if err := json.Unmarshal(itemB, &item); err != nil {
		t.Fatalf("inbox item not valid JSON: %v", err)
	}
	if item["kind"] != inboxbatch.KindPipelineRepair {
		t.Errorf("inbox kind = %v, want %s", item["kind"], inboxbatch.KindPipelineRepair)
	}
	if item["injected_by"] != escalationInjectedBy {
		t.Errorf("inbox injected_by = %v, want %q", item["injected_by"], escalationInjectedBy)
	}
	if item["priority"] != "P0" {
		t.Errorf("inbox priority = %v, want P0", item["priority"])
	}
	if w, ok := item["weight"].(float64); !ok || w < 0.9 {
		t.Errorf("inbox weight = %v, want >= 0.9 (P0)", item["weight"])
	}
}

func TestWritePipelineEscalation_ConsecutiveFailuresPreservesEvidence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	evidence := "consecutive audit failures (rule=consecutive-audit-failures fingerprint=abc123)"
	sf := &cyclestate.SystemFailureSignal{
		Category: "pipeline-blocker",
		Level:    "system",
		Evidence: evidence,
		Halt:     true,
	}
	writePipelineEscalation(evolveDir, root, 1579, filepath.Join(root, ".evolve/runs/cycle-1579"), sf, os.Stderr)

	escB, err := os.ReadFile(filepath.Join(evolveDir, "pipeline-escalation.json"))
	if err != nil {
		t.Fatal(err)
	}
	var esc map[string]any
	if err := json.Unmarshal(escB, &esc); err != nil {
		t.Fatal(err)
	}
	itemB, err := os.ReadFile(filepath.Join(root, ".evolve/inbox/pipeline-defect-pipeline-blocker-cycle1579.json"))
	if err != nil {
		t.Fatal(err)
	}
	var item map[string]any
	if err := json.Unmarshal(itemB, &item); err != nil {
		t.Fatal(err)
	}

	for name, text := range map[string]string{
		"next_action": esc["next_action"].(string),
		"repro_hint":  esc["repro_hint"].(string),
		"summary":     item["summary"].(string),
		"root_cause":  item["root_cause"].(string),
		"fix":         item["fix"].(string),
	} {
		if !strings.Contains(text, evidence) {
			t.Errorf("%s = %q, want original halt evidence %q", name, text, evidence)
		}
		if strings.Contains(strings.ToLower(text), "green") || strings.Contains(strings.ToLower(text), "forged") {
			t.Errorf("%s = %q, must not invent green artifacts or a forged verdict for pipeline-blocker", name, text)
		}
	}
}

func TestWritePipelineEscalation_VerdictIncoherenceKeepsArtifactGuidance(t *testing.T) {
	t.Parallel()
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
	writePipelineEscalation(evolveDir, root, 1579, filepath.Join(root, ".evolve/runs/cycle-1579"), sf, os.Stderr)

	escB, err := os.ReadFile(filepath.Join(evolveDir, "pipeline-escalation.json"))
	if err != nil {
		t.Fatal(err)
	}
	var esc map[string]any
	if err := json.Unmarshal(escB, &esc); err != nil {
		t.Fatal(err)
	}
	nextAction := esc["next_action"].(string)
	if !strings.Contains(nextAction, "audit-report.md + acs-verdict.json") || !strings.Contains(nextAction, "forged a negative verdict") {
		t.Errorf("next_action = %q, want verdict/artifact comparison guidance", nextAction)
	}
}

func TestWritePipelineEscalation_UnknownCategoryRetainsEvidenceOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	evidence := "unrecognized halt signal from a future system-failure rule"
	sf := &cyclestate.SystemFailureSignal{
		Category: "future-unknown-category",
		Level:    "system",
		Evidence: evidence,
		Halt:     true,
	}
	writePipelineEscalation(evolveDir, root, 1579, filepath.Join(root, ".evolve/runs/cycle-1579"), sf, os.Stderr)

	escB, err := os.ReadFile(filepath.Join(evolveDir, "pipeline-escalation.json"))
	if err != nil {
		t.Fatal(err)
	}
	var esc map[string]any
	if err := json.Unmarshal(escB, &esc); err != nil {
		t.Fatal(err)
	}
	itemB, err := os.ReadFile(filepath.Join(root, ".evolve/inbox/pipeline-defect-future-unknown-category-cycle1579.json"))
	if err != nil {
		t.Fatal(err)
	}
	var item map[string]any
	if err := json.Unmarshal(itemB, &item); err != nil {
		t.Fatal(err)
	}

	for name, text := range map[string]string{
		"next_action": esc["next_action"].(string),
		"repro_hint":  esc["repro_hint"].(string),
		"summary":     item["summary"].(string),
		"root_cause":  item["root_cause"].(string),
		"fix":         item["fix"].(string),
	} {
		if !strings.Contains(text, evidence) {
			t.Errorf("%s = %q, want original halt evidence %q for an unknown category", name, text, evidence)
		}
		if strings.Contains(strings.ToLower(text), "green") || strings.Contains(strings.ToLower(text), "forged") {
			t.Errorf("%s = %q, must not invent a verdict-incoherence root cause for an unrecognized category", name, text)
		}
	}
}

func TestWritePipelineEscalation_IdentityIncludesCycleNumber(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sf := &cyclestate.SystemFailureSignal{
		Category: "pipeline-blocker",
		Level:    "system",
		Evidence: "recorded=FAIL but audit=PASS and acs=PASS",
		Halt:     true,
	}
	writePipelineEscalation(evolveDir, root, 1543, filepath.Join(root, ".evolve/runs/cycle-1543"), sf, os.Stderr)

	wantID := "pipeline-defect-pipeline-blocker-cycle1543"
	itemPath := filepath.Join(root, ".evolve/inbox", wantID+".json")
	itemB, err := os.ReadFile(itemPath)
	if err != nil {
		t.Fatalf("inbox item not filed at cycle-scoped path %s: %v", itemPath, err)
	}
	var item map[string]any
	if err := json.Unmarshal(itemB, &item); err != nil {
		t.Fatalf("inbox item not valid JSON: %v", err)
	}
	if item["id"] != wantID {
		t.Errorf("inbox item id = %v, want %q (category alone is not a unique record identity)", item["id"], wantID)
	}
}

func TestWritePipelineEscalation_DistinctCyclesNeverCollideOnDisk(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sf := &cyclestate.SystemFailureSignal{
		Category: "pipeline-blocker",
		Level:    "system",
		Evidence: "recorded=FAIL but audit=PASS and acs=PASS",
		Halt:     true,
	}
	writePipelineEscalation(evolveDir, root, 1543, filepath.Join(root, ".evolve/runs/cycle-1543"), sf, os.Stderr)
	writePipelineEscalation(evolveDir, root, 1548, filepath.Join(root, ".evolve/runs/cycle-1548"), sf, os.Stderr)

	firstPath := filepath.Join(root, ".evolve/inbox/pipeline-defect-pipeline-blocker-cycle1543.json")
	secondPath := filepath.Join(root, ".evolve/inbox/pipeline-defect-pipeline-blocker-cycle1548.json")

	firstB, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("cycle-1543 record was destroyed by the cycle-1548 halt (same category, no collision guard): %v", err)
	}
	var first map[string]any
	if err := json.Unmarshal(firstB, &first); err != nil {
		t.Fatalf("cycle-1543 record not valid JSON: %v", err)
	}
	if first["summary"] == nil || !strings.Contains(first["summary"].(string), "cycle 1543") {
		t.Errorf("cycle-1543 record summary = %v, want it to still reference cycle 1543 (not overwritten by the later halt)", first["summary"])
	}

	secondB, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("cycle-1548 record not filed: %v", err)
	}
	var second map[string]any
	if err := json.Unmarshal(secondB, &second); err != nil {
		t.Fatalf("cycle-1548 record not valid JSON: %v", err)
	}
	if second["summary"] == nil || !strings.Contains(second["summary"].(string), "cycle 1548") {
		t.Errorf("cycle-1548 record summary = %v, want it to reference cycle 1548", second["summary"])
	}
}
