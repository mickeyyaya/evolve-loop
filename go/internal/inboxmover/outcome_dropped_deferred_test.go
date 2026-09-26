package inboxmover

import (
	"reflect"
	"testing"
)

func TestDeferredIDs(t *testing.T) {
	body := []byte(`{"deferred":[{"id":"a","reason":"later"},{"id":"a"},{"id":""},{"id":"b"}]}`)
	if got := DeferredIDs(body); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("DeferredIDs = %v, want [a b] (deduped, empties dropped)", got)
	}
	if got := DeferredIDs([]byte("not json")); got != nil {
		t.Fatalf("unmarshal error must yield nil, got %v", got)
	}
	// Only the "id" key resolves, matching the sibling dropped[] reader.
	if got := DeferredIDs([]byte(`{"deferred":[{"task_id":"x"}]}`)); len(got) != 0 {
		t.Fatalf("task_id must not resolve for deferred entries, got %v", got)
	}
}

func TestClosedDroppedIDs(t *testing.T) {
	body := []byte(`{"dropped":[
		{"id":"shipped","reason":"already-shipped: PR #479"},
		{"id":"dupe","reason":"DUPLICATE of other"},
		{"id":"split-me","reason":"requires-split"},
		{"id":"foreign","reason":"out-of-scope for this lane"},
		{"id":"mystery","reason":""},
		{"id":"premise","reason":"stale: superseded by #535"},
		{"id":"split-stale","reason":"requires-split (stale)"},
		{"id":"replaced","reason":"superseded: by #535"},
		{"id":"hyphen-joined","reason":"already-shipped-in-797b8518"},
		{"id":"invented","reason":"stale-completed"},
		{"id":"shipped"}
	]}`)
	// Real triage-corpus shapes: a reason is classified by its leading tag, so a
	// hyphen-joined close-class tag retires and anything leading with "stale" stays queued.
	if got := ClosedDroppedIDs(body); !reflect.DeepEqual(got, []string{"shipped", "dupe", "replaced", "hyphen-joined"}) {
		t.Fatalf("ClosedDroppedIDs = %v, want [shipped dupe replaced hyphen-joined] — close-class leading tags only, case-insensitive, deduped", got)
	}
	if got := ClosedDroppedIDs([]byte("not json")); got != nil {
		t.Fatalf("unmarshal error must yield nil, got %v", got)
	}
}
