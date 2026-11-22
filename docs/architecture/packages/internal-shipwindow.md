# internal/shipwindow

## Purpose

`internal/shipwindow` is the lease that serializes the critical section of a lane ship across concurrent lanes. The section starts where the lane binds its change to `main` and ends at the fast-forward push. When a sibling lane lands on main inside that window, ship sees a moved `HEAD` and forces a deep-tier `AUDIT_BINDING_HEAD_MOVED` re-audit. In cycles 767 to 774 this made the audit run about ten times for eight cycles. The retry of the binding lets a lane survive the race. This lease lets a lane avoid it, because the window becomes mutually exclusive.

Two holders take the lease. Each path holds it over its own part of the section:

| Path | Holder | The lease covers | Pinned by |
|---|---|---|---|
| A normal ship | the ship phase, `enterShipWindow` ([internal-phases-ship](internal-phases-ship.md)) | the audit-binding check, the commit, the re-proof of a recorded carry and the push | `TestShip_TheShipWindowCoversTheBindingTheCarryGateAndThePush` |
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
| D2 (changed by D4) | `DefaultTTL` stays 15 minutes. It is now the limit for a holder that stops beating, not the limit for a section. | D1: a recovery holds the lease over the composed-tree gates (compile, test, acs and apicover), which take minutes. | 5 minutes: a waiter breaks a healthy recovery during its gates, and two lanes then overlap in the window. |
| D3 | A waiter has no wait limit of its own. Only the context of the ship ends the wait. A waiter that breaks a live holder at the TTL reports it (`OnStaleBreak`), and the callers emit `SHIP_WINDOW_LEASE_BROKEN` or `ORCHESTRATOR_SHIP_WINDOW_LEASE_BROKEN`. | F1, the stale break | A wait limit equal to the TTL: a lane behind three healthy holders can wait longer than one TTL. It then ships unleased. |

| D4 | The ship that fails with a moved main fast-forwards the local main of the plane to origin before it ends its lease. It does so only while it holds the lease. | A mid-wave integration batch moves origin only. The loop syncs the local main at the wave boundary. Each lane then rebases onto a stale main, and the push fails again (cycle 2565). | A fast-forward in the batch fixes one writer, and writes the main tree outside a boundary. A reroute of the push rejection alone leaves the rebind on the stale main. |

*Consequence:* a sibling can now land between the audit binding and the ship of a lane. The ship then takes the identity carry, and a carry that does not prove goes back to audit. Operator choice (a) of 2026-10-19 accepts this cost. A carry that is not provable is the condition that changes this decision.

### Heartbeat while held (2026-11-21)

**F4** In wave 184 (2026-11-20 20:40Z to 2026-11-21 02:30Z), cycles 2563, 2575, 2578 and 2580 aborted at ship about 10 times after an audit PASS. Four lanes ran a deep-tier re-audit. The wave made five landings in 5.5 hours. Source: `runs/cycle-*/signals.ndjson` and `ship-journal.jsonl` of the wave.
*Implication:* the lease did not serialize the landing section in that wave.

**F5** `tryAcquire` writes `heartbeat_at` once (`shipwindow.go:158-162`). It breaks a holder whose heartbeat is older than `DefaultTTL` (`shipwindow.go:147`, `runlease.OwnerLive`). A held ship or recovery took 14 to 38 minutes in wave 184. It holds the rebase, the repo-contract scan of 177 packages, the composed gates and the push. Three breaks are recorded, each exactly 15:00 after the heartbeat of the holder (21:33:09, 22:24:09 and 23:24:54).
*Implication:* the premise of D2 and of the bullet "No heartbeat refresh while held" (a held section lasts seconds to minutes) is false.

**F6** Cycle 2578 was broken at 23:24:54 and went on without a lease. It landed `bca876212` at 23:57:48, 40 seconds after cycle 2575 entered ship under the same lease. Cycle 2575 then failed with `GIT_FLEET_REBASE_NEEDED` and `AUDIT_BINDING_HEAD_MOVED`.
*Implication:* a holder that is broken does not learn it, and it lands without the lease.

