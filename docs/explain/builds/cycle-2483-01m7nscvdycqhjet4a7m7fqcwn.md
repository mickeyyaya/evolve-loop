# Build Explanation — Cycle 2483

## Build Binding
- Cycle: 2483
- Base SHA: 43329a942327cd143a7e3bb977459514d77837cc

## Summary
The boundary of `evolve boundary run` and `evolve wave next` now runs two latency checks as advisory steps. The checks are `evolve ci latency check` and `evolve wave latency --check`. They run between the quality report and gc. A failed check prints a WARN, and the boundary continues to the launch.

## Rationale
Task T7 of the latency-gates plan asks for the T5 and T6 checks at the boundary. A breach files an inbox item inside the check itself, so the boundary only has to run the check and keep going. One shared step list serves both verbs, so one edit covers both plans. An `advisory` flag on the step keeps the stop-at-first-failure rule for every other step.

## Changed Areas
- `go/cmd/evolve/cmd_boundary.go` — `boundaryHaltSteps` adds the two advisory steps before gc. `dispatchPlaneVerbs` registers the `ci` and `wave` verbs in-process. `runBoundarySteps` prints a WARN for a failed advisory step and continues.
- `go/cmd/evolve/cmd_boundary_latency_test.go` — new tests for the plan order, the WARN path, the breach path, the dispatcher registration and the wave launch.
- `go/cmd/evolve/cmd_boundary_test.go` — step counts and indices follow the new 17-step plan.
- `go/cmd/evolve/cmd_boundary_launch_test.go` — the refused-launch test uses the shared full verb list.
- `go/cmd/evolve/cmd_boundary_ratchet_test.go` — the halt-step verb list includes the two checks.
- `go/cmd/evolve/cmd_quality_test.go` — the quality step is now four steps before the end.
- `go/cmd/evolve/cmd_wave_test.go` — the full wave step list and step counter include the two checks.
- `go/cmd/evolve/cmd_wave_errors_test.go` — the dry-run output has 19 steps.
- `go/cmd/evolve/cli_contract_test.go` — the failed-step message names step 3 of 16.
- `go/cmd/evolve/cli_contract_cover_test.go` — the JSON plan has 16 steps.
- `go/acs/cycle2483/predicates_test.go` — predicates for the plan order, the advisory rule and the hard-step twin.
- `.evolve/evals/latency-gates-t7-boundary-runs-checks.md` — eval graders for the task.

## Design Decisions
The step carries an `advisory` field, so the one runner decides between WARN and stop. Two options were rejected. One was a separate runner for advisory steps, which duplicates the loop. The other was a shell wrapper that hides the exit code, because the wave recorder must still record the failed step as not ok. The `ci` and `wave` verbs run in-process, so no built binary is needed.

## Verification
The cycle 2483 predicates and the `./cmd/evolve` package tests pass, and `go vet` reports nothing.

## Compatibility
The boundary plan grows by two steps. Operators who read the step numbers see 17 steps for `boundary run` and 19 for `wave next`. The exit codes of all hard steps stay the same.

## Limitations
A check that fails prints a WARN and files nothing by itself. A breach files an item only when the check runs to its end. The `evolve ci latency check` verb (task T5) does not exist yet: `ciSubverbRunner` in `go/cmd/evolve/cmd_ci.go` has no `latency` case. Until T5 lands, that step fails with a usage error and prints a WARN on every boundary, and the boundary continues.
