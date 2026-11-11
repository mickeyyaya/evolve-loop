# Build Explanation — Cycle 2266

## Build Binding
- Cycle: 2266
- Base SHA: eaea4edd8f5d28ea3491fff77a35cefc27d3b303

## Summary
At stage `enforce`, the closeout writes each recovery or judgment retrospective pair into `.evolve/retrospectives-pending/`. The boundary publish commits the pairs into `docs/retrospectives/`. Stage `shadow` still writes into the workspace only.

## Rationale
The stage key was built, but `enforce` wrote into the workspace like `shadow`. The pending dir lets a fleet lane write without a commit. The boundary publish then commits each pair with the dossiers. A write or publish failure is a WARN line, so the closeout and the boundary never fail on a retrospective.

## Changed Areas
- `go/internal/recoveryretro/shadow.go` — `WriteInto` takes the target dir. One writer serves the workspace and the pending dir. It stays create-only.
- `go/internal/recoveryretro/publish.go` — `PendingDir` and `PublishPending` commit each complete pair. They drop a pair whose document is in `docs/`. A half pair stays pending with a WARN line.
- `go/internal/core/retrospective_shadow.go` — the closeout selects the pending dir at `enforce` and the workspace at `shadow`.
- `go/cmd/evolve/cmd_loop_dossiers.go` — the boundary publish also publishes the pending retrospectives.
- `go/cmd/evolve/cmd_dossier.go` — `publishDossierPairs` (called by the dossier publish verb after the dossier pairs are listed) also publishes the pending retrospectives. The verb returns early when no dossier pair is pending, so it does not reach the retrospective publish in that case.
- `go/cmd/evolve/parallel_serial_tests_test.go` — the new publish-failure test is listed serial, because `gittest.FaultOn` sets PATH.
- `go/cmd/evolve/cmd_postmortem_test.go` — the fixture puts its transcript under the seat root of the fixture home. The ambient `CLAUDE_CONFIG_DIR` of a seat pane no longer hides it.
- `go/cmd/evolve/cmd_loop_dossiers_retro_test.go` — tests for the boundary publish and its failure paths.
- `go/internal/core/retrospective_enforce_test.go` — tests for the enforce route, the shadow route and the write failure.
- `go/internal/recoveryretro/publish_test.go` — tests for the publish, the held document and the half pair.
- `go/internal/recoveryretro/apicover_named_test.go` — names the new exports.
- `go/acs/cycle2266/predicates_test.go` — the acceptance predicates of the cycle.
- `docs/architecture/recovery-retrospective.md` — steps 1 to 3 are built, and the guard table names the new tests as built.
- `docs/architecture/packages/internal-recoveryretro.md` — the package page describes the pending dir and the publish.
- `docs/plans/recovery-retrospective-2026-10.md` — C4 is marked built.
- `.evolve/evals/recovery-retrospective-c4-stage-key-publish.md` — the eval graders of the task.
- `docs/private/research/archived-2026-11-10/superseded-predicate-packages/cycle2152/predicates_test.go` — the superseded predicates of cycle 2152, archived.
- `docs/private/research/archived-2026-11-10/unshipped-build-explanations/cycle-2152-01m6jfnhebtjtbkj7tk5d14zyr.md` — the explanation of the unshipped cycle 2152, archived.

## Design Decisions
One create-only writer takes the target dir. A second writer for the pending dir duplicates the create-only and link rules. Publish failure leaves the pending copy, so the next boundary retries.

## Verification
The 14 predicates of `go/acs/cycle2266` pass. The core and cmd tests named in the eval pass. The ste-lint of the changed documents reports 0 findings.

## Compatibility
Stage `shadow` and stage `off` keep their output. The dossier publish keeps its behavior when no retrospective is pending. When no dossier pair is pending, the verb still exits early.

## Limitations
The dossier publish verb returns at `go/cmd/evolve/cmd_dossier.go:60` when no dossier pair is pending, before the retrospective publish at line 147. A pending retrospective alone is not published by the verb. The loop boundary publish does publish it.

Steps 4 and 5 (retire the judgment todo, add the recall root) stay open as C5 and C6.
