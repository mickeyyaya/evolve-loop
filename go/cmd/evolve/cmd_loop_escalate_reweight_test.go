package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestApplyEscalationBoundary_ReweightsAnOpenItemThroughInboxmover(t *testing.T) {
	t.Parallel()
	const itemID = "recurring-boundary-item"
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	fixtures.MustWrite(t, filepath.Join(evolveDir, "policy.json"), `{"failure_disposition":{"stage":"enforce"}}`)
	if err := ledger.New(evolveDir).Append(context.Background(), core.LedgerEntry{
		TS: "2026-10-17T00:00:00Z", Role: "orchestrator", Kind: "cycle-start",
	}); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}
	itemPath := filepath.Join(evolveDir, "inbox", itemID+".json")
	fixtures.MustWrite(t, itemPath, `{
  "id": "`+itemID+`",
  "title": "a recurring failure the loop boundary reweights",
  "kind": "bug",
  "summary": "the same failure pattern recurred across cycles",
  "fix": "remove the recurring failure",
  "priority_class": "correctness",
  "weight": 0.5,
  "acceptance": ["the failure stops recurring"],
  "created_at": "2026-10-01T00:00:00Z"
}`)
	if _, err := dispositionrouter.StageIntent(filepath.Join(evolveDir, "escalations"), dispositionrouter.Intent{
		Cycle: 6, Pattern: "pattern:recurring-boundary", ItemID: itemID,
		Action: dispositionrouter.ActionEscalate, Route: dispositionrouter.RouteQueue,
		Recurrence: 4, Weight: 0.5, PriorityClass: "correctness",
	}); err != nil {
		t.Fatalf("StageIntent: %v", err)
	}
	cwdEvolve := filepath.Join(mustGetwd(t), ".evolve", "inbox", itemID+".json")

	var stderr bytes.Buffer
	applyEscalationBoundary(evolveDir, 7, &stderr, nil)

	raw, err := os.ReadFile(itemPath)
	if err != nil {
		t.Fatalf("read item: %v", err)
	}
	var item struct {
		Weight float64 `json:"weight"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("parse item: %v", err)
	}
	if item.Weight <= 0.5 {
		t.Fatalf("item weight = %v after the boundary, want it raised above 0.5; stderr=%q", item.Weight, stderr.String())
	}
	edits := inboxLifecycleEditLines(t, filepath.Join(evolveDir, "ledger.jsonl"), itemID)
	if want := "set weight=" + strconv.FormatFloat(item.Weight, 'f', -1, 64); len(edits) != 1 || edits[0].Message != want {
		t.Errorf("inbox-lifecycle edit lines = %+v, want one whose message is the mover's %q", edits, want)
	}
	if _, err := os.Stat(cwdEvolve); err == nil {
		t.Errorf("the reweight wrote %s under the process cwd: the mover must be rooted at the boundary's project", cwdEvolve)
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}

func inboxLifecycleEditLines(t *testing.T, ledgerPath, taskID string) []core.LedgerEntry {
	t.Helper()
	raw, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	var edits []core.LedgerEntry
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var e core.LedgerEntry
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatalf("ledger line does not parse: %v: %s", err, line)
		}
		if e.Kind == "inbox-lifecycle" && e.Action == "edit" && e.TaskID == taskID {
			edits = append(edits, e)
		}
	}
	return edits
}
