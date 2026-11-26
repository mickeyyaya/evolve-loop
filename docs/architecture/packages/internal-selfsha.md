# internal/selfsha

## Purpose

`internal/selfsha` hashes a file (`Of`) or the running executable (`Running`). `Path` gives the real path of the running executable: `os.Executable`, then `filepath.EvalSymlinks`. It is the one home of this rule, and the boot refresh, the worktree guard link and the bridge cell `PATH` call it. The per-phase integrity design ([ADR-0065](../adr/0065-per-phase-binary-integrity.md)) hashes the running `evolve` binary at each phase boundary. This package is the one shared implementation.

## Design

- **A stdlib-only leaf with no internal imports**, so checkpoint, ship and core can all depend on it without an import cycle.
- `Of` is a read-only hasher with no shared state.
- `Path` fails loud. A resolver error or a dangling link gives an error and an empty path, never a guess.

## Invariants

- **Concurrent calls on one file return the same digest with no race.** The per-phase capture hashes the binary from each phase's goroutine. Run the test with `-race` to prove it.
