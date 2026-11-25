# Build Explanation — Cycle 2647

## Build Binding
- Cycle: 2647
- Base SHA: b1a5a1cec52807e60609772f369c0224267ef1ed

## Summary
Required CI now runs the `yaml` floor tier inside `ste-lint`, `floorCIParity` tracks that row, and `go.yml` keeps host sampler stderr in an uploaded log file.

## Rationale
The `ste-lint` job already builds `$RUNNER_TEMP/evolve`, so one extra step runs `evolve yaml check` at zero extra build cost. Redirecting host sampler stderr to `go/runner-sampler.log` keeps diagnostic output that `/dev/null` discarded.

## Changed Areas
- `.github/workflows/required.yml` — adds a step to `ste-lint` that runs `"$RUNNER_TEMP/evolve" yaml check --root "$GITHUB_WORKSPACE"` so CI enforces workflow and config YAML checks on the binary that job already builds.
- `.github/workflows/go.yml` — truncates `go/runner-sampler.log`, redirects `hostsampler` stderr to it in `build-test`, `build-test-darwin`, and `build-test-current`, and uploads `go/runner-sampler.log` beside `go/runner-samples.ndjson`.
- `go/cmd/evolve/cmd_floor_test.go` — adds the `yaml` row to `floorCIParity` and lets `TestFloorRun_TierCommandsMatchRequiredCI` verify steps defined directly in `required.yml` without a reusable-workflow call.

## Design Decisions
- Reuse `$RUNNER_TEMP/evolve` instead of `go run ./cmd/evolve` in `ste-lint` because the binary already exists from the earlier build step.
- Create `go/runner-sampler.log` synchronously before backgrounding `hostsampler` so `treewalk snapshot` records the log path before `make test-integration` starts.
- Match the `floorCIParity` `yaml` row on `"yaml check"` because `floor` plans `go run ./cmd/evolve yaml check` while `ste-lint` invokes `"$RUNNER_TEMP/evolve" yaml check`.
- Keep all macOS spawn-layer files untouched because the operator excluded the macOS spawn shim from this cycle.

## Verification
`TestFloorRun_TierCommandsMatchRequiredCI`, `TestYAML`, `gofmt`, `go vet`, `GOOS=darwin go vet`, and all seven cycle-2647 ACS predicates pass against these edits.

## Compatibility
All existing `go.yml` artifacts and `required.yml` job names stay intact. The `go/runner-sampler.log` file is additive to each upload artifact list.

## Limitations
The `ste-lint` job runs only when `changes.outputs.docs` is `true`, so PRs without doc changes do not trigger `ste-lint`.
