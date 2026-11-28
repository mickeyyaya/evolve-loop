# Build Explanation — Cycle 2672

## Build Binding
- Cycle: 2672
- Base SHA: 319d3bcec855b5d2ef10087344a83b7825d937c4

## Summary
The `landcmd` and `inboxcmd` packages now pass their `Deps` value down through handlers and helpers instead of a package-level atomic pointer. A guard test scans `go/internal/cli` to reject any new package-level atomic `Deps` pointer. `TestRunCI_DocsTests` and parallelized `inboxcmd` tests now run with `t.Parallel()`.

## Rationale
Storing `Deps` in a process-wide `atomic.Pointer[Deps]` allowed concurrent or overlapping CLI calls in tests to overwrite each other's host functions. Passing `Deps` as a value parameter isolates each invocation without changing the public entrypoints in `go/cmd/evolve`.

## Changed Areas
- `go/internal/cli/landcmd/deps.go` — removes `current` and `host()`, and promotes `resolveRoot`, `printVerbJSON`, `newSignalCenter`, and `refreshWorktree` to methods on `Deps`.
- `go/internal/cli/landcmd/cmd_pr.go` — threads `deps` from `RunPR` through `dispatchPR` and `mergePRs` without global state.
- `go/internal/cli/landcmd/cmd_pr_window.go` — passes `deps` into `runPRWindow` to call `deps.resolveRoot`.
- `go/internal/cli/landcmd/cmd_pr_boundary.go` — passes `deps` through `runPRBoundary`, `runPRBoundaryWith`, and `boundaryReport`.
- `go/internal/cli/landcmd/cmd_pr_refresh.go` — passes `deps` into `runPRRefresh` to resolve the root and refresh the worktree.
- `go/internal/cli/landcmd/cmd_ci.go` — threads `deps` from `RunCI` through `dispatchCI`, `runCIClassify`, and `ciSubverbRunner`.
- `go/internal/cli/landcmd/cmd_ci_docstests.go` — passes `deps` into `runCIDocsTests` to resolve the project root.
- `go/internal/cli/landcmd/cmd_ci_watch.go` — passes `deps` into `runCIWatch` to resolve the root and open the signal center.
- `go/internal/cli/inboxcmd/deps.go` — removes `current`, `host()`, and `pushHost()`, and adds nil-safe methods on `Deps`.
- `go/internal/cli/inboxcmd/cmd_inbox.go` — passes `deps` from `Run` to verb handlers, `finishInboxWrite`, `loadPendingInbox`, `runInboxBatches`, and `runInboxLint`.
- `go/internal/cli/inboxcmd/cmd_inbox_add.go` — threads `deps` through `runInboxAdd` and `printFiledRank`.
- `go/internal/cli/inboxcmd/cmd_inbox_claim.go` — passes `deps` into `runInboxClaim` for lane-protection checks.
- `go/internal/cli/inboxcmd/cmd_inbox_consume.go` — passes `deps` into `runInboxConsume` for output formatting.
- `go/internal/cli/inboxcmd/cmd_inbox_curate.go` — passes `deps` through `curationOptions`, `runInboxEdit`, `runInboxWithdraw`, and `runInboxVerify`.
- `go/internal/cli/inboxcmd/cmd_inbox_curator.go` — stores `deps` on `curatorRun` and passes it from `runInboxCurate`.
- `go/internal/cli/inboxcmd/cmd_inbox_list.go` — passes `deps` into `runInboxList` for root checks and inbox loads.
- `go/internal/cli/inboxcmd/cmd_inbox_mover.go` — passes `deps` from `RunMover` into `moverOptions`.
- `go/internal/cli/inboxcmd/cmd_inbox_rank.go` — threads `deps` through `runInboxRank`, `rankInboxAt`, and `loadRankedInbox`.
- `go/internal/cli/inboxcmd/cmd_inbox_route_console.go` — passes `deps` into `runInboxRouteConsole` for the queue gate and JSON output.
- `go/internal/cli/inboxcmd/cmd_inbox_route_lane.go` — passes `deps` into `runInboxRouteLane` for the queue gate, lane checks, and JSON output.
- `go/internal/cli/inboxcmd/cmd_inbox_show.go` — passes `deps` into `runInboxShow` to load the pending inbox.
- `go/internal/cli/inboxcmd/cmd_inbox_split.go` — passes `deps` into `runInboxSplit` for lane-protection checks.
- `go/internal/cli/inboxcmd/cmd_inbox_trailercheck.go` — passes `deps` into `runInboxTrailerCheck` for JSON output.
- `docs/architecture/packages/internal-cli-landcmd.md` — documents value-passed `Deps` without `current`.
- `docs/architecture/packages/internal-cli-inboxcmd.md` — documents value-passed `Deps` without `pushHost` or `host()`.

## Design Decisions
Passing `Deps` by value beats a `context.Context` key because `Deps` is already a small struct. Explicit parameters keep wiring visible. Nil-safe methods preserve safe defaults when tests pass `Deps{}`.

## Verification
Race-enabled unit tests (`go test -race -count=1 ./internal/cli/landcmd/... ./internal/cli/inboxcmd/...`) pass. ACS predicates (`./go/bin/evolve acs suite --cycle 2672`) verify zero atomic `Deps` globals, parallel runs, overlapping runs, and the guard test.

## Compatibility
Exported entrypoints (`landcmd.RunPR`, `landcmd.RunCI`, `inboxcmd.Run`, `inboxcmd.RunMover`, `inboxcmd.NewFiler`, and `Deps` structs) keep their exact signatures and field layouts, so `go/cmd/evolve` callers stay unchanged.

## Limitations
Only CLI verb packages under `go/internal/cli` are scanned by `TestNoCLIPackageStoresDepsInAProcessGlobal`. Tests in `inboxcmd` that still call `t.Setenv` remain serial because Go forbids `t.Parallel()` alongside `t.Setenv`.
