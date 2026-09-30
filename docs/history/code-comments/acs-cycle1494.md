# Comment history: `acs/cycle1494`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1494/predicates_test.go:3` — above `package cycle1494`

```text
// Package cycle1494 materialises the cycle-1494 acceptance criteria for the one
// fleet-scoped task pinned to this lane, `sleep-time-kb-consolidation`.
//
// SCOPE NOTE (why this is not the Scout plan verbatim). The premise-challenge
// gate returned FAIL/BLOCK on the plan as framed, and this phase re-probed and
// CONFIRMED its two fatal seam findings before authoring:
//
//   - `research.maxResults = 5` (go/internal/research/filekb.go:21) has exactly
//     one production consumer — `Orchestrator.recallForPlan`
//     (go/internal/core/routing_dispatch.go:281), the ADVISOR's recall memory.
//     Scout receives no KB injection at all, so a "Scout top-k" framed against
//     this seam moves no Scout tokens and would silently NARROW advisor
//     failure-recall. The criteria below therefore make the bound TYPED POLICY
//     with the default HELD AT 5 — reproducible tuning, zero behaviour change.
//   - memo does NOT write the lessons corpus (agents/evolve-memo.md:19,91,133;
//     .evolve/profiles/memo.json allows Write only to carryover-todos.json and
//     memo.md). The REAL Go lesson-write seam is
//     `faillearn.WriteArtifacts` (go/internal/faillearn/writer.go:41, line 59),
//     reached from three production call sites
//     (cmd/evolve/cmd_loop_outcome.go:452, internal/core/failure_learning.go:477,
//     internal/core/reset.go:248). The novelty gate is materialised THERE, so a
//     passing predicate cannot be vacuous against an ungated production path.
//
// The inbox item's warm-start-brief criterion is NOT materialised this cycle;
// test-report.md records that omission and its reason explicitly rather than
// minting a second, competing brief contract next to the existing (dead)
// operator→scout one.
//
// Predicate strategy — every predicate invokes the system under test in-process
// and asserts on returned values or real on-disk side effects (the cycle-85
// degenerate-predicate ban): no source greps as the load-bearing check, no
// `go test` subprocess, no whole-package sweep, no wall-clock bound, no literal
// PID, no bare `git` against process cwd. 005 is the one structural predicate —
// the composition root lives in `package main` (cmd/evolve), which cannot be
// imported and whose whole-package `go test` is a banned flaky shape, so it is
// asserted over the parsed AST (not a text grep) and Builder must additionally
// name the caller file:line in build-report.md.
```
