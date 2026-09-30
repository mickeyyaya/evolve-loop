# Comment history: `internal/gitexec`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/gitexec/gitexec.go:1` — above `package gitexec`

```text
// Package gitexec isolates the git CLI behind one small, injectable type
// (P2/P3 of ADR-0050). It depends only on internal/sysexec — the command
// seam — so callers can fake every git invocation in the fast test tier and
// the git dependency lives in exactly one leaf package.
```

### `go/internal/gitexec/gitexec.go:117`

```text
// WorktreeToken was the cycle-360 hotfix for per-root branch namespacing; it is
// superseded by runscope.LaneFromRoot (the single-source naming Value Object,
// byte-identical token via projecthash) and removed in the v20 integration to
// honor single-source — see ADR-0054 and internal/runscope.
```

### `go/internal/gitexec/relation.go:20` — above `type MainRelation struct {`

```text
// MainRelation is the ONE resolution of "where is the local main relative to
// origin/main" — the wave boundary (fast-forward or not, halt or not) and the
// lane base (which ref a fresh lane starts from) both read this struct and
// render its String, so the two can never disagree on the state or the words
// (2026-09-09 token-waste root cause #3).
```

### `go/internal/gitexec/relation_test.go:58` — above `func TestRelationToRemote_FourKinds(t *testing.T) {`

```text
// TestRelationToRemote_FourKinds pins the single main-relation resolver the
// wave boundary and the lane base share (2026-09-09 token-waste root cause #3).
```

### `go/internal/gitexec/worktree.go:33` — above `Retryable func(code int, stderr string) bool`

```text
// Retryable, when non-nil, classifies a failed attempt BEFORE any backoff
// is paid: false ⇒ the condition is permanent, so the loop returns the
// failure immediately instead of sleeping the ladder for nothing.
//
// Why it exists: the loop retried on ANY non-zero exit, so a permanent
// `fatal: not a git repository` (rc=128) bought the full 2s+4s ladder. In
// go/cmd/evolve, 33 tests transitively reach this loop over a t.TempDir()
// that is not a repository — 33 × 6s = 198s of pure sleep in a package the
// build floor runs with `-timeout 120s`. Deterministic, not a flake.
//
// Nil ⇒ today's retry-everything, so the zero value and every existing
// caller keep their behaviour unchanged. This narrows only WHEN the loop
// sleeps: a persistent failure still returns the final exit code and git's
// own stderr intact (see the contract below) — the fail-fast alarm chain
// stays armed, which is what refuted PR #400 got wrong.
```

### `go/internal/gitexec/worktree.go:51` — above `var permanentWorktreeAddStderr = []string{`

```text
// permanentWorktreeAddStderr are the `git worktree add` failure conditions no
// amount of waiting can change. The list is a DENY-list on purpose: contention
// is the open-ended class (rc=255 with nothing on stderr beyond "Preparing
// worktree" is the live incident shape), so anything unrecognised stays
// retryable and PR #401's collision absorber is preserved verbatim. Only
// conditions proven permanent are subtracted from it.
```

### `go/internal/gitexec/worktree.go:81` — above `func (g Git) AddWorktreeWithRetry(ctx context.Context, r WorktreeAddRetry, args ...string) (stdout, stderr string, exitC…`

