package core

import "bytes"

type ScopedReviewVerdict string

const (
	ScopedReviewCompatible ScopedReviewVerdict = "compatible"
	ScopedReviewEntangled  ScopedReviewVerdict = "entangled"
)

const ScopedReviewMethod = "scoped-review"

func (v ScopedReviewVerdict) Composes() bool {
	return v == ScopedReviewCompatible
}

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
		out.WriteString(c.Body)
	}
	return out.Bytes()
}
