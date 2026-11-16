# Build Explanation — Cycle 2392

## Build Binding
- Cycle: 2392
- Base SHA: cc1f0c2b72f5fbcdc6e473e0a3a17d6a23117f9a

## Summary
`SIGNAL_EXPECTED_ABSENT` warnings from `absentEvent` now name `cycle`, `phase`, `run_id`, and `started` in `Reason` and `Fields`. `Event.Cycle`, `Event.Phase`, and `Event.RunID` stay populated.

## Rationale
Operators reading `signals.ndjson` or stderr output must identify which cycle, phase, run, and start time timed out. Updating `absentEvent` surfaces that identity on live and persisted warnings.

## Changed Areas
- `go/internal/signalcenter/expected.go` — updates `absentEvent` to populate `cycle`, `phase`, `run_id`, and `started` in `Fields` and `Reason`. Also updates `CodeExpectedAbsent` registration documentation.
- `go/internal/signalcenter/expected_test.go` — adds unit tests covering phase absent warnings and cycle-less wave absent warnings to maintain 100% statement coverage on `expected.go`.
- `docs/architecture/signal-center-design.md` — documents in section 8.1 that `SIGNAL_EXPECTED_ABSENT` names `cycle`, `phase`, `run_id`, and `started` time in `Reason` and `Fields`, and that cycle-less wave warnings omit `cycle`.
- `docs/architecture/packages/internal-signalcenter.md` — documents `ExpectedSignals` and the `SIGNAL_EXPECTED_ABSENT` identity fields in STE.
- `docs/architecture/signal-codes.md` — updates the generated `SIGNAL_EXPECTED_ABSENT` meaning row to match `RegisterCode`.

## Design Decisions
- Kept the existing `started` field key rather than adding a duplicate `started_at` key so `signal-codes.md` and existing consumers retain one canonical timestamp field.
- Omitted `cycle` from `Fields` and `Reason` when `o.event.Cycle == 0` so cycle-less wave warnings never print `cycle=0`.
- Fell back to `o.event.Fields["run_id"]` when `o.event.RunID` is empty so wave starts carry their `run_id` on both `Event.RunID` and `Fields["run_id"]`.

## Verification
`go test -count=1 ./internal/signalcenter`, `go test -count=1 ./cmd/evolve -run TestSignal`, `go test -tags acs -count=1 ./acs/cycle2392`, and `evolve docs ste-lint --strict` on the updated docs all pass.

## Compatibility
Existing `Fields` keys (`pair`, `start_kind`, `end_kind`, `started`, `deadline_seconds`, `key`) and `Event` struct fields remain unchanged; `cycle`, `phase`, and `run_id` are additive.

## Limitations
Only `ExpectedSignals.expire` (`absentEvent`) is modified; other `signalcenter` event producers already populate their own `Reason` and `Fields`.
