# Build Explanation — Cycle 2376

## Build Binding
- Cycle: 2376
- Base SHA: 5da3551a32ee8caacaa852c5a6517f98bbda7744

## Summary
A test process now leaves no tmux server and no socket file when it ends. `evolve gc` now also stops the `-own` server of a dead test process.

## Rationale
tmux leaves the socket file after a signal stops the server, and the gc owner pattern ended at the pid. Both gaps are closed in the place that owns each fact, so no new mechanism is added.

## Changed Areas
- `go/internal/swarm/reap_orphans.go` — the owner pattern accepts an optional `-own` suffix. Gc lists and stops the `-own` server of a dead pid, and spares a live pid.
- `go/internal/tmuxtest/tmuxtest.go` — `stopFamily` stops the main server and each `<socket>-*` server and removes their socket files, and the orphan guard removes each socket file.

## Design Decisions
`Main` calls one function for the whole server family at a normal end. The guard script removes files itself, because it dies with the main server.

## Verification
The seven predicates of cycle 2376 pass. `internal/tmuxtest` has 100% statement coverage, and the swarm and gc suites pass.

## Compatibility
Socket names and the public functions are unchanged. The owner pattern only accepts more names.

## Limitations
If the server pid is unknown, the guard falls back to `kill-server`, which can leave the main socket file for gc to remove.
