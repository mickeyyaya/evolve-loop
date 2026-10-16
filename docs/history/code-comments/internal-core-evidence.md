# Comment history: `internal/core/evidence`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d12

### `go/internal/core/evidence/trailer.go:1` — above `package evidence`

```text
// Package evidence implements the ADR-0027 commit-as-evidence trailer: the
// provenance block appended to a phase's commit on the per-cycle worktree
// branch. A phase has delivered iff it produced a commit carrying a well-formed
// Evolve-Phase trailer with the cycle's challenge token — completion is "HEAD
// advanced + trailer verified", never "a file appeared at the polled path".
//
// This package is PURE (stdlib only) and imports nothing from the bridge,
// orchestrator, or git layers, so both the completion detector (bridge) and the
// commit emitter (orchestrator, a later PR) can depend on it without a cycle.
```
