# Build Explanation — Cycle 2132

## Build Binding
- Cycle: 2132
- Base SHA: 9f39bb1cce8922976a930e3ed3cab242325a7b76

## Summary
The command `evolve cycle models` now prints a usage text that lists each of its flags, and the operator reference documents the command.

## Rationale
The command and the served-model recording existed in the salvaged work of cycle 2122. The salvage lacked a usage text, so `-h` did not list the flags that the operator bullet names. A custom usage function is the smallest change that makes the flags discoverable. A second option, a new help subcommand, was rejected because it adds a surface.

## Changed Areas
- `go/cmd/evolve/cmd_cycle_models.go` — adds a usage function that prints each registered flag with its description, so `-h` offers every flag the bullet names.
- `docs/operations/runtime-reference.md` — adds one bullet that documents the flags, the `unreported` marker and the exit codes `1` and `10`.
- `go/acs/cycle2132/predicates_test.go` — adds seven predicates that pin the output, the recording, the fallback, the bullet and two scope fences.
- `.evolve/evals/cli-cycle-models-console.md` — adds the eval that lists the score caps for those predicates.
- `.evolve/evals/cli-cycle-models-console-docs.md` — adds the eval for the operator bullet.
- `docs/private/research/archived-2026-11-03/superseded-predicate-packages/cycle2122/predicates_test.go` — archives the superseded cycle 2122 predicate package, moved from `go/acs/cycle2122/predicates_test.go`.
- `docs/private/research/archived-2026-11-03/unshipped-build-explanations/cycle-2122-01m6ey1rg6ke60p8r24f33g222.md` — archives the build explanation of cycle 2122, which never shipped.

## Design Decisions
The usage function uses `flags.VisitAll`, so a new flag appears in the help without a second edit. The exit codes stay `1` for no records and `10` for bad arguments, as in the other `evolve` verbs.

## Verification
- `go test -tags acs -count=1 ./acs/cycle2132` passes 7 of 7.
- The integration tests of `go/cmd/evolve` have one failure, `TestProcessStartTime_ReadsTheRunningProcess`. The sandbox denies `ps`, so the failure is environmental and is not related to this change.

## Compatibility
No flag, output format or exit code changes. Only the text printed for `-h` changes.

## Limitations
The usage text does not group flags. The `ps` dependency of `processStartTime` is out of scope.