**F7** The signal center writes an event with a cycle number to the run stream only. It writes to `.evolve/signals.ndjson` only the events that `legacyProjectRecord`, `pairProjectRecord` or `laneResumed` keep (`cmd/evolve/cmd_cycle.go:447-455`). The break signals carry a cycle number, so the wave watch never saw the 3 breaks.
*Implication:* a break must reach the project stream, and the broken holder must learn of it.

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D4 | A live `Lease` rewrites `heartbeat_at` every `TTL/3`. A goroutine that `Acquire` starts does it. The write checks the token first and then replaces the file in one rename. `Release` and the cancel of the `Acquire` context stop the goroutine. A holder that dies or stops stops the beat, and a waiter breaks it at the TTL. A holder that hangs with a live process keeps the beat, and D6 ends it. | F4, F5 | Raise `DefaultTTL` to 60 minutes: a dead holder with a reused pid wedges the queue for an hour. A section of more than 60 minutes still breaks. A land queue with one landing worker: lanes are separate processes, so it needs a new queue, IPC and crash recovery. It gives the same serialization that the lease gives with about 60 lines. |
| D5 | A beat that finds a missing or foreign token marks the lease lost and stops. `Lease.Err` returns a `LostError`. `HeldErr(ctx)` returns it for the lease that `WithHeld` marks. `pushWithRepair` checks `HeldErr` before the first send of each push. A lost lease ends the push with `GIT_FLEET_REBASE_NEEDED` (class transient), so the existing recovery rebases, takes a new lease and ships again. | F6 | A new error code: the recovery, the budget and the router already handle this code, and the message names the cause. Cancel the work of the holder at once: it needs a merged context, and the push check already prevents the harm. |
| D8 | `enterShipWindow` returns the context that carries the mark. `verifyAndLandInShipWindow` and `resumeLanding` pass it down, so the two ship-phase holders see their lease. A transient failure at a worktree push keeps the landing intent `prepared`. The retry resumes at the push under the witness (`landing_intent.go:285-291`, `:405`), and a stale commit is declined there. `ship --push-only` is an operator verb that runs outside the window (`native.go:226-227`). It does not take the lease, and this design leaves it out. | F6, the review of this design | A lease in push-only: a wait behind a 60-minute section blocks an operator recovery. A mark that only the orchestrator sets: the normal ship has no mark, so its check finds no lease. |
| D6 | A hard cap on the total hold time. `DefaultMaxHold` is 60 minutes and it is the one home of the value, as `DefaultTTL` is for the TTL. At the cap the beat removes the lease under the token check, stops, and marks the lease lost with the reason `hard cap`. | F5: a live but wedged holder must stay breakable, and the heartbeat no longer ends it | No cap: a hung holder with a live beat holds the lease for ever. The cap is a design limit, not an operator setting, so it is not in `policy.json`. |
| D7 | `Options.OnLost` tells the holder that its lease is lost. The beat goroutine calls it, so a callback may call only a concurrency-safe sink. | F7 | A callback that writes a log slice: it races the goroutine of the holder. |
| D9 | Both callers emit the existing `SHIP_WINDOW_LEASE_BROKEN` or `ORCHESTRATOR_SHIP_WINDOW_LEASE_BROKEN` from `OnLost`, with the cycle of the holder and the field `role=holder`. The ship phase adds its log line at the push check, on its own goroutine. `newRootSignalCenter` writes both codes to `.evolve/signals.ndjson` as well as to the run stream. The filter is keyed by code, beside `laneResumed`. | F7 | A new code for the holder: the same event is true for both sides, and the field tells them apart. A kind-keyed route: `legacyProjectRecord` needs a zero cycle, and the break events carry a cycle. |

*Consequences:* with a beat, a wave of two lanes lands about once per section time, 15 to 35 minutes. Wave 184 landed about once per hour. A live holder that hangs now holds the lease for at most 60 minutes (D6) and not 15. A sibling that waits for it is slower in that case. The condition that changes D6 is a measured hung-holder case that the cap handles too late.

