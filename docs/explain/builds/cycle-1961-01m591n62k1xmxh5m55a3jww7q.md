# Build Explanation — Cycle 1961

## Build Binding
- Cycle: 1961
- Base SHA: 39f4c93e562f99ea8a97fecba31de11c15909944

## Summary
The loop now halts after two consecutive zero-ship waves. A wave is the set of cycles launched between two iteration boundaries. The halt carries the Signal Center code LOOP_ZERO_SHIP_HALT and files a P0 pipeline-defect item of category zero-ship.

## Rationale
The operator guardrail in operating-policy.md section 4.1 had no compiled counterpart. SKIPPED no-work lanes and mixed EMPTY and FAIL outcomes trip neither the consecutive-failures rule nor the goal-stall escalation. The rule reuses the existing halt path (haltOnSystemFailure) and the outcome classifier (cyclehealth.ClassifyOutcome), so no new halting machinery exists.

## Changed Areas
- `go/internal/core/zero_ship.go` — adds the pure rule EvaluateZeroShip (trailing zero-ship run against a ceiling; a non-positive ceiling disables it) and the collectors CollectCycleShips and CountShips.
- `go/internal/policy/policy_failure.go` — adds the threshold failure_policy.thresholds.zero_ship_halt with a compiled default of 2; zero and negative values fall back to the default.
- `go/cmd/evolve/cmd_loop_zeroship.go` — adds the per-batch tally and the halt, which reads the ceiling from policy.json.
- `go/cmd/evolve/cmd_loop_window.go` — calls the zero-ship check in prepareIteration right after the blocker breaker, before any launch.
- `go/cmd/evolve/cmd_loop_batch.go` — adds the tally field to the batch coordinator.
- `go/cmd/evolve/signal_loop.go` — registers the LOOP_ZERO_SHIP_HALT code.
- `docs/architecture/signal-codes.md` — regenerated from the registry with `evolve signals codes generate`.
- `go/cmd/evolve/parallel_serial_tests_test.go` — lists the seven new loop tests as serial because they swap process-global seams.

## Design Decisions
SHIPPED and SALVAGED outcomes count as productive; FAILED, DEFERRED and NO_WORK count as zero-ship. Counting salvage as zero-ship was rejected because it would halt a loop that recovered work. The tally lives on the coordinator and starts after the batch start cycle, so earlier batches never count. An iteration that launched no cycle is not a wave.

## Verification
Targeted tests, the full internal/core, internal/policy and cmd/evolve packages, and the ACS suite for cycle 1961 (green=241 red=0) pass. One cmd/evolve test, TestBridgeModelCapturer_CapturesThroughTheResolvedDriver, fails only while EVOLVE_FLEET is set in the pane environment and passes with it unset.

## Compatibility
No existing threshold changes. The new threshold has a compiled default, so a policy.json without it behaves with a ceiling of 2.

## Limitations
The tally is in memory per batch: a restarted batch recounts from its own start cycle.
