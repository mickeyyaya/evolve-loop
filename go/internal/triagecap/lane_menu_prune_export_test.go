package triagecap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// writeLifecycleTodo places id where inboxmover.ResolveDispatchState classifies it as state.
func writeLifecycleTodo(t *testing.T, evolveDir, state, id string, deps ...string) {
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
	b, err := json.Marshal(map[string]any{"id": id, "weight": 0.5, "deps": deps})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPruneUndispatchable_KeepsExactlyTheIDsALaneMayTake(t *testing.T) {
	cases := []struct {
		state    string
		deps     []string
		wantKeep bool
	}{
		{inboxmover.StateProcessed, nil, false},
		{inboxmover.StateRejected, nil, false},
		{inboxmover.StateQuarantine, nil, false},
		{inboxmover.StateConsumed, nil, false},
		{inboxmover.StateProcessing, nil, false},
		{inboxmover.StateRetry, nil, false},
		{inboxmover.StatePending, []string{"blocker"}, false},
		{inboxmover.StatePending, nil, true},
		{"no-evidence", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.state, func(t *testing.T) {
			evolveDir := t.TempDir()
			writeLifecycleTodo(t, evolveDir, inboxmover.StatePending, "blocker")
			if tc.state != "no-evidence" {
				writeLifecycleTodo(t, evolveDir, tc.state, "subject", tc.deps...)
			}
			got := PruneUndispatchable(evolveDir, []FleetCandidate{
				cand("subject", 0.5, "go/internal/x/a.go"),
				cand("anchor", 0.4, "go/internal/z/c.go"),
			})
			kept := map[string]bool{}
			for _, c := range got {
				kept[c.ID] = true
			}
			if kept["subject"] != tc.wantKeep {
				t.Errorf("state %q deps %v: subject kept=%v, want %v", tc.state, tc.deps, kept["subject"], tc.wantKeep)
			}
			if !kept["anchor"] {
				t.Errorf("state %q: the dispatchable id `anchor` was dropped", tc.state)
			}
		})
	}
}

func TestPruneUndispatchable_EmptyInputIsIdentity(t *testing.T) {
	if got := PruneUndispatchable(t.TempDir(), nil); len(got) != 0 {
		t.Errorf("PruneUndispatchable(empty) = %v, want empty", got)
	}
}