**Limits.**

- A beat can replace the lease of a sibling, if it stalls for more than the TTL between the token check and the rename. This needs a goroutine that stalls for 15 minutes in a live process.
- The overwritten sibling finds a foreign token at its next beat and aborts at its push. A breaker that finds the rewritten lease fails its `link` with `EEXIST`, so mutual exclusion holds. The cost is one false break report.
- The abort happens at the push, so a holder whose lease is lost still does its gates first.
- The check covers the first send of a push. The transport retry (2, 4 and 8 seconds) and the repair resend do not check again. A lease is lost only at a beat, and the retry window is 14 seconds.
- The test cannot prove the behavior of a real clock. Tests inject the clock and the ticker.

**Red tests.** Each test fails on `main` for the reason that is stated.

| Test | Pins | Red on main because |
|---|---|---|
| `TestAcquire_LiveHolderPastTTLWithHeartbeatIsNotBroken` | a holder with a beat, 40 minutes past acquire, is not broken | the sibling breaks it at 15:00 |
| `TestHeartbeat_ReleaseAndContextCancelStopTheBeat` | the beat stops on `Release` and on cancel (goroutine count) | there is no beat |
| `TestHeartbeat_AForeignTokenMarksTheLeaseLost` | a beat that finds a foreign token marks the lease lost and calls `OnLost` | there is no beat |
| `TestHeartbeat_TheHardCapReleasesAndMarksLost` | the cap frees the lease and marks it lost | there is no cap |
| `TestShipWindowLease_HolderDeathRecovered` (existing) | a holder that does not beat is still broken at the TTL | stays green |
| `TestShipWindow_TwoLanesWithSectionsLongerThanTTLNeverOverlap` | two lanes through `enterShipWindow` and `holdShipWindow`, with a 20 minute section on a fake clock, do not overlap | the second lane breaks the first at 15:00 |
| `TestEnterShipWindow_ALostLeaseAbortsTheLandingWithoutAPush` | a normal ship and a resumed landing that pass the cap reach no `git push` (the runner fails the test on a push) and return `GIT_FLEET_REBASE_NEEDED` | the push runs unleased |
| `TestPushWithRepair_ALostShipWindowKeepsTheIntentResumable` | the abort leaves the landing intent `prepared` | there is no check |
| `TestRootSignalCenter_ShipWindowBreaksReachTheProjectStream` | both codes reach `.evolve/signals.ndjson` | the run stream only |

## Design

