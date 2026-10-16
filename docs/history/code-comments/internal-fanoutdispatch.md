# Comment history: `internal/fanoutdispatch`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d6

### `go/internal/fanoutdispatch/fanoutdispatch.go:114` — above `type result struct {`

```text
// Per-worker cache-prefix is injected into each worker's OWN env in
// runWorker (cfg.workerEnv), NOT via process-global os.Setenv: the process
// environment is a single MT-Unsafe global, so two concurrent
// DispatchParallel in one process would clobber each other (ADR-0049 S1 /
// gap G8).
```

### `go/internal/fanoutdispatch/fanoutdispatch.go:225` — above `func (cfg Config) workerEnv() []string {`

```text
// runWorker executes a single worker command with timeout. Stdout → name.out,
// stderr → name.err. Returns the exit code (124 for timeout, 143 for SIGTERM).
// workerEnv builds the explicit environment for a fan-out worker subprocess —
// the parent env plus cache-prefix (when configured). Injecting per-child
// (exec.Cmd.Env) rather than mutating the process-global env (os.Setenv) keeps
// two concurrent DispatchParallel calls in one process from clobbering each
// other (ADR-0049 S1 / gap G8). A later duplicate key wins in exec.
```

### `go/internal/fanoutdispatch/fanoutdispatch.go:245` — above `cmd.Env = cfg.workerEnv()`

```text
// per-worker env snapshot (ADR-0049 S1 / G8)
```

### `go/internal/fanoutdispatch/fanoutdispatch_test.go:172` — above `func TestRun_DoesNotMutateProcessEnv(t *testing.T) {`

```text
// TestRun_DoesNotMutateProcessEnv pins ADR-0049 S1 / gap G8: Run must inject
// per-worker budget/cache-prefix into each WORKER's env, never via a
// process-global os.Setenv — two concurrent DispatchParallel in one process
// would otherwise clobber each other's budget. Not t.Parallel: it asserts on
// the shared process env (t.Setenv pins the pre-state and restores it). RED
// before the fix (os.Setenv leaks "0.33" into the parent), GREEN after.
```
