# Build Explanation — Cycle 2664

## Build Binding
- Cycle: 2664
- Base SHA: 385a332d8b44739b65e14088b540a2b68988014f

## Summary
`evolve wave watch` now evaluates wave stall readings through `wavehealth.Evaluate` on each live poll. It emits a `stall` event once for each standing trigger and kind. Each wake names its trigger, its lanes, and its evidence paths. `docs/operations/wave-operations.md` now documents the zero-ship first look.

## Rationale
`waveWatcher.poll` previously emitted only cycle diff events and pool signals. A zero-ship wave with repeated abnormal events or dense lane stalls stayed silent on the watch stream. Reusing `wavehealth.Evaluate` and `policy.WaveCheckinSettings` keeps the thresholds and rules of A2, A4, and A5 in one home.

## Changed Areas
- `go/internal/wave/events.go` — adds `EventStall` (`"stall"`), optional JSON fields `Trigger`, `Lanes`, and `Evidence` on `Event`, and single-line text formatting for stall wakes.
- `go/internal/wave/pool.go` — adds `OpenLanes` to return sorted unexited lane names from the wave run's pool signals for `no-ship` wakes.
- `go/internal/wave/events_test.go` — adds `EventStall` vocabulary, text formatting, and JSON round-trip tests.
- `go/internal/wave/pool_lanes_test.go` — adds `TestOpenLanes_ReturnsSortedUnexitedLanesOfTheWaveRun` covering `wave.OpenLanes`.
- `go/internal/wavehealth/stalls.go` — adds `StallTrigger` and `StallFindings` to extract `no-ship` (A2), `abnormal-kind` (A4), and `lane-stalls` (A5) triggers from `Evaluate` results with cycle IDs and still-silent lane names.
- `go/internal/wavehealth/evaluate_test.go` — adds `TestStallFindings_ExtractsOnlyBrakingTriggersWithDetails` covering all three triggers and held-brake suppression.
- `go/cmd/evolve/cmd_wave_checkin.go` — extracts `populateWaveHealthSnapshot` from `checkinGatherer.withWave` so both `wave checkin` and `wave watch` build the wave, stall-line, and abnormal-event snapshot identically.
- `go/cmd/evolve/cmd_wave_status.go` — adds `waveWatcher.pollStalls`, trigger/kind deduplication (`stallsSeen`), reset on `followNewerWave`, and evidence path resolution for `.evolve/signals.ndjson` and per-cycle `abnormal-events.jsonl`.
- `go/cmd/evolve/cmd_wave_pool_test.go` — adds unit tests for single-emission deduplication, reset on a newer wave, and warn-once handling of corrupt `abnormal-events.jsonl`.
- `docs/operations/wave-operations.md` — adds the `### Zero-ship first look` subsection under section 2 naming `evolve wave watch`, `LOOP_ORCH_LANE_STALLED`, and `abnormal-events.jsonl`.

## Design Decisions
Calling `wavehealth.Evaluate` and `wavehealth.StallFindings` from `waveWatcher.poll` preserves the single-home rule of `internal/wavehealth`. It uses `wave.checkin` policy bounds without adding a second config block or a shadow stage that suppresses wakes under default policy. Deduplicating by trigger and abnormal kind for each wave prevents repeat wakes across polls. A new wave or a distinct shared-fault kind still wakes once.

## Verification
`gofmt -l`, `go vet`, and `evolve docs ste-lint --strict` pass with zero findings. Package tests in `go/internal/wave`, `go/internal/wavehealth`, and `go/cmd/evolve` verify all three triggers, deduplication, and policy bounds. All seven cycle 2664 ACS predicates in `go/acs/cycle2664` (`TestC2664_001` through `TestC2664_007`) pass (`7/7 PASS`).

## Compatibility
`wave.Event` adds `trigger`, `lanes`, and `evidence` with `omitempty` JSON tags, leaving existing watch event payloads unchanged. `wave checkin` JSON and text outputs remain unchanged because `StallFindings` is a separate helper rather than an added field on `wavehealth.Report`.

## Limitations
`waveWatcher.pollStalls` reads `abnormal-events.jsonl` and `signals.ndjson` when the loop is live. Once the loop exits or the brake is held, no new stall wakes are emitted.
