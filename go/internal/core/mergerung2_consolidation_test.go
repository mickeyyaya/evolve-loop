package core

import "testing"

// TestScopedReviewImplementations_MustAgreeOnOverlap builds one audited hunk
// (old-side [10,13), new-side [10,13)) and one composed hunk in the same
// file (old-side [12,16), new-side [20,24)): the old-side ranges intersect
// at line 12 (a real overlapping edit once line-shifting from earlier,
// unrelated insertions is accounted for) but the new-side ranges do not.
func TestScopedReviewImplementations_MustAgreeOnOverlap(t *testing.T) {
	auditedDiff := []byte("diff --git a/foo.go b/foo.go\n" +
		"--- a/foo.go\n+++ b/foo.go\n" +
		"@@ -10,3 +10,3 @@ func A() {\n" +
		" a\n-old1\n-old2\n+new1\n b\n")
	composedDiff := []byte("diff --git a/foo.go b/foo.go\n" +
		"--- a/foo.go\n+++ b/foo.go\n" +
		"@@ -12,4 +20,4 @@ func B() {\n" +
		" c\n-oldc1\n+newc1\n+newc2\n d\n")

	audited, err := parseUnifiedDiffToHunks(auditedDiff)
	if err != nil {
		t.Fatalf("parse audited fixture: %v", err)
	}
	composed, err := parseUnifiedDiffToHunks(composedDiff)
	if err != nil {
		t.Fatalf("parse composed fixture: %v", err)
	}
	canonical := intersectingHunks(audited, composed)
	if len(canonical) == 0 {
		t.Fatalf("fixture bug: the production-wired (old-side) intersectingHunks reports NO overlap for a fixture designed to overlap on the old side — fix the fixture, not the assertion below")
	}

	scoped := IntersectingHunks(auditedDiff, composedDiff)
	if len(scoped) == 0 {
		t.Errorf("composition_scoped_review.IntersectingHunks (new-side comparison) reports NO overlap for a hunk pair the production-wired mergerung2.intersectingHunks (old-side comparison) DOES dispatch for scoped review — the two duplicate rung-2 implementations disagree on what \"intersecting\" means, so the same edit could silently skip review under one code path while triggering it under the other; reconcile on a single implementation/semantic")
	}
}
