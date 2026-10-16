# Comment history: `internal/recoveryguard`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d13

### `go/internal/recoveryguard/guard.go:1` — above `package recoveryguard`

```text
// Package recoveryguard fences the recovery agent (ADR-0106): it may write the deliverables it was asked to
// repair and nothing else. Begin records every file in the run's workspace and fences the change's worktree
// before the agent runs; End puts back whatever the agent changed outside its grant and reports it. The kernel
// proves the grant, never the agent's own account of what it did.
```
