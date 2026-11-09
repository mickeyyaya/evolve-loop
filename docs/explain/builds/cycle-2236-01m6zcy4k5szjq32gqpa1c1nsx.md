# Build Explanation — Cycle 2236

## Build Binding
- Cycle: 2236
- Base SHA: fe9550c223267e2317b1e5b14b9f11b2a20ae788

## Summary
The garbage collection pipeline sweeps stale project session folders under the Claude Code seat directory. The isolation plan and package documentation record the readers.

## Rationale
Claude Code seats write session transcripts to `projects/<worktree>` under the seat root (`paths.SeatRoot`). When worktrees are removed, these session folders accumulate on disk. A sweep during `evolve gc` reclaims disk space. The sweep preserves live seats, foreign projects, symlinks, the token holder, and settings.

## Changed Areas
- `go/internal/gc/seatsessions.go` — exports `ReapSeatDirectory` and extracts shared session folder sweep logic.
- `go/internal/gc/seatsessions_test.go` — adds unit tests for `ReapSeatDirectory`.
- `go/cmd/evolve/cmd_gc.go` — wires the `seatDirectory` sweep into the `evolve gc` step pipeline.
- `docs/plans/seat-login-isolation-2026-11.md` — marks row L6 as built and updates step text.
- `docs/architecture/packages/internal-gc.md` — documents `ReapSeatDirectory` and why seat directory state is owned.

## Design Decisions
`ReapSeatDirectory` reuses the safety criteria of seat session sweeps. It matches the worktree prefix, checks for live worktrees, skips symlinks, and checks the cutoff age. It operates under `projects/` and never mutates credentials or settings in the seat root.

## Verification
ACS predicates TestC2236_001 through TestC2236_008 verify the readers, GC cleanup, dry-run previews, and documentation. Unit tests in `internal/gc` verify `ReapSeatDirectory` directly.

## Compatibility
Existing GC configurations continue to work without schema changes. Hosts without a seat directory report zero items and create nothing.

## Limitations
The sweep targets project folders that match the worktree naming convention. Foreign project directories stay preserved.
