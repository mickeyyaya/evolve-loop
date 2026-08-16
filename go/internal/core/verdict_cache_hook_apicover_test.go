package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/verdictcache"
)

// TestWithVerdictCacheLookupHook_AppliesObserver names AND executes
// WithVerdictCacheLookupHook in the DEFAULT (untagged) build. The option's only
// other users are `integration`-tagged tests, so the repo-wide apicover gate
// (ADR-0069) scored it FALSE-GREEN — named by a test but 0% executed. This test
// applies the returned Option to an Orchestrator and drives the installed hook,
// pinning that the option actually reaches o.verdictCacheLookupHook, the field
// the pre-loop shadow probe calls (orchestrator.go).
func TestWithVerdictCacheLookupHook_AppliesObserver(t *testing.T) {
	var o Orchestrator
	if o.verdictCacheLookupHook != nil {
		t.Fatal("hook must be nil before the option is applied")
	}

	var gotSHA string
	var gotSkipped, gotMatched bool
	var entry verdictcache.Entry
	WithVerdictCacheLookupHook(func(sha string, skipped, matched bool, e verdictcache.Entry) {
		gotSHA, gotSkipped, gotMatched, entry = sha, skipped, matched, e
	})(&o)

	if o.verdictCacheLookupHook == nil {
		t.Fatal("WithVerdictCacheLookupHook did not install the hook")
	}
	o.verdictCacheLookupHook("tree-abc", false, true, verdictcache.Entry{TreeSHA: "tree-abc", Cycle: 7, Verdict: VerdictPASS})

	if gotSHA != "tree-abc" || gotSkipped || !gotMatched {
		t.Errorf("observation = (%q, skipped=%t, matched=%t), want (tree-abc, false, true)", gotSHA, gotSkipped, gotMatched)
	}
	if entry.Cycle != 7 || entry.Verdict != VerdictPASS {
		t.Errorf("entry = %+v, want cycle 7 verdict %s", entry, VerdictPASS)
	}
}
