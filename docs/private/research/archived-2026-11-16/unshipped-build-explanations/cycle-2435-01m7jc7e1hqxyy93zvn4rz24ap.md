# Build Explanation — Cycle 2435

## Build Binding
- Cycle: 2435
- Base SHA: cecc133bace0e76cc2edf17bfc070ec09b618cb4

## Summary
The `SIGNAL_EXPECTED_ABSENT` warning from `absentEvent` in `go/internal/signalcenter/expected.go` now includes `cycle`, `phase`, `run_id`, and `started` in `Reason` and `Fields`. It also keeps `Event.Cycle`, `Event.Phase`, and `Event.RunID` set on the warning event. The design doc, package note, and generated signal code catalog document this behavior in ASD-STE100.

## Rationale
Previously, when `phase.dispatched` missed its `phase.outcome` or `phase.aborted` deadline, `absentEvent` omitted `cycle`, `phase`, and `run_id` from `Reason` and `Fields`. Operators who read the warning line can now see which stage stopped without inspecting the start event. Formatting those fields into `Reason` and `Fields` fixes this gap with no schema or policy change.

## Changed Areas
- `go/internal/signalcenter/expected.go` — updated `absentEvent` (`go/internal/signalcenter/expected.go:163-195`) to populate `cycle`, `phase`, `run_id`, and `started` in `Fields` and `Reason`. It omits `cycle` when zero, resolves `runID` from `Event.RunID` or `Fields["run_id"]`, and updates `RegisterCode` (`go/internal/signalcenter/expected.go:15-17`).
- `docs/architecture/signal-center-design.md` — added a §8.1 bullet (`docs/architecture/signal-center-design.md:484`) that documents the absent warning identity fields. It also notes that wave warnings omit `cycle`.
- `docs/architecture/packages/internal-signalcenter.md` — updated the `ExpectedSignals` bullet under `## Design` (`docs/architecture/packages/internal-signalcenter.md:16`) to name `cycle`, `phase`, `run_id`, and start time.
- `docs/architecture/signal-codes.md` — regenerated the `SIGNAL_EXPECTED_ABSENT` catalog row (`docs/architecture/signal-codes.md:546`) with `evolve signals codes generate`.

## Design Decisions
- Kept `fields["started"]` as `time.RFC3339Nano` UTC alongside `cycle`, `phase`, and `run_id`. Existing consumers stay compatible, and the field count stays within `MaxFields` (9 of 12).
- Omitted `cycle` from both `Reason` and `Fields` when `o.event.Cycle == 0`. Cycle-less wave warnings do not emit a synthetic zero cycle.
- Resolved `runID` with fallback to `o.event.Fields["run_id"]`. Both phase dispatches and wave starts populate `Event.RunID`, `Fields["run_id"]`, and `Reason` uniformly.

## Verification
- `cd go && gofmt -l ./internal/signalcenter ./acs/cycle2435` exited 0 with no output.
- `cd go && go vet ./internal/signalcenter` exited 0.
- `cd go && go test -count=1 -coverprofile=... ./internal/signalcenter` passed with `100.0%` statement coverage on every function in `expected.go`.
- `cd go && go test -count=1 ./cmd/evolve -run TestWireExpectedSignals` passed all 7 wiring tests exercising `wireExpectedSignals` → `ExpectedSignals.Poll` → `expire` → `absentEvent`.
- `cd go && go test -tags acs -count=1 -v ./acs/cycle2435` passed 12/12 predicates (`TestC2435_001` through `TestC2435_012`).
- `./go/bin/evolve docs ste-lint --strict docs/architecture/signal-center-design.md docs/architecture/packages/internal-signalcenter.md docs/architecture/signal-codes.md` reported `0 finding(s)`.

## Compatibility
Existing `Event` struct fields and `Fields` keys (`pair`, `start_kind`, `end_kind`, `started`, `deadline_seconds`, `key`) remain intact. No schema migration or policy change is required.

## Limitations
Wave-level or process-level start events with `Cycle == 0` omit `cycle` from `Reason` and `Fields` rather than synthesizing a zero cycle identifier.
