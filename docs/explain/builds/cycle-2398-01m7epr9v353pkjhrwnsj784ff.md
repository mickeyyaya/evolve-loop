# Build Explanation — Cycle 2398

## Build Binding
- Cycle: 2398
- Base SHA: 49bbd538137ee7b8f2ecce08450cb296de484c77

## Summary
The `cmd/evolve` tests now walk and read the Go module once for each test binary through `sharedModuleFiles` in `go/cmd/evolve/module_scan_loader_test.go`. All 14 direct module walks across 10 test files now use the shared loader.

## Rationale
Ten test files in `go/cmd/evolve` each walked `moduleRoot` and read or parsed every non-test Go file on their own. Centralizing the walk in `sync.OnceValues` removes redundant filesystem I/O and redundant AST parsing across parallel tests.

## Changed Areas
- `docs/architecture/packages/cmd-evolve.md` — documents `module_scan_loader_test.go` and `TestModuleScanGuard` in ASD-STE100.
- `go/cmd/evolve/module_scan_loader_test.go` — adds `sharedModuleFiles` and `moduleSourceFile.AST` backed by `sync.OnceValues`.
- `go/cmd/evolve/cmd_bridge_engine_roots_test.go` — migrates `TestBridgeEngineRootsAreEnumerated` and `TestBridgeEngineRoots_EachWiresATokenResolver` to `sharedModuleFiles` and shared ASTs.
- `go/cmd/evolve/cmd_cycle_config_test.go` — migrates `nonTestSourcesMatching` to `sharedModuleFiles` while keeping the `internal/config/` exclusion.
- `go/cmd/evolve/cmd_cycle_ledger_roots_test.go` — migrates `TestUnobservedLedgerRootsArePinned` to `sharedModuleFiles`.
- `go/cmd/evolve/cmd_cycle_observer_signals_test.go` — migrates `TestObserverAdapterConstructionsAreWired` to `sharedModuleFiles`.
- `go/cmd/evolve/cmd_cycle_signal_center_test.go` — migrates three module scans to `sharedModuleFiles`.
- `go/cmd/evolve/cmd_inbox_signals_test.go` — migrates `inboxmover.Options` scan and `nonTestConstructionsOf` to `sharedModuleFiles`.
- `go/cmd/evolve/cmd_loop_unit13_seam_test.go` — migrates `nonTestSourcesMentioningU13` and `TestEmitLoopWave_ProjectsToTheLeafProducerWithOneRegistrar` to `sharedModuleFiles`.
- `go/cmd/evolve/cmd_root_guard_test.go` — migrates `parsedModuleSources` to `sharedModuleFiles` and `f.AST`.
- `go/cmd/evolve/docs_contract_test.go` — migrates `collectEnvVarsFromCode` to `sourceFilesInDir` while keeping the `acs/` and `archive/` exclusions.
- `go/cmd/evolve/inbox_writers_scan_test.go` — migrates `scanProductionPackages` and `addScannedFile` to `sourceFilesInDir` and `f.AST`.

## Design Decisions
`cachedWalkSourceFiles` caches each directory walk once per absolute path using `dirWalks` (`sync.Map`) and `sync.Once`. Both `loadModuleSourceFiles` (backing `sharedModuleFiles` through `sync.OnceValues` on `../..`) and `sourceFilesInDir` call `cachedWalkSourceFiles`, so callers share the cached walk and file bytes. Each `moduleSourceFile` uses a per-file `sync.OnceValues` for `AST(t)` so tests that only grep bytes pay no parse cost.

## Verification
`TestModuleScanGuard`, `TestInboxWriters_EveryAutofiledItemCarriesAKnownClass`, `TestUnobservedLedgerRootsArePinned`, and the cycle 2398 ACS predicates verify the loader, guard, timing, and documentation.

## Compatibility
`walkSourceFiles` skips `vendor`, `bin`, `testdata`, and hidden directories other than `root`. Test helpers preserve their remaining directory exclusions and error checks.

## Limitations
The shared loader assumes test files in `go/cmd/evolve` do not mutate the returned `Src` slice or `*ast.File`.
