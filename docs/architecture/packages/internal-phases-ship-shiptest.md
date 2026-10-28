# internal/phases/ship/shiptest

## Purpose

`internal/phases/ship/shiptest` has two parts. The non-test files hold the real-git fixtures that the ship tests share. The test files hold the black-box ship scenarios. These scenarios run as their own test binary, at the same time as the tests of [`internal/phases/ship`](internal-phases-ship.md).

The scenarios moved out of the ship package on 2026-10-28, because its tests ran 131.39 s on the macOS runner of CI run 40582532714. The ratchet limit is 120 s. The reasons and the numbers are in [internal-phases-ship.md](internal-phases-ship.md#split-test-package).

## Design

- **No import of the ship phase.** The fixtures never import `internal/phases/ship`. The in-package ship tests import the fixtures, and Go refuses an import cycle in a test build. Thus this rule is the one that lets both packages use one fixture.
- **The fixtures.** `fixture.go` holds the repository helpers: `MakeRepo`, `AddRemote`, `MakeWorktree`, `RunGit`, `RunGitOut`, `HeadSHA` and `RemoteHeadSHA`. `audit.go` holds `SeedAudit`, which writes a sealed audit report and its auditor ledger row. `lane.go` holds the landing lane: `LaneReadyToLand`, `PushOutage` and `SleepLog`.
- **One failure path.** Each fixture takes `testing.TB`. One `check` helper turns an error into `Fatalf` and names the step. A fault test with a recording `testing.TB` reaches that branch (`fixture_test.go`).
- **Git through `sysexec`.** `RunGit` and `RunGitOut` start git through `sysexec.Command`, so each process has its cancel and its wait delay. The environment drops every `EVOLVE_` key.
- **The scenarios.** The test files of package `shiptest_test` drive `ship.Run` and `ship.Phase` through exported names only. A test that calls an unexported function of the ship phase stays in `internal/phases/ship`.
- **Code in two homes.** Four helpers use ship types, so they cannot live in the fixtures: `runShip`, `containsLog`, `landingLane.options` and `landingLane.failEveryPush`. Each exists in `internal/phases/ship` and in `ship_run_test.go` or `landing_lane_test.go` here. One home needs an import of the ship phase from the fixtures, and Go refuses that cycle.
- **Coverage for the ship phase.** [`go/.cover-split`](internal-coversplit.md) names this package as a split of `internal/phases/ship`. The coverage runs build it with `-coverpkg=./internal/phases/ship,./internal/phases/ship/shiptest`. Thus the scenarios count for the coverage of the ship phase.

## Invariants

- The fixtures never import the ship phase. The compiler pins this rule, because the ship tests import the fixtures.
- Each top-level test is parallel or listed serial. Pinned by `TestEveryTopLevelTestIsParallelOrListedSerial` of this package.
- The package keeps its line in `go/.cover-split`. Pinned by `TestEverySplitTestPackageHasItsCoverEntry` in `internal/coversplit`.
- The fixtures have 100% statement coverage (`go/.cover-strict`), and a test names each export (`go/.apicover-enforce`).
- The package is a protected surface (`internal/guards/integrity_surface.go`), because `LaneReadyToLand` seals a real build explanation.
- `SeedAudit` writes the run id of each auditor row from the `cycle-state.json` of the fixture, or the default `test-run`. Pinned by `TestSeedAudit_TheCycleStateAndTheOverridesShapeTheEntry`.