```text
// AddWorktreeWithRetry runs `git worktree add <args...>` with the bounded,
// backoff'd retry proven by PR #401 (a497ffe1).
//
// Why it exists: N lanes of one repo provision concurrently and `git worktree
// add` takes repo-level locks in the SHARED .git; the observed collision returned
// rc=255 with nothing on stderr beyond "Preparing worktree". One transient collision
// used to cost a lane its whole cycle (ActiveWorktree stayed empty, CB.2
// fail-fasted every dispatch, three identical fingerprints halted the batch).
// The fix landed at exactly one call site; this is that loop lifted to the
// package every provisioning site already depends on, so the three siblings
// adopt it instead of re-deriving it.
//
// Contract:
//   - A clean add costs exactly ONE invocation and ZERO backoff — the retry is a
//     collision absorber, not a rate limiter that taxes every cycle.
//   - A PERSISTENT failure still fails loudly after the bound, returning the
//     final exit code and git's own stderr intact: the downstream fail-fast
//     alarm chain is CORRECT and must stay armed (refuted PR #400 is the record
//     of what happens when the alarm is silenced instead).
//   - The `worktree add` prefix is prepended HERE, so no call site can drift
//     into a different subcommand while claiming this retry contract.
```

### `go/internal/gitexec/worktree_diagnostic_test.go:3` — above `import (`

```text
// worktree_diagnostic_test.go — cycle-1474 RED contract for
// `worktree-retry-diagnostic-integrity`.
//
// AddWorktreeWithRetry absorbs the live collision shape (rc=255, nothing on
// stderr but "Preparing worktree"), but its terminal diagnostic keeps ONLY the
// last attempt: worktree.go:101-126 overwrites (stdout, stderr, exitCode, err)
// on every pass of the loop. When a transient rc=255 is followed by a DIFFERENT
// terminal failure — the recorded SIGKILL/partial-directory shape, where the
// first attempt leaves a half-built directory and the second dies rc=128
// "already exists" — the initiating failure is erased and the operator reads a
// path-collision that never happened.
//
// The second defect is ordering: OnRetry is documented as "called before each
// RE-attempt" so a caller can ANNOUNCE contention, yet the loop sleeps the
// backoff ladder first (worktree.go:121-124) and announces afterwards. A lane
// that stalls inside the 2s/4s window therefore emits no contention line at
// all — the announcement arrives only if the sleep completes.
//
// Nothing here changes the retry BOUND, the permanent-failure classifier, or
// any caller signature: a persistent failure must still surface the FINAL exit
// code and git's own stderr (refuted PR #400 is the record of what silencing
// that costs).
```

### `go/internal/gitexec/worktree_diagnostic_test.go:56` — above `func TestAddWorktreeWithRetry_PreservesFirstFailure(t *testing.T) {`

```text
// TestAddWorktreeWithRetry_PreservesFirstFailure is the crux. Both the
// INITIATING and the TERMINAL failure must remain distinguishable in what the
// helper returns; today only the terminal one survives, so the rc=255 that
// actually started the incident is unrecoverable from the diagnostic.
```

### `go/internal/gitexec/worktree_retry_test.go:3` — above `import (`

```text
// worktree_retry_test.go — RED contract for cycle-1268 task
// `worktree-provisioning-retry-consolidate`.
//
// PR #401 (a497ffe1) proved that a transient `git worktree add` collision
// (rc=255, nothing on stderr but "Preparing worktree") under concurrent
// multi-lane provisioning must be retried, not treated as permanent — one
// collision left ActiveWorktree empty and cost the lane its whole cycle. That
// fix landed at exactly ONE call site (core.gitWorktree.Create). Three siblings
// still issue the bare, unretried add:
//
//	core.gitWorktree.CreateFrom          (continuation seeding, ADR-0076)
//	swarm.gitWorkerProvisioner.addWorktree (N concurrent workers — highest contention)
//	cmd/evolve runWorktreeCreate         (operator CLI)
//
// This file pins the SHARED helper the three adopt. gitexec is the home because
// it is the only package all three already depend on: swarm documents that it
// must not import core (provision.go:14-19), and `go list -deps ./cmd/evolve`
// confirms cmd/evolve already reaches gitexec — so no new import edge, and no
// cycle-644-shaped unsatisfiable pin.
//
// The retry knobs are passed as a value (WorktreeAddRetry), not read from
// exported mutable package globals: that keeps the loop single-sourced while
// letting core keep its existing worktreeAddAttempts/worktreeAddRetrySleep
// identifiers, so PR #401's worktree_retry_test.go stays green unmodified.
```

### `go/internal/gitexec/worktree_retry_test.go:38` — above `func addFailRunner(failures, attempts *int) sysexec.RunFunc {`

```text
// addFailRunner fails the first *failures `worktree add` invocations with the
// live incident's exact shape (rc=255 + "Preparing worktree" noise) and counts
// every attempt. Non-add git calls and post-failure attempts succeed silently —
// this package's helper is being tested, not git.
```

### `go/internal/gitexec/worktree_retry_test.go:87` — above `if code != 255 {`

```text
// The downstream alarm chain (CB.2 fail-fast on an empty ActiveWorktree) is
// CORRECT and must stay armed — a persistent failure still fails loudly with
// the git diagnosis intact. Silencing it is the refuted PR #400.
```

### `go/internal/gitexec/worktree_retryable_test.go:3` — above `import (`

```text
// worktree_retryable_test.go — cycle-1270 blocker (B-1/B-2/B-3).
//
// The retry loop slept the full 2s+4s ladder on ANY non-zero exit, including
// conditions no amount of waiting can change. Measured cost: 33 go/cmd/evolve
// tests reach this loop transitively over a t.TempDir() that is not a git
// repository — a permanent rc=128 — so the package paid 33 × 6s = 198s of pure
// backoff under a build floor that runs it with `-timeout 120s`. Deterministic,
// not a flake.
//
// Three axes, each load-bearing on its own:
//
//	Permanent → zero backoff        the fix
//	Transient → still rides the bound   the NEGATIVE guard: a "fix" that simply
//	                                    stopped retrying would pass the first
//	                                    test and re-break PR #401's absorber
//	Nil Retryable → unchanged       the zero value stays usable, so no existing
//	                                caller silently changes behaviour
```

### `go/internal/gitexec/worktree_retryable_test.go:67` — above `if code != 128 || err != nil {`

```text
// The fail-fast alarm chain stays armed: refuted PR #400 is the record of
// what silencing it costs. Speed must come from not WAITING, never from not
// reporting.
```

### `go/internal/gitexec/worktree_retryable_test.go:128` — above `{"lock collision", 255, "Preparing worktree (new branch 'lane')\n", true},`

```text
// The live incident shape: rc=255 with nothing but "Preparing worktree".
```
