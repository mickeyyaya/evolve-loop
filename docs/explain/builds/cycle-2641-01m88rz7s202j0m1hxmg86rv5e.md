# Build Explanation — Cycle 2641

## Build Binding
- Cycle: 2641
- Base SHA: 91b46370ab4b36290c541f2ab25afdf32d9ec291

## Summary
The boundary plan for `evolve boundary run` and `evolve wave next` no longer runs `evolve ci latency check`. The plan keeps `evolve wave latency --check` as an advisory step between `evolve quality report` and `evolve gc`. Usage exits from advisory steps now stop the boundary, and a test pins every step to a registered verb and subverb.

## Rationale
`boundaryHaltSteps` called `ci latency check` even though `ci` has no `latency` subverb. Because `runBoundarySteps` ignored every non-zero exit on advisory steps, `exitUsage` (code 10) never stopped the boundary. Removing the dead step and rejecting `exitUsage` prevents silent misconfiguration. `TestBoundarySteps_EachStepNamesARegisteredVerbAndSubverb` catches unregistered verbs or subverbs during unit tests.

## Changed Areas
- `go/cmd/evolve/cmd_boundary.go` — removes `ci latency check` from `boundaryHaltSteps` and fails advisory steps on `exitUsage` in `runBoundarySteps`.
- `go/cmd/evolve/cmd_wave.go` — lifts `waveSubcommands` to a package-level map so dispatch and the step-registration test share one table.
- `go/cmd/evolve/cmd_boundary_latency_test.go` — updates latency tests for one check, adds `TestBoundaryRun_AnAdvisoryStepUsageErrorFailsTheBoundary`, and adds `TestBoundarySteps_EachStepNamesARegisteredVerbAndSubverb`.
- `go/cmd/evolve/cmd_boundary_test.go` — adjusts the full verb count and step-failure indices from 17 steps to 16 steps.
- `go/cmd/evolve/cmd_boundary_launch_test.go` — adjusts the launch failure index from 17 to 16.
- `go/cmd/evolve/cmd_boundary_ratchet_test.go` — removes `ci` from the expected halt-step verb sequence.
- `go/cmd/evolve/cmd_quality_test.go` — adjusts the `quality` offset before `gc` from `len-4` to `len-3`.
- `go/cmd/evolve/cmd_wave_test.go` — removes `ci` from `waveFullSteps` and updates the dry-run step count from 19 to 18.
- `go/cmd/evolve/cmd_wave_errors_test.go` — updates the last numbered step in text dry-run output from 19 to 18.
- `go/cmd/evolve/cli_contract_test.go` — updates the failed step prefix in the envelope test from `3/16` to `3/15`.
- `go/cmd/evolve/cli_contract_cover_test.go` — updates the JSON boundary run plan and step counts from 16 to 15.

## Design Decisions
Removing `ci latency check` avoids adding a fake no-op `ci latency` subverb and keeps `evolve wave latency --check` as the boundary latency gate. Rejecting `exitUsage` on advisory steps ensures invalid flags or subverbs fail fast. Runtime errors such as `exitIO` remain advisory. Probing subverbs through `lookupCommand` verifies external CLI subverbs without duplicating tables across packages.

## Verification
The cycle 2641 ACS suite (`evolve acs suite --cycle 2641`) passes with `green=266 red=0 skip=53 total=319`. Both `go vet ./cmd/evolve/...` and `gofmt -l cmd/evolve` pass cleanly.

## Compatibility
`boundary run` drops from 17 steps (16 without `--merge`) to 16 steps (15 without `--merge`). `wave next` drops from 20 steps (19 without `--merge`) to 19 steps (18 without `--merge`). Runtime failures on `evolve wave latency --check` remain advisory, while `exitUsage` stops the boundary.

## Limitations
The step-registration test verifies subverbs by checking that each reaches its own flag parser with `exitUsage` and rejects `ci latency` as a negative control. It does not execute live side effects for every verb.
