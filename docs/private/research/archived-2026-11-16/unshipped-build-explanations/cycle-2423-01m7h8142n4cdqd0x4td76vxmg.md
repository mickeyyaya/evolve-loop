# Build Explanation — Cycle 2423

## Build Binding
- Cycle: 2423
- Base SHA: 38bc28bc639269d8e738b125443e9ce24a44a869

## Summary
The `SIGNAL_EXPECTED_ABSENT` warning from `absentEvent` now includes `cycle`, `phase`, `run_id`, and `started` in `Reason` and `Fields`. It also keeps `Event.Cycle`, `Event.Phase`, and `Event.RunID` set on the warning event.

## Rationale
When `phase.dispatched` timed out without `phase.outcome` or `phase.aborted`, the previous `Reason` omitted the cycle, phase, run id, and start time. Operators had to inspect raw event payloads to identify the stalled stage. Formatting those fields into `Reason` and `Fields` fixes this without configuration or schema changes.

## Changed Areas
- `go/internal/signalcenter/expected.go` — updated `absentEvent` to put `cycle`, `phase`, `run_id`, and `started` in `Fields` and `Reason`. Also updated `RegisterCode` and `Event.RunID` fallback.
- `docs/architecture/signal-center-design.md` — documented in §8.1 that the absent warning names `cycle`, `phase`, `run_id`, and started time. Wave warnings omit `cycle`.
- `docs/architecture/packages/internal-signalcenter.md` — updated the `ExpectedSignals` summary bullet to state that `SIGNAL_EXPECTED_ABSENT` names `cycle`, `phase`, `run_id`, and start time in reason and fields.
- `docs/architecture/signal-codes.md` — updated the `SIGNAL_EXPECTED_ABSENT` table entry to document `cycle`, `phase`, and `run_id` in `fields` and `reason` when present.

## Design Decisions
- Kept `fields["started"]` alongside `cycle`, `phase`, and `run_id` so existing stream consumers remain compatible while keeping field count within `MaxFields` (12).
- Omitted `cycle=` from `Reason` and `Fields` when `o.event.Cycle == 0` so cycle-less wave warnings do not emit a synthetic zero cycle.
- Resolved `runID` from `o.event.RunID` with fallback to `o.event.Fields["run_id"]` so both phase events and wave events populate `Event.RunID`, `Fields["run_id"]`, and `Reason` uniformly.

## Verification
- `cd go && gofmt -l .` produced no output.
- `cd go && go vet ./...` exited 0.
- `cd go && go test -count=1 -cover ./internal/signalcenter/...` passed with `100.0%` statement coverage.
- `cd go && go test -count=1 ./cmd/evolve -run TestWireExpectedSignals` passed, exercising the production wiring path.
- `cd go && go test -tags acs -count=1 ./acs/cycle2423` passed 12/12 predicates.
- `./go/bin/evolve docs ste-lint --strict docs/architecture/signal-center-design.md docs/architecture/packages/internal-signalcenter.md` reported 0 findings.

## Compatibility
Existing `Fields` keys (`pair`, `start_kind`, `end_kind`, `started`, `deadline_seconds`, `key`) and `Event` struct fields remain intact. No schema migration or policy change is required.

## Limitations
The `cycle` field is omitted when `Event.Cycle == 0` (such as wave-level starts that have no cycle number).
