package recurrence_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestApplyBoundary_AnAutofileCarriesTheStagedIntentsPriorityClass(t *testing.T) {
	root := t.TempDir()
	if _, err := dispositionrouter.StageIntent(filepath.Join(root, "escalations"), dispositionrouter.Intent{
		Cycle: 1, Pattern: "pattern:gate", ItemID: "gate", Action: dispositionrouter.ActionAutofile,
		Route: dispositionrouter.RouteConsole, Recurrence: 2, Weight: 0.75, PriorityClass: "correctness",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := recurrence.ApplyBoundary(opts(root, 1062, false)); err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "inbox", "auto-retro-1062-gate.json"))
	if err != nil {
		t.Fatal(err)
	}
	var item struct {
		PriorityClass string `json:"priority_class"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatal(err)
	}
	if item.PriorityClass != "correctness" {
		t.Errorf("the class rides from the staged intent into the item; priority_class = %q", item.PriorityClass)
	}
}

func TestApplyBoundary_AClasslessAutofileStagedBeforeClassesIsRefusedNotFiledAndDoesNotWedge(t *testing.T) {
	root := t.TempDir()
	staged := dispositionrouter.PendingActionsPath(filepath.Join(root, "escalations"))
	if err := os.MkdirAll(filepath.Dir(staged), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"cycle":1,"pattern":"pattern:legacy","item_id":"legacy","action":"autofile","route":"console","recurrence":2,"weight":0.75}` + "\n"
	if err := os.WriteFile(staged, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := recurrence.ApplyBoundary(opts(root, 1062, false))
	if err != nil {
		t.Fatalf("a legacy classless intent must not wedge the boundary: %v", err)
	}
	if len(res.Refused) != 1 || res.Refused[0] != "legacy" || len(res.Filed) != 0 {
		t.Errorf("result = %+v, want legacy refused and nothing filed", res)
	}
	if entries, _ := os.ReadDir(filepath.Join(root, "inbox")); len(entries) != 0 {
		t.Errorf("no classless item reaches the inbox: %v", entries)
	}
}

func TestApplyBoundary_AClasslessLegacyAutofileWhoseItemIsFiledIsSkippedNotRefused(t *testing.T) {
	root := t.TempDir()
	seedItem(t, filepath.Join(root, "inbox", "consumed"), "legacy", 0.75)
	staged := dispositionrouter.PendingActionsPath(filepath.Join(root, "escalations"))
	legacy := `{"cycle":1,"pattern":"pattern:legacy","item_id":"legacy","action":"autofile","route":"console","recurrence":2,"weight":0.75}` + "\n"
	fixtures.MustWrite(t, staged, legacy)
	res, err := recurrence.ApplyBoundary(opts(root, 1062, false))
	if err != nil {
		t.Fatalf("ApplyBoundary: %v", err)
	}
	if len(res.Skipped) != 1 || res.Skipped[0] != "legacy" || len(res.Refused) != 0 {
		t.Errorf("result = %+v, want the already-filed legacy intent skipped by the dedupe, not refused", res)
	}
}
