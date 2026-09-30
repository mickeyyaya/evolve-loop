# Comment history: `acs/cycle1255`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1255/predicates_test.go:3` — above `package cycle1255`

```text
// Package cycle1255 materialises the cycle-1255 acceptance criteria for the two
// fleet-scoped tasks triage committed to this lane:
//
//   - retro-fleet-worktree-empty-fallback     (inbox retro-fleet-worktree-dispatch, w=0.9)
//   - test-amplification-covering-tests-scope (inbox test-amplification-context-scope, w=0.89)
//
// Predicate strategy — behavioural-via-subprocess (the cycle-563/987 precedent).
// Each predicate shells `go test -run '^(names)$' -v -count=1 <one named package>`
// over the DEFAULT build suite and requires a `--- PASS: <name>` line per test.
// That genuinely exercises the system under test: the RED contracts authored this
// cycle drive the real retro phase (through its bridge seam) and the real
// changedpkgs deriver, so a predicate greens only when production code makes them
// pass.
//
//   - Asserting on the PASS LINE, not the exit code, is essential: `go test -run`
//     with a pattern matching nothing exits 0 ("no tests to run"), so a still-
//     missing contract would false-GREEN.
//   - A source-grep predicate (FileContains over a .go file) is deliberately
//     avoided — it passes the moment the magic string appears, fix or no fix (the
//     cycle-85 degenerate-predicate ban).
//   - Every `go test` here names EXACTLY ONE package and is neither ./... nor one
//     of the known 40s+ suites (./internal/core, ./cmd/evolve) — the
//     flaky-predicate-shape rules.
//
// 002 is the anti-regression half: it re-runs the UNTOUCHED bridge fleet-refusal
// tests, so "fix" the empty-worktree window by widening the guard itself (the
// blind-widen regression this task's own notes name) stays RED.
```
