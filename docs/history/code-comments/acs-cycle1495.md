# Comment history: `acs/cycle1495`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1495/predicates_test.go:3` — above `package cycle1495`

```text
// Package cycle1495 materialises the cycle-1495 acceptance criteria for the one
// fleet-scoped inbox item pinned to this lane, `verdict-cache-fresh-base-collision`,
// which triage split into two coupled tasks:
//
//   - verdict-cache-empty-worktree-miss (CONSUMER): a clean/fresh worktree —
//     one whose staged content is identical to its base commit's tree — must not
//     produce an ADR-0048 Slice B shadow verdict-cache reuse match. Every sibling
//     fleet lane cut from the same base carries that identity, so a match there is
//     cross-lane contamination, not conserved work.
//   - verdict-cache-empty-worktree-projection (PRODUCER): the audit-binding cache
//     projection must apply the SAME no-delta rule, so a no-op audit cannot seed
//     the very entry a later clean lane collides with.
//
// Predicate strategy (the cycle-85 degenerate-predicate ban): every predicate
// below drives a REAL production seam — 001/002 run `core.Orchestrator.RunCycle`
// against an on-disk git repo so the pre-loop probe executes exactly as it does
// in production; 003/004 run the same entry point with an audit runner that
// emits a real audit-report.md, so `recordAuditBinding`'s projection executes
// and the on-disk `.evolve/verdict-cache.json` is asserted as a real side effect. No source grep
// carries any assertion. Each suppression predicate is paired with a CHANGED
// control (002, 004) so a blanket "disable the cache" implementation fails.
//
// Reliability (flaky-predicate-shape rules): no `go test` subprocess, no `/...`
// sweep, no wall-clock deadline, no literal PID; every `git` invocation is
// `git -C <dir>` against a `t.TempDir()` repo, never process cwd.
```

### `go/acs/cycle1495/predicates_test.go:48` — above `func TestC1495_001_CleanWorktreeProducesNoShadowReuseMatch(t *testing.T) {`

```text
// TestC1495_001_CleanWorktreeProducesNoShadowReuseMatch pins AC-1a: a clean
// staged worktree cannot produce a shadow reuse match, EVEN when the verdict
// cache already holds an entry under that exact tree SHA (the fresh-base
// collision the incident records). The probe must report the lookup as
// suppressed and NOT matched, and the tdd/build/audit phases must still run.
//
// Anti-vacuity: the cache is deliberately SEEDED at the fresh-base tree, so a
// miss here can only come from the guard, never from an empty cache.
```
