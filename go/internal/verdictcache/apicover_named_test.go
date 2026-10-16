package verdictcache

import "testing"

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

	if err := s.Put(Entry{Verdict: "PASS"}); err != nil {
		t.Fatalf("Put(empty TreeSHA): %v", err)
	}
	if _, ok := s.Lookup(""); ok {
		t.Error("Lookup(\"\") must miss — empty-TreeSHA Put is a no-op")
	}
}
