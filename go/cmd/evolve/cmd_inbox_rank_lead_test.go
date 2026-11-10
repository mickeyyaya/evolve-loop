package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

var redOnMain = baseattr.Verdict{
	Main: "0123456789abcdef",
	MainRed: []baseattr.Key{
		{Package: "example.com/m/pkg/a", Test: "TestOne"},
		{Package: "example.com/m/pkg/a", Test: "TestTwo"},
		{Package: "example.com/m/pkg/b", Test: "TestThree"},
	},
}

func TestRank_TheFixItemLeadsWhileTheStopHolds(t *testing.T) {
	root := curationRoot(t, map[string]string{
		"a.json": `{"id":"a-recurring","title":"t","priority_class":"correctness","weight":1,"created_at":"2026-10-06"}`,
	})
	ledger := `{"entries":{"verify-gate-flake":{"pattern":"verify-gate-flake","cycles":[1,2,3],"count":3,"fix_item_id":"a-recurring"}}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "recurrence-ledger.json"), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := core.OpenMainRedStop(root, redOnMain); err != nil {
		t.Fatal(err)
	}

	rc, stdout, stderr := rankAt(t, "--json")

	var doc rankedInboxDoc
	if err := json.Unmarshal([]byte(stdout), &doc); rc != 0 || err != nil || len(doc.Lists) == 0 || len(doc.Lists[0].Items) < 2 {
		t.Fatalf("rc = %d err = %v stderr = %q stdout = %s", rc, err, stderr, stdout)
	}
	if doc.Lists[0].List != "ready" {
		t.Fatalf("list = %q, want the ready list that a lane takes work from", doc.Lists[0].List)
	}
	if first := doc.Lists[0].Items[0]; first.ID != "fix-main-red-01234567" || first.Rank != 1 {
		t.Errorf("first = %+v, want the fix item of the held stop", first)
	}
	if second := doc.Lists[0].Items[1]; second.ID != "a-recurring" || second.Score <= doc.Lists[0].Items[0].Score {
		t.Errorf("second = %+v, want the item with the higher score behind the lead", second)
	}
}
