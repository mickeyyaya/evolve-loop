# ADR-0083 — Classify `git worktree add` failures before paying backoff

- **Status:** Accepted (cycle-1270)
- **Supersedes nothing.** Amends [ADR-0082](0082-shared-worktree-add-retry.md), whose §83 claim
  ("test tiers install a no-op sleep … so no suite pays the ladder") is false on the
  transitive-dispatch axis.

## Context

ADR-0082 lifted PR #401's bounded, backoff'd `git worktree add` retry into
`internal/gitexec` so core, swarm and the operator CLI share one loop. That loop retried on
**any** non-zero exit with a real `time.Sleep` ladder (2s + 4s).

Retrying everything is wrong for conditions no amount of waiting can change. Measured on
cycle-1268/1270:

| Claim | Evidence |
|---|---|
| `./cmd/evolve` FAILS under the build floor's exact invocation | `go test -count=1 -timeout 120s ./cmd/evolve` → `FAIL … 120.234s` |
| The failure is backoff, not a broken test | same package at `-timeout 600s` → `ok … 241s` |
| 33 tests each pay the full ladder | `grep -c "\[worktree\] retry 1/2"` → 33 (33 × 6s = 198s) |
| The retried condition is PERMANENT | `fatal: not a git repository` on a `t.TempDir()`, rc=128 |

Those 33 tests reach the loop transitively (`runLoop → Orchestrator.RunCycle → newCycleRun →
gitWorktree.Create`). ADR-0082's mitigation — core's unexported `worktreeAddRetrySleep` seam —
is in a different package than the tests that pay, so it cannot reach them. The result was a
deterministic build-floor RED that killed cycle-1268 and re-fires on any diff touching
`go/cmd/evolve`.

A second, independent defect made it undiagnosable: the floor truncated a failing package's
output with `output[:400]`, but `go test` writes `--- FAIL`, panics and stack traces at the
**tail**. Cycle-1268's recorded reason was 400 bytes of `[engine] WARN: Deps.TokenResolver is
nil` and nothing about what failed.

## Decision

1. `gitexec.WorktreeAddRetry` gains `Retryable func(code int, stderr string) bool`, consulted
   **before** each backoff. Nil ⇒ today's retry-everything, so the zero value and every
   pre-existing caller are unchanged.
2. `gitexec.RetryableWorktreeAddFailure` is the shared classifier, beside the loop, so all
   four provisioning sites classify identically instead of re-deriving the question.
3. The classifier is a **deny-list**: only conditions *proven* permanent (`not a git
   repository`, `is already checked out`, `already exists`) are non-retryable. Contention is
   the open-ended class — rc=255 with nothing but "Preparing worktree" is the live incident
   shape — so anything unrecognised stays retryable. A misclassified transient costs a lane
   its cycle; a misclassified permanent costs 6s. The asymmetry sets the default.
4. The retry announcement says **"retryable"**, not "transient". `OnRetry` fires for failures
   the classifier did not rule out, which is not the same as one established to be
   contention; the old wording is how a permanent rc=128 was logged as contention 33× a run.
5. The build floor keeps the **tail** of a failing package's output
   (`core.floorFailureDiagnostic`), at the same byte cap, marked with a leading `…`.

## Consequences

- `./cmd/evolve` under the floor's own invocation: **FAIL @ 120s → ok in 46.3s.**
- The fail-fast alarm chain stays armed. A persistent failure still returns the final exit
  code and git's own stderr intact — refuted PR #400 is the record of silencing it instead.
  Speed comes from not *waiting*, never from not *reporting*.
- Rejected alternatives: raising the floor's `-timeout 120s` (hides a 198s regression behind
  a bigger number and leaves every runtime lane paying the tax); exporting
  `core.worktreeAddRetrySleep` (exported mutable test state across a package boundary is a
  worse contract than not sleeping on permanent failures).
