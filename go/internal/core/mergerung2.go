// mergerung2.go — the merge ladder's RUNG 2: scoped merge review
// (cycle-941, fleet-scoped todo merge-rung2-scoped-merge-review;
// knowledge-base/research/merge-concurrency-2026, MergeBERT lineage).
//
// RUNG 0 (composition_carryforward.go) carries an audit verdict forward when a
// clean fleet rebase leaves the composed diff's patch-id UNCHANGED. When the
// patch-id DID change — real overlapping edits, not a trivial rebase — today's
// only fallback is RUNG 3, a full re-audit. RUNG 2 is the missing middle: it
// reviews ONLY the hunks that actually intersect between the audited change and
// the composed change and, if that overlap is compatible, composes directly;
// only genuine entanglement escalates to the full re-audit.
//
// The reviewer is an injected closure (Option seam, mirroring RUNG 0) so this
// pure core stays adapter- and LLM-agnostic. An LLM-assisted resolution the
// reviewer may return is SUGGESTION-GRADE: it is trusted only after re-entering
// RUNG 0 patch-id verification (ResolutionMatchesAudited), never on the
// reviewer's word.
package core

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// ScopedMergeDisposition is a scoped merge review's verdict on the intersecting
// hunks: compatible (the overlap composes) or entangled (escalate to re-audit).
type ScopedMergeDisposition string

const (
	// ScopedMergeCompatible means the intersecting hunks compose without a full
	// re-audit; it is also the result of an EMPTY intersection (nothing to
	// entangle).
	ScopedMergeCompatible ScopedMergeDisposition = "compatible"
	// ScopedMergeEntangled means the overlap is genuine conflicting work that
	// must escalate to RUNG 3 (full re-audit).
	ScopedMergeEntangled ScopedMergeDisposition = "entangled"
)

// MergeHunk is one unified-diff hunk handed to the reviewer: its file, its
// `@@ … @@` header, and the hunk body (context + added/removed lines).
type MergeHunk struct {
	File   string
	Header string
	Body   string
}

// ScopedMergeReviewOutcome is what a reviewer returns: its disposition and an
// OPTIONAL LLM-assisted resolution diff (suggestion-grade — trusted only after
// ResolutionMatchesAudited re-verifies its patch-id).
type ScopedMergeReviewOutcome struct {
	Disposition    ScopedMergeDisposition
	ResolutionDiff []byte
}

// ScopedMergeReviewer reviews exactly the intersecting hunks (plus both change
// summaries) and returns a disposition. It is injected so the core stays pure;
// nil (default) leaves RUNG 2 dark and recovery behaves exactly as today.
type ScopedMergeReviewer func(hunks []MergeHunk, auditedSummary, composedSummary string) ScopedMergeReviewOutcome

// ScopedMergeInput carries the two diffs (and human-readable summaries) a scoped
// merge review compares: what the audit reviewed vs. the composed (rebased) tree.
type ScopedMergeInput struct {
	AuditedDiff     []byte
	ComposedDiff    []byte
	AuditedSummary  string
	ComposedSummary string
}

// ScopedMergeResult is the pure core's report: the disposition, the exact hunks
// dispatched to the reviewer, whether the reviewer was invoked at all, and the
// reviewer's optional (suggestion-grade) resolution diff carried through for the
// caller to re-verify via ResolutionMatchesAudited.
type ScopedMergeResult struct {
	Disposition     ScopedMergeDisposition
	DispatchedHunks []MergeHunk
	Dispatched      bool
	ResolutionDiff  []byte
}

// scopedReviewMethod is core's mirror of ledger.ScopedReviewMethod (core cannot
// import internal/adapters/ledger, which already imports core); the two must
// stay identical — the ship-side reader and audit key off this exact string.
const scopedReviewMethod = "scoped-review"

// parsedHunk augments a MergeHunk with its old-side (pre-image) line range — the
// common base coordinate two changes to the same file are compared on.
type parsedHunk struct {
	MergeHunk
	oldStart int
	oldCount int
}

// RunScopedMergeReview parses both diffs into hunks, computes the hunks whose
// file + old-side line ranges intersect, and — only when the intersection is
// non-empty — dispatches EXACTLY those intersecting hunks to the reviewer,
// carrying its disposition through verbatim. Fail-closed on a malformed diff
// (returns a non-nil error, does NOT invoke the reviewer, does NOT report
// compatible). An empty intersection returns compatible without invoking the
// reviewer (nothing entangled — no wasted review).
func RunScopedMergeReview(in ScopedMergeInput, review ScopedMergeReviewer) (ScopedMergeResult, error) {
	audited, err := parseUnifiedDiff(in.AuditedDiff)
	if err != nil {
		return ScopedMergeResult{Disposition: ScopedMergeEntangled}, fmt.Errorf("scoped merge review: audited diff: %w", err)
	}
	composed, err := parseUnifiedDiff(in.ComposedDiff)
	if err != nil {
		return ScopedMergeResult{Disposition: ScopedMergeEntangled}, fmt.Errorf("scoped merge review: composed diff: %w", err)
	}
	dispatched := intersectingHunks(audited, composed)
	if len(dispatched) == 0 {
		return ScopedMergeResult{Disposition: ScopedMergeCompatible, Dispatched: false}, nil
	}
	outcome := review(dispatched, in.AuditedSummary, in.ComposedSummary)
	return ScopedMergeResult{
		Disposition:     outcome.Disposition,
		DispatchedHunks: dispatched,
		Dispatched:      true,
		ResolutionDiff:  outcome.ResolutionDiff,
	}, nil
}

