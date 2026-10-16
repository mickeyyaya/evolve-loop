# Comment history: `internal/campaign`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h2

### `go/internal/campaign/campaign_test.go:80` — above `if len(waves) != 2 || len(waves[0]) != 1 || waves[0][0].Scope[0] != "a" || waves[1][0].Scope[0] != "b" {`

```text
// a (no deps) in wave 0; b (deps a) in wave 1.
```

### `go/internal/campaign/executor.go:42` — above `BeforeWave func(context.Context) error`

```text
// BeforeWave runs before each non-skipped wave — e.g. a CLI-health canary that
// clears expired benches so the wave doesn't re-hit a recovered wall. nil = no-op.
// It takes the run's live context and its error aborts the run before the
// wave is dispatched (F20: an interrupt during the probes must not spawn a
// wave that is cancelled at birth).
```

### `go/internal/campaign/executor_test.go:261` — above `func TestRunWaves_BeforeWaveErrorAbortsBeforeTheWave(t *testing.T) {`

```text
// F20 (2026-09-15): the pre-wave probes run inside BeforeWave; an interrupt
// that lands there must abort BEFORE the wave is dispatched (the loop runner
// printed "wave 2: 0/2 lanes ok" for a wave cancelled at spawn). The hook
// takes the live context and its error aborts the run.
```

### `go/internal/campaign/ownership.go:3` — above `import (`

```text
// ownership.go — cross-session campaign ownership lease (ADR-0059). A campaign
// is keyed by goal hash, but its progress checkpoint lives under the running
// worktree's .evolve dir — so two autonomous sessions running the SAME plan
// from DIFFERENT worktrees never see each other and silently clobber, and a
// relaunch SIGTERM-reaps the incumbent. This lease moves the OWNERSHIP signal
// into a namespace every worktree shares (the git common dir), keyed by goal
// hash, guarded by a non-blocking flock: a second `campaign run` on the same
// goal hash learns who owns it and refuses (attach or stop-then-start) instead
// of clobbering. The flock auto-releases on holder death (even SIGKILL), so a
// dead owner's lease is immediately re-acquirable — no stale-PID heuristics.
```
