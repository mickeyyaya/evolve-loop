# Build Explanation — Cycle 1863

## Build Binding
- Cycle: 1863
- Base SHA: 56cdf89abf503eef9daae72beec003fcfa8d9357

## Summary
Adds `evolve carryover list` to inspect `carryoverTodos` from `state.json` without taking a write lock.

## Rationale
Operators previously inspected state.json carryover todos with jq. The evolve carryover list subcommand provides a native read-only CLI interface. It computes todo age from lastCycleNumber minus first_seen_cycle. It also supports sorting, row limits, and JSON output.

## Changed Areas
- `docs/operations/runtime-reference.md` — documents `evolve carryover list` and its flags.
- `go/cmd/evolve/cmd_carryover.go` — implements `runCarryoverList` supporting `--json`, `--sort`, `--limit`, and `--state` without locks.
- `go/cmd/evolve/cmd_carryover_test.go` — TDD test coverage for `evolve carryover list` covering flags, errors, read-only behavior, and table output.
- `go/acs/cycle1863/predicates_test.go` — TDD ACS predicates verifying cycle 1863 contract requirements.
- `.evolve/evals/cli-carryover-list.md` — TDD eval criteria for the `cli-carryover-list` task.

## Design Decisions
- `runCarryoverList` reads state through `statemap.ReadStateMap` without a lock to keep read-only semantics.
- Non-JSON table output uses `tabwriter` to match the formatting conventions of other inspection subcommands.
- Sorting supports `age` and `unpicked` with stable ordering, rejecting any unsupported sort parameter with exit code 10.
- Negative limit values exit 10, whereas limit 0 displays all items.
- Unreadable or malformed state files exit 2 with no stdout.

## Verification
- Unit test suite `go test -v -run '^TestCarryoverList' ./cmd/evolve` passes 8/8 tests.
- ACS predicate test suite `go test -v -tags acs ./acs/cycle1863` passes 10/10 predicates.

## Compatibility
- Existing `evolve carryover apply-decisions` behavior remains untouched.
- The `state.json` schema and file contents are never mutated by `evolve carryover list`.

## Limitations
- Only `age` and `unpicked` sort modes are currently supported.
