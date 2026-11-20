# Build Explanation — Cycle 2556

## Build Binding
- Cycle: 2556
- Base SHA: f25d49e4ae89c9d1c528d1c4fbc1687743635999

## Summary
This build fixes four test hygiene defects in `go/internal/phaseobserver` and updates agent-based file names in `docs/architecture/phase-observer.md`.

## Rationale
Tightening the existing tests and updating the architecture document removes silent passes without edits to production code in `go/internal/phaseobserver/phaseobserver.go`.

## Changed Areas
- `docs/architecture/phase-observer.md` — updates `{phase}-observer-report.json`, `{phase}-observer-events.ndjson`, and `{phase}-stdout.log` to `{agent}-*` so the document matches `observerengine.PathsFor(cfg.Workspace, cfg.Agent)` in `go/internal/phaseobserver/phaseobserver.go`.
- `go/internal/phaseobserver/maxnoprogress_test.go` — fails with `t.Fatalf` when `builder-observer-events.ndjson` is absent in `TestRun_MaxNoProgress_ToolUsingAgent_DoesNotFire` and `TestRun_MaxNoProgress_Disabled_IsLegacyByteIdentical`.
- `go/internal/phaseobserver/phaseobserver_test.go` — checks `os.ReadFile` and `json.Unmarshal` errors instead of discarding them in `TestRun_WritesShutdownReport` and `TestRun_TailsStdoutLog`.
- `go/internal/phaseobserver/order_pins_test.go` — removes the unused `seen` field from `killRecorder`.
- `go/internal/phaseobserver/process_dead_test.go` — replaces the stale `RED (cycle-274)` failure message with an expectation-focused `t.Fatalf` message.

## Design Decisions
The build keeps `go/internal/phaseobserver/phaseobserver.go` untouched and fixes each defect in the test files and specification document. Changing production paths to match `{phase}-*` breaks the agent-keyed subcommand contract and touches a guard-protected file, so the document aligns with the code instead.

## Verification
The `internal/phaseobserver` suite, `internal/testaudit` suite, `acs/cycle2556` predicates, and `evolve docs ste-lint --strict` all verify the changes.

## Compatibility
No public API, CLI flag, or runtime output format changes.

## Limitations
The `core_adapter` live loop observer still keys files by phase in `internal/adapters/observer`, which `docs/architecture/packages/internal-adapters-observer.md` covers separately.
