// mergerung2_test.go — RED contract for the merge ladder RUNG 2 scoped
// merge review (cycle-941, fleet-scoped todo merge-rung2-scoped-merge-review;
// knowledge-base/research/merge-concurrency-2026, MergeBERT lineage).
//
// RUNG 0 (composition_carryforward.go) carries an audit verdict forward when a
// clean fleet rebase leaves the composed diff's patch-id unchanged. When the
// patch-id DID change (real overlapping edits, not a trivial rebase), today's
// only fallback is RUNG 3 — a full re-audit. RUNG 2 is the missing middle: it
// reviews ONLY the hunks that actually intersect between the audited change and
// the composed change and, if that overlap is compatible, composes directly;
// only genuine entanglement escalates to the full re-audit.
//
// This file is the pure-core RED contract (Task A merge-rung2-scoped-review-core
// + the Task B re-entry invariant and orchestrator wiring observability). It
// references symbols the Builder must create in go/internal/core/mergerung2.go
// and the Method field on CompositionVerdictInput; at authoring every reference
// is undefined, so the package fails to compile — the correct RED. Builder
// contract: implement the documented API to turn these GREEN; DO NOT modify
// this file.
//
// Documented API the Builder must implement (go/internal/core/mergerung2.go):
//
//	type ScopedMergeDisposition string
//	const ScopedMergeCompatible ScopedMergeDisposition = "compatible"
//	const ScopedMergeEntangled  ScopedMergeDisposition = "entangled"
//	type MergeHunk struct { File, Header, Body string }
//	type ScopedMergeReviewOutcome struct {
//	    Disposition    ScopedMergeDisposition
//	    ResolutionDiff []byte // optional LLM-assisted resolution (suggestion-grade)
//	}
//	type ScopedMergeReviewer func(hunks []MergeHunk, auditedSummary, composedSummary string) ScopedMergeReviewOutcome
//	type ScopedMergeInput struct { AuditedDiff, ComposedDiff []byte; AuditedSummary, ComposedSummary string }
//	type ScopedMergeResult struct {
//	    Disposition     ScopedMergeDisposition
//	    DispatchedHunks []MergeHunk
//	    Dispatched      bool
//	}
//	func RunScopedMergeReview(in ScopedMergeInput, review ScopedMergeReviewer) (ScopedMergeResult, error)
//	func ResolutionMatchesAudited(auditedPatchID string, resolutionDiff []byte) (bool, error)
//	// plus: CompositionVerdictInput.Method string (core mirror), and on
//	// *Orchestrator: WithScopedMergeReviewer(fn) Option + ScopedMergeReviewWired() bool.
//
// Contract details Builder must honor:
//   - RunScopedMergeReview parses both diffs into hunks and computes the hunks
//     whose file+line ranges intersect. A malformed diff (a `@@` hunk header
//     that does not parse as `@@ -old,n +new,m @@`) is fail-closed: it returns a
//     non-nil error, does NOT invoke the reviewer, and does NOT report
//     compatible.
//   - Empty intersection: the reviewer is NOT invoked, Dispatched is false,
//     Disposition is compatible (nothing entangled — no wasted review).
//   - Non-empty intersection: exactly the intersecting hunks (never the
//     audited-only or composed-only hunks) are dispatched; the reviewer's
//     Disposition is carried through verbatim.
//   - ResolutionMatchesAudited recomputes the resolution diff's OWN patch-id
//     (rung-0 verification) and returns true only if it equals auditedPatchID —
//     an LLM-assisted resolution is trusted by patch-id, never on the reviewer's
//     word (MergeBERT lineage). A malformed resolution fails closed (error).
package core

import "testing"

// --- unified-diff fixtures -------------------------------------------------
//
// "shared.txt" is touched by BOTH the audited and composed change on
// overlapping line ranges (the one intersecting file). "audonly.txt" is
// audited-only; "cmponly.txt" is composed-only — neither intersects. Distinct
// filenames let the assertions test membership by substring, robust to whether
// the parser records "shared.txt" or "a/shared.txt".

const auditedDiffSharedPlusAudOnly = `diff --git a/shared.txt b/shared.txt
--- a/shared.txt
+++ b/shared.txt
@@ -1,3 +1,3 @@
 alpha
-beta
+beta-audited
 gamma
diff --git a/audonly.txt b/audonly.txt
--- a/audonly.txt
+++ b/audonly.txt
@@ -1,3 +1,3 @@
 one
-two
+two-audited
 three
`

const composedDiffSharedPlusCmpOnly = `diff --git a/shared.txt b/shared.txt
--- a/shared.txt
+++ b/shared.txt
@@ -2,3 +2,3 @@
 beta
-gamma
+gamma-composed
 delta
diff --git a/cmponly.txt b/cmponly.txt
--- a/cmponly.txt
+++ b/cmponly.txt
@@ -1,3 +1,3 @@
 red
-green
+green-composed
 blue
`

