package triagecap

// lane_menu_prune_export_test.go — cycle-1182 RED contract for
// wave-planner-pass-scope-prune.
//
// pruneConsumed already implements the terminal-state prune, but it is
// package-private, so the PRIMARY per-wave planning seam
// (widenNarrowDecision in package main, go/cmd/evolve/cmd_loop_wave.go) cannot
// reach it and carries consumed ids forward verbatim (cycle-1116 re-pinned
// tdd-topn-binding-gate after cycle-1113 consumed it).
//
// CONTRACT for Builder (do NOT modify these tests — implement production code):
//   - triagecap exports PruneConsumed(evolveDir string, committed []FleetCandidate) []FleetCandidate
//     with pruneConsumed's exact semantics (terminal states drop; everything
//     else, including no-evidence ids, is retained — fail open).
//   - The existing seed path keeps its behaviour; this is a single-source
//     export, not a second implementation (never_duplicate_centralize).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
)

// writeLifecycleTodo places an inbox todo in a lifecycle dir so
// inboxmover.ResolveDispatchState classifies id as that state. state=="pending"
// is the inbox root; state=="processing" lives under processing/cycle-N/.
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

// TestPruneConsumed_ExportedTerminalDropNonTerminalKeep is the export AC: the
// prune primitive must be callable from outside the package AND keep its
// fail-open contract. Terminal lifecycle states drop; pending/processing/retry
// and — load-bearing — an id with NO lifecycle evidence at all are retained (a
// prune that dropped what it cannot resolve would starve every wave of
// non-inbox-backed cards).
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
		{"no-evidence", true}, // nothing written anywhere — fail open
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

// TestPruneConsumed_ExportedEmptyInputIsIdentity pins the trivial edge: an empty
// committed prefix returns empty, no inbox read required.
func TestPruneConsumed_ExportedEmptyInputIsIdentity(t *testing.T) {
	if got := PruneConsumed(t.TempDir(), nil); len(got) != 0 {
		t.Errorf("PruneConsumed(empty) = %v, want empty", got)
	}
}
