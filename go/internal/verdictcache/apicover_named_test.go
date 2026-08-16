package verdictcache

import "testing"

// TestProbeEligible_FreshBaseGuard names verdictcache.ProbeEligible and pins the
// shared fresh-base guard both production call sites (the ADR-0048 shadow probe
// and the audit-binding Put) route through: only a candidate that carries a
// content identity distinct from a RESOLVED base is cache-eligible, while an
// unresolvable base leaves the candidate eligible (frozen pre-guard behaviour).
func TestProbeEligible_FreshBaseGuard(t *testing.T) {
	cases := []struct {
		name      string
		base      string
		candidate string
		want      bool
	}{
		{name: "fresh worktree equals base", base: "tree-aaa", candidate: "tree-aaa", want: false},
		{name: "empty candidate has no identity", base: "tree-aaa", candidate: "", want: false},
		{name: "both identities empty", base: "", candidate: "", want: false},
		{name: "changed worktree", base: "tree-aaa", candidate: "tree-bbb", want: true},
		{name: "unresolvable base stays eligible", base: "", candidate: "tree-bbb", want: true},
	}
	for _, tc := range cases {
		if got := ProbeEligible(tc.base, tc.candidate); got != tc.want {
			t.Errorf("%s: ProbeEligible(%q, %q) = %t, want %t", tc.name, tc.base, tc.candidate, got, tc.want)
		}
	}
}

// TestStore_PutLookupRoundTrip names the verdictcache.Store type (NewStore
// returns *Store but the bare type is never named in a test) and pins the core
// contract: a Put'd verdict is retrievable by its tree SHA, and Put on an empty
// TreeSHA is a no-op (verdictcache.go:90) that stores nothing.
func TestStore_PutLookupRoundTrip(t *testing.T) {
	var s *Store = NewStore(t.TempDir(), fixedNow)
	want := Entry{TreeSHA: "deadbeef", Cycle: 5, Verdict: "PASS", ArtifactSHA256: "h", ArtifactPath: "audit-report.md"}
	if err := s.Put(want); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok := s.Lookup("deadbeef")
	if !ok {
		t.Fatal("Lookup miss after Put")
	}
	if got.Verdict != "PASS" || got.Cycle != 5 || got.ArtifactSHA256 != "h" {
		t.Errorf("round-trip mismatch: %+v", got)
	}

	// Put with an empty TreeSHA is a documented no-op: nothing becomes findable.
	if err := s.Put(Entry{Verdict: "PASS"}); err != nil {
		t.Fatalf("Put(empty TreeSHA): %v", err)
	}
	if _, ok := s.Lookup(""); ok {
		t.Error("Lookup(\"\") must miss — empty-TreeSHA Put is a no-op")
	}
}
