package ship

// consume_lanescope_union_test.go — consumption-id-linkage-lane-scope-union
// (0.86). Two live burns of one class: triage's bookkeeping defeats the #466
// in-commit consumption on exactly the lanes that matter. Cycle-1515: triage
// DECOMPOSED the assigned id into three sub-ids, so top_n named none of the
// inbox files. Cycle-1552 (soak-20260824a wave 2's burn): triage DROPPED the
// assigned id as "already-shipped" with top_n:[], build shipped the item's
// implementation anyway (df322f6c), consumption resolved zero ids, and the
// stale item cost wave 2 a full lane re-proving finished work. The contract:
// a PASS lane ship retires its ASSIGNED scope ids regardless of triage's
// renaming/decomposition/drop — the one exception is an id triage EXPLICITLY
// deferred, which stays pickable (its remainder rides carryover).

import (
	"os"
	"path/filepath"
	"testing"
)

func writeLaneScope(t *testing.T, dir string, ids string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "lane-scope.json"),
		[]byte(`{"todo_ids":[`+ids+`]}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The cycle-1552 shape: scope id dropped by triage, top_n empty — the id must
// still resolve for consumption.
func TestCommittedInboxIDs_UnionsLaneScopeWhenTriageDroppedTheScope(t *testing.T) {
	ws := t.TempDir()
	writeLaneScope(t, ws, `"premise-challenge-fail-never-reaches-failure-learning"`)
	body := []byte(`{"schema_version":1,"top_n":[],"deferred":[],
		"dropped":[{"id":"premise-challenge-fail-never-reaches-failure-learning","reason":"already-shipped"}]}`)
	ids := committedInboxIDs(ws, body, true)
	if len(ids) != 1 || ids[0] != "premise-challenge-fail-never-reaches-failure-learning" {
		t.Fatalf("ids = %v — a PASS lane ship must retire its assigned scope id even when triage dropped it (cycle-1552)", ids)
	}
}

// The cycle-1515 shape: triage decomposed into sub-ids; top_n names things
// that are not inbox files. The scope id joins the set (the sub-ids stay too —
// FindFileByTaskID misses them harmlessly).
func TestCommittedInboxIDs_UnionsLaneScopeWithDecomposedTopN(t *testing.T) {
	ws := t.TempDir()
	writeLaneScope(t, ws, `"park-consume-releases-continuation-binding"`)
	body := []byte(`{"schema_version":1,"top_n":[{"id":"registry-release-on-park-consume"},{"id":"planner-adoption-live-item-guard"}]}`)
	ids := committedInboxIDs(ws, body, true)
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	if !want["park-consume-releases-continuation-binding"] {
		t.Fatalf("ids = %v — the assigned scope id must survive triage decomposition (cycle-1515)", ids)
	}
	if !want["registry-release-on-park-consume"] {
		t.Fatalf("ids = %v — triage's committed sub-ids must stay in the set", ids)
	}
}

// The deferred guard's LIVE shape after the named-engagement discriminator:
// triage RENAMED the work (top_n names only sub-ids — no scope id committed
// by name, so the rename arm would union the whole scope) while EXPLICITLY
// deferring one scope id. The deferral must beat the rename-shape union.
// (When triage engages the scope by name, the discriminator alone keeps
// unmentioned mates open — the guard's job is exactly this rename+defer
// overlap.)
func TestCommittedInboxIDs_TriageDeferredScopeIDStaysPickable(t *testing.T) {
	ws := t.TempDir()
	writeLaneScope(t, ws, `"renamed-work","postponed-item"`)
	body := []byte(`{"schema_version":1,"top_n":[{"id":"sub-task-a"}],"deferred":[{"id":"postponed-item","reason":"needs the other fix first"}]}`)
	ids := committedInboxIDs(ws, body, true)
	got := map[string]bool{}
	for _, id := range ids {
		got[id] = true
	}
	if got["postponed-item"] {
		t.Fatalf("ids = %v — an explicitly-deferred scope id must beat the rename-shape union", ids)
	}
	if !got["renamed-work"] {
		t.Fatalf("ids = %v — the renamed sibling must consume via the rename arm", ids)
	}
}

// Nil body (no triage decision at all) keeps the existing lane-scope fallback.
func TestCommittedInboxIDs_NilBodyStillResolvesLaneScope(t *testing.T) {
	ws := t.TempDir()
	writeLaneScope(t, ws, `"solo-item"`)
	ids := committedInboxIDs(ws, nil, true)
	if len(ids) != 1 || ids[0] != "solo-item" {
		t.Fatalf("ids = %v, want the lane-scope fallback preserved", ids)
	}
}

// The declined-menu contract survives the union (fourth rule branch): a
// PRESENT decision that committed zero ids and says nothing about the scope
// id keeps it pickable — lane-scope must not override an explicit empty
// commitment. (The postship promotion site pins the same contract end-to-end
// in TestPromoteInbox_EmptyCommittedDeclinedMenuStaysOpen.)
func TestCommittedInboxIDs_DeclinedMenuUnmentionedScopeStaysPickable(t *testing.T) {
	ws := t.TempDir()
	writeLaneScope(t, ws, `"declined-item"`)
	body := []byte(`{"schema_version":1,"top_n":[],"deferred":[],"dropped":[]}`)
	if ids := committedInboxIDs(ws, body, true); len(ids) != 0 {
		t.Fatalf("ids = %v — an explicit zero-commitment decision must keep the unmentioned scope id open", ids)
	}
}

// N1 (design review): lane scopes are multi-item MENUS — triage may commit a
// subset and leave a menu mate pending as dispatchable backlog. When triage
// engaged the scope BY NAME, the unmentioned mate must NOT be consumed.
func TestCommittedInboxIDs_PendingMenuMateStaysPickable(t *testing.T) {
	ws := t.TempDir()
	writeLaneScope(t, ws, `"worked-item","pending-mate"`)
	body := []byte(`{"schema_version":1,"top_n":[{"id":"worked-item"}]}`)
	got := map[string]bool{}
	for _, id := range committedInboxIDs(ws, body, true) {
		got[id] = true
	}
	if got["pending-mate"] {
		t.Fatalf("an unworked menu mate was consumed — false close of queued work")
	}
	if !got["worked-item"] {
		t.Fatalf("the named scope id must consume")
	}
}

// N2 (design review): the triage persona routes VALID work into dropped[]
// (requires-split, out-of-scope). Only close-class reasons may consume; any
// unknown reason keeps the item — forgetting a live todo is worse than
// carrying a stale one.
func TestCommittedInboxIDs_DropReasonGate(t *testing.T) {
	for _, tc := range []struct {
		reason  string
		consume bool
	}{
		{"already-shipped: PR #479 fceaf39d", true},
		{"duplicate of other-item", true},
		{"superseded by the ADR-0090 design", true},
		{"requires-split", false},
		{"out-of-scope for this lane", false},
		{"needs redesign first", false},
		{"", false},
	} {
		t.Run(tc.reason, func(t *testing.T) {
			ws := t.TempDir()
			writeLaneScope(t, ws, `"the-item"`)
			body := []byte(`{"schema_version":1,"top_n":[],"dropped":[{"id":"the-item","reason":"` + tc.reason + `"}]}`)
			ids := committedInboxIDs(ws, body, true)
			got := len(ids) == 1 && ids[0] == "the-item"
			if got != tc.consume {
				t.Fatalf("reason %q: consumed=%v, want %v (ids=%v)", tc.reason, got, tc.consume, ids)
			}
		})
	}
}

// N3 (design review): the widened union is PASS-only. A WARN landing promotes
// exactly the pre-union set — partial work stays pickable.
func TestCommittedInboxIDs_WarnLandingGetsNoScopeUnion(t *testing.T) {
	ws := t.TempDir()
	writeLaneScope(t, ws, `"scoped-item"`)
	body := []byte(`{"schema_version":1,"top_n":[{"id":"named-item"}],"dropped":[{"id":"scoped-item","reason":"already-shipped"}]}`)
	ids := committedInboxIDs(ws, body, false)
	if len(ids) != 1 || ids[0] != "named-item" {
		t.Fatalf("ids = %v — a WARN landing must resolve only the triage-named set", ids)
	}
}
