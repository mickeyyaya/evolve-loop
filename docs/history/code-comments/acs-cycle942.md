# Comment history: `acs/cycle942`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle942/predicates_test.go:3` — above `package cycle942`

```text
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
```
