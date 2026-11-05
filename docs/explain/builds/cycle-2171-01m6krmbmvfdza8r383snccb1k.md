# Build Explanation — Cycle 2171

## Build Binding
- Cycle: 2171
- Base SHA: 02766082270aaf5e98ac01231b04691a27bfa54f

## Summary
A pool start that the control-plane preflight refuses now emits a typed WARN signal. The wave status names the reason, and the events watch of the errors channel wakes on the signal.

## Rationale
Before this change, a refused preflight wrote one console line and the pool fell back to one sequential cycle. The console and the wave status never saw the reason. The signal reuses the existing code LOOP_WAVE_DISPATCH_FAILED with path=pool and step=preflight. The errors channel routes on severity WARN, so no new channel route is needed.

## Changed Areas
- `go/cmd/evolve/cmd_loop_pool.go` — adds the typed error poolPreflightError. The text of the error stays the same.
- `go/cmd/evolve/cmd_loop_window.go` — dispatchPool emits the WARN signal when the error is a poolPreflightError. A backlog plan failure emits no signal.
- `go/cmd/evolve/cmd_wave_status.go` — the wave status adds the reason of each pool dispatch signal of the current wave to its warnings.
- `go/cmd/evolve/parallel_serial_tests_test.go` — lists the two events-watch tests as serial, because they swap the root events seam.
- `go/cmd/evolve/cmd_loop_pool_preflight_signal_test.go` — adds the tests for the signal, the status line and the events watch.
- `go/acs/cycle2171/predicates_test.go` — adds the acceptance predicates of the cycle.
- `.evolve/evals/wave-preflight-failure-is-loud-and-does-not-silently-go-serial.md` — adds the eval graders of the task.

## Design Decisions
A typed error replaces a text match, so the signal does not depend on the wording of the message. The signal reuses an existing code, so the code registry does not change. The status filter uses the start time of the wave, so an old signal does not show.

## Verification
The TestPoolPreflight and TestWaveStatus tests pass, and so does the parallel-default rule test. The events-watch tests call useRealRootEvents, because the package test setup replaces the publisher.

## Compatibility
The console line, the error text and the sequential fall back are unchanged. The signal and the status warning are new output.

## Limitations
A backlog plan failure in the pool still emits no signal. This change does not add a retry of a transient preflight failure. It also does not add a documentation decision on the sequential fall back. The tests of dispatchPool in go/cmd/evolve/cmd_loop_window_backfill_e2_test.go cover its statements under the integration build tag.
