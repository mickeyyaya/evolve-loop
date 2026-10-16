# Comment history: `internal/paths`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d5

### `go/internal/paths/paths.go:175` — above `func AbsoluteRoot(label, p string, warn func(string)) string {`

```text
// AbsoluteRoot resolves p — a project root, possibly relative or "." — to an
// absolute path. A relative root silently breaks path-crossing contracts: a
// worktree-phase agent (cwd=worktree) and the in-process bridge (cwd=main repo)
// resolve the same relative artifact path to DIFFERENT absolute locations, which
// caused cycle-119's ExitArtifactTimeout. This is the single shared helper
// applied at every command entrypoint (replacing the one-off closure in
// cmd_loop.go and the cwd-bound envOrCwd pattern).
//
// On the rare filepath.Abs error it WARNs via warn (when non-nil) and returns p
// UNCHANGED rather than silently proceeding with a broken relative path — fail
// loud, since continuing would reproduce the very timeout this guards against.
// label names the source (e.g. "--project-root") for the warning.
```

### `go/internal/paths/paths.go:212` — above `func RunWorkspace(projectRoot string, cycle int) string {`

```text
// RunWorkspace returns <projectRoot>/.evolve/runs/cycle-<cycle>, the per-cycle
// run workspace — the ONE home of that layout (ADR-0103 unit 09).
// core.RunWorkspacePath projects it for the orchestrator's callers; the defect
// ledger, which lives below core, reads it directly.
```

### `go/internal/paths/paths_absroot_test.go:9` — above `func TestAbsoluteRoot(t *testing.T) {`

```text
// TestAbsoluteRoot encodes Workstream A: a single shared helper that resolves a
// (possibly relative) project root to an absolute path. Cycle 119/120 proved a
// relative root silently breaks path-crossing contracts (the agent's worktree
// cwd vs the in-process bridge's main cwd), and the fix in commit 80f4206 lived
// only in cmd_loop.go. This helper is the reusable, tested version applied at
// every entrypoint.
```

### `go/internal/paths/paths_runworkspace_test.go:8` — above `func TestRunWorkspace_JoinsTheCycleWorkspace(t *testing.T) {`

```text
// TestRunWorkspace_JoinsTheCycleWorkspace pins the ONE spelling of the
// per-cycle run-workspace layout, <root>/.evolve/runs/cycle-<N> (ADR-0103 unit
// 09): core.RunWorkspacePath projects it and the defect ledger reads through it.
```
