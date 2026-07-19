// mergerung2_consolidation_test.go — RED contract for cycle-946's
// reconcile-rung2-duplicate-implementations task (fleet-scoped todo
// merge-rung2-scoped-merge-review, campaign merge-efficiency-2026-07).
//
// go/internal/core carries TWO unreconciled rung-2 implementations of the
// same concept ("which hunks intersect between an audited diff and a
// composed diff"):
//
//   - mergerung2.go's intersectingHunks/RunScopedMergeReview (cycle-941) —
//     the PRODUCTION-WIRED path: compares each hunk's OLD-side (pre-image)
//     line range and is the one recoverFromShipError actually dispatches to
//     (scopedMergeCarryForward, composition_carryforward.go).
//   - composition_scoped_review.go's IntersectingHunks (cycle-942) — built
//     and unit/ACS-tested (go/acs/cycle942/predicates_test.go) but never
//     wired into any production call site; it compares each hunk's NEW-side
//     (post-image) line range instead.
//
// Because the two compare DIFFERENT coordinate spaces, they can disagree on
// whether the same diff pair overlaps at all — a hunk pair whose old-side
// ranges intersect (dispatched for review on the real, wired path) can have
// disjoint new-side ranges (silently skipped as "no conflict" on the unwired
// path). This test pins that divergence as a failing (RED) contract: the
// Builder's consolidation must make both call sites agree on ONE overlap
// semantic (the production-wired old-side comparison), not merely delete
// one file and hope the ACS-pinned public API (IntersectingHunks,
// ScopedReviewVerdict, ScopedReviewMethod, ReverifyResolution — see
// go/acs/cycle942/predicates_test.go) keeps behaving as before.
//
// RED today: composition_scoped_review.IntersectingHunks reports the fixture
// pair as NON-overlapping while mergerung2's canonical intersectingHunks
// reports it as overlapping (see the Errorf message for the exact
// assertion). This is a genuine behavioral divergence, not a source-text
// grep — both implementations are invoked and asserted on their real output.
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
