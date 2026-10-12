# Build Explanation — Cycle 1870

## Build Binding
- Cycle: 1870
- Base SHA: 117cbf6122246af6aa4f5a62c6b97d50be9be8c9

## Summary
A shared state.json reader in cyclestate replaces private JSON decoders in redteamcheck and wave. Wave merged PR extraction now counts GitHub squash-merged PRs.

## Rationale
Before this change, redteamcheck and wave used private JSON decoders for state.json. This duplicated decoding rules and risked state drift. In addition, wave inspected only commits matched by `--merges`. This missed GitHub squash-merged pull requests formatted as `(#N)` on the first-parent branch. Unifying state reading in cyclestate and expanding PR parsing resolves these problems cleanly.

## Changed Areas
- `go/cmd/evolve/cmd_wave.go` — adds squash PR regex matching trailing `(#N)` in `parseMergedPRs` and drops `--merges` from `gitMergedPRs` to inspect all first-parent commits.
- `go/internal/cyclestate/read.go` — provides shared `ReadState` and `ReadLastCycleNumber` helpers that decode state.json and handle missing/malformed files consistently.
- `go/internal/redteamcheck/redteamcheck.go` — removes the private `stateLastCycle` decoder and calls `cyclestate.ReadLastCycleNumber` in `NoBatchCycleJump`.
- `go/internal/wave/facts.go` — replaces ad-hoc state.json unmarshaling in `CycleFloor` with `cyclestate.ReadState`.

## Design Decisions
The shared state reading helpers are isolated in `go/internal/cyclestate/read.go` rather than modifying `state.go`, strictly preserving control-plane integrity under `ProtectedSurfaceManifest`. The PR parser supports both merge commits and squash merge commits without duplication when a merge subject also contains a parenthesized PR number.

## Verification
Verified with unit tests in `cyclestate/read_test.go`, `redteamcheck/redteamcheck_test.go`, `wave/facts_test.go`, `cmd_wave_test.go`, git integration tests in `cmd_wave_git_integration_test.go`, and the full ACS regression suite `evolve acs suite --cycle 1870` (172 green, 0 red).

## Compatibility
The JSON structure and semantics of `state.json` remain unchanged. `ReadState` and `ReadLastCycleNumber` are additive exports. `parseMergedPRs` preserves existing merge commit parsing while recognizing squash commits.

## Limitations
Only PR numbers formatted as trailing `(#N)` in commit subjects are recognized for squash merges, ignoring non-trailing references.
