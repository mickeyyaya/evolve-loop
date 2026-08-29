package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	itemB, err := os.ReadFile(filepath.Join(root, ".evolve/inbox/pipeline-defect-verdict-incoherence-cycle899.json"))
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

func TestWritePipelineEscalation_ConsecutiveFailuresPreservesEvidence(t *testing.T) {
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

// TestWritePipelineEscalation_UnknownCategoryRetainsEvidenceOnly is AC3: a
// system-failure category the escalation renderer has never seen (neither
// "pipeline-blocker" nor "verdict-incoherence") must still surface the
// signal's own evidence verbatim, and must never invent the
// verdict-incoherence root-cause narrative ("forged", "green") that only
// applies to that one specific category.
func TestWritePipelineEscalation_UnknownCategoryRetainsEvidenceOnly(t *testing.T) {
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

// TestWritePipelineEscalation_IdentityIncludesCycleNumber pins the fix for the
// todo-halt-autofiler-mints-unique-ids carryover (state.json): the auto-filed
// inbox item's id must be minted as pipeline-defect-<category>-cycle<N>, never
// pipeline-defect-<category> alone. A category-only id is not a record
// identity — cycle 1550 dispatched into scope committed to a scope snapshot of
// the bare category id, but the on-disk record it once pointed at had already
// been overwritten by a LATER halt sharing that category, so scout's live scan
// found no matching content and the lane ran 8 phases for an empty diff (the
// same inst-L1543c empty-scope class that FAILed audit at cycle 1548, defect
// H1). Stamping the cycle into the id makes every halt's record identity
// unique, so a scope snapshot can never resolve to a different halt's content.
func TestWritePipelineEscalation_IdentityIncludesCycleNumber(t *testing.T) {
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

// TestWritePipelineEscalation_DistinctCyclesNeverCollideOnDisk is the negative
// case for the same defect: two ADR-0072 halts sharing a category (the common
// case — "pipeline-blocker" alone has 17 on-disk records today per
// state.json) must NOT collapse onto one inbox file. Under the current
// category-only filename, the second halt's atomicwrite.JSON silently
// destroys the first halt's evidence and a future lane scoped to the id
// resolves to whichever halt happened to write last — never the one it was
// actually scoped to. Both cycles' records must survive side by side.
func TestWritePipelineEscalation_DistinctCyclesNeverCollideOnDisk(t *testing.T) {
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
