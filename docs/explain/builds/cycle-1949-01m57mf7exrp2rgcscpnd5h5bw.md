# Build Explanation — Cycle 1949

## Build Binding
- Cycle: 1949
- Base SHA: 383c0f92f7ca510a5123b35ed93174b42706cf9e

## Summary
Five LOW follow-ups of the `evolve wave` review are closed. The `--json` flag comes from the parser, the record save goes through one seam, the old wave closes after the new record is saved, an unknown process start time gives one WARN, and the ledger seal test builds its own fixture.

## Rationale
Each item was a small gap between the code and its stated contract. Each fix reuses an existing structure: the parsed args, the `waveEnv` seam, the watcher `seen` map and the `seedLedger` helper.

## Changed Areas
- `go/cmd/evolve/cmd_wave.go` — adds the `saveRecord` seam to `waveEnv`; `waveOwnsPID` and `waveLoopLive` return a warning when the start time is unknown.
- `go/cmd/evolve/cmd_wave_next.go` — parses the args once in `runWaveNext`, so the JSON mode follows the parser; saves the new record before the old wave closes.
- `go/cmd/evolve/cmd_wave_status.go` — status adds the start-time warning; the watcher prints it once through `warnOnce`.
- `go/cmd/evolve/cmd_wave_test.go` — the harness gets a `save` hook for the new seam.
- `go/cmd/evolve/cmd_wave_followups_test.go` — tests the five items.
- `go/internal/adapters/ledger/seal_test.go` — `TestSeal_RealLedgerCopy` uses `seedLedger` and never skips.
- `go/acs/cycle1949/predicates_test.go` — acceptance predicates for the five items.
- `.evolve/evals/wave-cli-review-followups.md` — eval graders for the task.
- `docs/operations/wave-operations.md` — exit 2 row says the old wave stays open.

## Design Decisions
The notes are still consumed before the save, because the loop already runs with them. A failed save leaves the old wave open, so the next `wave status` still shows a wave that owns the loop.

## Verification
`go test -count=1 ./cmd/evolve ./internal/adapters/ledger` and the cycle 1949 ACS predicates pass, except one unrelated test (see the build report).

## Compatibility
No CLI flag and no JSON field changes. A usage error with `--json` still prints the envelope.

## Limitations
An unknown start time still counts the pid as the loop; it only warns.
