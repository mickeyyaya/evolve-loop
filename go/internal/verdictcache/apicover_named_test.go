package verdictcache

import "testing"

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
