# Build Explanation — Cycle 2676

## Build Binding
- Cycle: 2676
- Base SHA: 1b9b478be003292dbbb262a96899671756f6ce39

## Summary
The host-adaptive fleet window now reads the host verification lock wait as a pressure input. A lock wait over a policy bound stops a widening of the window. A decrease is never stopped. `wave status` shows the total lock wait of the wave.

## Rationale
In wave 186 the waits ran from 280 s to 701 s, and one wait of 900 s ran unserialized. The wave-level readers saw 0, because the signal is written only to the run signal logs. A window that widens while the lock is saturated adds more lanes to the same queue.

## Changed Areas
- `go/internal/hostpressure/decide.go` — `Decide` holds a due widening when `Reading.LockWait` is over `Config.LockWaitMax`. The hold sets `Decision.LockWaitHeld` and the cause `lock_wait_high`. The hold does not stamp `State.Changed`, so the first sample after the wait clears widens at once.
- `go/internal/hostpressure/reader.go` — `Reading` carries `LockWait`.
- `go/internal/hostpressure/decide_test.go` — tests the hold and the immediate widening after it clears.
- `go/internal/policy/fleet_adaptive.go` — adds the policy key `lock_wait_max_s` with the default 600. The strict parse refuses a negative value.
- `go/internal/policy/fleet_adaptive_test.go` — pins the default, the override and the refusal.
- `go/cmd/evolve/cmd_loop_hostpressure.go` — `startHostPressure` feeds the sampler the sum of the waits in the last window from the run signal logs. The shadow sample line records `lock_wait_ms` and `lock_wait_held`.
- `go/cmd/evolve/cmd_wave_status.go` — `wave status` reports `lock_wait_ms` and a `lock wait:` text line. The total counts each event once, by process id and sequence, across the run logs and the global log. `lastWaveSummary` is extracted to keep `buildWaveStatus` within 50 lines.
- `go/cmd/evolve/cmd_loop_hostpressure_lockwait_test.go` — tests the production caller and the shadow line.
- `go/cmd/evolve/cmd_wave_lockwait_test.go` — tests the run-folder reader, the window sum and the wave total, including de-duplication.

## Design Decisions
The bound is a policy value. A policy value of 0 resolves to the default 600 (`go/internal/policy/fleet_adaptive.go:110`). Only a `Config.LockWaitMax` of 0 set directly switches the rule off (`go/internal/hostpressure/decide.go:68`). The code holds a default, because `.evolve/policy.json` is a protected path. The rule compares one total against one bound. It does not model a lock with K slots.

## Verification
`go test -count=1` passes for `internal/hostpressure` and `internal/policy`. The ACS predicates 001 to 007 pass (7/7 PASS, `go/acs/cycle2676/predicates_test.go`). The ACS suite reports red=0.

## Compatibility
Without `lock_wait_max_s` in the policy, the default 600 s applies. A window that never sees a wait over 600 s decides as before.

## Limitations
The rule compares one total against one bound. It does not model a lock with K slots.
