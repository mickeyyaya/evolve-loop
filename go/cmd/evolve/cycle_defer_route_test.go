package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

func TestCloseoutCycleOutcome_AReintentDeferEndRoutesTheItemToTheConsole(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		retroDecision string
		wantRoute     string
	}{
		"a cycle ended by the defer":    {"audit-fail-defer: intent-malformed maps to defer-or-quarantine (reintent), per failure-decision.json; the cycle ends", "console-manual"},
		"a cycle that failed otherwise": {"proceed: no retry", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			evolveDir, ws := seedFailedCycleInbox(t, root, "lane-item", 1953)
			decision := `{"category":"intent-malformed","level":"task","action":"defer-or-quarantine","fix_type":"reintent"}`
			if err := os.WriteFile(filepath.Join(ws, "failure-decision.json"), []byte(decision), 0o644); err != nil {
				t.Fatal(err)
			}
			result := core.CycleResult{Cycle: 1953, FinalVerdict: cyclestate.VerdictFAIL, RetroDecision: c.retroDecision}

			if _, err := closeoutCycleOutcome(result, root, evolveDir, io.Discard, newFakeLedger(), nil); err != nil {
				t.Fatalf("closeout: %v", err)
			}

			path, err := inboxmover.FindFileByTaskID(filepath.Join(evolveDir, "inbox"), "lane-item")
			if err != nil {
				t.Fatal(err)
			}
			body, _ := os.ReadFile(path)
			var item map[string]any
			_ = json.Unmarshal(body, &item)
			if got, _ := item["route"].(string); got != c.wantRoute {
				t.Errorf("route = %q, want %q: only the defer end routes by fix_type", got, c.wantRoute)
			}
		})
	}
}

func TestApplyResultFailureOutcome_ACycleZeroResultTouchesNoInbox(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir, _ := seedFailedCycleInbox(t, root, "lane-item", 7)
	fake := newFakeLedger()

	err := applyResultFailureOutcome(core.CycleResult{FinalVerdict: cyclestate.VerdictFAIL}, root, evolveDir, io.Discard, fake, nil)

	if err != nil || len(fake.lifecycle) != 0 {
		t.Errorf("err = %v, lifecycle lines %v; want no closeout for a result with no cycle", err, fake.lifecycle)
	}
	if _, ferr := inboxmover.FindFileByTaskID(filepath.Join(evolveDir, "inbox"), "lane-item"); ferr != nil {
		t.Errorf("the item left the inbox root: %v", ferr)
	}
}
