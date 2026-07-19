// composition_scoped_review.go — RUNG 2 of the merge ladder (campaign
// merge-efficiency-2026-07, inbox weight 0.987).
//
// When a fleet-rebase composition is NOT a clean rung-0 carry-forward (the
// composed patch-id drifts from the pre-rebase audited snapshot) and the
// changes still have overlapping file footprints, rung 2 dispatches a SCOPED
// merge-review of ONLY the conflicting hunks instead of a full re-audit
// (rung 3). This file holds the pure, adapter-agnostic building blocks that
// logic needs:
//
//   - IntersectingHunks  — computes the reviewer payload (conflict regions only)
//   - ScopedReviewVerdict — the compatible|entangled reviewer enum + routing
//   - ReverifyResolution  — rung-0 re-entry for LLM-proposed resolutions
//
// Overlap detection delegates to the CANONICAL, production-wired primitives in
// mergerung2.go (parseUnifiedDiffToHunks + rangesOverlap, OLD-side pre-image
// comparison) so this file and RunScopedMergeReview can never disagree on what
// "intersecting" means — the two rung-2 code paths share one overlap semantic
// (see mergerung2_consolidation_test.go for the regression contract). This file
// keeps the byte-payload shape and the exported names the cycle-942 ACS
// predicates pin; only the overlap engine is reconciled. LLM merges are
// suggestion-grade (MergeBERT-lineage evidence) — verified via patch-id,
// never trusted.
package core

import "bytes"

// ScopedReviewVerdict is the two-value rung-2 reviewer enum.
type ScopedReviewVerdict string

const (
	// ScopedReviewCompatible means the reviewer judged the intersecting hunks
	// safe to compose — skip the full re-audit.
	ScopedReviewCompatible ScopedReviewVerdict = "compatible"
	// ScopedReviewEntangled means the intersecting hunks interact in a way the
	// scoped review cannot clear — escalate to today's full re-audit.
	ScopedReviewEntangled ScopedReviewVerdict = "entangled"
)

// ScopedReviewMethod is the composition-verdict method tag rung 2 writes when a
// compatible verdict composes (distinguishes it from rung 0's implicit
// clean-carry-forward verdict).
const ScopedReviewMethod = "scoped-review"

// Composes reports whether the verdict permits composition (skipping the full
// re-audit). ONLY ScopedReviewCompatible composes; ScopedReviewEntangled and
// any unknown value fall through to full re-audit — fail-closed, matching
// rung 0's contract that this path can only narrow, never widen, what ships.
func (v ScopedReviewVerdict) Composes() bool {
	return v == ScopedReviewCompatible
}

// ReverifyResolution recomputes the patch-id of an LLM-proposed conflict-
// resolution diff and reports whether it matches the audited diff's patch-id
// (rung-0 re-entry). Match (true) => the resolution preserved the audited
// semantics and may compose; mismatch (false) => the resolution changed
// semantics and MUST fall through to full re-audit. Reuses the existing
// compositionPatchID helper — LLM merges are verified, never trusted.
func ReverifyResolution(auditedDiff, resolvedDiff []byte) (bool, error) {
	auditedID, err := compositionPatchID(auditedDiff)
	if err != nil {
		return false, err
	}
	resolvedID, err := compositionPatchID(resolvedDiff)
	if err != nil {
		return false, err
	}
	return auditedID == resolvedID, nil
}

// IntersectingHunks returns a SCOPED unified diff: for every file present in
// BOTH diffs, only the hunks from composedDiff whose OLD-side (pre-image) line
// ranges overlap a hunk in auditedDiff (the conflict regions), with a
// synthesized `diff --git` file header preserved. Non-overlapping hunks, and
// files present in only one diff, are excluded. The result is empty (no bytes)
// when the footprints do not intersect. This is the reviewer payload — conflict
// regions only, never the full composedDiff.
//
// Overlap is computed on the SAME old-side pre-image coordinate the
// production-wired RunScopedMergeReview uses (mergerung2.go's rangesOverlap):
// two changes to one file are compared on their common base, so this payload
// and the wired dispatch path can never disagree on what "intersecting" means.
// A malformed diff fails closed to an empty payload (no bytes) — the scoped
// path can only narrow, never widen, what composes.
func IntersectingHunks(auditedDiff, composedDiff []byte) []byte {
	audited, err := parseUnifiedDiffToHunks(auditedDiff)
	if err != nil {
		return nil
	}
	composed, err := parseUnifiedDiffToHunks(composedDiff)
	if err != nil {
		return nil
	}

	auditedByFile := make(map[string][]parsedHunk)
	for _, a := range audited {
		auditedByFile[a.File] = append(auditedByFile[a.File], a)
	}

	var out bytes.Buffer
	lastFile := ""
	haveFile := false
	for _, c := range composed {
		aHunks, ok := auditedByFile[c.File]
		if !ok {
			continue // file absent from the audited footprint
		}
		overlaps := false
		for _, a := range aHunks {
			if rangesOverlap(a, c) {
				overlaps = true
				break
			}
		}
		if !overlaps {
			continue // no conflict region for this composed hunk
		}
		if !haveFile || c.File != lastFile {
			out.WriteString("diff --git a/" + c.File + " b/" + c.File + "\n")
			lastFile = c.File
			haveFile = true
		}
		out.WriteString(c.Header)
		out.WriteByte('\n')
		out.WriteString(c.Body) // Body is already newline-terminated per line
	}
	return out.Bytes()
}
