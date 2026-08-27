package verdictcache

import (
	"testing"
	"time"
)

// reusable_test.go — cycle-1571 H2. The cacheable-verdict vocabulary was
// documented on Entry.Verdict ("PASS or WARN") and enforced nowhere: Put
// accepted any string, and the FAIL exclusion lived as an inline `verdict ==
// VerdictFAIL` check in core/phase_bindings.go. Two consumers now need the same
// rule — the cache write side and the RUNG 0 composition snapshot, which must
// refuse to carry a REJECTED audit forward — so the rule gets one definition.

func TestReusable_Vocabulary(t *testing.T) {
	t.Parallel()
	// A verdict is reusable when a later run may skip work on its strength.
	// PASS and WARN ship; FAIL is a rejection; SKIPPED means no audit happened;
	// anything else is out-of-vocabulary and must not be trusted by default.
	for verdict, want := range map[string]bool{
		"PASS":    true,
		"WARN":    true,
		"FAIL":    false,
		"SKIPPED": false,
		"":        false,
		"banana":  false,
		"pass":    false, // vocabulary is upper-case; a case slip is not a silent PASS
	} {
		if got := Reusable(verdict); got != want {
			t.Errorf("Reusable(%q) = %v, want %v", verdict, got, want)
		}
	}
}

// TestPut_RejectsNonReusableVerdict: the store is the last line of defence for
// its own vocabulary. A poisoned entry sits in a SHARED file for every future
// consumer, and the enforce-stage lookup (ADR-0048 Slice B) would skip real
// work on its strength — so the write side fails closed, matching the
// deliberately asymmetric posture already documented at the Put call site.
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

// TestPut_EmptyTreeSHAStaysNoOp guards the pre-existing contract: no content
// identity is a silent no-op, not an error. The new verdict guard must not
// convert that into a failure for best-effort callers that do not branch.
func TestPut_EmptyTreeSHAStaysNoOp(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir(), func() time.Time { return time.Unix(0, 0).UTC() })
	if err := s.Put(Entry{TreeSHA: "", Verdict: "FAIL"}); err != nil {
		t.Errorf("Put with empty TreeSHA = %v, want nil (no-op precedes vocabulary)", err)
	}
}
