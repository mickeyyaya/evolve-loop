# Comment history: `acs/cycle1438`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1438/predicates_test.go:3` — above `package cycle1438`

```text
// Package cycle1438 materialises the cycle-1438 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	salvage-backtick-regression-guard → land the missing regression coverage that
//	formally closes the cycle-1406/1407 `isQuotedEcho` backtick-adjacency defect.
//
// What this cycle is (and is NOT). Scout verified against the live tree — not the
// retro text — that the buggy `isQuotedEcho`/`insideStringLiteral` heuristic is
// GONE: `ClassifyBadVerdict` (go/internal/deliverable/salvage_instrument.go) was
// rewritten wholesale into a 4-shape precedence classifier that does no
// adjacency detection at all. So the production fix is already present and the
// acceptance bar is NOT "change the classifier" — it is "pin the fixed behaviour
// with a durable, committed test so it cannot regress silently". Predicate 004
// is the anti-scope-creep crux: it asserts the classifier's observable contract
// is UNCHANGED, so a Builder who 'improves' already-correct production code
// fails this cycle.
//
// Predicate strategy — every predicate exercises the system under test (a real
// call into ClassifyBadVerdict, or a real `go test` subprocess whose PASS lines
// are read), never a source-grep of production code as the load-bearing check
// (the cycle-85 degenerate-predicate ban):
//
//   - 001 shells the named regression test and requires a real `--- PASS:` line
//     for it, so "no tests to run" (exit 0, test absent) cannot green it. RED
//     today: the test does not exist.
//   - 002 does the same for the symbol-reintroduction guard test. RED today.
//   - 003 is the behavioural property itself, asserted by DIRECT CALL: a stray
//     unmatched backtick must not perturb classification, proven by a paired
//     control (backtick-bearing input vs its backtick-free twin) across all four
//     classifier shapes. Pre-existing GREEN — it pins the already-landed fix.
//   - 004 is the golden contract of the classifier over the four canonical
//     shapes (exact Pattern + Recoverable). Pre-existing GREEN; it red-fails any
//     production-code edit that alters observable classification, and its
//     negative axis (a genuinely-absent verdict must stay NOT recoverable)
//     kills a stub that blanket-returns Recoverable.
//   - 005 is the no-regression floor: the whole named package still greens.
//
// Root resolution: acsassert.RepoRoot(t) is the worktree, where Builder writes
// the new test — the deliverable is a worktree source change, so its absence is
// a FAILURE, not a skip. Subprocesses set cmd.Dir explicitly (never inherit the
// process cwd, which differs between main tree, worktree and each fleet lane).
```

### `go/acs/cycle1438/predicates_test.go:58` — above `const targetPkg = "./internal/deliverable"`

```text
// targetPkg is the ONE named package these predicates compile and test. Never a
// `./...` sweep: whole-repo staleness is the regression suite's job, and a
// multi-package invocation under fleet load is the flaky-predicate shape that
// false-redded cycles 1173/1175/1178.
```

### `go/acs/cycle1438/predicates_test.go:69` — above `const guardTestName = "TestNoQuotedEchoRegression"`

```text
// guardTestName is the symbol-reintroduction guard. The cycle-1406/1407 defect
// lived in `isQuotedEcho` (helped by `insideStringLiteral`); both are gone, and
// this test is the tripwire that fires if adjacency-as-proof logic is ever
// reintroduced into the package. The name is PINNED here (scout left "same test
// or a separate func" open) so the acceptance criterion is deterministic.
```
