# Build Explanation — Cycle 2598

## Build Binding
- Cycle: 2598
- Base SHA: ee1a3bacc92f494ffb2f0e084c1a4c6b65cbdc1b

## Summary
The wave latency reader and check now separate host verification lock wait from deterministic gate work.

## Rationale
`HOST_VERIFY_LOCK_WAIT` signals mark time spent in the host verification lock queue. Counting that wait as gate work inflated the gate share of a wave. Attributing each positive wait to its span and subtracting it from host time exposes true gate work and `lock_wait_share`.

## Changed Areas
- `go/internal/lanetime/cycle.go` — adds `LockWait` and `Cycle.LockWaits` to carry parsed lock wait timestamps and durations without importing `signalcenter`.
- `go/internal/lanetime/read.go` — parses positive `wait_ms` fields on `HOST_VERIFY_LOCK_WAIT` signals into `Cycle.LockWaits` and ignores malformed or non-positive values.
- `go/internal/lanetime/phases.go` — adds `PhaseStat.LockWait`, attributes each lock wait once to the first phase span that holds its timestamp, and clamps the total to `PhaseStat.Host`.
- `go/internal/lanetime/check.go` — defines `CodeHostVerifyLockWait`, adds `Verdict.LockWaitShare`, computes `GateShare` from `host - lockWait` over span time, and names the lock wait share in gate breach reasons.
- `go/cmd/evolve/cmd_wave_latency.go` — prints the `lock-wait` column in the phase breakdown table of `evolve wave latency`.
- `go/cmd/evolve/cmd_wave_latency_check.go` — prints `lock wait` in the per-wave summary line of `evolve wave latency --check`.
- `go/internal/lanetime/lanetime_test.go` — adds lock wait parsing, attribution, clamping, and JSON key coverage for `ReadCycles` and `Phases`.
- `go/internal/lanetime/check_test.go` — adds lock wait exclusion and breach reason coverage for `Check`.
- `go/cmd/evolve/cmd_wave_latency_test.go` — asserts that the phase table text output shows `lock-wait`.
- `go/cmd/evolve/cmd_wave_latency_check_test.go` — asserts that the check text output shows `lock wait`.
- `docs/architecture/packages/internal-lanetime.md` — documents `Lock wait`, `lock_wait`, and `lock_wait_share` in the package spec.
- `docs/operations/wave-operations.md` — documents `lock_wait` and `evolve wave latency --check` with `lock_wait_share`.

## Design Decisions
Each lock wait matches the first span that holds its timestamp. This avoids double-counting and drops waits outside every span. `PhaseStat.LockWait` clamps to `PhaseStat.Host`, so `GateShare` never drops below zero.

## Verification
Unit tests in `go/internal/lanetime` and CLI tests in `go/cmd/evolve` cover parsing, attribution, clamping, and output. The cycle 2598 and 2377 ACS suites pass.

## Compatibility
`PhaseStat.Host` keeps the full host duration so existing latency consumers and pinned tests do not break. `PhaseStat` adds `lock_wait` and `Verdict` adds `lock_wait_share` as additive JSON fields.

## Limitations
`HOST_VERIFY_LOCK_WAIT` carries `wait_ms` only from wave 186 on, so earlier waves still show zero `lock_wait`.
