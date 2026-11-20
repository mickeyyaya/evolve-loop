# Build Explanation — Cycle 2569

## Build Binding
- Cycle: 2569
- Base SHA: ca8b6f025676462ddc5ff46dccc1007e2cf7caae

## Summary
The quality panel now reads the three outcome indexes. OQ2 is the 14-day churn, OQ3 is the revert rate and OQ4 is the recovery count with the hours to green. OQ2 and OQ3 run in `evolve quality report --scope tree`. OQ4 runs in `--scope wave`.

## Rationale
The registry already listed OQ2 to OQ4, but no reader produced a reading. The readers reuse the existing `Reading`, `Wilson` and bootstrap code, the existing `runIn` git runner and the existing stage defaults. OQ2 and OQ3 have a baseline, so they start at stage report. OQ4 has none, so it starts at shadow.

## Changed Areas
- `go/internal/qualitypanel/churn.go` — `MeasureChurn` counts the production Go lines added 14 to 30 days ago and the lines still blamed to their commit 14 days later.
- `go/internal/qualitypanel/revert_rate.go` — `MeasureRevertRate` counts a `Revert` subject or a `This reverts commit` body line in the last 30 days.
- `go/internal/qualitypanel/gitreader.go` — one git seam for both readers. It keeps the first error, so each reader checks once.
- `go/internal/qualitypanel/stats.go` — adds `ClusterBootstrapRatio` for the pooled churn ratio. `Wilson` now gives an exact 0 lower bound for k = 0 and an exact 1 upper bound for k = n.
- `go/internal/qualitypanel/revert_sensitivity_hunks.go` — extracts `isProductionGo` so that churn and the hunk parser share one test.
- `go/internal/pipelinerates/recovery.go` — `MeasureRecovery` pairs the first failed try of a task with the later shipped try and gives the hours.
- `go/internal/recoveryretro/recovery.go` — exports `Shipped`, the one definition of a shipped dossier. `Recoveries` keeps its minimum of two failures.
- `go/cmd/evolve/cmd_quality_outcome.go` — the three `qualityReader` values for OQ2, OQ3 and OQ4.
- `go/cmd/evolve/cmd_quality.go` — adds the OQ2 and OQ3 readers to the tree scope and the OQ4 reader to the wave scope.
- `go/internal/qualitypanel/gitfixture_test.go` — shared helpers for dated commits and for a git runner that fails at a chosen call.
- `go/internal/qualitypanel/churn_test.go` — churn tests for the 14-day rule, deleted files, no-data windows and every git failure.
- `go/internal/qualitypanel/revert_rate_test.go` — revert tests for both forms, lookalikes, the zero case and the window edges.
- `go/internal/qualitypanel/stats_test.go` — tests for `ClusterBootstrapRatio` and for the exact Wilson bounds.
- `go/internal/qualitypanel/apicover_named_test.go` — names the new exports of the package.
- `go/internal/pipelinerates/recovery_test.go` — recovery tests for first-failure hours, new episodes, missing times and no data.
- `go/internal/pipelinerates/apicover_named_test.go` — names the new exports of the package.
- `go/internal/recoveryretro/apicover_named_test.go` — names `Shipped`.
- `go/cmd/evolve/cmd_quality_test.go` — the tree scope has four readers and six readings.
- `go/cmd/evolve/cmd_quality_compare_test.go` — the wave scope has eleven readers. OQ4 carries the extra field `median_hours`.
- `docs/architecture/packages/internal-qualitypanel.md` — documents the two git readers and the exact Wilson bounds.
- `docs/architecture/packages/internal-pipelinerates.md` — documents the recovery reader.
- `docs/explain/builds/cycle-2569-01m7wkbjwn6che26vt5165ymys.md` — this document.

## Design Decisions
OQ4 lives in `pipelinerates`, because `qualitypanel` cannot import `dossier`. The reader reuses `recoveryretro.Shipped` and does not copy it. Rejected: `git log --numstat` with a line diff for churn, because it cannot tell a rewrite from a move. Blame at the 14-day snapshot is the standard survival test. Rejected: a second Wilson function that clamps the bound, because the floating error is in `Wilson` itself.

## Verification
The 18 predicates of cycle 2569 pass. The changed readers are at 100% coverage under their own tests. The packages `qualitypanel`, `pipelinerates`, `recoveryretro`, `policy` and `dossier` pass. The command tests with the new counts pass.

## Compatibility
No signature changes. The wave scope adds one reading and, with no dossier, one `QUALITY_VACUOUS_DENOMINATOR` warning. A project root that is not a git repository gives two source warnings in the tree scope.

## Limitations
Churn runs one `git blame` for each file of each commit in the window, so a large window costs time. A `quality report` run at the cycle boundary has no cap on that work.
