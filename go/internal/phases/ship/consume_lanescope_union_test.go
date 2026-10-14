package ship

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

func TestCommittedInboxIDs_UnionsLaneScopeWhenTriageDroppedTheScope(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeLaneScope(t, ws, `"premise-challenge-fail-never-reaches-failure-learning"`)
	body := []byte(`{"schema_version":1,"top_n":[],"deferred":[],
		"dropped":[{"id":"premise-challenge-fail-never-reaches-failure-learning","reason":"already-shipped"}]}`)
	ids := committedInboxIDs(ws, body, true)
	if len(ids) != 1 || ids[0] != "premise-challenge-fail-never-reaches-failure-learning" {
		t.Fatalf("ids = %v — a PASS lane ship must retire its assigned scope id even when triage dropped it (cycle-1552)", ids)
	}
}

func TestCommittedInboxIDs_UnionsLaneScopeWithDecomposedTopN(t *testing.T) {
	t.Parallel()
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

func TestCommittedInboxIDs_TriageDeferredScopeIDStaysPickable(t *testing.T) {
	t.Parallel()
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

func TestCommittedInboxIDs_NilBodyStillResolvesLaneScope(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeLaneScope(t, ws, `"solo-item"`)
	ids := committedInboxIDs(ws, nil, true)
	if len(ids) != 1 || ids[0] != "solo-item" {
		t.Fatalf("ids = %v, want the lane-scope fallback preserved", ids)
	}
}

func TestCommittedInboxIDs_DeclinedMenuUnmentionedScopeStaysPickable(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeLaneScope(t, ws, `"declined-item"`)
	body := []byte(`{"schema_version":1,"top_n":[],"deferred":[],"dropped":[]}`)
	if ids := committedInboxIDs(ws, body, true); len(ids) != 0 {
		t.Fatalf("ids = %v — an explicit zero-commitment decision must keep the unmentioned scope id open", ids)
	}
}

func TestCommittedInboxIDs_PendingMenuMateStaysPickable(t *testing.T) {
	t.Parallel()
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

func TestCommittedInboxIDs_DropReasonGate(t *testing.T) {
	t.Parallel()
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
		{"stale: superseded by #535 (acssuite.ReadVerdict)", false},
		{"stale", false},
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

func TestCommittedInboxIDs_WarnLandingGetsNoScopeUnion(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeLaneScope(t, ws, `"scoped-item"`)
	body := []byte(`{"schema_version":1,"top_n":[{"id":"named-item"}],"dropped":[{"id":"scoped-item","reason":"already-shipped"}]}`)
	ids := committedInboxIDs(ws, body, false)
	if len(ids) != 1 || ids[0] != "named-item" {
		t.Fatalf("ids = %v — a WARN landing must resolve only the triage-named set", ids)
	}
}
