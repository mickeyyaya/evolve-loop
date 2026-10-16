package verdictcache

import (
	"testing"
	"time"
)

func TestReusable_Vocabulary(t *testing.T) {
	t.Parallel()
	for verdict, want := range map[string]bool{
		"PASS":    true,
		"WARN":    true,
		"FAIL":    false,
		"SKIPPED": false,
		"":        false,
		"banana":  false,
		"pass":    false,
	} {
		if got := Reusable(verdict); got != want {
			t.Errorf("Reusable(%q) = %v, want %v", verdict, got, want)
		}
	}
}

func TestPut_RejectsNonReusableVerdict(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir(), func() time.Time { return time.Unix(0, 0).UTC() })
	if err := s.Put(Entry{TreeSHA: "tree-abc", Verdict: "FAIL"}); err == nil {
		t.Error("Put accepted a FAIL verdict — a rejection must never become a reusable cache entry")
	}
	if _, ok := s.Lookup("tree-abc"); ok {
		t.Error("a rejected Put must leave nothing behind in the store")
	}
}

func TestPut_AcceptsReusableVerdicts(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir(), func() time.Time { return time.Unix(0, 0).UTC() })
	for _, v := range []string{"PASS", "WARN"} {
		if err := s.Put(Entry{TreeSHA: "tree-" + v, Verdict: v}); err != nil {
			t.Errorf("Put(%s) = %v, want nil", v, err)
		}
		if _, ok := s.Lookup("tree-" + v); !ok {
			t.Errorf("Put(%s) stored nothing", v)
		}
	}
}

func TestPut_EmptyTreeSHAStaysNoOp(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir(), func() time.Time { return time.Unix(0, 0).UTC() })
	if err := s.Put(Entry{TreeSHA: "", Verdict: "FAIL"}); err != nil {
		t.Errorf("Put with empty TreeSHA = %v, want nil (no-op precedes vocabulary)", err)
	}
}
