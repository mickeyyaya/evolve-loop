# ADR-0082 — One shared retry contract for `git worktree add`

- **Status:** Accepted (cycle-1268)
- **Supersedes nothing.** Generalizes the single-site fix from PR #401 (`a497ffe1`).
- **Related:** ADR-0076 (continuation architecture — `CreateFrom` is its seeding path), ADR-0032 (swarm harness), refuted PR #400.

## Context

N lanes of one repo provision worktrees concurrently, and `git worktree add` takes
repo-level locks in the SHARED `.git` (the plane itself is a linked worktree). A
collision returns `rc=255` with nothing on stderr beyond `Preparing worktree`.

One transient collision used to cost a lane its entire cycle: `ActiveWorktree`
stayed empty, CB.2 fail-fasted every dispatch with `exit=10`, and three identical
fingerprints halted the batch — observed in cycles 1221/1231/1232/1234/1240, twice
in one day, once with zero console git activity (proving lane-vs-lane, not
operator-vs-lane, contention).

PR #401 fixed this with a bounded, backoff'd retry at exactly **one** of the four
`git worktree add` call sites. The other three still issued the bare, unretried add:

| Site | Path | Exposure |
| --- | --- | --- |
| `core.gitWorktree.Create` | `go/internal/core/worktree.go` | fixed by PR #401 |
| `core.gitWorktree.CreateFrom` | `go/internal/core/worktree.go` | ADR-0076 continuation seeding — a collision drops **salvaged** work |
| `swarm.gitWorkerProvisioner.addWorktree` | `go/internal/swarm/provision.go` | highest contention: N workers, one `.git` |
| `runWorktreeCreate` | `go/cmd/evolve/cmd_worktree.go` | operator CLI; also a raw `exec.Command` with no exit-code reporting and no test seam |

Wiring a fix into one execution path only is the same defect, just narrower (#373).

## Decision

Lift the retry loop into **`internal/gitexec`** as `Git.AddWorktreeWithRetry`, and
adopt it at all four sites.

```go
const DefaultWorktreeAddAttempts = 3            // the ONE bound; no private copies

type WorktreeAddRetry struct {
    Sleep   func(time.Duration)                            // nil ⇒ time.Sleep
    OnRetry func(attempt, attempts, code int, stderr string) // nil ⇒ silent
}

func (g Git) AddWorktreeWithRetry(ctx context.Context, r WorktreeAddRetry, args ...string) (stdout, stderr string, exitCode int, err error)
```

**Why `gitexec` is the home.** It is the only package all four already depend on:
`swarm` documents that it must not import `core` (`provision.go:14-19`), and
`go list -deps ./cmd/evolve` confirms `cmd/evolve` already reaches `gitexec`. No new
import edge, and no cycle-644-shaped unsatisfiable pin.

**Why knobs-as-a-value, not package globals.** Each caller keeps its own test seam
and its own announcement voice (`core` keeps `worktreeAddRetrySleep`, so PR #401's
tests stay green unmodified) while the *loop and the bound* stay single-sourced.
`core` wraps its `Sleep` in a closure because `worktreeAddRetrySleep` is a var the
tests swap — the value must be read at call time, not at config time.

**Why the helper prepends `worktree add` itself.** Callers pass only the add's own
arguments, so no site can drift into a different subcommand while claiming this
retry contract.

## Invariants

1. **A clean add costs exactly one invocation and zero backoff.** The retry is a
   collision absorber, not a rate limiter that taxes every cycle in the fleet.
2. **A persistent failure still fails loudly** after the bound, surfacing the final
   exit code and git's own stderr. The downstream fail-fast alarm chain is CORRECT
   and stays armed — refuted PR #400 is the record of what happens when the alarm
   is silenced instead of the collision absorbed.
3. **Bounded, never unbounded.** Three attempts (`2s`, then `4s` backoff). A third
   identical failure is a real condition the fail-fast must surface.

## Consequences

- The operator CLI gains rc/stderr parity it never had: the old raw
  `exec.Command` path could only report `exit status 255` from `err`. It also gains
  its first test seam (`worktreeGitRunner`, `worktreeAddRetry`) — that surface had
  no injection point, which is why it had no retry coverage to begin with.
- A **permanent** failure (e.g. a non-repo root) now costs the full 6s ladder before
  erroring. This is the tradeoff PR #401 already accepted for `Create`; a
  transient-vs-permanent classifier was rejected as speculative — `rc=255` with an
  empty-ish stderr is not reliably distinguishable from other faults, and a wrong
  classifier would silently re-open the incident. Test tiers install a no-op sleep
  (the `core` precedent, now mirrored in `cmd/evolve`) so no suite pays the ladder.
- `core.worktreeAddAttempts` is removed: the bound now lives in `gitexec` and
  nowhere else.
