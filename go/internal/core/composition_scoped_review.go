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
// These mirror rung 1 (internal/fleet/packagegraph.go): the primitives land
// and are exercised as tested library API this cycle; the recovery-path
// dispatch branch that consumes them is a follow-on wiring cycle. LLM merges
// are suggestion-grade (MergeBERT-lineage evidence) — verified via patch-id,
// never trusted.
package core

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

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
// BOTH diffs, only the hunks from composedDiff whose new-side line ranges
// overlap a hunk in auditedDiff (the conflict regions), with each file's diff
// header preserved. Non-overlapping hunks, and files present in only one diff,
// are excluded. The result is empty (no bytes) when the footprints do not
// intersect. This is the reviewer payload — conflict regions only, never the
// full composedDiff.
//
// Overlap is computed on unified-diff hunk-header ranges (@@ -a,b +c,d @@) — a
// first-cut heuristic (no full AST/tree-sitter parse). New-side ranges are the
// resulting-file coordinate the reviewer reasons about.
func IntersectingHunks(auditedDiff, composedDiff []byte) []byte {
	auditedByFile := make(map[string][]scopedHunk)
	for _, f := range parseUnifiedDiff(auditedDiff) {
		auditedByFile[scopedFileKey(f)] = f.hunks
	}

	var out bytes.Buffer
	for _, cf := range parseUnifiedDiff(composedDiff) {
		aHunks, ok := auditedByFile[scopedFileKey(cf)]
		if !ok {
			continue // file absent from the audited footprint
		}
		var kept []scopedHunk
		for _, ch := range cf.hunks {
			if anyOverlap(ch, aHunks) {
				kept = append(kept, ch)
			}
		}
		if len(kept) == 0 {
			continue // no conflict region in this file — omit its header too
		}
		for _, hl := range cf.header {
			out.WriteString(hl)
			out.WriteByte('\n')
		}
		for _, kh := range kept {
			for _, l := range kh.lines {
				out.WriteString(l)
				out.WriteByte('\n')
			}
		}
	}
	return out.Bytes()
}

// scopedHunk is one @@ hunk with its new-side [newStart,newEnd) range and the
// raw lines (header + body) needed to reproduce it in the scoped payload.
type scopedHunk struct {
	newStart int
	newEnd   int // exclusive
	lines    []string
}

// scopedFile is one file section of a unified diff: the header lines up to the
// first @@, plus the parsed hunks.
type scopedFile struct {
	header []string
	hunks  []scopedHunk
}

func anyOverlap(ch scopedHunk, aHunks []scopedHunk) bool {
	for _, ah := range aHunks {
		if ch.newStart < ah.newEnd && ah.newStart < ch.newEnd {
			return true
		}
	}
	return false
}

// parseUnifiedDiff splits a unified diff into per-file sections. File
// boundaries are "diff --git " lines; hunk boundaries are "@@" lines.
func parseUnifiedDiff(diff []byte) []scopedFile {
	var files []scopedFile
	var cur *scopedFile
	var hunk *scopedHunk

	flush := func() {
		if cur != nil && hunk != nil {
			cur.hunks = append(cur.hunks, *hunk)
			hunk = nil
		}
	}

	sc := bufio.NewScanner(bytes.NewReader(diff))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			files = append(files, scopedFile{header: []string{line}})
			cur = &files[len(files)-1]
		case strings.HasPrefix(line, "@@"):
			flush()
			if cur == nil {
				files = append(files, scopedFile{})
				cur = &files[len(files)-1]
			}
			ns, nl := parseNewRange(line)
			hunk = &scopedHunk{newStart: ns, newEnd: ns + max(nl, 1), lines: []string{line}}
		default:
			switch {
			case hunk != nil:
				hunk.lines = append(hunk.lines, line)
			case cur != nil:
				cur.header = append(cur.header, line)
			}
		}
	}
	flush()
	return files
}

// parseNewRange extracts the new-side start line and length from a hunk header
// like "@@ -10,3 +10,4 @@ func A() {". A missing ",length" means length 1.
func parseNewRange(header string) (start, length int) {
	plus := strings.Index(header, "+")
	if plus < 0 {
		return 0, 0
	}
	rest := header[plus+1:]
	if end := strings.IndexAny(rest, " \t@"); end >= 0 {
		rest = rest[:end]
	}
	if comma := strings.Index(rest, ","); comma >= 0 {
		start, _ = strconv.Atoi(rest[:comma])
		length, _ = strconv.Atoi(rest[comma+1:])
		return start, length
	}
	start, _ = strconv.Atoi(rest)
	return start, 1
}

// scopedFileKey identifies a file section by its post-image path (the "+++ b/…"
// line), falling back to the raw "diff --git" line. Two diffs of the same file
// key equal so their hunks can be intersected.
func scopedFileKey(f scopedFile) string {
	for _, h := range f.header {
		if strings.HasPrefix(h, "+++ ") {
			p := strings.TrimSpace(strings.TrimPrefix(h, "+++ "))
			if tab := strings.IndexByte(p, '\t'); tab >= 0 {
				p = p[:tab]
			}
			return strings.TrimPrefix(p, "b/")
		}
	}
	for _, h := range f.header {
		if strings.HasPrefix(h, "diff --git ") {
			return h
		}
	}
	return ""
}
