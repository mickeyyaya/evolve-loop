# internal/shipwindow

## Purpose

`internal/shipwindow` is the lease that serializes the critical section of a lane ship across concurrent lanes. The section starts where the lane binds its change to `main` and ends at the fast-forward push. When a sibling lane lands on main inside that window, ship sees a moved `HEAD` and forces a deep-tier `AUDIT_BINDING_HEAD_MOVED` re-audit. In cycles 767 to 774 this made the audit run about ten times for eight cycles. The retry of the binding lets a lane survive the race. This lease lets a lane avoid it, because the window becomes mutually exclusive.

Two holders take the lease. Each path holds it over its own part of the section:

| Path | Holder | The lease covers | Pinned by |
|---|---|---|---|
| A normal ship | the ship phase, `enterShipWindow` ([internal-phases-ship](internal-phases-ship.md)) | the audit-binding check with the trivial-rebase carry, the commit, the re-proof of a recorded carry and the push | `TestShip_TheShipWindowCoversTheBindingTheCarryGateAndThePush` |
| A resumed landing | the ship phase, in `resumeLanding` | the roll-forward and the push of a prepared landing | `TestShip_AResumedLandingPushesInsideTheShipWindow` |
| A fleet-rebase recovery | the orchestrator, `holdShipWindow` ([internal-core](internal-core.md)) | the rebase onto `main`, the identity-carry composed-tree gates (ADR-0105), the commit and the push of the re-ship | `TestRecoverFromShipError_HoldsTheShipWindowFromTheRebaseThroughTheReShip` |

The slow ship checks run before the lease: the repo-contract scanner pack, the explanation gate and the self-SHA check (`TestPhase_TheRepoContractPackRunsOutsideTheShipWindow`). A recovery that routes to Audit or Build releases the lease before that phase (`TestRecoverFromShipError_ARebaseRoutedToAuditReleasesTheShipWindow`).

### Why the window moved (2026-10-19)

**F1** In wave 130, cycle 1956 waited the full 15-minute `DefaultTTL` after its audit passed (01:13:10 to 01:28:10, no child process, no CPU). It then shipped unleased with `WARN ship-window lease not acquired (context deadline exceeded); proceeding unleased` (inbox item `ship-window-holds-only-the-rebase-and-push`).
*Implication:* the waiter timed out on a healthy holder, and then raced the push that the lease guards.

**F2** Before this change, the orchestrator took the lease at audit PASS (`cyclerun_record.go:26`). It held the lease across the whole ship phase. The ship phase runs the repo-contract pack first, about 10 minutes, with a `ship-repocontract-scan.log` of about 500 KB.
*Implication:* with 4 lanes, the lanes ran the audit-PASS-to-push window one at a time.

**F3** Over 66 audits in waves 130 to 133, the lease wait totals 0.8 hours. The spikes are 15.0, 9.5, 8.8 and 8.6 minutes ([wave-latency research F5.2](../../research/wave-latency-2026-10.md)).

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | The ship phase takes the lease after the repo-contract pack. It holds the lease over `verifyClass` and `atomicShip`. It releases the lease before `postShip`. A recovery holds the lease from the rebase to the end of the re-ship. | F1, F2 | Keep the lease at audit PASS with a longer wait: the lanes still ship one at a time for 10 minutes each. Drop the lease: the plan's limit says the composed-tree gate must stay inside the leased window ([wave-latency plan §7](../../plans/wave-latency-2026-10.md)). |
| D2 | `DefaultTTL` stays 15 minutes. | D1: a recovery holds the lease over the composed-tree gates (compile, test, acs and apicover), which take minutes. | 5 minutes: a waiter breaks a healthy recovery during its gates, and two lanes then overlap in the window. |
| D3 | A waiter has no wait limit of its own. Only the context of the ship ends the wait. A waiter that breaks a live holder at the TTL reports it (`OnStaleBreak`), and the callers emit `SHIP_WINDOW_LEASE_BROKEN` or `ORCHESTRATOR_SHIP_WINDOW_LEASE_BROKEN`. | F1, the stale break | A wait limit equal to the TTL: a lane behind three healthy holders can wait longer than one TTL. It then ships unleased. |

*Consequence:* a sibling can now land between the audit binding and the ship of a lane. The ship then takes the trivial-rebase carry or the identity carry, and a carry that does not prove goes back to audit. Operator choice (a) of 2026-10-19 accepts this cost. A carry that is not provable is the condition that changes this decision.

## Design

