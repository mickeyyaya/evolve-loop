# Build Explanation — Cycle 1850

## Build Binding
- Cycle: 1850
- Base SHA: ba41c8b976dd58aa1d197cc7d6ef2a02a4c11282

## Summary
The opt-in exit-transport-hang reclassifier now matches the cycle number as a whole number when it searches main for a "cycle N" commit. Before this change, a commit about cycle 42 or cycle 400 could make cycle 4 count as shipped. The cycleclassify tests now turn the classifier on through the live toggle and use the standard library instead of hand-rolled helpers.

## Rationale
`git log --grep=cycle 4` does a substring match, so "cycle 42 shipped" matched cycle 4. Requiring a non-digit or the end of the line after the number is the smallest change that fixes this. It keeps the single git call and still matches messages such as "cycle 4, shipped" and "cycle 42 — shipped".

## Changed Areas
- `go/internal/cycleclassify/classify.go` — gitLogMatchesCycle passes `--extended-regexp` with the pattern `cycle <N>([^0-9]|$)`. N goes through `regexp.QuoteMeta`, so a non-numeric workspace suffix is still treated as literal text.
- `go/internal/cycleclassify/classify_git_test.go` — the expected git arguments now include the new flag and the anchored pattern.
- `go/internal/cycleclassify/hang_coverage_test.go` — the fallthrough test turns the classifier on with `setHangClassifierForTest(t, true)` instead of setting the removed `EVOLVE_HANG_CLASSIFIER` env flag, which did nothing. The real-git production-path test now also checks that cycle 4 does not match a cycle 42 commit.
- `go/internal/cycleclassify/empty_output_test.go` — the hand-written `stringRepeat` is replaced with `strings.Repeat`.
- `go/internal/cycleclassify/echo_veto_c654_test.go` — the hand-written `itoa` is replaced with `strconv.Itoa`.

## Design Decisions
- A POSIX ERE suffix class (`[^0-9]|$`) is used instead of `\b`, because `\b` support differs between the platform regex libraries that git may be built with.
- The leading side is not anchored. Leading digits cannot produce a false match, because "cycle 14" does not contain "cycle 4".

## Verification
- The cycle-1850 ACS predicates 001–006 were red before the change (001, 002, 005, 006) and are all green after it.
- The new real-git assertion in TestGitLogFn_ProductionPath fails when the old grep line is put back and passes with the fix.
- `go test -count=1 ./internal/cycleclassify/...` passes, and the cycle-1850 ACS suite reports PASS (red=0).

## Compatibility
The behavior changes only when the hang classifier is enabled, and only by rejecting false matches on longer numbers. The real commits for a cycle still match.

## Limitations
A commit message that mentions "cycle 4" for an unrelated reason still matches. Ruling that out would need a structured commit trailer, which is out of scope.
