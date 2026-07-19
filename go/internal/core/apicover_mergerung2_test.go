package core

// apicover_mergerung2_test.go — ADR-0050 Phase 5 public-API coverage for the
// RUNG-2 scoped-merge review surface (mergerung2.go): NAMES + EXERCISES the
// three exports the apicover per-symbol gate flagged uncovered on main
// (ScopedMergeDisposition, ScopedMergeResult, ScopedMergeReviewer) after cycle
// ship 6459f9ae landed them without a naming test. Each test asserts real
// RunScopedMergeReview behavior (Rule 9 — no `_ = pkg.X` padding).

import (
	"strings"
	"testing"
)

const scopedMergeAuditedDiff = `--- a/pkg/file.go
+++ b/pkg/file.go
@@ -1,3 +1,4 @@
 alpha
+audited-added
 beta
 gamma
`

// Overlaps the audited hunk: same file, old-side lines 2-3 vs 1-3.
const scopedMergeOverlappingDiff = `--- a/pkg/file.go
+++ b/pkg/file.go
@@ -2,2 +2,3 @@
 beta
+composed-added
 gamma
`

// Same file but old-side lines 50-51 — no overlap with lines 1-3.
const scopedMergeDisjointDiff = `--- a/pkg/file.go
+++ b/pkg/file.go
@@ -50,2 +50,3 @@
 omega
+composed-far-away
 psi
`

// TestRunScopedMergeReview_DisjointHunks_CompatibleWithoutReview — an empty
// intersection is ScopedMergeCompatible WITHOUT invoking the reviewer (no
// wasted review): the ScopedMergeReviewer func type is named by the fake, and
// ScopedMergeResult's Dispatched=false proves the reviewer stayed dark.
func TestRunScopedMergeReview_DisjointHunks_CompatibleWithoutReview(t *testing.T) {
	t.Parallel()
	invoked := false
	var reviewer ScopedMergeReviewer = func(_ []MergeHunk, _, _ string) ScopedMergeReviewOutcome {
		invoked = true
		return ScopedMergeReviewOutcome{Disposition: ScopedMergeEntangled}
	}

	res, err := RunScopedMergeReview(ScopedMergeInput{
		AuditedDiff:  []byte(scopedMergeAuditedDiff),
		ComposedDiff: []byte(scopedMergeDisjointDiff),
	}, reviewer)
	if err != nil {
		t.Fatalf("RunScopedMergeReview: %v", err)
	}
	var got ScopedMergeResult = res // names ScopedMergeResult
	if got.Disposition != ScopedMergeCompatible {
		t.Errorf("disposition = %q, want %q (empty intersection composes)", got.Disposition, ScopedMergeCompatible)
	}
	if got.Dispatched || len(got.DispatchedHunks) != 0 {
		t.Errorf("empty intersection must not dispatch: Dispatched=%v hunks=%d", got.Dispatched, len(got.DispatchedHunks))
	}
	if invoked {
		t.Error("reviewer must NOT be invoked on an empty intersection")
	}
}

// TestRunScopedMergeReview_OverlappingHunks_ReviewerDispositionCarried — a
// genuine old-side overlap dispatches EXACTLY the intersecting hunks to the
// reviewer and carries its ScopedMergeDisposition + suggestion-grade
// ResolutionDiff through verbatim on the ScopedMergeResult.
func TestRunScopedMergeReview_OverlappingHunks_ReviewerDispositionCarried(t *testing.T) {
	t.Parallel()
	var seenHunks []MergeHunk
	reviewer := ScopedMergeReviewer(func(hunks []MergeHunk, auditedSummary, composedSummary string) ScopedMergeReviewOutcome {
		seenHunks = hunks
		if auditedSummary != "audited summary" || composedSummary != "composed summary" {
			t.Errorf("summaries not threaded: (%q, %q)", auditedSummary, composedSummary)
		}
		return ScopedMergeReviewOutcome{
			Disposition:    ScopedMergeEntangled,
			ResolutionDiff: []byte("suggested-resolution"),
		}
	})

	res, err := RunScopedMergeReview(ScopedMergeInput{
		AuditedDiff:     []byte(scopedMergeAuditedDiff),
		ComposedDiff:    []byte(scopedMergeOverlappingDiff),
		AuditedSummary:  "audited summary",
		ComposedSummary: "composed summary",
	}, reviewer)
	if err != nil {
		t.Fatalf("RunScopedMergeReview: %v", err)
	}
	var disp ScopedMergeDisposition = res.Disposition // names ScopedMergeDisposition
	if disp != ScopedMergeEntangled {
		t.Errorf("disposition = %q, want the reviewer's %q carried verbatim", disp, ScopedMergeEntangled)
	}
	// Exactly 2: intersectingHunks appends the audited-side then the composed-side
	// hunk of the one overlapping pair (go-review nit — pin the count, not just non-empty).
	if !res.Dispatched || len(res.DispatchedHunks) != 2 || len(seenHunks) != 2 {
		t.Fatalf("overlap must dispatch exactly the 2 intersecting hunks (audited+composed): Dispatched=%v result-hunks=%d reviewer-hunks=%d",
			res.Dispatched, len(res.DispatchedHunks), len(seenHunks))
	}
	for _, h := range res.DispatchedHunks {
		if h.File != "pkg/file.go" || !strings.HasPrefix(h.Header, "@@") {
			t.Errorf("dispatched hunk not the parsed intersection: %+v", h)
		}
	}
	if string(res.ResolutionDiff) != "suggested-resolution" {
		t.Errorf("ResolutionDiff = %q, want the reviewer's suggestion carried through", res.ResolutionDiff)
	}
}
