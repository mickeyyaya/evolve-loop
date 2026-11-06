# Build Explanation — Cycle 2199

## Build Binding
- Cycle: 2199
- Base SHA: a927cbbeecadd49900199ee5bd102a8ae62963bc

## Summary
A repeated failure closeout of one cycle now adds no second block to the item. Two distinct failed cycles still add two blocks.

## Rationale
The failure path claims the released item again on a repeat closeout. The bump then raised `failure_count` a second time and pushed the item to the ceiling. The item now records the cycle of its last charge, and a bump for that same cycle does nothing.

## Changed Areas
- `go/internal/inboxbatch/fieldowner.go` — adds the `last_failure_cycle` stamp constant and registers it as loop-stamp owned.
- `go/internal/inboxmover/lifecycle/item.go` — the bump skips a cycle that already charged the item and stamps the cycle.
- `go/internal/inboxmover/lifecycle/release.go` — passes the releasing cycle to the bump.
- `go/internal/inboxbatch/fieldowner_test.go` — lists the new stamp in the owner table expectations.
- `go/internal/inboxmover/lifecycle/release_test.go` — expects the new stamp in the quarantined bytes.
- `go/internal/inboxmover/lifecycle_seam_test.go` — expects the new stamp in the bumped item.
- `go/internal/inboxmover/testdata/g2.item.golden` — golden bytes carry the new stamp.
- `docs/architecture/adr/0160-the-host-lineage-record-is-the-one-trust-root-for-what-a-cycle-may-do-to-the-queue.md` — rule 10 states one block per charge, as in ADR-0126 decision 16.

## Design Decisions
One stamp on the item is the smallest change. A cycle-keyed ledger lookup was rejected because it adds a second store. The public `BumpFailureCount` passes cycle 0 and keeps its old behavior.

## Verification
The ACS predicates TestC2199_001 to 007 pass. The ACS suite reports green=270 red=0. The inboxbatch, inboxmover and cycleoutcome packages pass.

## Compatibility
Older items have no stamp and read as cycle 0, so the first bump charges. The stamp survives a quarantine release. A later failure under the same cycle number therefore charges nothing. An alternating A, B, A repeat of cycles is also not deduplicated.

## Limitations
The stamp holds one cycle. A repeat of an older cycle after a newer one charges again. An item that a quarantine release keeps with its stamp does not charge when a later failure carries the same cycle number.
