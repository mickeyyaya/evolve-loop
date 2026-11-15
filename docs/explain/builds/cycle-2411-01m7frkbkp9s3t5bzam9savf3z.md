# Build Explanation — Cycle 2411

## Build Binding
- Cycle: 2411
- Base SHA: 5ce7e48e242115eb98c6deab81e97ea5ad159cf0

## Summary
This build moves 32 `cmd/evolve` tests and 2 `TestSeatTokenVerbs_*` tests off the serial chain. It injects `env []string`, `bridgeSessionsDeps`, and `rootEventsAttacher` instead of calling `t.Setenv` or swapping seam globals. Each moved test starts with `t.Parallel()`, and all 32 `serialTests` entries are removed.

## Rationale
`TestLegacyAdvisorDispatch_MatchesTheGolden` (`t.Setenv`) and global seam swaps around tmux session listing and root signal-center attachment serialized `cmd/evolve` tests. Passing explicit dependencies (`env []string`, `bridgeSessionsDeps`, `rootEventsAttacher`) into `WithEnv`, `WithDeps`, and `WithAttach` helpers lets tests run concurrently. Production callers and default globals remain intact.

## Changed Areas
- `go/cmd/evolve/router_dispatch.go` — added `resolveRouterDispatchHealthyWithEnv` accepting an explicit `env []string` and updated `routerDecision` to filter that slice; `resolveRouterDispatchHealthy` delegates with `os.Environ()`.
- `go/cmd/evolve/cmd_bridge_sessions.go` — introduced `bridgeSessionsDeps` and `defaultBridgeSessionsDeps`, added `runBridgeSessionsWithDeps` and `collectBridgeSessionsWithDeps`, and threaded `deps.liveSockets` / `deps.tmux` through `socketsOf` and `socketActivity`.
- `go/cmd/evolve/cmd_wave_status_seat.go` — added `collectWaveSeatsWithDeps` and threaded `bridgeSessionsDeps` into `waveSessionsAlive`; `collectWaveSeats` delegates to `defaultBridgeSessionsDeps()`.
- `go/cmd/evolve/cmd_wave.go` — added optional `seatDeps *bridgeSessionsDeps` field to `waveEnv`.
- `go/cmd/evolve/cmd_wave_status.go` — updated `buildWaveStatus` to call `collectWaveSeatsWithDeps` when `env.seatDeps` is non-nil and `collectWaveSeats` otherwise.
- `go/cmd/evolve/cmd_cycle.go` — added `rootEventsAttacher` and `newRoleSignalCenterWithAttach`, with `newRoleSignalCenter` delegating to `attachRootEventsFn` when `attach` is nil.
- `go/cmd/evolve/cmd_quality.go` — added `newQualityCenterWithAttach` and delegated `newQualityCenter` to it.
- `go/cmd/evolve/cmd_integration_stop.go` — added `emitIntegrationSignalWithAttach` and delegated `emitIntegrationSignal` to it.
- `go/cmd/evolve/cmd_auth.go` — added `Attach rootEventsAttacher` field to `authEnv`.
- `go/cmd/evolve/cmd_auth_seat_token.go` — updated `seatTokenCall.signal` to pass `c.env.Attach` into `newRoleSignalCenterWithAttach`.

## Design Decisions
Each seam keeps its package-level default (`os.Environ()`, `bridgeSessionsTmux`, `bridgeLiveSockets`, `bridgeSessionsNow`, `attachRootEventsFn`) wired through production entry points. Keeping the defaults ensures `TestBridgeTmuxListing_OneFunctionOwnsTheListSessionsCall`, `TestRootSignalCenter_EveryRootSubscribesThePublisher`, and existing serial tests still pass.

## Verification
All 9/9 cycle 2411 ACS predicates (`go test -tags acs -count=1 -v ./acs/cycle2411`) and `-race` runs on all moved `cmd/evolve` tests (`go test -race -count=1 ./cmd/evolve`) passed. `gofmt -l go/` and `go vet ./cmd/evolve/...` exited 0.

## Compatibility
All existing CLI flags, production defaults, and unmoved serial tests remain unchanged.

## Limitations
Other serial clusters in `go/cmd/evolve` (`wireOrchestratorDepsFn`, `runner.DefaultRouter`, `SetModelCatalogDirFn`, `cmd_selfheal.go`, and helper `PATH` mutations) remain serial and are deferred to follow-up cycles.
