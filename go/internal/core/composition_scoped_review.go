package core

import "bytes"

// ScopedReviewVerdict is the two-value rung-2 reviewer enum.
type ScopedReviewVerdict string

const (
	// ScopedReviewCompatible means the intersecting hunks compose safely.
	ScopedReviewCompatible ScopedReviewVerdict = "compatible"
	// ScopedReviewEntangled means the hunks interact; escalate to the full re-audit.
	ScopedReviewEntangled ScopedReviewVerdict = "entangled"
)

// ScopedReviewMethod is the composition-verdict method tag a composing rung-2 review writes.
const ScopedReviewMethod = "scoped-review"

// Composes reports whether the verdict skips the full re-audit; only ScopedReviewCompatible does, so unknown values fail closed.
func (v ScopedReviewVerdict) Composes() bool {
	return v == ScopedReviewCompatible
}

// ReverifyResolution reports whether a proposed resolution keeps the audited diff's patch-id; LLM merges are verified, never trusted.
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

// IntersectingHunks returns, as a diff, the composed hunks whose old-side range overlaps an audited hunk; a malformed diff yields nothing.
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

	// rangesOverlap is the wired RunScopedMergeReview's old-side test, so both agree on what intersects.
	var out bytes.Buffer
	lastFile := ""
	haveFile := false
	for _, c := range composed {
		aHunks, ok := auditedByFile[c.File]
		if !ok {
			continue
		}
		overlaps := false
		for _, a := range aHunks {
			if rangesOverlap(a, c) {
				overlaps = true
				break
			}
		}
		if !overlaps {
			continue
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
