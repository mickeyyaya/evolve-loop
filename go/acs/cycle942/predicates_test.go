//go:build acs

// Package cycle942 materializes the cycle-942 acceptance criteria for this
// fleet lane's sole committed task, merge-rung2-scoped-merge-review (inbox
// weight 0.987, campaign merge-efficiency-2026-07, ladder rung 2 of 4).
//
// Goal: when a fleet-rebase composition is NOT a clean rung-0 carry-forward
// (composed patch-id drifts from the pre-rebase audited snapshot) and the
// changes have overlapping file footprints, dispatch a SCOPED merge-review of
// ONLY the conflicting hunks instead of a full re-audit (rung 3). A reviewer
// verdict of "compatible" composes (composition-verdict{method:scoped-review}
// + native gates); "entangled" escalates to today's full re-audit. Any
// LLM-assisted conflict RESOLUTION produces a new tree that must re-enter
// rung-0 verification (patch-id of the resolved diff vs the audited diff) —
// LLM merges are suggestion-grade per MergeBERT-lineage evidence, verified,
// never trusted.
//
// Every predicate below EXERCISES THE SYSTEM UNDER TEST — it invokes the new
// core functions/methods and asserts on their return values. None is a
// source-grep predicate (the cycle-85 degenerate-predicate failure mode is
// avoided). RED today is a COMPILE FAILURE: the four exported symbols below do
// not yet exist in package core. The Builder makes them GREEN by implementing
// the SUT CONTRACT — WITHOUT modifying this file.
//
// SUT CONTRACT the Builder must implement in go/internal/core (see
// test-report.md handoff). All symbols are exported so this external ACS
// package can exercise them AND so the apicover gate sees normal-suite
// coverage the Builder adds alongside:
//
//	// IntersectingHunks returns a SCOPED unified diff: for every file present
//	// in BOTH diffs, only the hunks from composedDiff whose line ranges
//	// overlap a hunk in auditedDiff (the conflict regions), with each file's
//	// diff header preserved. Non-overlapping hunks, and files present in only
//	// one diff, are excluded. Empty result (no bytes) when the footprints do
//	// not intersect. This is the reviewer payload — conflict regions only,
//	// never the full composedDiff.
//	func IntersectingHunks(auditedDiff, composedDiff []byte) []byte
//
//	// ScopedReviewVerdict is the two-value rung-2 reviewer enum.
//	type ScopedReviewVerdict string
//	const (
//	    ScopedReviewCompatible ScopedReviewVerdict = "compatible"
//	    ScopedReviewEntangled  ScopedReviewVerdict = "entangled"
//	)
//	// Composes reports whether the verdict permits composition (skip
//	// re-audit). ONLY "compatible" composes; "entangled" and any other
//	// (unknown) value fall through to full re-audit — fail-closed.
//	func (v ScopedReviewVerdict) Composes() bool
//
//	// ScopedReviewMethod is the composition-verdict method tag rung 2 writes
//	// when a compatible verdict composes.
//	const ScopedReviewMethod = "scoped-review"
//
//	// ReverifyResolution recomputes the patch-id of an LLM-proposed conflict-
//	// resolution diff and reports whether it matches the audited diff's
//	// patch-id (rung-0 re-entry). Match (true) => the resolution preserved the
//	// audited semantics and may compose; mismatch (false) => the resolution
//	// changed semantics and MUST fall through to full re-audit. Reuses the
//	// existing compositionPatchID helper.
//	func ReverifyResolution(auditedDiff, resolvedDiff []byte) (bool, error)
package cycle942

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// --- diff fixtures ----------------------------------------------------------
//
// Two changes to foo.go. The audited change and the composed change both touch
// the func A() region around new-side line 10 (they OVERLAP). The composed
// change additionally touches func B() around line 50 (DISJOINT from audited).
// The added-line markers are unique so a predicate can prove which hunks made
// it into the scoped payload.

const auditedFooDiff = "diff --git a/foo.go b/foo.go\n" +
	"--- a/foo.go\n+++ b/foo.go\n" +
	"@@ -10,3 +10,4 @@ func A() {\n" +
	" \ta := 1\n \tb := 2\n+\tAUDITED_LINE := 3\n \treturn\n"

// composedFooDiff overlaps audited at func A() (hunk A) and adds a disjoint
// hunk at func B() (hunk B).
const composedFooDiff = "diff --git a/foo.go b/foo.go\n" +
	"--- a/foo.go\n+++ b/foo.go\n" +
	"@@ -10,3 +10,4 @@ func A() {\n" +
	" \ta := 1\n \tb := 2\n+\tCOMPOSED_HUNK_A := 3\n \treturn\n" +
	"@@ -50,2 +51,3 @@ func B() {\n" +
	" \tx := 1\n+\tCOMPOSED_HUNK_B := 2\n \ty := 2\n"

// composedDisjointDiff touches ONLY func B() — no overlap with the audited
// func A() hunk at all.
const composedDisjointDiff = "diff --git a/foo.go b/foo.go\n" +
	"--- a/foo.go\n+++ b/foo.go\n" +
	"@@ -50,2 +51,3 @@ func B() {\n" +
	" \tx := 1\n+\tCOMPOSED_HUNK_B := 2\n \ty := 2\n"

// resolvedInjectedDiff is a would-be LLM resolution of the audited hunk that
// SILENTLY ADDS a line the audit never reviewed — its patch-id must differ.
const resolvedInjectedDiff = "diff --git a/foo.go b/foo.go\n" +
	"--- a/foo.go\n+++ b/foo.go\n" +
	"@@ -10,3 +10,5 @@ func A() {\n" +
	" \ta := 1\n \tb := 2\n+\tAUDITED_LINE := 3\n+\tINJECTED := 99\n \treturn\n"

