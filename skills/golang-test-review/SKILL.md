---
name: golang-test-review
description: Use when reviewing Go test code (new or changed *_test.go files, test harnesses, fixtures, or test tooling) for a Go-test-expert pass — behavior-over-surface, determinism, parallel-safety, build-tag correctness, and harness reuse. Complements the general code-reviewer with test-specific rigor.
---

# /golang-test-review

A focused Go-test-engineering review. Run it over the changed `*_test.go` files
(and any test harness / fixtures / test tooling) **after** writing tests and
before merge. It does not replace `code-reviewer` — it adds the test-specific
lens that general review misses.

## How to run

1. Identify the changed test surface: `git diff --stat <base>...HEAD -- '*_test.go' 'go/test/**' 'go/cmd/testlatency/**'`.
2. Read each changed test file and score it against the checklist below.
3. Report findings by severity (CRITICAL / HIGH / MEDIUM / LOW), each with
   `file:line`, the rule violated, and a concrete fix. End with a verdict:
   APPROVE (no CRITICAL/HIGH) / CHANGES-REQUESTED.

## Checklist

### 1. Behavior over surface (the highest-signal rule)
- Does the test pin the **invariant under change**, or just re-walk lines for
  coverage? A test that would still pass if the behavior broke is a no-op —
  flag it CRITICAL. (AGENTS.md Rule 9.)
- Are assertions specific (exact value / error) rather than "not nil" / "len > 0"
  where an exact check is possible?
- Negative + edge cases present, not only the happy path?

### 2. Determinism (non-negotiable)
- No dependence on ambient state: live repo, `$HOME`, real clock, network,
  `.evolve/runs/`. Tests build isolated state (`t.TempDir()` + `git init`, or the
  shared `fixtures.NewWorkspace`).
- No real `time.Sleep` for synchronization where an injected clock / channel /
  poll-with-deadline would be deterministic. Real sleeps belong only in
  build-tagged integration/e2e tiers.
- No ordering dependence between tests; no shared mutable package globals.

### 3. Parallel-safety
- `t.Parallel()` is present where safe — BUT never combined with `t.Setenv`
  (panics) or shared-mutable-state tests.
- Subtests that capture a loop variable are Go-1.22-safe or copy the variable.
- `-race` clean (the suite runs with `-race`).

### 4. Build-tag / cost-axis correctness (this repo's two-axis model)
- Slow tests (real subprocess: git/gh/tmux/bash; real timing) carry
  `//go:build integration` or `//go:build e2e` and are NOT in the fast default.
- **Self-containment:** an untagged (fast) test must not reference a symbol
  defined only in a tagged file — the default build would break. Shared helpers
  the fast suite needs live in an untagged file.
- Tag sits at the top, followed by a blank line, before `package`.

### 5. Harness reuse (no re-duplication)
- Uses `go/test/fixtures` (Workspace builder, Fake{Storage,Ledger,Runner,Bridge},
  FixedClock, assertion facade) instead of a new local temp-dir builder, fake,
  clock, or `must*` helper. Reintroducing a local clone of a harness primitive
  is a HIGH finding.
- `fixtures.FilePresent` (pure bool) for skip preconditions — NOT
  `acsassert.FileExists` (which logs an `Errorf` and shouldn't gate a skip).

### 6. Naming & structure
- Behavior-named (`TestShipGate_BlocksWhenRedCountNonZero`), never cycle-pegged
  (`TestC102_*`).
- AAA (Arrange / Act / Assert) visually distinct; table-driven where it reduces
  repetition; `t.Run` subtests named for the case.
- `t.Helper()` on every helper so failures point at the caller.

### 7. Debuggability of failures
- Failure messages state got-vs-want and the value under test
  (`got %q, want %q`), not bare `t.Fatal("failed")`.
- Skips explain why and what's missing (`t.Skip("git not on PATH")`).

### 8. Hygiene
- `t.Cleanup` / `defer` restore any mutated state; no leaked goroutines,
  temp files (prefer `t.TempDir`), or open handles.
- No `time.Now()`/`rand` without a seam where the result is asserted.

## Severity guide
- CRITICAL: tautological/no-op test, non-deterministic test, `-race` violation,
  build-tag self-containment break.
- HIGH: missing negative/edge coverage on changed behavior, re-duplicated
  harness primitive, `t.Setenv`+`t.Parallel` combo, real sleep in fast tier.
- MEDIUM: weak assertions, missing `t.Helper()`, poor failure messages.
- LOW: naming, AAA structure, comment nits.
