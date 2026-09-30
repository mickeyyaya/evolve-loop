# Comment history: `acs/cycle1196`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1196/predicates_test.go:3` — above `package cycle1196`

```text
// Package cycle1196 materialises the cycle-1196 acceptance criteria for the one
// triage-committed top_n task, `lane-base-fetch-origin-main` (fleet-scope todo
// `loop-must-base-lanes-on-origin-main-not-stale-local`).
//
// Defect: gitWorktree.Create bases every new lane branch on the LOCAL HEAD of
// projectRoot (`git worktree add -B <branch> <wt> HEAD`, go/internal/core/worktree.go:97)
// with zero fetch/origin interaction in the file. In a multi-lane fleet the local
// checkout drifts behind origin/main as sibling lanes land, so each new lane forks
// from a stale tip — silently reproducing already-fixed defects and inflating
// ship-time merge conflicts.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…1098 precedent).
// gitWorktree and its gitRunner seam are UNEXPORTED, so an out-of-package
// predicate cannot call Create directly; each predicate therefore shells
// `go test -run` over the RED contract tests authored this cycle in
// go/internal/core/worktree_lanebase_test.go. Every one of those drives the real
// Create() through a scripted git seam and asserts on the recorded git argv,
// call ORDER, returned worktree path and returned error — none is a source-grep
// of production code (the cycle-85 degenerate-predicate ban).
```
