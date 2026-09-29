package cycleoutcome

// lanescope_fallback_test.go — RED contract for the stable failure identity
// (2026-08-10 investigation, agent C): continuation/retry cycles have NO
// triage-decision.json (the continuation path binds the task directly), so
// ApplyFailure derived a nil committed set and the durable failure_count was
// never bumped — all 81 live inbox items sat at 0 after 15 FAILs, leaving
// TaskRetryCeiling quarantine and deep-escalation permanently unreachable.
// Fallback: when triage committed nothing, the lane-scope pin
// (<workspace>/lane-scope.json TodoIDs) is the worked set.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, _ := json.Marshal(v)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func itemFailureCount(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read item: %v", err)
	}
	var it struct {
		FailureCount int `json:"failure_count"`
	}
	if err := json.Unmarshal(raw, &it); err != nil {
		t.Fatalf("parse item: %v", err)
	}
	return it.FailureCount
}

func TestApplyFailure_LaneScopeFallbackBumpsFailureCount(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	item := filepath.Join(root, ".evolve", "inbox", "salvage-lane.json")
	writeJSON(t, item, map[string]any{"id": "salvage-lane", "weight": 0.9})
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1431")
	// NO triage-decision.json — the continuation shape. The lane-scope pin
	// carries the worked id.
	writeJSON(t, filepath.Join(ws, "lane-scope.json"), map[string]any{"todo_ids": []string{"salvage-lane"}, "goal_hash": "h"})

	if _, err := ApplyFailure(FailureInputs{ProjectRoot: root, Workspace: ws, Cycle: 1431, Ceiling: 3}); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
	// The item was claimed then released back open; find it wherever it landed.
	moved := findItemFile(t, filepath.Join(root, ".evolve", "inbox"), "salvage-lane")
	if got := itemFailureCount(t, moved); got != 1 {
		t.Fatalf("failure_count = %d, want 1 — the continuation FAIL never reached the durable counter (quarantine/escalation blind)", got)
	}
}

func TestApplyFailure_APinTriageAnsweredIsNotCharged(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeJSON(t, filepath.Join(root, ".evolve", "inbox", "declined.json"), map[string]any{"id": "declined", "weight": 0.6})
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1433")
	writeJSON(t, filepath.Join(ws, "triage-decision.json"), map[string]any{"top_n": []any{}, "dropped": []map[string]string{{"id": "declined", "reason": "premise already landed"}}})
	writeJSON(t, filepath.Join(ws, "lane-scope.json"), map[string]any{"todo_ids": []string{"declined"}, "goal_hash": "h"})

	if _, err := ApplyFailure(FailureInputs{ProjectRoot: root, Workspace: ws, Cycle: 1433, Ceiling: 3}); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
	moved := findItemFile(t, filepath.Join(root, ".evolve", "inbox"), "declined")
	if got := itemFailureCount(t, moved); got != 0 {
		t.Fatalf("failure_count = %d, want 0 — triage declined this item with a reason", got)
	}
}

func TestApplyFailure_APinAnEmptyDecisionLeftUnansweredIsCharged(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		decision map[string]any
	}{
		{"cycle 1757: only an alias card was escalated", map[string]any{"top_n": []any{}, "escalate_block": []map[string]string{{"task_id": "owed-alias", "reason": "protected-surface: go/internal/loopwave/loopwave.go"}}}},
		{"nothing at all was said about the pin", map[string]any{"top_n": []any{}}},
		{"the pin was deferred, which is owed work", map[string]any{"top_n": []any{}, "deferred": []map[string]string{{"id": "owed"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeJSON(t, filepath.Join(root, ".evolve", "inbox", "owed.json"), map[string]any{"id": "owed", "weight": 0.81})
			ws := filepath.Join(root, ".evolve", "runs", "cycle-1757")
			writeJSON(t, filepath.Join(ws, "triage-decision.json"), tc.decision)
			writeJSON(t, filepath.Join(ws, "lane-scope.json"), map[string]any{"todo_ids": []string{"owed"}, "goal_hash": "h"})

			if _, err := ApplyFailure(FailureInputs{ProjectRoot: root, Workspace: ws, Cycle: 1757, Ceiling: 3}); err != nil {
				t.Fatalf("ApplyFailure: %v", err)
			}
			moved := findItemFile(t, filepath.Join(root, ".evolve", "inbox"), "owed")
			if got := itemFailureCount(t, moved); got != 1 {
				t.Fatalf("failure_count = %d, want 1 — a pin the lane owed and never answered is charged, or it is redrawn every wave", got)
			}
		})
	}
}

// A workspace with NEITHER triage-decision nor lane-scope keeps the legacy
// whole-dir-drain semantics (nil committed set, no bumps).
func TestApplyFailure_NoScopeStaysLegacy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	item := filepath.Join(root, ".evolve", "inbox", "untouched.json")
	writeJSON(t, item, map[string]any{"id": "untouched", "weight": 0.5})
	ws := filepath.Join(root, ".evolve", "runs", "cycle-1432")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyFailure(FailureInputs{ProjectRoot: root, Workspace: ws, Cycle: 1432, Ceiling: 3}); err != nil {
		t.Fatalf("ApplyFailure: %v", err)
	}
	moved := findItemFile(t, filepath.Join(root, ".evolve", "inbox"), "untouched")
	if got := itemFailureCount(t, moved); got != 0 {
		t.Fatalf("failure_count = %d, want 0 — an unworked item must not be blamed", got)
	}
}

func findItemFile(t *testing.T, inboxDir, id string) string {
	t.Helper()
	var found string
	_ = filepath.Walk(inboxDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(p) != ".json" {
			return nil
		}
		raw, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		var it struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &it) == nil && it.ID == id {
			found = p
		}
		return nil
	})
	if found == "" {
		t.Fatalf("item %q not found anywhere under %s", id, inboxDir)
	}
	return found
}
