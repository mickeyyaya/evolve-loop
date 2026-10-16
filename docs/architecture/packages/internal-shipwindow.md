# internal/shipwindow

## Purpose

`internal/shipwindow` is the lease that serializes the critical section from the audit binding snapshot to the ship push across concurrent lanes. A sibling lane can land on main between the audit-binding `HEAD` snapshot and the ship push. Ship then sees a moved `HEAD` and forces a deep-tier `AUDIT_BINDING_HEAD_MOVED` re-audit. In cycles 767 to 774 this made the audit run about ten times for eight cycles. The retry of the binding lets a lane survive the race. This lease lets a lane avoid it, because the window becomes mutually exclusive.

## Design

- **The lease follows the run-lease liveness pattern.** `runlease.OwnerLive` classifies a held lease. It lives at `<evolveDir>/ship-window.lock` as one JSON object. The writer creates it atomically (temporary file, then `link`), so a reader never sees a torn file. `link` is the mutual-exclusion point: it creates the file with its full content and fails with `EEXIST` if a sibling won.
- **A held lease is respected only while live.** Its heartbeat must be fresh within the TTL and the holder pid must be alive. A dead holder before the TTL, or a TTL expiry for a live but hung holder, makes the lease stale. A waiter breaks a stale lease and goes on. The lease is the only liveness oracle, so no operator is needed.
- **Breaking renames the lock away.** Only one breaker wins the rename. A loser can then never remove a fresh lease that a faster sibling just created at the lock path.
- **A lock that does not parse is crash debris.** Atomic link writes make a torn file impossible. The zero record has an empty heartbeat, which classifies as stale, so the next waiter breaks it.
- **No heartbeat refresh while held.** The guarded window is about five to ten minutes, from binding snapshot to push. `DefaultTTL` (15 minutes) is a ceiling above it. It still bounds how long a hung holder can wedge sibling ships. A holder that outlives the TTL is hung by definition and must be breakable. A refresh defeats that.
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
- **Lease path.** `TestShipWindowLease_PathIn` pins `<evolveDir>/ship-window.lock`, the path that operators search for and that `gc` sweeps.

## Findings

- None.
