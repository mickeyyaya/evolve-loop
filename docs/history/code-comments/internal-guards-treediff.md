# Comment history: `internal/guards/treediff`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d13

### `go/internal/guards/treediff/treediff.go:1` — above `package treediff`

```text
// Package treediff is Workstream B's post-phase main-tree diff guard.
//
// The bridge's OS sandbox confines source-writing phases to their worktree at
// runtime, but the trust-kernel design wants belt-and-suspenders. Each git
// worktree is a separate working directory: `git diff --name-only HEAD` run
// in the main repo root only shows changes to the main tree, NOT to any
// worktree. So after a source-writing phase runs, comparing the main-tree
// dirty set before-and-after isolates exactly the leaks the sandbox is
// supposed to prevent — any newly-dirty *main-tree* path = a write that
// escaped the worktree (Issue 2 / cycle-119 cross-CLI trust bypass).
//
// This package is pure logic: it takes a git-exec seam as a function value so
// the orchestrator can inject test fakes. No package globals.
```

### `go/internal/guards/treediff/treediff_test.go:50` — above `before := []string{}`

```text
// A path that wasn't dirty before but is dirty after → leak. This is the
// cycle-119 cross-CLI bypass signature: Gemini scout wrote to main during
// a read-only phase.
```
