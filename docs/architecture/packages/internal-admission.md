# internal/admission

> The spec of record: [wave-orchestrator §6](../wave-orchestrator.md#6-admission-by-cell). The decision: [ADR-0139](../adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md). The plan: [wave-orchestrator-2026-10](../../plans/wave-orchestrator-2026-10.md) (O3, D6, D7, Q2). The landing design that names the rule: [fleet-landing-at-scale-2026-10](../../plans/fleet-landing-at-scale-2026-10.md) (S14). The shadow wiring: [internal-loopwave.md](internal-loopwave.md). The global-zone list: [internal-fleet.md](internal-fleet.md).

## Purpose

`internal/admission` decides which todos can run at the same time. It maps each todo to its package cells and admits a todo only when no lane in flight holds a cell that conflicts with it. It is pure and deterministic. The only process boundary is `LoadModuleGraph`, which runs one `go list` through an injected `sysexec.RunFunc`.

Built on 2026-10-17 as O3 of the wave-orchestrator plan, in the `shadow` stage. The live loop runs it after the file partition of each wave. It records each todo that admission holds back. It does not change which lanes launch.

## Design

- **The `Graph` port.** It has four methods: `Packages(files)`, `Deps(pkg)`, `Hubs()` and `IsLeaf(pkg)`. `Admitter` takes the port, so the tests need no `go list`. `ModuleGraph` is the production adapter.
- **One `go list` for each plan.** `LoadModuleGraph` runs `go list -f '{{.ImportPath}}|{{join .Deps " "}}' ./...` once in the module directory. On this module it takes 0.8 s for 303 packages (2026-10-17). The loop gives it a deadline of 30 s.
  - The shadow runs in a goroutine with a recover. The loop joins it after the launch. A slow `go list` or a panic thus cannot delay or fail a launch.
  - `NewModuleGraph` parses the lines. It keeps only the module packages, so the standard library is never a cell.
- **Paths to packages.** A `.go` file under `go/` maps to its directory package. Another path under `go/` (a package directory, or package data such as `testdata/`) maps to the nearest enclosing known package. A path outside `go/` maps to no package. The module root is not a package, so `go/.cover-strict` maps to no package.
  - The files `go/go.mod`, `go/go.sum` and `go/Makefile` map to no package, but each holds the hub cell. `go.mod` and `go.sum` are global-zone files (`fleet.GlobalZoneFiles`). The package names the Makefile.
- **The hub set.** A package is a hub when at least `hub_min_importers` module packages import it, directly or transitively. The count is the transitive importer count, because it reproduces the ADR-0134 research F2.16 numbers (§Findings).
- **The footprint of a todo.** `E(t)` is the set of packages that the todo edits. `D(E)` is the union of the module-internal transitive deps of those packages, minus the hubs. A leaf package has no importer, so nothing compiles against it: it adds nothing to `D(E)`. `cmd/evolve` is the main leaf. The todo holds the hub cell in three cases:
  - it edits a hub package;
  - it names a global-zone file;
  - it has no files, and its text names no known package.
- **The conflict rule.** Two todos conflict when one of these holds:
  1. both hold the hub cell;
  2. `E(t1)` and `E(t2)` meet;
  3. `E(t1)` meets `D(E(t2))`;
  4. `E(t2)` meets `D(E(t1))`;
  5. both have the same id family (§Id family).
- **No files.** A todo whose only file is its own id has no files: that is the placeholder of `fleet.TodosFromTriage`. A todo with no files takes the packages of the `go/internal/<pkg>` paths in its text. `Admitter.Text` gives the text. The loop gives the inbox title and summary.
- **The scan.** `Admit` takes the candidates in the given rank order. It stops after `scanK` admitted todos. A waiting todo keeps its rank in `Decision.Rank`. `Decision.BlockedBy` names the lane in flight, or the earlier admitted todo, that holds the conflicting cell.

## Known limits

- **Todos outside `go/`.** A todo whose files are all outside `go/` (docs, scripts), or that has non-Go files that no known package encloses, maps to no cell. It conflicts with nothing and `Admit` always admits it (`TestAdmit_FilesOutsideGoMapToNoCellAndConflictWithNothing`). Two such todos can edit the same file; only the landing proof of ADR-0128 catches it.

## Invariants

- Two admitted todos never conflict. `TestAdmit_TwoAdmittedTodosNeverConflict` checks 500 random rounds against an oracle that the test writes from the rule. Two mutants (no reverse reach, no hub capacity) each fail it.
- The hub cell admits one todo (`TestAdmit_TheGlobalCellAdmitsOne`). The module-root files `go/go.mod`, `go/go.sum` and `go/Makefile` hold it (`TestAdmit_TheModuleRootFilesMapToTheHubCell`).
- An import of a hub never conflicts (`TestAdmit_ImportingAHubNeverConflicts`).
- An edited package that the other todo imports conflicts, in both directions (`TestAdmit_AnEditedPackageThatTheOtherImportsConflicts`).
- A todo outside `go/` is always admitted (`TestAdmit_ATodoOutsideGoIsAlwaysAdmitted`).
- The package has 100% statement coverage in `go/.cover-strict` and is listed in `go/.apicover-enforce`.

## Findings

- **The hub distribution (2026-10-17, 303 packages).** The transitive importer counts are: `sysexec` 151, `log` 127, `atomicwrite` 126, `signalcenter` 122, then `ipcenv` 116 and `adapters/flock` 115. The median is 5, the 90th percentile 68 and the 99th percentile 123. The default `hub_min_importers` 120 gives exactly the four hubs of ADR-0134 F2.16. The gap from 122 to 116 is the margin. Direct importer counts do not reproduce F2.16: `sysexec` has 64 and `log` is not in the top 25.
- **The width on real data (2026-10-17, runtime plane, width 4).**

| Input | File-partition width | Admission width | Cost |
|---|---|---|---|
| The inbox ready list | 0 (0 items) | 0 | none |
| Each of the last 10 triage decisions | 1, or 0 for cycle 1918 | the same | 0% |
| The last 10 decisions as one wave (9 todos) | 4 | 4 | 0% |
| The whole pending queue (244 items), with the scan of 32 | 4 | 4 | 0% |
| The pending queue in 30 windows of 8 todos | 120 lanes in total | 58 lanes in total | 52% |

- **The cause of the 52%.** 120 of the 244 pending items edit `cmd/evolve` or `internal/core`. These packages import most of the module, so their `D` meets almost every other todo. The window test is a stress case: the orchestrator scans the whole queue, not 8 items. The S14 exit (at most 10% of the width over 3 runs) is measured by the shadow events of live runs. If the live cost is above 10%, the plan's fallback applies: the non-hub cells fall back to files (plan §4, "What changes our mind").

## Id family (2026-10-21)

The inbox item `same-subsystem-wave-scheduling` asks that two items of one subsystem do not run at the same time. In cycle 1484 a `context-fill` lane shipped the contract "unmeasured families stay unmeasured" while its sibling `context-fill` lane ran. The new test made the running lane red. The two lanes edited disjoint files, so no cell rule saw the coupling ([incident 2026-08-17](../../incidents/2026-08-17-failure-rate-review-1481-1503.md) §4 item 7).

- **The rule.** `idFamily` takes the text before the first hyphen of the todo id. The footprint holds it as the cell `family:<token>`. `sharedEdit` calls `sameFamily`, which reports a conflict when two footprints have the same family. Thus the reach rule and the optimistic rule both hold the second todo. An id without a hyphen, or with a hyphen first, has no family.
- **The record.** `Decision.Cells` holds the family cell. The census refusal reads `cells family:context,internal/inboxrank held by context-fill-cap`, and `supply.jsonl` keeps it. The wave shadow writes the same cells into `LOOP_ORCH_ADMIT_WAIT`. Thus no family hold is silent.
- **The cost (2026-10-21).** The runtime inbox holds 209 items in 128 families. 35 families have more than one item, and they hold 116 items. A scan of `scan_k` todos thus finds other families to fill the width of 4.
- **Rejected: the first two tokens.** `tokenopt-cache-discipline-audit` and `tokenopt-session-resume-on-retry` then get different families, and so do `handoff-h9-*` and `handoff-h10-*`. The item names `tokenopt-*` as one family.
- **Rejected: one multi-slug lane for the family.** The pool launches one todo for each lane (`fleet.poolSpec`). A shared lane needs the multi-slug scope work that the item names as a precondition.
- **Rejected: a declared `subsystem` field on each item.** No filed item has the field, so the rule holds nothing until every filer sets it.
- **The limit.** A first token can be wider than one subsystem.
  - Example: `cli-*`. Most `cli-*` items edit `cmd/evolve`, and their package cell already holds them.
  - A generic verb such as `fix-*` is one family too. On 2026-10-21 two runtime inbox ids start with `fix`, `add`, `feat`, `docs` or `test`. Both start with `docs-`.
  - The cost is a later start for a lane, not a lost lane.
- **The proof.**
  - `TestAdmit_TwoTodosOfOneIDFamilyWaitForEachOther` checks both rules. `TestAdmit_ALaneInFlightHoldsItsIDFamily` checks a lane in flight.
  - `TestAdmit_TodosOfDifferentIDFamiliesAreUnaffected` checks the preservation case. `TestAdmit_AFamilyHoldIsNoReachShadow` checks that a family hold sends no shadow record.
  - `orchestrator` `TestSupply_RecordsATodoHeldBehindASiblingOfItsIDFamily` checks the census record.
  - The first two tests and the census test fail when `sharedEdit` drops the family term.

## Optimistic rule and shadow record (ADR-0143 W1)

- `Admitter.Optimistic` drops the import-reach term. Two todos conflict only when they edit one package or both touch the hub. A core lane no longer holds a router todo.
- `Admitter.OnShadow` runs only under the reach rule. It receives a `ShadowAdmission` for each todo that the reach rule holds and the optimistic rule would admit, with the first holder as `HeldBy`. It changes no decision.
- `ShadowAdmission.Reason` names the package where the edit of one todo meets the reach of the other, for example `reach meets internal/router`. When more than one package meets, it names the first in sort order.
- `Admitter.OnOptimistic` runs only under the optimistic rule (ADR-0143 W1b). It receives a `ShadowAdmission` for each admitted todo that the reach rule holds, with the first such holder as `HeldBy`. The orchestrator carries that holder to the ship of the lane. It changes no decision. Pinned by `TestAdmit_AnOptimisticAdmissionNamesTheHolderTheReachRuleWouldHaveUsed` and `TestAdmit_TheReachRuleNamesNoHolderForItsOwnAdmissions`.
- The policy key `fleet.orchestrator.admission` (`shadow` by default, `enforce`) selects the rule through `policy.OrchestratorConfig.OptimisticAdmission()`. Three callers take it: the pool (`cellAdmission`, `go/cmd/evolve/cmd_loop_orchestrator.go`), the supply census (`orchestrator.Census.Optimistic`) and the wave shadow (`admissionShadow`, `go/cmd/evolve/cmd_loop_admission.go`). The shadow event is `LOOP_ORCH_ADMISSION_SHADOW`. The spec: [wave-orchestrator §6](../wave-orchestrator.md#6-admission-by-cell).
