package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxrank/rankinputs"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const escalatedItemBody = `{
  "id": "%s",
  "title": "a failure item",
  "kind": "bug",
  "summary": "the same failure pattern recurred across cycles",
  "fix": "remove the recurring failure",
  "priority_class": "correctness",
  "weight": 0.5,
  "acceptance": ["the failure stops recurring"],
  "created_at": "2026-10-01T00:00:00Z"
}`

func writeEscalationItem(t *testing.T, evolveDir, id string) string {
	t.Helper()
	path := filepath.Join(evolveDir, "inbox", id+".json")
	fixtures.MustWrite(t, path, fmt.Sprintf(escalatedItemBody, id))
	return path
}

func lifecycleEditLinesFor(t *testing.T, ledgerPath, taskID string) []core.LedgerEntry {
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

func TestApplyEscalationBoundary_RecurrenceMovesRankAndLeavesWeight(t *testing.T) {
	t.Parallel()
	const itemID = "recurring-boundary-item"
	const peerID = "steady-peer-item"
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	fixtures.MustWrite(t, filepath.Join(evolveDir, "policy.json"), `{"failure_disposition":{"stage":"enforce"}}`)
	fixtures.MustWrite(t, filepath.Join(evolveDir, "recurrence-ledger.json"),
		`{"entries":{"`+itemID+`":{"pattern":"`+itemID+`","cycles":[3,4,5,6],"count":4,"last_seen":6}}}`)
	if err := ledger.New(evolveDir).Append(context.Background(), core.LedgerEntry{
		TS: "2026-10-17T00:00:00Z", Role: "orchestrator", Kind: "cycle-start",
	}); err != nil {
		t.Fatalf("seed ledger: %v", err)
	}
	itemPath := writeEscalationItem(t, evolveDir, itemID)
	peerPath := writeEscalationItem(t, evolveDir, peerID)
	if _, err := dispositionrouter.StageIntent(filepath.Join(evolveDir, "escalations"), dispositionrouter.Intent{
		Cycle: 6, Pattern: "pattern:recurring-boundary", ItemID: itemID,
		Action: dispositionrouter.ActionEscalate, Route: dispositionrouter.RouteQueue,
		Recurrence: 4, Weight: 0.5, PriorityClass: "correctness",
	}); err != nil {
		t.Fatalf("StageIntent: %v", err)
	}
	itemBefore, err := os.ReadFile(itemPath)
	if err != nil {
		t.Fatal(err)
	}
	peerBefore, err := os.ReadFile(peerPath)
	if err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	applyEscalationBoundary(evolveDir, 7, &stderr, nil)

	if itemAfter, _ := os.ReadFile(itemPath); !bytes.Equal(itemBefore, itemAfter) {
		t.Errorf("the escalation boundary wrote the item; its weight must stay as filed:\nbefore %s\nafter  %s\nstderr=%q", itemBefore, itemAfter, stderr.String())
	}
	if peerAfter, _ := os.ReadFile(peerPath); !bytes.Equal(peerBefore, peerAfter) {
		t.Errorf("the escalation boundary wrote the peer item:\nbefore %s\nafter  %s", peerBefore, peerAfter)
	}
	if edits := lifecycleEditLinesFor(t, filepath.Join(evolveDir, "ledger.jsonl"), itemID); len(edits) != 0 {
		t.Errorf("the escalation boundary recorded %d inbox edit lines for %s, want 0: a recurrence moves the rank, not the weight", len(edits), itemID)
	}

	in, warnings := rankinputs.Load(evolveDir, time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC))
	if len(warnings) > 0 {
		t.Fatalf("rank inputs warnings: %v", warnings)
	}
	scan, err := inboxbatch.ScanDir(filepath.Join(evolveDir, "inbox"))
	if err != nil || len(scan.Items) != 2 {
		t.Fatalf("ScanDir = %d items, err %v, want the two seeded items", len(scan.Items), err)
	}
	scores := map[string]float64{}
	recurrences := map[string]int{}
	for _, r := range in.Order(scan.Items, nil) {
		scores[r.Item.ID] = r.Breakdown.Score
		recurrences[r.Item.ID] = r.Breakdown.Facts.Recurrence
	}
	if recurrences[itemID] != 4 || recurrences[peerID] != 0 {
		t.Errorf("rank recurrence facts = %v, want %s at 4 and %s at 0", recurrences, itemID, peerID)
	}
	if !(scores[itemID] > scores[peerID]) {
		t.Errorf("score of the recurring item %v is not above its equal-weight peer %v: the recurrence must move the rank", scores[itemID], scores[peerID])
	}
}
