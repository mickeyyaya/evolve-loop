# internal/cli/inboxcmd

> The plan of record is [split-cmd-evolve-2026-11.md](../../plans/split-cmd-evolve-2026-11.md), step 1.

## Purpose

`internal/cli/inboxcmd` holds the `evolve inbox` verb group and the `evolve inbox-mover` verb. The group moved out of `go/cmd/evolve` so that a lane which edits an inbox verb does not hold the admission cell of `package main`.

## Design

- `Run(args, stdin, stdout, stderr, deps)` dispatches the verb. `RunMover(args, stdin, stdout, stderr, deps)` runs `inbox-mover`.
- `go/cmd/evolve/cmd_inbox.go` is the only inbox file left in `main`. It builds `Deps` from the functions of `main` and calls `Run` and `RunMover`.
- `Deps` has six fields: `QueueGate`, `LaneForbidden`, `PrintVerbJSON`, `HumanWriter`, `CheckRootHasEvolve` and `NewSignalCenter`. Each field is a function that only `main` can build. `Run` and `RunMover` store the value, and a verb reads it through `host()`.
- The flag helpers `TakeFlagValue` and `TakeProjectRoot` live in `cmd/evolve/cmdutil`, because `main` and this package both use them.
- `main` calls five exported functions: `ReconcileConsumedFingerprints`, `ReconcileConsumedBindings`, `CheckInboxTrailerBytes`, `ReadInboxItemObject` and `SplitForbidden`.

## Invariants

- The exit codes and the output of each verb are the same as before the move (`TestC2466_003_TheInboxVerbBehavesTheSameThroughTheProductionBinary`).
- Only the shim stays among the non-test `cmd_inbox*.go` files of `main` (`TestC2466_002_OnlyTheShimRemainsAmongTheNonTestInboxFilesOfMain`).
- The package tests use `testDeps` in `host_test.go`. The tests that need the real queue gate stay in `main` and go through the shim (`cmd_inbox_live_lane_test.go`).
