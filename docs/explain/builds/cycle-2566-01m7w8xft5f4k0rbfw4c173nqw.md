# Build Explanation — Cycle 2566

## Build Binding
- Cycle: 2566
- Base SHA: 31b6bc80fd0f98c26a512cdd710a923e5c120ded

## Summary
The inbox mover now emits an `inbox.released` signal for each item that the cycle drain moves out of `processing/cycle-N/`. The stale-claim sweep also warns with `INBOX_CLAIM_STALE` when it releases a claim of a cycle that already sealed.

## Rationale
In wave 182, the claims of failed cycles stayed in `processing/` and no `inbox.released` event existed. The drain (`Mover.Release`, `releaseOne`) moved items without a signal. The sweep gave no warning for a sealed cycle that still held a claim. The smallest fix reuses the existing signal center, the existing event kind and the existing holder verdict.

## Changed Areas
- `go/internal/inboxmover/lifecycle/release.go` — `releaseOne` emits one `inbox.released` event after a successful move, so each drained item is signalled once.
- `go/internal/inboxmover/lifecycle/release_signal.go` — adds the registered code `INBOX_CLAIM_STALE` and the `emitReleased` helper of the mover. A new file keeps the protected `lifecycle.go` unchanged.
- `go/internal/inboxmover/claims.go` — `ReleaseStaleClaims` warns when the holder verdict carries a closeout, which means that the cycle sealed.
- `go/internal/inboxmover/lifecycle_events.go` — adds `warnSealedClaim`, which builds the WARN event for the sealed cycle.
- `docs/architecture/signal-codes.md` — lists the new code in the generated catalog table.
- `go/internal/inboxmover/lifecycle/mover_test.go` — the code table names the new code, and the recorder gains a `faults` view of the warning events.
- `go/internal/inboxmover/lifecycle/release_test.go` — existing assertions read the warning events only, because the drain now also emits an INFO release event.
- `go/internal/inboxmover/lifecycle_seam_test.go` — the code assertion filters on warning events for the same reason.
- `go/internal/inboxmover/testdata/sequence.stderr.wired.golden` — the sequence numbers of the signals shift by the new release events.
- `docs/explain/builds/cycle-2566-01m7w8xft5f4k0rbfw4c173nqw.md` — this document.

## Design Decisions
The system-halted failure path needed no new code: `ApplyCycleOutcome` already calls the drain, so the release signal covers it. The sweep warning keys on the existing closeout evidence of the holder, and no second sealed-cycle probe exists. Rejected: a new signal kind for the drain, because `inbox.released` already exists and the mover already emits it for single claims.

## Verification
The three tests of the task and the three ACS predicates of cycle 2566 pass. The `inboxmover` packages and the `signalcenter` package pass with the updated assertions.

## Compatibility
No signature changes. A mover without a wired signal center behaves as before. Consumers of the warning stream see one new WARN event, `INBOX_CLAIM_STALE`, which the stale-claim sweep emits when it releases the claim of a sealed cycle. The drain release event is INFO and adds nothing to the warning stream.

## Limitations
The sweep warns only when closeout evidence exists. A lane that exits before it writes any closeout releases its claim through the same sweep without the warning.