// -----------------------------------------------------------------------------
// AC-1 — the scoped-review payload contains the CONFLICT REGIONS only, not the
//        full composed diff.
// -----------------------------------------------------------------------------

// TestC942_001_ScopedReviewSeesOnlyIntersectingHunks pins the inbox's first RED
// criterion (TestScopedReview_SeesOnlyIntersectingHunks): IntersectingHunks
// returns composed hunk A (which overlaps the audited hunk) but EXCLUDES the
// disjoint composed hunk B. Positive assertion: hunk A present. Negative /
// anti-no-op assertion: hunk B absent — a function that returned the whole
// composedDiff would leak hunk B and fail here.
func TestC942_001_ScopedReviewSeesOnlyIntersectingHunks(t *testing.T) {
	scoped := string(core.IntersectingHunks([]byte(auditedFooDiff), []byte(composedFooDiff)))
	if !strings.Contains(scoped, "COMPOSED_HUNK_A") {
		t.Errorf("scoped payload dropped the intersecting hunk (COMPOSED_HUNK_A absent):\n%s", scoped)
	}
	if strings.Contains(scoped, "COMPOSED_HUNK_B") {
		t.Errorf("scoped payload leaked the DISJOINT hunk (COMPOSED_HUNK_B present) — not scoped to conflict regions:\n%s", scoped)
	}
}

// TestC942_002_DisjointHunksProduceEmptyScope is the EDGE / anti-no-op
// predicate: when the composed change touches only a region the audit never
// touched (func B()), the intersecting-hunk set is EMPTY — there is no conflict
// region to review. A no-op that echoes composedDiff back would carry
// COMPOSED_HUNK_B and fail. Asserts the result is empty (whitespace-only).
func TestC942_002_DisjointHunksProduceEmptyScope(t *testing.T) {
	scoped := string(core.IntersectingHunks([]byte(auditedFooDiff), []byte(composedDisjointDiff)))
	if strings.TrimSpace(scoped) != "" {
		t.Errorf("disjoint footprints produced a non-empty scoped payload (want empty):\n%s", scoped)
	}
}

// -----------------------------------------------------------------------------
// AC-2 — compatible composes (method:scoped-review); entangled escalates to
//        full re-audit.
// -----------------------------------------------------------------------------

// TestC942_003_CompatibleComposesEntangledEscalates pins the inbox's second RED
// criterion (TestScopedReview_CompatibleComposesEntangledEscalates): the
// compatible verdict composes (skip re-audit) and the entangled verdict does
// NOT (falls through to today's full re-audit). Also pins the method tag the
// composition-verdict carries.
func TestC942_003_CompatibleComposesEntangledEscalates(t *testing.T) {
	if !core.ScopedReviewCompatible.Composes() {
		t.Errorf("ScopedReviewCompatible.Composes() = false, want true (compatible must skip re-audit)")
	}
	if core.ScopedReviewEntangled.Composes() {
		t.Errorf("ScopedReviewEntangled.Composes() = true, want false (entangled must escalate to full re-audit)")
	}
	if core.ScopedReviewMethod != "scoped-review" {
		t.Errorf("ScopedReviewMethod = %q, want %q", core.ScopedReviewMethod, "scoped-review")
	}
}

// TestC942_004_UnknownVerdictFailsClosed is the anti-no-op guard for the verdict
// router: an UNKNOWN verdict value must NOT compose — Composes cannot be a
// hardcoded `return true`. Fail-closed is the rung-0 contract (this can only
// narrow, never widen, what ships).
func TestC942_004_UnknownVerdictFailsClosed(t *testing.T) {
	if core.ScopedReviewVerdict("garbage").Composes() {
		t.Errorf("an unknown verdict composed — Composes() is not fail-closed")
	}
}

// -----------------------------------------------------------------------------
// AC-3 — an LLM-assisted resolution re-enters rung-0 (patch-id) verification;
//        a resolution that changed semantics is rejected, not trusted.
// -----------------------------------------------------------------------------

// TestC942_005_LLMResolutionReentersRung0Verification pins the inbox's third RED
// criterion (TestLLMResolution_ReentersRung0Verification). Positive: a resolved
// diff byte-identical to the audited diff re-verifies (patch-ids match => may
// compose). Negative / anti-no-op: a resolved diff that silently injected a line
// the audit never reviewed has a DIFFERENT patch-id and is REJECTED (must fall
// through to full re-audit). A no-op that always trusts the resolution fails the
// negative case.
func TestC942_005_LLMResolutionReentersRung0Verification(t *testing.T) {
	ok, err := core.ReverifyResolution([]byte(auditedFooDiff), []byte(auditedFooDiff))
	if err != nil {
		t.Fatalf("ReverifyResolution(identical) errored: %v", err)
	}
	if !ok {
		t.Errorf("ReverifyResolution(identical) = false, want true (a semantics-preserving resolution re-verifies)")
	}

	rejected, err := core.ReverifyResolution([]byte(auditedFooDiff), []byte(resolvedInjectedDiff))
	if err != nil {
		t.Fatalf("ReverifyResolution(injected) errored: %v", err)
	}
	if rejected {
		t.Errorf("ReverifyResolution(injected) = true, want false — a resolution that changed semantics must be rejected, not trusted")
	}
}