const auditedDiffAudOnly = `diff --git a/audonly.txt b/audonly.txt
--- a/audonly.txt
+++ b/audonly.txt
@@ -1,3 +1,3 @@
 one
-two
+two-audited
 three
`

const composedDiffCmpOnly = `diff --git a/cmponly.txt b/cmponly.txt
--- a/cmponly.txt
+++ b/cmponly.txt
@@ -1,3 +1,3 @@
 red
-green
+green-composed
 blue
`

const malformedHunkDiff = `diff --git a/x.txt b/x.txt
--- a/x.txt
+++ b/x.txt
@@ this is not a valid hunk header @@
 body line
`

// scopedMergeReviewSpy is a spy: it counts invocations and captures the hunks it
// was handed, returning a caller-set outcome.
type scopedMergeReviewSpy struct {
	called  int
	hunks   []MergeHunk
	outcome ScopedMergeReviewOutcome
}

func (r *scopedMergeReviewSpy) review(hunks []MergeHunk, auditedSummary, composedSummary string) ScopedMergeReviewOutcome {
	r.called++
	r.hunks = hunks
	return r.outcome
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(sub) > 0 && stringContains(s, sub) {
			return true
		}
	}
	return false
}

// stringContains is strings.Contains inlined to keep this leaf test's imports
// minimal; the assertions only need substring membership.
func stringContains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// A1 (semantic + intersecting-hunk core): only the hunks for the shared file
// are dispatched — never the audited-only or composed-only hunks.
func TestScopedReview_SeesOnlyIntersectingHunks(t *testing.T) {
	spy := &scopedMergeReviewSpy{outcome: ScopedMergeReviewOutcome{Disposition: ScopedMergeCompatible}}
	res, err := RunScopedMergeReview(ScopedMergeInput{
		AuditedDiff:     []byte(auditedDiffSharedPlusAudOnly),
		ComposedDiff:    []byte(composedDiffSharedPlusCmpOnly),
		AuditedSummary:  "audited change",
		ComposedSummary: "composed change",
	}, spy.review)
	if err != nil {
		t.Fatalf("RunScopedMergeReview: unexpected error: %v", err)
	}
	if spy.called != 1 {
		t.Fatalf("reviewer called %d times, want exactly 1 (shared.txt intersects)", spy.called)
	}
	if !res.Dispatched {
		t.Error("Dispatched=false, want true (shared.txt hunks intersect)")
	}
	if len(spy.hunks) == 0 {
		t.Fatal("reviewer received 0 hunks, want the shared.txt hunk(s)")
	}
	for _, h := range spy.hunks {
		if !stringContains(h.File, "shared") {
			t.Errorf("dispatched hunk for file %q, want only the intersecting shared.txt", h.File)
		}
		if containsAny(h.File, "audonly", "cmponly") {
			t.Errorf("dispatched a non-intersecting hunk for file %q — only intersecting hunks may be reviewed", h.File)
		}
	}
	if len(res.DispatchedHunks) != len(spy.hunks) {
		t.Errorf("res.DispatchedHunks=%d but reviewer saw %d — result must mirror what was dispatched",
			len(res.DispatchedHunks), len(spy.hunks))
	}
}

// A2 (semantic): the reviewer's compatible/entangled disposition is carried
// through verbatim — compatible composes, entangled escalates.
func TestScopedReview_CompatibleComposesEntangledEscalates(t *testing.T) {
	in := ScopedMergeInput{
		AuditedDiff:  []byte(auditedDiffSharedPlusAudOnly),
		ComposedDiff: []byte(composedDiffSharedPlusCmpOnly),
	}
	compat := func(hunks []MergeHunk, a, c string) ScopedMergeReviewOutcome {
		return ScopedMergeReviewOutcome{Disposition: ScopedMergeCompatible}
	}
	entangle := func(hunks []MergeHunk, a, c string) ScopedMergeReviewOutcome {
		return ScopedMergeReviewOutcome{Disposition: ScopedMergeEntangled}
	}
	if res, err := RunScopedMergeReview(in, compat); err != nil || res.Disposition != ScopedMergeCompatible {
		t.Errorf("compatible reviewer → disposition=%q err=%v, want %q", res.Disposition, err, ScopedMergeCompatible)
	}
	if res, err := RunScopedMergeReview(in, entangle); err != nil || res.Disposition != ScopedMergeEntangled {
		t.Errorf("entangled reviewer → disposition=%q err=%v, want %q", res.Disposition, err, ScopedMergeEntangled)
	}
}

