package ship

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func readCarryoverIDs(t *testing.T, root string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "state.json"))
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	var doc struct {
		CarryoverTodos []struct {
			ID string `json:"id"`
		} `json:"carryoverTodos"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse state.json: %v", err)
	}
	ids := make([]string, 0, len(doc.CarryoverTodos))
	for _, td := range doc.CarryoverTodos {
		ids = append(ids, td.ID)
	}
	return ids
}

func seedCarryoverState(t *testing.T, root, committedID string) {
	t.Helper()
	mustWriteState(t, filepath.Join(root, ".evolve", "state.json"), map[string]any{
		"carryoverTodos": []any{
			map[string]any{"id": committedID, "action": "do the committed work", "priority": "HIGH"},
			map[string]any{"id": "still-open-item", "action": "unrelated open follow-up", "priority": "MEDIUM"},
		},
	})
}

func hasID(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func TestPromoteInbox_LandedPassRetiresCommittedCarryover(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const cid = 1440
	const id = "carryover-pass-retirement"
	writeInboxFixture(t, root, cid, id)
	seedCarryoverState(t, root, id)

	r := landingScriptedRunner(0)
	opts := &Options{ProjectRoot: root, Runner: r.runner(), Stderr: io.Discard}
	res := &RunResult{CommitSHA: "cafebabecafebabecafebabecafebabecafebabe"}

	if err := promoteInbox(context.Background(), opts, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	ids := readCarryoverIDs(t, root)
	if hasID(ids, id) {
		t.Errorf("committed id %q still in state.json:carryoverTodos after a landed PASS closeout (ids=%v) — the retirement seam is not wired into the production PASS path", id, ids)
	}
	if !hasID(ids, "still-open-item") {
		t.Errorf("retirement removed an UNCOMMITTED carryover entry (ids=%v) — only committed/fingerprint-matched ids may retire", ids)
	}
}

func TestPromoteInbox_UnlandedPassKeepsCarryover(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const cid = 1440
	const id = "carryover-pass-retirement"
	writeInboxFixture(t, root, cid, id)
	seedCarryoverState(t, root, id)

	r := landingScriptedRunner(1)
	opts := &Options{ProjectRoot: root, Runner: r.runner(), Stderr: io.Discard}
	res := &RunResult{CommitSHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}

	if err := promoteInbox(context.Background(), opts, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	if ids := readCarryoverIDs(t, root); !hasID(ids, id) {
		t.Errorf("unlanded ship retired carryover id %q (ids=%v) — retirement must ride the same landing gate as promotion", id, ids)
	}
}

func TestPromoteInbox_NoStateFileIsNoOp(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const cid = 1440
	writeInboxFixture(t, root, cid, "some-item")

	r := landingScriptedRunner(0)
	opts := &Options{ProjectRoot: root, Runner: r.runner(), Stderr: io.Discard}
	res := &RunResult{CommitSHA: "cafebabecafebabecafebabecafebabecafebabe"}

	if err := promoteInbox(context.Background(), opts, res); err != nil {
		t.Fatalf("missing state.json must not fail the PASS closeout, got: %v", err)
	}
}