// ResolutionMatchesAudited re-enters RUNG 0 patch-id verification for an
// LLM-assisted resolution (MergeBERT lineage): it recomputes the resolution
// diff's OWN patch-id and returns true only if it equals auditedPatchID. A
// resolution is trusted by recomputed patch-id, never on the reviewer's word; a
// malformed resolution fails closed (non-nil error, never a silent trust).
func ResolutionMatchesAudited(auditedPatchID string, resolutionDiff []byte) (bool, error) {
	pid, err := compositionPatchID(resolutionDiff)
	if err != nil {
		return false, fmt.Errorf("resolution rung-0 re-entry: %w", err)
	}
	return pid == auditedPatchID, nil
}

// intersectingHunks returns the hunks — audited-side first, then composed-side —
// whose file matches and whose old-side line ranges overlap. Audited-only and
// composed-only hunks (no counterpart on the other side) are never included.
func intersectingHunks(audited, composed []parsedHunk) []MergeHunk {
	var out []MergeHunk
	for _, a := range audited {
		for _, c := range composed {
			if a.File == c.File && rangesOverlap(a, c) {
				out = append(out, a.MergeHunk)
				break
			}
		}
	}
	for _, c := range composed {
		for _, a := range audited {
			if a.File == c.File && rangesOverlap(a, c) {
				out = append(out, c.MergeHunk)
				break
			}
		}
	}
	return out
}

// rangesOverlap reports whether two hunks' old-side line ranges intersect. A
// zero-count range (pure insertion point) is treated as a unit-width point so an
// insertion at a line still overlaps a change spanning it.
func rangesOverlap(a, b parsedHunk) bool {
	aEnd := a.oldStart + max(a.oldCount, 1)
	bEnd := b.oldStart + max(b.oldCount, 1)
	return a.oldStart < bEnd && b.oldStart < aEnd
}

// parseUnifiedDiff splits a unified diff into hunks tagged with their file and
// old-side line range. A `@@` header that does not parse as
// `@@ -old[,n] +new[,m] @@` is fail-closed: a non-nil error, no partial result.
func parseUnifiedDiff(diff []byte) ([]parsedHunk, error) {
	var (
		hunks       []parsedHunk
		currentFile string
		cur         *parsedHunk
		body        strings.Builder
	)
	flush := func() {
		if cur != nil {
			cur.Body = body.String()
			hunks = append(hunks, *cur)
			cur = nil
			body.Reset()
		}
	}
	sc := bufio.NewScanner(bytes.NewReader(diff))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "@@"):
			flush()
			oldStart, oldCount, perr := parseHunkHeader(line)
			if perr != nil {
				return nil, perr
			}
			cur = &parsedHunk{
				MergeHunk: MergeHunk{File: currentFile, Header: line},
				oldStart:  oldStart,
				oldCount:  oldCount,
			}
		case strings.HasPrefix(line, "diff --git "):
			// A new file section: end the current hunk. The file name is taken
			// from the `+++ ` header below (robust to rename headers).
			flush()
		case cur == nil && strings.HasPrefix(line, "+++ "):
			// File header (only in the pre-hunk region; a `+++foo` inside a hunk
			// body is an added line, handled by the default arm).
			currentFile = parseDiffPath(line[len("+++ "):])
		case cur == nil && strings.HasPrefix(line, "--- "):
			// Old-file header — ignored; `+++ ` carries the post-image path.
		default:
			if cur != nil {
				body.WriteString(line)
				body.WriteByte('\n')
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read diff: %w", err)
	}
	flush()
	return hunks, nil
}

// parseDiffPath extracts the file path from a `--- `/`+++ ` header value,
// dropping any trailing tab-separated timestamp and a leading `a/` or `b/`.
func parseDiffPath(s string) string {
	if i := strings.IndexByte(s, '\t'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "a/")
	s = strings.TrimPrefix(s, "b/")
	return s
}

// parseHunkHeader parses `@@ -old[,n] +new[,m] @@ …` and returns the old-side
// (start, count). It is strict: a header whose ranges do not parse is an error
// (fail-closed) — the sole gate that keeps a corrupt diff from silently
// greening a composition.
func parseHunkHeader(line string) (int, int, error) {
	fields := strings.Fields(line)
	if len(fields) < 4 || fields[0] != "@@" || fields[3] != "@@" {
		return 0, 0, fmt.Errorf("malformed hunk header %q", line)
	}
	oldField, newField := fields[1], fields[2]
	if !strings.HasPrefix(oldField, "-") || !strings.HasPrefix(newField, "+") {
		return 0, 0, fmt.Errorf("malformed hunk header %q: want `-old +new`", line)
	}
	start, count, err := parseDiffRange(oldField[1:])
	if err != nil {
		return 0, 0, fmt.Errorf("malformed hunk header %q: %w", line, err)
	}
	if _, _, err := parseDiffRange(newField[1:]); err != nil {
		return 0, 0, fmt.Errorf("malformed hunk header %q: %w", line, err)
	}
	return start, count, nil
}

// parseDiffRange parses a `start[,count]` range; a missing count defaults to 1
// (unified-diff convention).
func parseDiffRange(s string) (int, int, error) {
	parts := strings.SplitN(s, ",", 2)
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("bad range %q: %w", s, err)
	}
	count := 1
	if len(parts) == 2 {
		if count, err = strconv.Atoi(parts[1]); err != nil {
			return 0, 0, fmt.Errorf("bad range %q: %w", s, err)
		}
	}
	return start, count, nil
}