- **The lease follows the run-lease liveness pattern.** `runlease.OwnerLive` classifies a held lease. It lives at `<evolveDir>/ship-window.lock` as one JSON object. The writer creates it atomically (temporary file, then `link`), so a reader never sees a torn file. `link` is the mutual-exclusion point: it creates the file with its full content and fails with `EEXIST` if a sibling won.
- **A held lease is respected only while live.** Its heartbeat must be fresh within the TTL and the holder pid must be alive. A dead holder before the TTL, or a TTL expiry for a live but hung holder, makes the lease stale. A waiter breaks a stale lease and goes on. The lease is the only liveness oracle, so no operator is needed.
- **Breaking renames the lock away.** Only one breaker wins the rename. A loser can then never remove a fresh lease that a faster sibling just created at the lock path.
- **A lock that does not parse is crash debris.** Atomic link writes make a torn file impossible. The zero record has an empty heartbeat, which classifies as stale, so the next waiter breaks it.
- **No heartbeat refresh while held.** A normal ship holds the lease for seconds to about one minute. A recovery holds it for minutes, over the composed-tree gates, and `DefaultTTL` (15 minutes) is a ceiling above both. It still bounds how long a hung holder can wedge sibling ships. A holder that outlives the TTL is hung by definition and must be breakable. A refresh defeats that.
- **A waiter waits for a live holder.** The stale break makes the queue progress: a dead holder breaks at once, and a hung holder breaks at the TTL. Thus a waiter needs no wait limit of its own, and the caller passes only its context.
- **A break of a live holder is visible.** `Options.OnStaleBreak` receives a `StaleHolder` (the pid and the heartbeat) when a waiter breaks a live holder at the TTL. A dead holder is crash debris, so its break is silent.
- **The holder marks its context.** `WithHeld` marks the context that the orchestrator passes to the re-ship, and `Held` reads the mark. A ship in a marked context does not take the lease again. A second `Acquire` in the holder process waits on itself until the TTL break.
- **Waiters queue as ticket files** under `<evolveDir>/ship-window.queue` and acquire in FIFO order of their `Acquire` calls, so no lane starves. A ticket name has three parts. The first is the zero-padded nanosecond time. The second is a per-process sequence number (`ticketSeq`), which breaks ties when a test freezes the clock. The third is the pid, which is unique across processes. The names sort in the order of arrival, and `os.ReadDir` returns them sorted.
- **A crashed waiter cannot wedge the queue.** `queueHead` removes the ticket of a dead pid. It never sweeps the caller's own ticket. A ticket whose pid does not parse is never swept (fail-safe), and it holds the head.
- **`Release` is idempotent and safe after a break.** The record carries a token (the ticket). If the lease was broken and taken again by another lane, the token does not match and `Release` removes nothing. A lock that does not parse also counts as already broken.
- **Pid probe.** `pidAlive` sends signal 0. `EPERM` means the pid exists under another user, so it is alive.
- **Options.** The zero value is ready for production. A zero TTL means `DefaultTTL`. A nil `Now` means `time.Now`.
- A nil `Alive` means the pid probe. A zero `Poll` means 500 ms. A zero `OwnerPID` means `os.Getpid()`.

## Invariants

- **Mutual exclusion and no re-audit.** `TestShipWindowLease_SiblingWaitsInsteadOfReaudit` and `TestShipWindowLease_HeldLeaseBlocksSibling` pin them.
- **Stale break.** `TestShipWindowLease_HolderDeathRecovered` pins both paths (dead holder, TTL expiry).
- **FIFO.** `TestShipWindowLease_FIFOFairness` pins the order.
- **Safe `Release`, corrupt lock and dead waiter.** The edge tests in `shipwindow_edge_test.go` pin them.
- **Each path holds the lease only over its part of the section.** The table in Purpose names the tests (integration tier). `TestRecordAndBranch_AnAuditPassLeavesTheShipWindowFreeForASibling` (integration tier) pins that an audit PASS leaves the lease free.
- **A waiter outlasts a healthy holder without a WARN.** `TestEnterShipWindow_AWaiterOutlastsAHealthyHolderWithoutAWarn` pins it.
- **A live holder break is reported, a dead one is not.** `TestStaleHolder_OnStaleBreakReportsALiveHolderBrokenAtTheTTL` and `TestOnStaleBreak_IsSilentForADeadHolder` pin it. `TestHeld_WithHeldMarksTheContextOfTheHolder` pins the mark.
- **Lease path.** `TestShipWindowLease_PathIn` pins `<evolveDir>/ship-window.lock`, the path that operators search for and that `gc` sweeps.

## Findings

- None.
