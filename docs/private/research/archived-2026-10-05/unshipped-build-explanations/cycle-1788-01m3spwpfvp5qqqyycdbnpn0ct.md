# Build Explanation — Cycle 1788

## Build Binding
- Cycle: 1788
- Base SHA: 60821745599e0b37848be0f533f345b4042d8cab

## Summary
The evalqualitycheck package gains a lint for ACS predicate sources for unsatisfiable absence assertions, the cycle-352/1488 class where a positive FileContains-family call is used to assert absence.

## Rationale
acsassert.FileNotContains already exists, but nothing caught callers who kept using the positive primitive. A static AST lint in the existing evalqualitycheck package catches the shape at authoring time with no new configuration surface.

## Changed Areas
- `go/internal/evalqualitycheck/unsatisfiable.go` — adds LintUnsatisfiablePredicates, its report and finding types, and the two detection shapes.
- `go/internal/evalqualitycheck/unsatisfiable_test.go` — table test for the absence-intent heuristic's false-positive guards.
- `go/internal/evalqualitycheck/apicover_named_test.go` — names the new exports for the repo-wide apicover gate.
- `docs/architecture/packages/internal-evalqualitycheck.md` — documents the lint and its invariants.

## Design Decisions
Findings are advisory (PASS to WARN, never HALT), matching the flaky lint. The absence-message heuristic requires an absence word and no missing-state marker so that "does not mention X — still undocumented" is not flagged.

## Verification
The cycle-1788 ACS predicates, the package unit tests and a sweep over 486 existing cycle packages (zero findings) cover both shapes.

## Compatibility
No existing flag, output line or exit code changes; new lines are additive and only raise PASS to WARN when a finding exists.

## Limitations
The lint recognises only direct `acsassert.FileContains`/`FileMatchesRegex` calls in an if condition, not results stored in a variable first.