- **The lease follows the run-lease liveness pattern.** `runlease.OwnerLive` classifies a held lease. It lives at `<evolveDir>/ship-window.lock` as one JSON object. The writer creates it atomically (temporary file, then `link`), so a reader never sees a torn file. `link` is the mutual-exclusion point: it creates the file with its full content and fails with `EEXIST` if a sibling won.
- **A held lease is respected only while live.** Its heartbeat must be fresh within the TTL and the holder pid must be alive. A dead holder, or a holder whose beat stopped for a TTL, makes the lease stale. A holder that beats and hangs is not stale (D6). A waiter breaks a stale lease. The lease is the only liveness oracle, so no operator is needed.
- **Breaking renames the lock away.** Only one breaker wins the rename. A loser can then never remove a fresh lease that a faster sibling just created at the lock path.
- **A lock that does not parse is crash debris.** Atomic link writes make a torn file impossible. The zero record has an empty heartbeat, which classifies as stale, so the next waiter breaks it.
- **A held lease is refreshed by a beat (D4), within a hard cap (D6).** `Acquire` starts a goroutine that rewrites `heartbeat_at` every `TTL/3` under the token check. `Release` and the cancel of the `Acquire` context stop it. A holder that dies or stops stops the beat, so the TTL still breaks it. A holder that hangs while its process lives keeps the beat, and `DefaultMaxHold` ends it. A lost lease shows in `Lease.Err` and `HeldErr` (D5).
- **A waiter waits for a live holder.** The stale break and the cap make the queue progress. A dead holder breaks at once. A stopped holder breaks at the TTL. A hung holder with a live beat ends at the hard cap. Thus a waiter needs no wait limit of its own, and the caller passes only its context.
- **A break of a live holder is visible.** `Options.OnStaleBreak` receives a `StaleHolder` (the pid and the heartbeat) when a waiter breaks a live holder at the TTL. A dead holder is crash debris, so its break is silent.
- **The holder marks its context.** `WithHeld` marks the context that the orchestrator passes to the re-ship, and `Held` reads the mark. A ship in a marked context does not take the lease again. A second `Acquire` in the holder process waits on itself until the hard cap, because the beat keeps the lease fresh.
- **Waiters queue as ticket files** under `<evolveDir>/ship-window.queue` and acquire in FIFO order of their `Acquire` calls, so no lane starves. A ticket name has three parts. The first is the zero-padded nanosecond time. The second is a per-process sequence number (`ticketSeq`), which breaks ties when a test freezes the clock. The third is the pid, which is unique across processes. The names sort in the order of arrival, and `os.ReadDir` returns them sorted.
- **A crashed waiter cannot wedge the queue.** `queueHead` removes the ticket of a dead pid. It never sweeps the caller's own ticket. A ticket whose pid does not parse is never swept (fail-safe), and it holds the head.
- **`Release` is idempotent and safe after a break.** The record carries a token (the ticket). If the lease was broken and taken again by another lane, the token does not match and `Release` removes nothing. A lock that does not parse also counts as already broken.
- **Pid probe.** `pidAlive` sends signal 0. `EPERM` means the pid exists under another user, so it is alive.
- **Options.** The zero value is ready for production. A zero TTL means `DefaultTTL`. A nil `Now` means `time.Now`.
- A zero `MaxHold` means `DefaultMaxHold`. A nil `Ticker` means `time.NewTicker`. A nil `OnLost` means no report.
- A nil `Alive` means the pid probe. A zero `Poll` means 500 ms. A zero `OwnerPID` means `os.Getpid()`.

## Invariants

- **Mutual exclusion and no re-audit.** `TestShipWindowLease_SiblingWaitsInsteadOfReaudit` and `TestShipWindowLease_HeldLeaseBlocksSibling` pin them.
- **Stale break.** `TestShipWindowLease_HolderDeathRecovered` pins both paths (dead holder, TTL expiry).
- **FIFO.** `TestShipWindowLease_FIFOFairness` pins the order.
- **Safe `Release`, corrupt lock and dead waiter.** The edge tests in `shipwindow_edge_test.go` pin them.
- **Each path holds the lease only over its part of the section.** The table in Purpose names the tests (integration tier). `TestRecordAndBranch_AnAuditPassLeavesTheShipWindowFreeForASibling` (integration tier) pins that an audit PASS leaves the lease free.
- **A waiter outlasts a healthy holder without a WARN.** `TestEnterShipWindow_AWaiterOutlastsAHealthyHolderWithoutAWarn` pins it.
- **A live holder break is reported, a dead one is not.** `TestStaleHolder_OnStaleBreakReportsALiveHolderBrokenAtTheTTL` and `TestOnStaleBreak_IsSilentForADeadHolder` pin it. `TestHeld_WithHeldMarksTheContextOfTheHolder` pins the mark.
- **A live holder past the TTL is not broken.** `TestAcquire_LiveHolderPastTTLWithHeartbeatIsNotBroken` and `TestShipWindow_TwoLanesWithSectionsLongerThanTTLNeverOverlap` pin it.
- **A lost lease never lands.** `TestEnterShipWindow_ALostLeaseAbortsTheLandingWithoutAPush` pins it.
- **Lease path.** `TestShipWindowLease_PathIn` pins `<evolveDir>/ship-window.lock`, the path that operators search for and that `gc` sweeps.

## Findings

- None.
