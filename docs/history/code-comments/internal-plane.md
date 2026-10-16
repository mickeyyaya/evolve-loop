# Comment history: `internal/plane`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d12

### `go/internal/plane/plane.go:1` — above `package plane`

```text
// Package plane classifies which worktree plane a project root occupies
// (ADR-0080 runtime/console separation). The loop runs in a dedicated LINKED
// worktree; the PRIMARY checkout is the operator's console. Batch-15 lost
// four lanes (1149-1152) to operator activity in a shared checkout — the
// boot-time classification below is the standing tripwire against relaunching
// the loop into the operator's tree.
//
// Filesystem-only by design: `.git` is a directory in the primary checkout
// and a `gitdir:` pointer file in a linked worktree, and HEAD is a one-line
// ref file in both. No git subprocess — hermetic, dependency-free, callable
// before any bridge or exec seam exists at boot.
```

### `go/internal/plane/plane.go:23` — above `IsLinkedWorktree bool`

```text
// IsLinkedWorktree is true when .git is a gitdir pointer file — the
// ADR-0080 runtime plane shape. False = the primary checkout (console).
```

### `go/internal/plane/plane.go:90` — above `func CommonGitDir(i Info) (string, error) {`

```text
// CommonGitDir resolves the SHARED git directory (the hub) for a classified
// root: the primary's .git itself, or a linked worktree's gitdir walked
// through its `commondir` file. The hub sits OUTSIDE every worktree — the
// operator console lease lives there precisely so no lane phase can author
// or shadow it from inside a checkout (ADR-0080 S4 review BLOCK).
```

### `go/internal/plane/plane.go:110` — above `func BootLine(i Info) string {`

```text
// BootLine renders the one-line plane report the loop prints at launch. A
// linked worktree (the runtime plane) is informational; the PRIMARY checkout
// carries the ADR-0080 warning — operator edits in a shared tree are exactly
// what killed lanes 1149-1152.
```

### `go/internal/plane/plane_test.go:3` — above `import (`

```text
// plane_test.go — ADR-0080 S2: the loop must KNOW which worktree plane it
// occupies. Batch-15 lost four lanes to operator activity in the shared
// primary checkout (1149-1152); the classification below is what lets the
// loop boot-WARN when launched from the primary and lets `evolve doctor
// plane` report the layout. Filesystem-only on purpose: .git is a directory
// in the primary checkout and a `gitdir:` pointer FILE in a linked worktree —
// no git subprocess, hermetic under t.TempDir.
```

### `go/internal/plane/plane_test.go:96` — above `func TestBootLine_WarnsOnlyOnPrimary(t *testing.T) {`

```text
// TestBootLine_WarnsOnlyOnPrimary pins the operator-facing contract: the boot
// line for a linked worktree is informational; for the PRIMARY checkout it
// carries the ADR-0080 warning that operator activity here kills lanes.
```
