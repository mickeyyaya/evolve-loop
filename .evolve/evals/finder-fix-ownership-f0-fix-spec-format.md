---
score_cap:
  - criterion: "The reader parses a fix spec into its location, expected behavior and replacement lines"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestParseFixSpec_ReadsLocationBehaviorAndReplacement' ./internal/retryladder"
  - criterion: "The reader refuses a malformed fix spec with ErrMalformedFixSpec"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestParseFixSpec_RefusesMalformedSpecs' ./internal/retryladder"
  - criterion: "A fix that touches a file outside the spec fails the scope guard"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestFixScopeViolations_ReportsEachFileOutsideTheSpecOnce|TestFixScopeViolations_RefusesARenameThatLeavesTheSpec' ./internal/retryladder"
  - criterion: "A fix that changes lines outside the cited lines fails the scope guard, and a fix inside them passes"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestFixScopeViolations_ReportsAHunkOutsideTheCitedLines|TestFixScopeViolations_AcceptsAChangeInsideTheCitedLines' ./internal/retryladder"
---

# Eval: Finder fix ownership F0, the fix spec format and scope guard

> Pins the fix spec type, its reader and the scope guard of decisions D1 and D5 in
> docs/plans/finder-fix-ownership-2026-10.md (ADR-0146). The finder writes the spec after rung R2
> fails; an R3 fix may touch only the files and lines that the spec names. Source: inbox item
> finder-fix-ownership-f0-fix-spec-format, cycle 2047.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| reader-positive | spec parses to location, behavior, lines | 6/10 | `go test -run TestParseFixSpec_ReadsLocationBehaviorAndReplacement` |
| reader-negative | malformed specs refused | 6/10 | `go test -run TestParseFixSpec_RefusesMalformedSpecs` |
| file-scope | file outside the spec fails | 6/10 | `go test -run TestFixScopeViolations_ReportsEachFileOutsideTheSpecOnce` |
| line-scope | lines outside the cited lines fail | 6/10 | `go test -run TestFixScopeViolations_ReportsAHunkOutsideTheCitedLines` |
