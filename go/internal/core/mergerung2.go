package core

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

type ScopedMergeDisposition string

const (
	ScopedMergeCompatible ScopedMergeDisposition = "compatible"
	ScopedMergeEntangled  ScopedMergeDisposition = "entangled"
)

type MergeHunk struct {
	File   string
	Header string
	Body   string
}

type ScopedMergeReviewOutcome struct {
	Disposition    ScopedMergeDisposition
	ResolutionDiff []byte
}

type ScopedMergeReviewer func(hunks []MergeHunk, auditedSummary, composedSummary string) ScopedMergeReviewOutcome

type ScopedMergeInput struct {
	AuditedDiff     []byte
	ComposedDiff    []byte
	AuditedSummary  string
	ComposedSummary string
}

type ScopedMergeResult struct {
	Disposition     ScopedMergeDisposition
	DispatchedHunks []MergeHunk
	Dispatched      bool
	ResolutionDiff  []byte
}

const scopedReviewMethod = "scoped-review"

type parsedHunk struct {
	MergeHunk
	oldStart int
	oldCount int
}

func RunScopedMergeReview(in ScopedMergeInput, review ScopedMergeReviewer) (ScopedMergeResult, error) {
	audited, err := parseUnifiedDiffToHunks(in.AuditedDiff)
	if err != nil {
		return ScopedMergeResult{Disposition: ScopedMergeEntangled}, fmt.Errorf("scoped merge review: audited diff: %w", err)
	}
	composed, err := parseUnifiedDiffToHunks(in.ComposedDiff)
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

func ResolutionMatchesAudited(auditedPatchID string, resolutionDiff []byte) (bool, error) {
	pid, err := compositionPatchID(resolutionDiff)
	if err != nil {
		return false, fmt.Errorf("resolution rung-0 re-entry: %w", err)
	}
	return pid == auditedPatchID, nil
}

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

func rangesOverlap(a, b parsedHunk) bool {
	aEnd := a.oldStart + max(a.oldCount, 1)
	bEnd := b.oldStart + max(b.oldCount, 1)
	return a.oldStart < bEnd && b.oldStart < aEnd
}

func parseUnifiedDiffToHunks(diff []byte) ([]parsedHunk, error) {
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
			flush()
		case cur == nil && strings.HasPrefix(line, "+++ "):
			currentFile = parseDiffPath(line[len("+++ "):])
		case cur == nil && strings.HasPrefix(line, "--- "):
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

func parseDiffPath(s string) string {
	if i := strings.IndexByte(s, '\t'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "a/")
	s = strings.TrimPrefix(s, "b/")
	return s
}

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
