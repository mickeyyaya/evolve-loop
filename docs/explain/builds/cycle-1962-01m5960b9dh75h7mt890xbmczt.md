# Build Explanation — Cycle 1962

## Build Binding
- Cycle: 1962
- Base SHA: 6aa8ea99c217fb450cfc84a50e55c8ce7c24dd0b

## Summary
`evolve floor run` runs the local CI floor tier by tier. Each tier prints PASS, RED or ERROR. A red tier exits 1, and a tier that cannot run exits 2.

## Rationale
Skipped local tiers shipped CI reds (the e2e tier in #711, the size ratchet and acs/regression in a comment-reduction round). One verb now runs every tier with the same commands as CI. The tier table is plain data, so a test can compare it with the workflow steps. The fmt tier takes its toolchain from the `lint` job of `go.yml`, because a newer gofmt can hide a rewrite that CI's Go flags. The rejected option was a Makefile target: it cannot pin the toolchain or print one verdict per tier.

## Changed Areas
- `go/cmd/evolve/cmd_floor.go` — adds the verb: the tier table, the fmt toolchain pin, the runner and the exit-code rules.
- `go/cmd/evolve/registry.go` — registers the `floor` command.
- `go/cmd/evolve/cmd_floor_run_test.go` — covers the runner, the verdicts, the exit codes and the refusals with an injected executor.
- `go/cmd/evolve/cmd_floor_test.go` — dry-run tests that compare the tier commands with `required.yml` and the workflows it calls.
- `go/acs/cycle1962/predicates_test.go` — ACS predicates that run the built binary against fixture modules.
- `.evolve/evals/cli-floor-run-lane.md` — the eval of the acceptance criteria.
- `docs/operations/runtime-reference.md` — documents the verb, its output and its exit codes.

## Design Decisions
Every tier command is a shell string run with `sh -c` in `<root>/go`, and `--dry-run` prints the same string. Exit 127 and a start failure are ERROR, other non-zero codes are RED. The fmt command reads gofmt's output before its exit code, because gofmt 1.27 exits 1 with a diff and CI's Go 1.23 exits 0.

## Verification
`go test -count=1 -run TestFloorRun_ ./cmd/evolve` passes, every function of `cmd_floor.go` is at 100% statement coverage, and `go test -tags acs ./acs/cycle1962` passes 8/8. `evolve acs suite --cycle 1962` reports green=244 red=0.

## Compatibility
The change only adds a verb and a registry entry. No existing command, flag or file changes behavior.

## Limitations
The integration, e2e and cover tiers need a long local run. The tier table does not follow a new CI step by itself: the parity test fails when the workflow and the table differ.
