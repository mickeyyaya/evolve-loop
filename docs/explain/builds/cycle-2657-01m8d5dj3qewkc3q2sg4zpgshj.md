# Build Explanation — Cycle 2657

## Build Binding
- Cycle: 2657
- Base SHA: 9a01a96b47635f8bd59af81d0f6affdeb0fb71b2

## Summary
This build enforces write-once deliverable record immutability in `WriteRecordWith` and blocks enforce-stage member re-admission in `Driver.admits`. It also maps `ErrRecordExists` to `core.ShipClassIntegrity` in `worktreeShip.deliver` and stabilizes the consumed timestamp in `itemConsumer.stage`.

## Rationale
`WriteOnce` previously returned a generic error on any `os.ErrExist` collision. Same-patch ship retries failed, and differing-patch retries looked transient. Stabilizing the consumed inbox timestamp in the cycle workspace preserves commit patch IDs across second boundaries. Same-patch retries then resolve as no-ops while differing-patch deliveries refuse permanently.

## Changed Areas
- `go/internal/integration/deliverable.go` — adds `RecordRefusedError` and `recordFile.existingOutcome`. `WriteOnce` returns the path on equal patch IDs and refuses differing patch IDs without modifying the record.
- `go/internal/integration/driver.go` — guards `Driver.admits` so known members are never re-admitted when `Stage` is not `FleetIntegrationShadow`.
- `go/internal/phases/ship/deliverable.go` — maps `ErrRecordExists` in `worktreeShip.deliver` to `core.ShipClassIntegrity` so permanent write-once refusals do not retry as transient failures.
- `go/internal/phases/ship/consume.go` — extracts `itemConsumer.consumedAt` to save the first consumed RFC3339 timestamp in `ship-consumed-at.txt` inside `WorkspacePath`. This keeps patch IDs identical across ship retries.

## Design Decisions
Persisting the consumed timestamp per cycle workspace beats a process-wide `sync.Once` or a constant timestamp. A constant fails recency checks on the consumed record. A process-wide `sync.Once` cannot preserve patch IDs across separate `evolve ship` or `--resume` processes, nor isolate parallel tests. Checking patch IDs inside `recordFile.existingOutcome` under `os.Link` keeps the race-free exclusive-create invariant. `ReplaceExisting` remains untouched for shadow lanes and console PRs.

## Verification
`go test -count=1 ./internal/integration` passes with 100.0% statement coverage on `deliverable.go` and `driver.go`. Under `-tags integration`, `TestDeliverWriteOnce_*` passes 5/5, and `TestShipFromWorktree_NonFleet_DivergedFFMerge_Terminal` passes with `EVOLVE_FLEET` unset. Coverage is 100.0% across `ship/deliverable.go`, `itemConsumer.stage`, and `itemConsumer.consumedAt`.

## Compatibility
`RecordRefusedError` unwraps to `ErrRecordExists` for existing `errors.Is` callers. `ReplaceExisting` keeps `os.Rename` behavior, and `itemConsumer.consumedAt` falls back to the system clock when `WorkspacePath` is empty.

## Limitations
Row 10c (lane `Heads` ejection) belongs to card part 2 and remains outside this item.
