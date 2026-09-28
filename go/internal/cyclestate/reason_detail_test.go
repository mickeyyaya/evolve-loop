package cyclestate

import "testing"

func TestWithDetail_WithoutDetail_KeepsTheIdentityAndDropsTheDetail(t *testing.T) {
	reason := "predicate execution tree includes undeclared inputs"
	full := WithDetail(reason, "docs/explain/builds/cycle-9-run.md")
	if full == reason || WithoutDetail(full) != reason {
		t.Fatalf("WithDetail(%q) = %q; WithoutDetail gives %q", reason, full, WithoutDetail(full))
	}
	if WithDetail(reason, "") != reason {
		t.Fatalf("an empty detail adds no marker: %q", WithDetail(reason, ""))
	}
	if WithoutDetail(reason) != reason {
		t.Fatalf("a reason without detail is its own identity: %q", WithoutDetail(reason))
	}
	joined := WithDetail("first", "a.go, b.go") + "; " + WithDetail("second", "c.go") + "; third"
	if got := WithoutDetail(joined); got != "first; second; third" {
		t.Fatalf("every detail span is removed and the reasons after it survive: %q", got)
	}
}
