package core

// apicover naming cover for the ScopePathProbe wiring window.

import "testing"

func TestScopePathProbe_NilResolverReportsUnwired(t *testing.T) {
	o := &Orchestrator{}
	if _, wired := o.ScopePathProbe("/p", "id"); wired {
		t.Fatalf("no resolver means unwired")
	}
	o.scopePathFor = func(_, id string) string { return "/live/" + id }
	got, wired := o.ScopePathProbe("/p", "id")
	if !wired || got != "/live/id" {
		t.Fatalf("probe must expose the wired resolver's answer; got (%q,%v)", got, wired)
	}
}
