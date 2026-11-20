# internal/cli/landcmd

> The plan of record is [split-cmd-evolve-2026-11.md](../../plans/split-cmd-evolve-2026-11.md), step 2.

## Purpose

`internal/cli/landcmd` holds the `evolve pr` and `evolve ci` verb groups. Both groups moved out of `go/cmd/evolve` so that a lane that edits `pr` or `ci` does not hold the `package main` admission cell.

## Design

- `RunPR(args, stdin, stdout, stderr, deps)` dispatches `pr merge`, `pr window`, `pr boundary` and `pr refresh`.
- `RunCI(args, stdin, stdout, stderr, deps)` dispatches `ci classify`, `ci watch`, `ci telemetry` and `ci darwin-set`.
- `go/cmd/evolve/cmd_pr.go` and `go/cmd/evolve/cmd_ci.go` are the only non-test `cmd_pr*` and `cmd_ci*` files left in `main`. `cmd_pr.go` builds `Deps` with `landDeps()`, and both shims hand over to `RunPR` and `RunCI`.
- `Deps` holds five functions from `main`: `ResolveRoot`, `PrintVerbJSON`, `NewSignalCenter`, `DarwinSet` and `RefreshWorktree`. Both entrypoints store the `Deps` value in `current`, and handlers read it through helper wrappers.
- The shared flag parser (`CLIFlags`), `GHIn` and `PlaneLiveRuns` live in `go/cmd/evolve/cmdutil` so `main` and `landcmd` share one copy.

## Invariants

- Exit codes and output of each `pr` and `ci` verb match the pre-move binary (`TestC2564_004_PrAndCiBehaveTheSameThroughTheProductionBinary`).
- Only `cmd_pr.go` and `cmd_ci.go` stay among the non-test `cmd_pr*` and `cmd_ci*` files of `main`, each at 40 lines or fewer (`TestC2564_002_OnlyTheTwoShimsRemainAmongTheNonTestPrAndCiFilesOfMain`).
- Package tests use `testDeps` in `host_test.go`; channel and real-generator integration tests stay in `main` and run through the shims.
