# Worktree provisioning contention: the retry, and the refuted alternative
**Period:** 2026-08-02 → 2026-08-04 · **Status:** shipped
**Primary artifacts:** PR #401 (merge `a497ffe1`) · refuted PR #400 (CLOSED, never landed) · `go/internal/core/worktree.go` · `go/internal/core/worktree_retry_test.go` · `go/internal/gitexec/worktree.go` (consolidation, cycle commit `751791ac`) · ADR-0082 · ADR-0083 · inbox `2026-08-03T02-20-00Z-worktree-provisioning-retry.json`

## Problem

Fleet lanes provisioning per-cycle worktrees of one shared repo were dying at
`git worktree add -B`: the command returned rc=255 with nothing on stderr
beyond `Preparing worktree` when two lanes collided on locks in the shared
`.git` (the runtime plane is itself a linked worktree of the console repo).
One transient collision cost the lane its **entire cycle**: `ActiveWorktree`
stayed empty, the CB.2 fleet guard fail-fasted every subsequent dispatch with
`exit=10`, and three identical failure fingerprints tripped the breaker and
halted the whole batch.

Blast radius per the PR #401 body: cycles 1221/1231/1232/1234/1240/1250 plus
the 1252 halt — **seven collisions across three batch windows, two batch
halts**, one window with zero console git activity (so lane-vs-lane
contention, not operator-vs-lane).

## Context & evidence

What the operator saw first was not the defect. It was the alarm:

