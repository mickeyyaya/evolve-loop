package dispositionrouter_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
)

func TestExportedVocabularyNamed(t *testing.T) {
	t.Parallel()
	if dispositionrouter.RecurrenceConsoleFloor != 3 {
		t.Fatalf("RecurrenceConsoleFloor = %d, want 3", dispositionrouter.RecurrenceConsoleFloor)
	}
	if dispositionrouter.PendingActionsFile != "pending-actions.jsonl" {
		t.Fatalf("PendingActionsFile = %q", dispositionrouter.PendingActionsFile)
	}
	if got, want := dispositionrouter.PendingActionsPath("d"), filepath.Join("d", dispositionrouter.PendingActionsFile); got != want {
		t.Fatalf("PendingActionsPath = %q, want %q", got, want)
	}
	var d dispositionrouter.Decision = dispositionrouter.Decide(dispositionrouter.GuardAbortClass, 1, "")
	if d.Route != dispositionrouter.RouteConsole || d.Reason == "" || !d.Forced {
		t.Fatalf("Decision = %+v", d)
	}
	raw, err := json.Marshal(dispositionrouter.Intent{
		Cycle: 1, Pattern: "p", ItemID: "i", Title: "t", Summary: "s", Acceptance: []string{"a"},
		Action: dispositionrouter.ActionAutofile, Route: dispositionrouter.RouteQueue, Recurrence: 2, Weight: 0.5, Reason: "r",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"action":"autofile"`, `"route":"queue"`, `"recurrence":2`, `"title":"t"`, `"summary":"s"`, `"acceptance":["a"]`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("Intent wire %s missing %s", raw, want)
		}
	}
}

func TestLayerNames_AreTheRetroLayerVocabulary(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ got, want string }{
		{dispositionrouter.LayerTaskCode, "task-code"},
		{dispositionrouter.LayerPipelineCode, "pipeline-code"},
		{dispositionrouter.LayerHarness, "harness"},
		{dispositionrouter.LayerInfra, "infra"},
		{dispositionrouter.LayerEvalContract, "eval-contract"},
	} {
		if c.got != c.want {
			t.Errorf("layer = %q; want %q, the disposition.json root_cause.layer value", c.got, c.want)
		}
	}
}
