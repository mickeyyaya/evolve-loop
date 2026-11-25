# Build Explanation — Cycle 2634

## Build Binding
- Cycle: 2634
- Base SHA: e144bfba9d3fbb79d29d399556aec7b277e1c18f

## Summary
One stdlib-only resolver leaf package (`go/internal/diskfloor`) now handles all host free-disk floor and warn checks. It reads `disk.floor_gb` and `disk.warn_gb`, warns on old aliases, and refuses conflicting values.

## Rationale
Three separate readers previously interpreted incompatible keys and units (`gc.disk_floor_gb`, `preflight.min_free_gib`, and fixed percent thresholds). That caused `HOST_DISK_LOW` pauses while other checks remained silent. Putting one resolver in a leaf package lets every caller share one parser and validator with no import cycles. Keeping the whole-tree AST guard in `go/internal/policy` avoids editing `repocontract.Packages()`.

## Changed Areas
- `go/.apicover-enforce` — enrolls `./internal/diskfloor` in the API coverage gate at package creation.
- `go/cmd/evolve/cmd_cycle_start.go` — resolves `pol.DiskFloor(warn)` and wires `FloorBytes` into `looppreflight.NewDiskGuard`, refusing conflicting keys with `errCycleStartPolicyRefused`.
- `go/cmd/evolve/cmd_gc.go` — passes the full `policy.Policy` to `diskSignals` and `loadPolicy`, fails `loadFullPolicy` on `diskfloor.ErrConflict`, and rewords the cache-cap warning.
- `go/cmd/evolve/cmd_gc_disk.go` — resolves floor and warn levels through `pol.GCDiskFloor(r.stderr)` and reports `disk.floor_gb` and `disk.warn_gb`.
- `go/cmd/evolve/cmd_gc_scope.go` — passes the full `policy.Policy` through `cacheScope` and `salvageScope` so `evolve gc --only cache` reads `disk.floor_gb` and `preflight.min_free_gib`.
- `go/cmd/evolve/cmd_loop_preflight.go` — resolves `pol.DiskFloor(stderr)` and sets `Options.DiskFloorErr` on conflict so `evolve loop --preflight-only` halts `disk-space`.
- `go/cmd/evolve/cmd_loop_sentinel.go` — resolves `pol.GCDiskFloor(b.stderr)` in `startSentinel` so the live host sentinel reads `disk.floor_gb` and `disk.warn_gb`.
- `go/cmd/evolve/cmd_loop_window.go` — routes `diskSpaceHalt` through `pol.DiskFloor(b.stderr)` and halts on `diskfloor.ErrConflict`.
- `go/cmd/evolve/cmd_wave_next.go` — routes `checkWaveDisk` through `pol.DiskFloor` and refuses with `waveRefusedDiskLow` on breach or `diskfloor.ErrConflict`.
- `go/internal/diskfloor/diskfloor.go` — adds the single resolver (`Policy`, `Resolution`, `Resolve`, `ApplyToPreflight`, `PreflightBytes`, `ErrConflict`) with GB/GiB conversion, deprecation warnings, and conflict checks.
- `go/internal/gcpolicy/gcpolicy.go` — routes `DiskFloorBytes` and `DiskWarnBytes` through `diskfloor.Resolve` instead of reading `DiskFloorGB` and `DiskWarnGB` directly.
- `go/internal/hostsentinel/signals.go` — updates disk-floor and disk-warn signal documentation strings from `gc.disk_floor_gb` and `gc.disk_warn_gb` to `disk.floor_gb` and `disk.warn_gb`.
- `go/internal/looppreflight/disk_percent.go` — adds `FloorBytes` to `DiskGuardConfig` so `DiskGuard.probe` pauses when either percent or byte floor fails.
- `go/internal/looppreflight/diskspace.go` — updates `CheckDiskSpace` to name `disk.floor_gb` and halts `checkDiskSpace` when `DiskFloorErr` is set.
- `go/internal/looppreflight/looppreflight.go` — adds `DiskFloorErr` to `Options` and passes it to `checkDiskSpace`.
- `go/internal/orchestrator/orchestrator.go` — updates `CodePoolFloorHold` signal documentation from `gc.disk_floor_gb` to `disk.floor_gb`.
- `go/internal/policy/patch.go` — extracts `Policy.validate` and validates disk-floor key consistency during `Parse`.
- `go/internal/policy/policy.go` — adds `Disk *diskfloor.Policy` (`json:"disk,omitempty"`) to `Policy`.
- `go/internal/policy/policy_wave_checkin.go` — resolves `WaveCheckinConfig.DiskFloorBytes` through `p.GCDiskFloor(nil)` so wave check-in honors `disk.floor_gb` and `preflight.min_free_gib`.
- `go/internal/policy/preflight_config.go` — routes `PreflightConfig` and `MinFreeBytes` through `diskfloor` and adds `DiskFloor`, `GCDiskFloor`, and `validateDiskFloor`.

## Design Decisions
- `go/internal/diskfloor` reads legacy `DiskFloorGB`, `DiskWarnGB`, and `MinFreeGiB` fields with reflection. `gcpolicy.Policy`, `policy.PreflightPolicy`, and `policy.PreflightConfig` keep their existing exported fields (`strict_keys_shape_test.go` forbids embedded structs). No non-test code outside `internal/diskfloor` reads those fields.
- Unset defaults remain `10` GB for `evolve gc` and `1` GiB for preflight and cycle start. When any floor key is set, every reader gets that same byte floor.
- `policy.Load` refuses conflicting floor or warn keys at parse time. Callers that use `LoadOrDefaults` (`cmd_loop_preflight`, `cmd_loop_window`, `cmd_wave_next`) check `policy.Load` for `diskfloor.ErrConflict`. They halt or refuse instead of falling back to defaults.
- The whole-tree AST guard sits in `go/internal/policy/disk_floor_single_reader_test.go` (`internal/policy` is in `repocontract.Packages()`), so `go/internal/repocontract/repocontract.go` needs no change.

## Verification
Unit tests (`./internal/diskfloor`, `./internal/gcpolicy`, `./internal/policy`, `./internal/looppreflight`, `./internal/hostsentinel`, `./internal/orchestrator`, `./internal/repocontract`), `./cmd/evolve` disk tests, `go vet`, `ste-lint`, and `./acs/cycle2634` (`9/9`) are verified against this workspace.

## Compatibility
Legacy keys (`gc.disk_floor_gb`, `gc.disk_warn_gb`, `preflight.min_free_gib`) still work and print a `WARN` naming the new key. Equal byte values on old and new keys succeed; unequal values fail with an error naming both keys.

## Limitations
Unset defaults still differ between `evolve gc` (10 GB) and preflight (1 GiB) when no disk floor key is configured.
