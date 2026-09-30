# Comment history: `acs/cycle1403`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1403/predicates_test.go:3` — above `package cycle1403`

```text
// Package cycle1403 materialises the cycle-1403 acceptance criteria for the
// three fleet-scoped tasks pinned to this lane, all serving one inbox item
// (`defect-disposition-contract-unsatisfiable`):
//
//   - disposition-evidence-tolerant-unmarshal   → the JSON-type fix (crux)
//   - disposition-schema-literal-example        → prompt + arch-doc example
//   - disposition-parse-error-surfaced-inline   → self-sufficient rejection
//
// Predicate strategy. The subject is `readDispositions` / the disposition
// branch of `hooks.Classify` in package `audit` — all unexported, so an
// external predicate package cannot call them. Each predicate therefore drives
// the RED contract this cycle's TDD phase froze into
// go/internal/phases/audit/*_test.go, which itself reaches the subject through
// the production seam `hooks{}.Classify` (and, for the doc example, the
// production reader `readDispositions`). No predicate greps production source
// for a magic string — the cycle-85 degenerate-predicate ban.
//
// Vacuity guard. `go test -run` exits 0 when the pattern matches NOTHING, so a
// builder who deletes or renames a frozen test would turn every predicate here
// green. runNamedTests therefore asserts each expected test name appears as an
// executed `--- PASS:` line, not merely that the process exited 0.
//
// Flaky-shape posture. Each predicate invokes ONE named package
// (./internal/phases/audit — ~22s cold, well under the banned 40s+ suites) with
// `-run` narrowing where the criterion is specific; cmd.Dir is set explicitly
// rather than relying on process cwd, which differs between main tree,
// worktree, and fleet lane. No wall-clock assertions, no literal PIDs, no
// unreaped load generators.
```

### `go/acs/cycle1403/predicates_test.go:79` — above `func TestC1403_001_DispositionEvidenceShapeTolerance(t *testing.T) {`

```text
// TestC1403_001_DispositionEvidenceShapeTolerance — Task 1, the crux. The
// array-shaped `evidence` that killed cycle-1399 must be READ and honoured,
// while an unresolvable array, an empty array, and a non-array/non-string shape
// must all still block. Tolerance widens the accepted shape, never the accepted
// claim.
```