- Every *second* cycle in a fleet lane died at scout with
  `exit=10: fleet mode: explicit worktree required (refusing process-cwd
  fallback)` — identical fingerprints, breaker halt (PR #400 body). Retro had
  hit the same refusal non-fatally the day before (filed as
  `retro-fleet-worktree-dispatch`).
- The decisive evidence came from the adversarial review of PR #400, whose F4
  demanded: *check lane stderr for `WARN worktree provisioning failed`*. The
  WARN was present **on exactly the failing cycles** — `git worktree add -B
  cycle-…-1231/1232/1234` had died rc=255 (PR #400 closing comment,
  2026-08-03). Scout's `exit=10` was the **alarm**; provisioning was the
  **defect**. CB.2 had correctly fail-fasted lanes whose worktree never
  existed.
- The rc=255 shape — nothing on stderr beyond `Preparing worktree` — and the
  zero-console-activity window proving lane-vs-lane contention are recorded in
  the `worktree.go` retry comment and the `worktree_retry_test.go` file
  header on main.

## Approaches considered

**1. Refuted: make pre-worktree phases dispatch the project root (PR #400).**
The plausible fix, fully built with two RED-first tests: since scout/intent/
triage run *before* the cycle worktree exists (and post-teardown retro after
it is gone), `ActiveWorktree` is legitimately empty there, so let the runner
bridge fall back to `BridgeRequest.Worktree = req.ProjectRoot` — "deliberate
and named", guard untouched (PR #400 body). The adversarial review **refuted
it**: `Worktree` is not a cwd field but the **write-authority predicate**,
keyed by multiple subsystems — the PR's own text enumerates the
`Worktree == ""` consumers it was trying not to disturb (build-floor skip,
contract roots, preservation), and the review's F1–F3 and F5 showed what
dispatching `ProjectRoot` actually does: reclassifies read-only phases as
write-authorized (F1), opens a sandbox write-path onto the shared main tree
(F2), kills the ollama driver (F3), and enables cross-lane artifact theft
(F5). Worse, it would have **silenced the alarm** for a defect it did not fix:
the lanes were failing because provisioning failed, and this change would have
sent them to the shared main tree instead of surfacing that. Closed with a
full evidentiary comment; the branch's two dispatch tests "pin a contract the
codebase deliberately rejects; nothing from this branch should land" (PR #400
closing comment). The refutation is now load-bearing: `retroWorktree`'s doc
comment cites it when rejecting `ProjectRoot` as a fallback
(`go/internal/phases/retro/retro.go:88-93`), and
`gitexec.AddWorktreeWithRetry`'s contract cites it to justify keeping the
fail-fast armed (`go/internal/gitexec/worktree.go`).

**2. Considered: cross-lane git serialization.** The verifylock single-flight
is the named precedent for serializing cross-lane git operations
(`worktree.go` comment; PR #400 closing comment). Rejected as first response
in favor of the narrower change: a bounded retry at the failing operation
treats contention as what it is without adding a new lock surface every git
caller must honor.

**3. Chosen: bounded, backoff'd retry at provisioning.** First try + two
retries (2s/4s), announced per attempt; a persistent failure still fails
loudly with the identical error after attempt 3.

## Decision & reasoning

Fix the defect, not the alarm. The reasoning chain, from the PR #401 body and
the code comments it left behind:

- rc=255 under concurrent lane provisioning is transient lock contention on
  the shared `.git`; the same command succeeds standalone. Contention deserves
  a bounded wait, not a dead lane.
- The downstream alarm chain — WARN → source-phases-blocked → CB.2 fail-fast —
  is **correct and must stay armed**. So the retry is a collision absorber
  only: after the bound, the original loud error surfaces unchanged. "The
  refuted #400 is the permanent record of what silencing that alarm would have
  cost" (PR #401 body).
- Named trade-off: a genuinely persistent failure now costs up to ~6s of
  backoff before failing. That trade-off later proved sharper than expected —
  see Results.

## Implementation

Shipped in PR #401 (merged `a497ffe1`, 2026-08-03), now living in
`gitWorktree.Create` / `CreateFrom` (`go/internal/core/worktree.go`) on top of
the shared `gitexec.AddWorktreeWithRetry` loop:

- RED first, both directions (`go/internal/core/worktree_retry_test.go`):
  `TestGitWorktreeCreate_RetriesTransientAddFailure` — transient-once
  succeeds on attempt 2 with **exactly one** backoff sleep counted;
  `TestGitWorktreeCreate_PersistentFailureStillFailsLoudly` — identical
  error after **exactly 3** attempts, rc=255 preserved in the message. The
  fake reproduces the live incident shape (rc=255 + "Preparing worktree")
  through the existing `gitRunner` seam; the fixture is real git.
- **The test-sleep balloon, kept in the record as a confession** (PR #401 body
  and the `init()` comment in `worktree_retry_test.go`): the first version's
  *real* backoff sleeps ran inside every core test whose fixture fails
  provisioning by design (the scenario engine, dozens of times) — ballooning
  the package ~52s → >600s and timing out the commit-gate **twice while
  looking exactly like the host contention the fix addresses**. The author
  blamed load; a quiet host falsified it. Fix: a package-init no-op sleep
  (`func init() { worktreeAddRetrySleep = func(time.Duration) {} }`),
  mirroring the runner's `settleSleep` precedent; tests that *count* sleeps
  install their own recorder and restore the no-op. core back to 54.7s under
  `-race`.
- Verification: `go test -race -count=1 ./internal/core/ ./cmd/evolve/` — PASS
  (PR #401 test plan).
- **Consolidation (cycle commit `751791ac`, 2026-08-04, ADR-0082):** #401
  fixed exactly one of the **four** `git worktree add` call sites. Residual M1
  from the review (inbox item notes) drove `AddWorktreeWithRetry` into
  `internal/gitexec` — the one bound (`DefaultWorktreeAddAttempts = 3`),
  knobs-as-a-value so each caller keeps its own test seam — adopted at
  `core.Create`, `core.CreateFrom` (a collision there drops *salvaged* work),
  `swarm/provision.go` (highest contention), and `cmd_worktree.go` (operator
  CLI). "Wiring a fix into one execution path only is the same defect, just
  narrower" (ADR-0082).

## Results (measured)

- **Ten live saves, zero losses.** The runtime console-loop logs
  (`evolve-loop-runtime/.evolve/loop-console-20260804-010604.log`: 8 lines;
  `…-205556.log`: 2 lines) record `[worktree] retry 1/2 … after transient
  rc=255` for cycles 1254, 1257, 1265, 1267, 1268, 1272, 1283, 1286, 1292 and
  1296 — every one succeeded on attempt 2. Zero `retry 2/2` lines, zero
  terminal `worktree add` failures in the same logs. Each of those ten lines
  is a cycle that under the old code would have died with the exit=10 alarm;
  three of them in one window would have halted the batch again.
- **The fix minted its own defect class, measured and closed.** The
  consolidated loop initially retried on *any* non-zero exit, so a permanent
  `fatal: not a git repository` (rc=128, over a `t.TempDir()`) bought the full
  2s+4s ladder — 33 `cmd/evolve` tests reach the loop transitively, 33 × 6s =
  198s in a package the build floor runs with `-timeout 120s`. Deterministic
  RED that killed cycle-1268's build floor; measured with a
  `grep -c "\[worktree\] retry 1/2"` → 33 (ADR-0083 evidence table). Fixed by
  classification: `RetryableWorktreeAddFailure`, a deny-list of
  proven-permanent stderr markers, consulted **before** any backoff is paid
  (`gitexec/worktree.go`; ADR-0083).

## Retrospective — what we learned

- **Alarm vs defect.** The loudest symptom (scout exit=10) named the guard;
  the defect lived three layers down in provisioning. The move that cracked it
  was an evidence demand — *read the lane stderr on exactly the failing
  cycles* — not more reasoning about the guard. Before touching a guard, prove
  the guarded operation succeeded.
- **A refuted PR is an asset.** #400 stays closed-and-cited: two production
  comment blocks (`retro.go`, `gitexec/worktree.go`) now use it as the
  argument for keeping `Worktree` a write-authority predicate and the
  fail-fast armed. Writing the refutation into a closing comment made it
  durable; the chronicle exists because most refutations are not.
- **The incident's own shape was used to misattribute twice.** The collisions
  were first blamed on console git activity (falsified by the zero-activity
  window); the test balloon was blamed on host load (falsified by a quiet
  host). When the suspected cause is ambient, remove it and re-measure.
- **Retry-everything converts permanent errors into latency.** A retry needs a
  transience classifier, and the announcement must not claim a diagnosis the
  code never made — the OnRetry text says "retryable", not "transient",
  precisely because a permanent rc=128 was once logged as contention 33 times
  per run (`worktree.go` comment).
- **One-site fixes recur as fresh incident classes.** M1 → ADR-0082's
  four-site consolidation, before the next site presented independently.
- **Still open** (recorded in the inbox item's notes; verified against main):
  M2 — the terminal error still reports only the *last* attempt's rc/stderr
  (a SIGKILL'd attempt 1 can leave the dir and mask the transient behind
  attempt 2's rc=128 "already exists"); L4's ordering half — the loop still
  sleeps before announcing; L5 — the "repo-level lock" comment wording; and
  fingerprinting the *provisioning* error rather than the downstream scout
  alarm.

## Links

- ADR-0082 (shared retry contract) · ADR-0083 (transience classification)
- Refuted PR #400 (closed with evidence) · PR #401 (merge `a497ffe1`) ·
  consolidation `751791ac`
- Inbox record: `evolve-loop-runtime/.evolve/inbox/2026-08-03T02-20-00Z-worktree-provisioning-retry.json`
- Sibling entries: [Retro-fleet stale-worktree](2026-08-retro-fleet-stale-worktree.md)
  — cycle-1270 falsely claimed *this* PR fixed the torn-down-worktree shape
  (it fixes the never-provisioned one); [The graduation test-only
  class](2026-08-graduation-test-only.md) — the same week's other
  identical-fingerprint batch-halt class; [Channel e2e
  deflake](2026-08-channel-e2e-deflake.md) — the companion lesson on time in
  tests; [Fingerprint identity](2026-07-fingerprint-identity.md) — the breaker
  that turned these collisions into halts.
