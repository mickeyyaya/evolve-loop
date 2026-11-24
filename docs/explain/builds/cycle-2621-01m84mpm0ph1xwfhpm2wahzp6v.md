# Build Explanation — Cycle 2621

## Build Binding
- Cycle: 2621
- Base SHA: f652fc150adc04db2dc58becf3162c7c8778d883

## Summary
`convergence.Decide` now routes subjects past three blocks to `ActionRedesign` on the first pass and `ActionToOperator` after one redesign.
`cyclestate.State` holds the host-owned per-subject block and redesign counts in `BlockCounts`, which `evolve convergence decide` reads when `subject` is set.

## Rationale
Storing `BlockCounts` on `cyclestate.State` keeps the record in host-controlled `.evolve/state.json` across resumes while the role guard blocks seat writes to that file.
Tagging `Input.Blocks` and `Input.Redesigns` with `json:"-"` lets `convergence.Parse` reject seat-authored counts as unknown JSON fields without duplicating validation logic.

## Changed Areas
- `go/internal/convergence/convergence.go` — adds `ActionRedesign` and `ActionToOperator`, plus `Input.Subject` (`json:"subject,omitempty"`) and host-only `Input.Blocks` and `Input.Redesigns` (`json:"-"`).
- `go/internal/convergence/decide.go` — escalates strict-open rounds with more than three blocks to `ActionRedesign` or `ActionToOperator` while leaving clean landings and counts at or below three unchanged.
- `go/internal/convergence/reasons.go` — records `cause=blocks` in `CONVERGENCE_RUNG` when `Decide` returns `ActionRedesign` or `ActionToOperator`.
- `go/internal/cyclestate/state.go` — adds `BlockCount` and `State.BlockCounts` (`json:"blockCounts,omitempty"`) so host state persistence preserves per-subject counts across resumes.
- `go/cmd/evolve/cmd_convergence.go` — reads `BlockCounts[in.Subject]` from `.evolve/state.json` in `readConvergenceInput` and exits 2 if reading host state fails.

## Design Decisions
`choose()` checks `c.in.Blocks > maxBlocks` after `len(strict) == 0` so clean rounds still return `ActionLand` at any block count.
`escalate()` preserves `c.redesign(exit)` for `Decision.Redesign` and computes `LandRound` and `File` with `keepBest` and `openIDs` without altering `Blocks <= 3` behavior.
Policy keys `max_blocks` and `max_redesigns` remain outside `internal/policy` until their own item lands, so `decide.go` uses compiled limits of 3 blocks and 1 redesign.

## Verification
All six cycle-2621 ACS predicates pass (`TestC2621_001` through `TestC2621_006`, the last one in `go/acs/cycle2621/predicates_test.go`), along with unit tests in `internal/convergence`, `internal/cyclestate`, `internal/core`, `internal/adapters/storage`, and `cmd/evolve`.

## Compatibility
Inputs without `subject` or with three or fewer blocks produce identical `Decision` values and JSON output.

## Limitations
This change introduces the host record, decision actions, and CLI lookup; wiring failure events to increment `BlockCounts` and queue routing are deferred to R3–R5.