// A3 (NEGATIVE — strongest anti-no-op): a disjoint change set has an empty
// intersection, so the reviewer is NOT consulted and the result is compatible
// (no wasted review, no false entanglement). The spy is armed to return
// entangled: if the core ever dispatched, the result would be entangled and
// this test would catch it.
func TestScopedReview_EmptyIntersectionNoDispatch(t *testing.T) {
	spy := &scopedMergeReviewSpy{outcome: ScopedMergeReviewOutcome{Disposition: ScopedMergeEntangled}}
	res, err := RunScopedMergeReview(ScopedMergeInput{
		AuditedDiff:  []byte(auditedDiffAudOnly),
		ComposedDiff: []byte(composedDiffCmpOnly),
	}, spy.review)
	if err != nil {
		t.Fatalf("RunScopedMergeReview(disjoint): unexpected error: %v", err)
	}
	if spy.called != 0 {
		t.Errorf("reviewer called %d times on empty intersection, want 0 (no wasted review)", spy.called)
	}
	if res.Dispatched {
		t.Error("Dispatched=true on empty intersection, want false")
	}
	if res.Disposition != ScopedMergeCompatible {
		t.Errorf("empty intersection → %q, want %q (nothing entangled)", res.Disposition, ScopedMergeCompatible)
	}
	if len(res.DispatchedHunks) != 0 {
		t.Errorf("DispatchedHunks=%d on empty intersection, want 0", len(res.DispatchedHunks))
	}
}

// A4 (EDGE — malformed): a corrupt hunk header is fail-closed. The reviewer is
// never invoked and the result is NOT compatible — a broken diff must never
// silently green a composition.
func TestScopedReview_MalformedDiffFailsClosed(t *testing.T) {
	spy := &scopedMergeReviewSpy{outcome: ScopedMergeReviewOutcome{Disposition: ScopedMergeCompatible}}
	res, err := RunScopedMergeReview(ScopedMergeInput{
		AuditedDiff:  []byte(malformedHunkDiff),
		ComposedDiff: []byte(composedDiffSharedPlusCmpOnly),
	}, spy.review)
	if err == nil {
		t.Fatalf("malformed audited diff: want a fail-closed error, got nil (res=%+v)", res)
	}
	if spy.called != 0 {
		t.Errorf("reviewer called %d times on malformed diff, want 0", spy.called)
	}
	if res.Disposition == ScopedMergeCompatible {
		t.Error("malformed diff reported compatible — must NOT silently green (fail-closed)")
	}
}

// B2 (MergeBERT invariant): an LLM-assisted resolution re-enters RUNG 0
// patch-id verification. A resolution whose recomputed patch-id matches the
// audited change is trusted; a different resolution is rejected regardless of
// what the reviewer claimed; a malformed resolution fails closed.
func TestLLMResolution_ReentersRung0Verification(t *testing.T) {
	// compositionPatchID is core's own patch-id (git patch-id --stable); it is
	// exactly what ResolutionMatchesAudited must recompute internally.
	auditedPID, err := compositionPatchID([]byte(auditedDiffAudOnly))
	if err != nil {
		t.Fatalf("compositionPatchID(audited): %v", err)
	}

	// Matching resolution (same change) → trusted.
	ok, err := ResolutionMatchesAudited(auditedPID, []byte(auditedDiffAudOnly))
	if err != nil {
		t.Fatalf("ResolutionMatchesAudited(match): %v", err)
	}
	if !ok {
		t.Error("a resolution whose patch-id matches the audited change was rejected — rung-0 re-entry must accept a patch-id match")
	}

	// Different resolution (different patch-id) → NOT trusted, even though a
	// reviewer might have claimed it resolves the conflict.
	ok, err = ResolutionMatchesAudited(auditedPID, []byte(composedDiffCmpOnly))
	if err != nil {
		t.Fatalf("ResolutionMatchesAudited(mismatch): %v", err)
	}
	if ok {
		t.Error("a resolution with a different patch-id was trusted — an LLM resolution must be verified by recomputed patch-id, never trusted directly")
	}

	// Malformed resolution → fail-closed (error, never a silent trust).
	ok, err = ResolutionMatchesAudited(auditedPID, []byte("not a diff at all"))
	if err == nil {
		t.Errorf("malformed resolution: want a fail-closed error, got ok=%v", ok)
	}
	if ok {
		t.Error("malformed resolution reported trusted — must fail closed")
	}
}

// B1 (wiring observability): WithScopedMergeReviewer binds the rung-2 reviewer
// closure; ScopedMergeReviewWired reports it. Nil (default) is off, so recovery
// behaves exactly as before (zero regression). Mirrors
// CompositionFastPathWired (composition_carryforward_wired_test.go).
func TestOrchestrator_ScopedMergeReviewWired(t *testing.T) {
	t.Parallel()

	dummy := func(hunks []MergeHunk, auditedSummary, composedSummary string) ScopedMergeReviewOutcome {
		return ScopedMergeReviewOutcome{Disposition: ScopedMergeCompatible}
	}

	bare := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	if bare.ScopedMergeReviewWired() {
		t.Error("bare orchestrator must report ScopedMergeReviewWired()=false (default off, no regression)")
	}

	wired := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithScopedMergeReviewer(dummy))
	if !wired.ScopedMergeReviewWired() {
		t.Error("orchestrator with WithScopedMergeReviewer must report ScopedMergeReviewWired()=true")
	}
}
