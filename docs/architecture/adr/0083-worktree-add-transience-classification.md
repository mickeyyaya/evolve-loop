# ADR-0083 — Classify `git worktree add` failures before paying backoff

- **Status:** Accepted (cycle-1270)
- **Supersedes nothing.** Amends [ADR-0108](0108-shared-worktree-add-retry.md) (filed as ADR-0082), whose §83 claim
  ("test tiers install a no-op sleep … so no suite pays the ladder") is false on the
  transitive-dispatch axis.

## Context

ADR-0108 lifted PR #401's bounded, backoff'd `git worktree add` retry into
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
gitWorktree.Create`). ADR-0108's mitigation — core's unexported `worktreeAddRetrySleep` seam —
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

## Amendment (2026-10-05) — the floor's deadline is the shared go-test budget

The rejected alternative above, raising the floor's `-timeout 120s`, was right for its
evidence: cycle 1270's 241 s was 198 s of backoff, a regression the deadline exposed. By
cycle 1798 the same package took 105–127 s when green, with no regression behind it, and
the floor killed it on every correction round (cycles 1787, 1791, 1792 and 1798). A deadline
that a green package crosses on host load alone blames whichever lane touches the package,
and the lane may not raise it (inst-L1763b).

The floor's untagged self-check and its coverage pass now run under
`addedtests.PackageTimeout`, the budget its tagged added-test check and ship's repo contract
already used. A source scan (`TestGoTestTimeouts_AreTheSharedPackageBudget`,
`internal/guards`) refuses any other `-timeout` value in pipeline code. The slow-package
alarm this ADR relied on moves to a package wall-time budget check, inbox
`cmd-evolve-unit-wall-time-and-package-budget-check`.

The deadline a run holds the host verification lock under and the time a lane waits for
that lock are separate budgets. The hold time is for correctness, so the coverage pass
holds the lock for up to the 20 m go-test budget, as ship's repo contract does. The wait
is for fleet fairness: a lane queued behind a hung holder waits at most
`verifylock.MaxWait` (15 min), then runs unserialized and says so, the bound the ACS
suite already used (the verification single-flight,
[change log 2026-07-30 §1](../../operations/change-log-2026-07-30.md)). Record:
[internal-core.md](../packages/internal-core.md) (build handoff floor) and the CHANGELOG
entry of 2026-10-05.
