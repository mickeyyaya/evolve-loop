package verdictcache

import (
	"fmt"
	"testing"
	"time"
)

func TestPut_EvictsOldestBeyondCap(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	s := NewStore(t.TempDir(), nil).Keep(3)
	for i := 0; i < 4; i++ {
		if err := s.Put(Entry{TreeSHA: fmt.Sprintf("tree-%d", i), Verdict: "PASS", CachedAt: base.Add(time.Duration(i) * time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := s.Load()
	if len(got) != 3 {
		t.Fatalf("%d entries after K+1 puts, want K=3", len(got))
	}
	if _, ok := got["tree-0"]; ok {
		t.Error("the oldest entry survived the cap")
	}
}

func TestLookup_MissAfterEviction(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	s := NewStore(t.TempDir(), nil).Keep(1)
	_ = s.Put(Entry{TreeSHA: "old", Verdict: "PASS", CachedAt: base})
	_ = s.Put(Entry{TreeSHA: "new", Verdict: "WARN", CachedAt: base.Add(time.Minute)})
	if _, ok := s.Lookup("old"); ok {
		t.Error("an evicted tree still hits")
	}
	if e, ok := s.Lookup("new"); !ok || e.Verdict != "WARN" {
		t.Errorf("the newest entry = %+v, %v", e, ok)
	}
}

func TestKeep_AZeroValueStoreKeepsTheGCPolicyDefault(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	s := NewStore(t.TempDir(), nil)
	for i := 0; i < 51; i++ {
		_ = s.Put(Entry{TreeSHA: fmt.Sprintf("t%02d", i), Verdict: "PASS", CachedAt: base.Add(time.Duration(i) * time.Second)})
	}
	if got, _ := s.Load(); len(got) != 50 {
		t.Errorf("%d entries, want the 50-entry default of gc.verdict_cache_keep", len(got))
	}
	if got, _ := NewStore(t.TempDir(), nil).Keep(0).Load(); len(got) != 0 {
		t.Errorf("an empty store holds %d entries", len(got))
	}
}
