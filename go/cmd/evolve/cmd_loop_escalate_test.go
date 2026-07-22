package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// TestRunLoop_EscalatesAtIterationBoundary is the WIRING proof for the
// failure-disposition-router S4 boundary applier: it drives the real runLoop
// (stubbed orchestrator deps) over a temp .evolve carrying one open inbox item
// and one staged escalate intent, and asserts the loop APPLIED the escalation —
// weight bumped + apply report written. A commented-out call site or a defined-
// but-uncalled applier fails this test, which a source grep would not catch.
func TestRunLoop_EscalatesAtIterationBoundary(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	inboxDir := filepath.Join(evolveDir, "inbox")
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	// dispatch off (skip the verify pipeline), escalation stage ENFORCE so the
	// boundary mutates — proving the stage is config-injected, not a flag.
	pol := `{"dispatch":{"policy":"off"},"workflow":{"auto_prune":false},` +
		`"failure_disposition":{"stage":"enforce"}}`
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(pol), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	itemPath := filepath.Join(inboxDir, "recurring-defect.json")
	body, _ := json.MarshalIndent(map[string]any{
		"id": "recurring-defect", "action": "fix pattern:flaky-tier", "weight": 0.80,
	}, "", "  ")
	if err := os.WriteFile(itemPath, body, 0o644); err != nil {
		t.Fatalf("write inbox item: %v", err)
	}
	if _, err := dispositionrouter.StageIntent(filepath.Join(evolveDir, "escalations"), dispositionrouter.Intent{
		Pattern: "pattern:flaky-tier", ItemID: "recurring-defect",
		Action: dispositionrouter.ActionEscalate, Route: dispositionrouter.RouteQueue,
		Recurrence: 4, Weight: 0.80,
	}); err != nil {
		t.Fatalf("StageIntent: %v", err)
	}

	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()

	var stdout, stderr bytes.Buffer
	if rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "escalation boundary wiring",
		"--cycles", "1",
	}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("runLoop rc=%d; stderr=%q", rc, stderr.String())
	}

	reportPath := filepath.Join(evolveDir, "escalation-apply-report.json")
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("loop wrote no escalation apply report at %s — the iteration-boundary call site is unwired: %v", reportPath, err)
	}
	raw, err := os.ReadFile(itemPath)
	if err != nil {
		t.Fatalf("read inbox item: %v", err)
	}
	var item struct {
		Weight float64 `json:"weight"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("parse inbox item: %v", err)
	}
	// min(0.99, 0.80 + 0.03*(4-1)) = 0.89
	if item.Weight <= 0.80 {
		t.Fatalf("inbox weight = %v, want the escalated 0.89 — the boundary applier did not run at the iteration boundary", item.Weight)
	}
}
