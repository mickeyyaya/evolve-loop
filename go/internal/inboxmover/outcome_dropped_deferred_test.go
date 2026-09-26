package inboxmover

// Direct contract pins for the dropped[]/deferred[] readers (apicover-enforce:
// the exported surface must be named in-package; the production-path tests
// live in phases/ship where committedInboxIDs composes these).

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
	// id-key ONLY — the core sibling reader parses the same key; a task_id
	// spelling here must NOT resolve (the divergence the design review closed).
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
	// A stale drop never retires (F40 C1: the console confirms a premise
	// re-check), and a reason is classified by its LEADING tag — "stale:
	// superseded by #535" is stale, "requires-split (stale)" a split.
	// Real triage-corpus shapes (F40 re-review MINOR 1): a hyphen-joined
	// close-class tag still retires (6 of 16 real close-class reasons are
	// hyphen-joined — an exact-match rule would drop them), while an invented
	// "stale-completed" leads with stale and stays queued for the console.
	if got := ClosedDroppedIDs(body); !reflect.DeepEqual(got, []string{"shipped", "dupe", "replaced", "hyphen-joined"}) {
		t.Fatalf("ClosedDroppedIDs = %v, want [shipped dupe replaced hyphen-joined] — close-class leading tags only, case-insensitive, deduped", got)
	}
	if got := ClosedDroppedIDs([]byte("not json")); got != nil {
		t.Fatalf("unmarshal error must yield nil, got %v", got)
	}
}
