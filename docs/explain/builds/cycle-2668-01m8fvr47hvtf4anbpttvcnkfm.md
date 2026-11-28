# Build Explanation — Cycle 2668

## Build Binding
- Cycle: 2668
- Base SHA: 59e7b0750b9210903d273be167b7434d045f8691

## Summary
The failure closeout now charges pre-build refusals to the pinned item, skips host faults, and keeps continuation stamps when quarantine fails.

## Rationale
Classifying pre-build refusals in `cycleoutcome` and shedding continuation after `Promote` keeps the change local and preserves post-build behavior.

## Changed Areas
- `go/internal/cyclestate/result.go` — adds `HostFault` to `Disposition` and marks `TRIAGE_LANE_PIN_MISSING`, `TRIAGE_COMMITMENT_INVALID`, and `BLOCKER_INCOMPLETE` so host faults charge no item.
- `go/internal/cyclestate/phase.go` — adds `Phase.IsPreBuild` so `cycleclassify` and `cycleoutcome` share one definition of pre-build phases.
- `go/internal/cycleclassify/classify.go` — keeps pre-build coded refusals past retro as `ClassPhaseRefusal` so `FailureInputsFor` sees the phase and code.
- `go/internal/cycleoutcome/cycleoutcome.go` — updates `IsTaskLevelResult` to treat pre-build coded refusals as task-level unless `HostFault` is true.
- `go/internal/inboxmover/lifecycle/release.go` — moves continuation deletion in `parkAtCeiling` to `shedAfterPark` after `Promote` succeeds so blocked parks preserve `continuation`.

## Design Decisions
Placing `IsPreBuild` on `cyclestate.Phase` avoids duplicate phase lists across `cycleclassify` and `cycleoutcome`. An alternative was checking phase names only in `cycleoutcome`, which failed when `retro` followed a coded pre-build refusal. Moving continuation deletion out of `bumpWith` in `parkAtCeiling` into `shedAfterPark` leaves `BumpFailureCount` and `bumpWith` callers unchanged.

## Verification
Unit tests in `cyclestate`, `cycleclassify`, `cycleoutcome`, `inboxmover/lifecycle`, and `cmd/evolve` pass alongside `acs/cycle2668` and `evolve acs suite --cycle 2668`.

## Compatibility
Post-build uncoded refusals, `BUILD_DELIVERABLE_REJECTED`, infrastructure sentinels, and successful quarantine parks retain prior behavior. Blocked quarantine parks and console-routed ceiling paths now keep `continuation` and `retry_plan`.

## Limitations
Un-coded pre-build failures without a diagnostic code remain system-level, and `blocker_breaker.go` was left unchanged because closeout charging lives in `cycleoutcome` and `lifecycle`.
