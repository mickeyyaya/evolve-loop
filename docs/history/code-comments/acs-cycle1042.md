# Comment history: `acs/cycle1042`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1042/predicates_test.go:3` — above `package cycle1042`

```text
// Package cycle1042 encodes the cycle-1042 ACS predicates for
// `retro-role-gate-lessons-write-allowance`.
//
// The role guard (go/internal/guards/role.go) documents a retro/learn write
// allowance for the lessons corpus but never implemented it, so every
// retro-phase Edit/Write to the instincts lessons corpus falls through to the
// terminal deny ("phase=retro may not write outside workspace ..."). These
// predicates are BEHAVIORAL: each one constructs a Role over an in-memory
// core.Storage and calls Decide — none of them greens on a source-grep.
//
// Lessons-dir derivation pinned by these predicates: the corpus root is
// <evolveDir>/instincts/lessons, where evolveDir is the grandparent of
// cs.WorkspacePath (workspace == <evolveDir>/runs/cycle-<N>). CycleState
// carries no evolve-root field, so this is the only root available to the
// guard.
```
