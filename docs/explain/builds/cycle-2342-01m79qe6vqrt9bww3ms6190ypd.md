# Build Explanation — Cycle 2342

## Build Binding
- Cycle: 2342
- Base SHA: 76abf5b42b851c60f7ec22f7b9c0d0d11395814a

## Summary
The wave status headline, the cost line and the next wave goal now give one shipped count. A cycle that ships late counts in all three. A shipped flag with no landing record counts in none.

## Rationale
The headline counted the ships of the time window. The cost line counted the flag in run.json. A late ship fell outside the window, and a forged flag passed the cost line. One rule for each cycle removes the split.

## Changed Areas
- `go/internal/wave/read_wave.go` — sets the shipped state of each cycle from the ship window or from a landing record. It never uses the run.json flag. It also returns the landed list.
- `go/cmd/evolve/cmd_wave_status.go` — the headline takes the landed list.
- `go/cmd/evolve/cmd_wave_compose.go` — the next wave goal takes the landed list.
- `go/internal/wave/read_wave_test.go` — names the landed list in an assertion.
- `go/acs/cycle2342/predicates_test.go` — predicates with the fixture of wave 166.
- `.evolve/evals/wave-status-headline-counts-each-shipped-cycle.md` — permanent eval for both criteria.

## Design Decisions
The Ships list keeps the strict window, because the zero-ship halt needs it. Only the per-cycle state changes, so the zero-ship tests stay valid.

## Verification
Both predicates pass. The wave and cmd/evolve wave, compose, zero-ship and cost tests pass. The ACS suite reports red=0.

## Compatibility
The JSON shape is unchanged. A flag in run.json no longer makes a cycle count as shipped. A commit SHA in the dossier no longer makes a cycle count as shipped. Only the ship window or a complete landing record does.

## Limitations
A ship with no journal entry and no landing record counts as not shipped.
