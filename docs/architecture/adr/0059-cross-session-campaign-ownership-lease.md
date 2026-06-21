# ADR-0059: Cross-session campaign ownership lease

Status: Accepted
Date: 2026-06-21
Relates: ADR-0049 (concurrent multi-cycle execution), ADR-0054 (concurrent sibling worktrees); closes a gap in the loop-large-scale-readiness hardening (Fixes A–E) found during the live 47→0 flag-reduction proving run.

## Context

The campaign orchestration layer was hardened (readiness audit Fixes A–E: checkpoint/resume, signal-aware cancellation, failure classification, log attribution, shared-write locking) so a single multi-hour run survives crashes, quota walls, and poison tasks. But the audit assumed a **single operator**. The live 47→0 run exposed a different failure class: it was killed **three times** by a *second autonomous session*.

Root cause, verified in source:

- A campaign is identified by `goalHash = sha256(plan.Goal)` — **identical** across sessions running the same plan (`cmd_campaign.go` `campaignGoalHash`).
- But every cross-session coordinate is **worktree-local**: the progress checkpoint lives under `campaignEvolveDir(projectRoot)` = `<worktree>/.evolve/`, and each autonomous session runs its campaign from its **own** worktree (`flag-campaign-3/4/5`).
- So two campaigns with the same goal-hash but different worktrees are **mutually invisible**. They silently clobber each other's `/tmp` logs/pids and goal-hash progress, and a relaunch's "launch hygiene" SIGTERM reaps the incumbent.

There was **no cross-session lease** on "who owns this campaign." The per-cycle integrity core (ledger, ship gate, state.json RMW) is sound and untouched; this gap is purely in the campaign *instance* layer.

## Decision

Add an **advisory exclusive ownership lease**, keyed by goal hash, stored in a namespace **every worktree of the repo shares** — the git **common dir**.

1. **A non-blocking flock primitive** — `flock.TryLock(path) (release, held, err)` — added to the existing `flock` SSOT package (which only had the blocking `Lock`). It takes `LOCK_EX|LOCK_NB`; `EWOULDBLOCK` maps to `held=true` (refuse, don't queue). A process-local held-set guards same-process re-acquire because flock alone does not deduplicate same-process callers cross-platform (the same belt-and-suspenders the `storage` project lock uses).

2. **The lease object** lives in `internal/campaign/ownership.go` (alongside `progress.go`, its natural home — not a new package). `AcquireOwnership(leaseDir, goalHash, self)` flocks `<leaseDir>/campaign-lease-<goalHash>.json.lock` and, on success, records the owner (`atomicwrite.JSON`). On contention it returns `*HeldError{Owner}` naming the live incumbent.

3. **The shared namespace** is `git rev-parse --git-common-dir`/`evolve/campaign-leases`, resolved by `campaignLeaseDir(projectRoot)`. From any linked worktree this returns the **one** main `.git`, so `flag-campaign-3/4/5` contend on the same lease file. Off a git repo (tests, non-repo roots) it falls back to the worktree-local `.evolve`, so isolated roots self-contain.

4. **`runCampaignRun` refuses-or-attaches:** it acquires the lease after computing the goal-hash and before launching any wave; a `*HeldError` prints the owner and exits non-zero with **zero cycles launched**; otherwise it `defer`s `Release`. `--simulate` (a dry plumbing check, not an owned run) does not take ownership.

5. **Liveness is the flock, not a heartbeat.** The OS releases the flock when the holder dies — even on SIGKILL — so a dead owner's lease is immediately re-acquirable with no stale-PID polling and no races. The recorded owner JSON is informational only (refuse message + `campaign status`); it is authoritative only while the lock is held (the sole moment it is read on the refuse path).

## Why this breaks the reap-loop

A second `campaign run` on the same goal-hash now **refuses at the code level** ("already owned by PID X on `<worktree>` since T — attach or stop it first") instead of clobbering. The correct operator behavior — *adopt, don't relaunch* — is enforced mechanically rather than by discipline (which failed three times). Stop-then-start still works (the lease frees on the incumbent's death); a determined takeover is possible but now obviously deliberate.

## Consequences

Positive:
- A single owner per goal-hash across all worktrees/sessions — the missing invariant of the large-scale hardening.
- Minimal blast radius: one new flock primitive (93.8% covered) + one leaf file in `internal/campaign` + a guarded acquire in `runCampaignRun`. The per-cycle integrity core is untouched.
- Free, race-free liveness via flock auto-release; no daemon, no registry, no PID-reuse heuristics.

Negative / accepted:
- The owner JSON can go stale after a clean release, but it is only ever read while a *live* holder owns the lock, so the staleness is never observed. Documented in `ReadOwner`.
- The `storage` project lock independently re-implements the same `LOCK_EX|LOCK_NB`+mutex pattern; consolidating it onto `flock.TryLock` is a deliberate **non-goal here** (it is integrity-core code the readiness audit said not to touch) — noted as a follow-up.

## Alternatives considered

- **A. PID-file with liveness polling** (write `<pid>`; a contender reads it and checks `kill -0`). Rejected: racy (PID reuse, write/read interleaving) and reinvents what flock gives for free. Auto-release on death is the property that makes the lease robust.
- **B. Lease in the worktree-local `.evolve`** (mirror the progress file). Rejected: that **is** the bug — worktree-local state is not shared across sessions.
- **C. Operator discipline only** ("scan for an existing run before launching"). Rejected: it is the status quo that failed three times; nothing enforces it.
- **D. A global lease registry / coordinating daemon.** Rejected: large surface, new failure mode, overkill for a single-file flock.
- **E. A git merge driver / lock in `.git/config`.** Rejected: wrong layer; does not express process ownership or liveness.
