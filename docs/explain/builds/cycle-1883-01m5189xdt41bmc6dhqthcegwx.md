# Build Explanation — Cycle 1883

## Build Binding
- Cycle: 1883
- Base SHA: 795d4157243f9ddb978c38d3a081b50bf9c0cac4

## Summary
The new operator command `evolve lessons backfill` reconstructs `.evolve/recurrence-ledger.json` from stored lesson files under `.evolve/instincts/lessons/*.yaml`. Dry-run execution prints pattern counts and lists skipped malformed files without disk writes. The `--write` flag replaces the recurrence ledger atomically with the backfilled result.

## Rationale
The core backfill logic in `recurrence.BackfillFromLessons` had no CLI entry point. A damaged or missing recurrence ledger lacked an automated repair mechanism. Adding `evolve lessons backfill` connects the existing core engine to the operator interface and guarantees repeatable ledger state reconstruction.

## Changed Areas
- `docs/operations/runtime-reference.md` — documents the `evolve lessons backfill` operator command syntax, flags, and exit codes.
- `go/cmd/evolve/cmd_lessons.go` — implements `runLessonsBackfill`, registers the `backfill` subcommand in `runLessons`, and handles command flags and exit codes without comments.
- `go/cmd/evolve/cmd_lessons_test.go` — adds unit test coverage for `runLessonsBackfill` argument validation, missing directory handling, dry-run mode, and ledger write mode without comments.

## Design Decisions
The backfill command defaults to dry-run mode to prevent accidental ledger overwrite. The `--write` flag replaces the existing ledger rather than merging to guarantee that stale entries are pruned. When the lessons directory is missing or unreadable, the command writes a diagnostic to standard error and exits with code 2. Zero comments are included in the new implementation per repository standards.

## Verification
Verification covers native unit tests in `go/cmd/evolve/cmd_lessons_test.go` and all 8 ACS cycle 1883 predicates in `go/acs/cycle1883/predicates_test.go`. The test suite confirms dry-run behavior, `--write` persistence parity with `evolve lessons recurrence`, skipped malformed file logging, exit code 2 on missing directory, exit code 10 on usage errors, documentation presence, and zero comment markers.

## Compatibility
Existing behavior of `evolve lessons recurrence` remains unchanged. The command continues reading from `EVOLVE_PROJECT_ROOT` when configured.

## Limitations
The backfill scan inspects local YAML files in `.evolve/instincts/lessons/` only and does not fetch lessons from remote repositories.
