package triagecap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// writeLifecycleTodo places id where inboxmover.ResolveDispatchState classifies it as state.
func writeLifecycleTodo(t *testing.T, evolveDir, state, id string) {
	t.Helper()
	dir := filepath.Join(evolveDir, "inbox")
	switch state {
	case inboxmover.StatePending:
		// inbox root
	case inboxmover.StateProcessing:
		dir = filepath.Join(dir, "processing", "cycle-1181")
	default:
		dir = filepath.Join(dir, state)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(map[string]any{"id": id, "weight": 0.5})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPruneConsumed_ExportedTerminalDropNonTerminalKeep(t *testing.T) {
	cases := []struct {
		state    string
		wantKeep bool
	}{
		{inboxmover.StateProcessed, false},
		{inboxmover.StateRejected, false},
		{inboxmover.StateQuarantine, false},
		{inboxmover.StatePending, true},
		{inboxmover.StateProcessing, true},
		{inboxmover.StateRetry, true},
		{"no-evidence", true},
	}
	for _, tc := range cases {
		t.Run(tc.state, func(t *testing.T) {
			evolveDir := t.TempDir()
			if tc.state != "no-evidence" {
				writeLifecycleTodo(t, evolveDir, tc.state, "subject")
			}
			got := PruneConsumed(evolveDir, []FleetCandidate{
				cand("subject", 0.5, "go/internal/x/a.go"),
				cand("anchor", 0.4, "go/internal/z/c.go"),
			})
			kept := map[string]bool{}
			for _, c := range got {
				kept[c.ID] = true
			}
			if kept["subject"] != tc.wantKeep {
				t.Errorf("state %q: subject kept=%v, want %v", tc.state, kept["subject"], tc.wantKeep)
			}
			if !kept["anchor"] {
				t.Errorf("state %q: unrelated committed id `anchor` was dropped — prune must only touch consumed ids", tc.state)
			}
		})
	}
}

func TestPruneConsumed_ExportedEmptyInputIsIdentity(t *testing.T) {
	if got := PruneConsumed(t.TempDir(), nil); len(got) != 0 {
		t.Errorf("PruneConsumed(empty) = %v, want empty", got)
	}
}
