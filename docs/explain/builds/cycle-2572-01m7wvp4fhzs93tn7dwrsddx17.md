# Build Explanation — Cycle 2572

## Build Binding
- Cycle: 2572
- Base SHA: 7d81f465c049123d143f96a14b5cae5fa793c516

## Summary
The test binary of `./internal/bridge` now builds `./cmd/evolve` one time. All contract hosts share that binary.

## Rationale
Each contract test ran `go build ./cmd/evolve` with its own dispatch store path. The build is the largest cost of these tests on macOS. A shared store path in the linker flag removes the need for a build per host.

## Changed Areas
- `go/internal/bridge/sandbox_contract_integration_test.go` — `withEvolveBinary` returns one shared binary, and each host uses the shared dispatch store path. `evolveBinaryBuiltFrom` holds the single build, and `goModuleDir` names the module directory for the guard.
- `go/internal/bridge/evolve_build_once_integration_test.go` — `TestEvolveBuildRunsOnce` requests the shared binary three times and counts the builds through a `go` shim. It names the module directory through `go list`, so it does not read the whole tree.
- `go/internal/bridge/testmain_cleanup_test.go` — a hook lets the `TestMain` remove the shared directory.
- `go/internal/bridge/tmuxtest_main_test.go` — `TestMain` runs the cleanup hooks.
- `go/internal/bridge/parallel_serial_tests_test.go` — the guard test is serial, because it sets `PATH`.
- `docs/architecture/agent-sandbox-contract.md` — the Related tests list names the single build and its guard.

## Design Decisions
The shared path is fixed for the whole test binary. The sandbox profile of each host denies that same path, so the credential rows keep their meaning. A per-host linker path was rejected, because it forces one build per host.

## Verification
`TestEvolveBuildRunsOnce` passes. The ACS predicates of cycle 2572 pass, and the ACS suite reports red=0.

## Compatibility
No production code changes. The tests of the contract rows are unchanged.

## Limitations
The contract tests skip on this host, because the native sandbox is unavailable here. The macOS wall time is not measured in this cycle.
