# Build Explanation — Cycle 1782

## Build Binding
- Cycle: 1782
- Base SHA: a24c24c937acdb2f1bae3efde6bc7bc616f402c7

## Summary
Two fleet-lane tasks. The starvation observer now compares the lanes a wave realized with the width the wave was sized to after the quota-aware shrink, so a shrunk wave that still misses its sized width counts as starved. A new `disk-space` readiness check halts `evolve loop` boot below a policy floor (`preflight.min_free_gib`, default 1 GiB), and the same check runs at every wave boundary, where it stops the run with an ADR-0072 system halt before the next wave launches. Both halts name `evolve gc`.

## Rationale
Waves 19 and 20 realized 1 and 0 of 2 sized lanes after a 3→2 quota shrink and were counted as not starved, because any shrink switched the detector off. A shrink explains only the lanes it removed, so the sized width is what realized lanes are compared with. Wave 27 went 0/2 on 143 MiB free and both lane failures read as code FAILs. A full disk is a system failure, so it has to halt the batch before lanes run, not fail them.

## Changed Areas
- `go/internal/fleet/starvation.go` — `WaveObservation` gains `SizedLanes`; `Starved` compares `RealizedLanes` with the sized width. `DesiredLanes`/`QuotaShrunk` remain only as the fallback for observations that leave `SizedLanes` unset, so `go/acs/cycle544` still compiles and passes. The todo text reports the sized width.
- `go/cmd/evolve/cmd_loop_window.go` — `observeWorkSupply` passes `waveConfig.Count` as `SizedLanes`. `dispatchFleetIteration` first runs `diskSpaceHalt`, which loads the policy floor, runs `looppreflight.CheckDiskSpace` on `.evolve`, and on a halt emits `loop.halt`/`LOOP_HALT` with `stop_reason=disk_space_halt` and returns rc=4 before any pool or wave launches.
- `go/cmd/evolve/cmd_loop_preflight.go` — `defaultLoopPreflight` passes the policy floor as `Options.MinFreeBytes`.
- `go/internal/looppreflight/diskspace.go` — new exported `CheckDiskSpace` (halt below the floor, pass at or above it, warn when the disk cannot be measured) and the `disk-space` check in `Run`.
- `go/internal/looppreflight/looppreflight.go` — `Options.MinFreeBytes`; `Run` adds `checkDiskSpace(o, opts.MinFreeBytes)`, and 0 resolves to the policy default inside `checkDiskSpace`, which keeps `resolve` within its size-ratchet allowance.
- `go/internal/looppreflight/checks_test.go` — the low-disk test no longer asserts that 100 MiB free leaves the batch running. It asserts that `host-capabilities` still warns and `disk-space` halts.
- `go/internal/looppreflight/diskspace_check_test.go` — table test for `CheckDiskSpace`.
- `go/internal/policy/policy.go` — `Policy.Preflight` (`preflight` block).
- `go/internal/policy/preflight_config.go` — `PreflightPolicy`, `PreflightConfig`, `Policy.PreflightConfig()` (absent, zero or negative values take the 1 GiB default) and `PreflightConfig.MinFreeBytes()`.
- `go/internal/policy/preflight_config_bytes_test.go` — GiB to bytes conversion.
- `go/acs/cycle544/predicates_test.go` — a doc-only note on C544_005 records that its quota-shrink rule is superseded by the sized-width comparison of this cycle and why; no assertion changed, and the package still passes through the legacy fallback.
- `docs/operations/runtime-reference.md` — documents `preflight.min_free_gib`, its default, both halt points and the fix.
- `docs/architecture/packages/internal-looppreflight.md` — documents the policy disk-space floor next to the protected 500 MiB host warning.
- `go/acs/cycle1782/predicates_test.go`, `go/internal/fleet/starvation_sized_width_test.go`, `go/internal/looppreflight/diskspace_test.go`, `go/internal/policy/preflight_config_test.go`, `go/cmd/evolve/cmd_loop_diskspace_starvation_test.go`, `.evolve/evals/disk-space-preflight.md`, `.evolve/evals/starvation-compares-sized-width.md` — the TDD phase's RED tests and evals, committed with the build.

## Design Decisions
The wave boundary reuses the boot check through the exported `CheckDiskSpace`, so there is one threshold rule, and the floor is policy rather than a Go literal. The wave-boundary halt reuses the existing wave-boundary code `LOOP_HALT` instead of registering a new signal code. An unmeasurable disk only warns, because a failed `statfs` is not evidence of a full disk. `host-capabilities` keeps its 500 MiB low-disk warning because `checks.go` is a protected control-plane path (ADR-0064). Below the policy floor, the redundant warning sits next to the halt. The legacy `DesiredLanes`/`QuotaShrunk` fields stay, marked `minimal:`, because C544_005 in `go/acs/cycle544` still constructs observations with them, and rewriting its assertions is outside this build.

## Verification
`go test -count=1` passes for `./cmd/evolve`, `./internal/fleet`, `./internal/policy` and `./internal/looppreflight`. `go vet ./...` and `gofmt -l` are clean. `go test -tags acs -count=1 ./acs/cycle1782` is 8 of 8 green, including C1782_003, which reads the supersession note added to `go/acs/cycle544/predicates_test.go`. `./acs/cycle544` is 5 of 5 green through the legacy fallback, and the full ACS suite reports red=0.

## Compatibility
Without a `preflight` block, the default floor is 1 GiB. That is stricter than the old 500 MiB warning and now halts. Callers of `WaveObservation` that set only `DesiredLanes`/`QuotaShrunk` keep the old behavior.

## Limitations
The boot halt reports through the readiness-gate summary, not a dedicated Signal Center code. The legacy `DesiredLanes`/`QuotaShrunk` fallback stays because C544_005 still builds observations without `SizedLanes`; removing it needs a later cycle that moves those cycle544 observations to `SizedLanes`. The redundant 500 MiB `host-capabilities` warning in the protected `checks.go` also stays until console work removes it.
