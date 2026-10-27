# internal/fixability

> Decision: [ADR-0158](../adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md) decisions 1 to 6. Plan: [cost-aware-repair-2026-10](../../plans/cost-aware-repair-2026-10.md) items 2, 3 and 11. Callers: `cmd/evolve` (`repairFixability`, the check that `core.WithRepairFixability` receives) and `internal/inboxmover` (`PartitionLaneMenu`).

## Purpose

`internal/fixability` sorts each audit defect into a lane finding or a handoff finding. It stops a repair round when a handoff finding blocks the lane. It also finds the open console companions of a lane item.

## Design

- **Classification.** `Classify` reads the defect strings, the red paths, the gate failures and the open console items. It checks each defect for console items and fence paths.
- **Handoff findings.** A finding becomes a handoff finding when it names an open console item or cites a protected surface. The item id must be whole: `x-console-two` does not name `x-console`.
- **Blockers.** A handoff finding blocks in one of these cases:
  - Its severity is HIGH or CRITICAL.
  - A red path matches a cited path on a path segment boundary.
  - The defect says that tests or predicates fail. The fail verb must come after the noun, with at most a count between them. Thus "floor tests fail (3)" blocks, and "add a test that fails" does not.
  - A gate failure names a cited path, the package of a cited Go file, or a test that the defect names. The name must be whole: a gate on `go/internal/inboxfilex` does not name `go/internal/inboxfile`.
- **Stop decision.** When blockers exist, `Classify` sets `Stop` to true, sets `Reason` to `handoff_blocker`, and attaches the salvage branch.
- **Companions.** `Companions(laneID, open)` gives the open console items that a lane item waits on. A console item is a companion in three cases. Its id starts with the lane id and a dash. Its id starts with the lane id without its `-lane` suffix, and a dash. Its `deps` or `connects_to` names the lane item.
- **Open console items.** `OpenConsole` keeps each inbox item that `inboxbatch.ConsoleRouted` routes to the console. A name that ends with `-console` is not enough.
- **Isolation.** The package does not start or direct any process. It remains a pure classifier.

## Where the decision acts

The package imports `internal/guards`, and `guards` imports `internal/core`. Thus `core` cannot import this package. `cmd/evolve` gives `core` the check through `core.WithRepairFixability`. The check scans the inbox of the cycle for the open console items and calls `Classify`.

| Caller | Stage `shadow` | Stage `enforce` |
|---|---|---|
| `core.Orchestrator.repairHandoff`, before a repair round | `REPAIR_HANDOFF` with `decision: would_stop`; the repair round runs | `REPAIR_HANDOFF` with `decision: stop`; `repair-handoff.json`; the cycle seals `handed-off` with no repair round |
| `inboxmover.PartitionLaneMenu`, for triage, the supply census, the wave seed, the no-work check, `evolve inbox batches` and `evolve inbox rank` | the lane item stays ready; the census records `would defer <id>: console_companion_pending: <ids>` | the lane item waits with that reason |

The gate failures are `AuditFailReasons` and the message of `ship-error.json`. `repairHandoff` emits the signal only when a handoff finding exists.

The seal sets the termination `handed-off` and puts a `HANDOFF_ROUTE` diagnostic for the committed item on the audit timing entry. `HANDOFF_ROUTE` has the disposition of `BLOCKER_ROUTE`. The loop does not count a `handed-off` cycle as a code FAIL (`core.IsHandedOffResult`). `evolve wave status` prints `handed-off (<reason>: <blockers>)` on the line of the cycle.

## Invariants

- A finding on an unprotected path with no console item remains a lane finding.
- A handoff finding at MEDIUM without red proof does not stop the cycle.
- A stop decision carries the open console item of each blocking finding.
- The ACS of cycle 2039 (`go/acs/cycle2039`) pins the API and the four rounds of cycle 2035.
