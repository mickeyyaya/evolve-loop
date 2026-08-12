package ship

// postship_carryover_retire_test.go — WIRING PROOF for cycle-1440 task
// `carryover-pass-retirement`.
//
// core.RetireCarryoverTodos passing its unit tests proves nothing on its own: a
// seam whose only caller is a test is dead code. These tests drive the PRODUCTION
// PASS-closeout caller (promoteInbox) end to end and assert the observable side
// effect on .evolve/state.json, so they stay RED until a real production path
// reaches the retirement seam.
//
// Deliberately asserts the STATE, not the call: any implementation that retires
// the committed ids at PASS closeout satisfies it.

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// readCarryoverIDs returns the ids in .evolve/state.json:carryoverTodos[].
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

// seedCarryoverState writes .evolve/state.json with two carryover todos: the id
// this cycle commits, and an unrelated one that must survive.
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

// TestPromoteInbox_LandedPassRetiresCommittedCarryover is the primary wiring
// proof: a LANDED PASS closeout whose triage decision committed <id> must leave
// state.json without that carryover entry — and with every other entry intact.
func TestPromoteInbox_LandedPassRetiresCommittedCarryover(t *testing.T) {
	root := t.TempDir()
	const cid = 1440
	const id = "carryover-pass-retirement"
	writeInboxFixture(t, root, cid, id)
	seedCarryoverState(t, root, id)

	r := landingScriptedRunner(0) // commit IS an ancestor of HEAD — landed
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

// TestPromoteInbox_UnlandedPassKeepsCarryover is the negative twin: promotion is
// already gated on the ship commit reaching durable history (cycle-598), and
// retirement must obey the same gate. An unlanded commit that retired the todo
// would erase the only record of work that never shipped.
func TestPromoteInbox_UnlandedPassKeepsCarryover(t *testing.T) {
	root := t.TempDir()
	const cid = 1440
	const id = "carryover-pass-retirement"
	writeInboxFixture(t, root, cid, id)
	seedCarryoverState(t, root, id)

	r := landingScriptedRunner(1) // NOT an ancestor of HEAD or origin — unlanded
	opts := &Options{ProjectRoot: root, Runner: r.runner(), Stderr: io.Discard}
	res := &RunResult{CommitSHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"}

	if err := promoteInbox(context.Background(), opts, res); err != nil {
		t.Fatalf("promoteInbox: %v", err)
	}

	if ids := readCarryoverIDs(t, root); !hasID(ids, id) {
		t.Errorf("unlanded ship retired carryover id %q (ids=%v) — retirement must ride the same landing gate as promotion", id, ids)
	}
}

// TestPromoteInbox_NoStateFileIsNoOp is the edge case: a project with no
// .evolve/state.json (fresh checkout, or a lane whose state lives elsewhere)
// must not error the whole PASS closeout over bookkeeping.
func TestPromoteInbox_NoStateFileIsNoOp(t *testing.T) {
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
