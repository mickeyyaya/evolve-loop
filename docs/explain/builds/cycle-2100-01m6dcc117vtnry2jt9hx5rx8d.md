# Build Explanation — Cycle 2100

## Build Binding
- Cycle: 2100
- Base SHA: 737352d1bd7eb54da1ee46c0a0e00a600540729e

## Summary
The CI telemetry report now names the limit of a package with the C12 rules. The rules use window saturation with the runner CPU count, a 0.5 s CPU floor, and contention.

## Rationale
The first instrumented CI run gave wrong limits for hundreds of packages. The fix keeps the one rule function and adds the missing inputs, so no second code path exists.

## Changed Areas
- `go/internal/citelemetry/packages.go` — adds the run queue to the sampler, the runner CPU count, the test floor and the new rule order.
- `go/internal/citelemetry/citelemetry.go` — passes the critical job name to the package join.
- `go/internal/citelemetry/citelemetry_test.go` — updates the call of the limit function to the new signature.
- `docs/architecture/packages/internal-citelemetry.md` — lists the new rule order.

## Design Decisions
The lowest run queue of the window decides saturation, so one spike does not count. A window without a run queue on every sample uses busy percent alone.

## Verification
The ACS predicates of cycle 2100 and the package tests cover each rule and its boundary.

## Compatibility
The report JSON keeps every current field. The report writes no file. The new rule order is not fully backward compatible for the limit value. A package with a wall above twice its floor now gets `contention`, where it got `subprocesses-and-syscalls` or `wait` before. This change turns two cycle 2066 predicates red: `go/acs/cycle2066/predicates_test.go:454` (internal/core, `subprocesses-and-syscalls`) and `go/acs/cycle2066/predicates_test.go:470` (internal/idle, `wait`). The builder must not edit predicates, so the console must adjudicate or update those two cycle 2066 expectations in a follow-up.

## Limitations
The window is the span from the first event to the package result event.
