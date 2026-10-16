# internal/selfsha

## Purpose

`internal/selfsha` hashes a file (`Of`) or the running executable (`Running`, resolved through `os.Executable`). The per-phase integrity design ([ADR-0065](../adr/0065-per-phase-binary-integrity.md)) hashes the running `evolve` binary at each phase boundary. This package is the one shared implementation.

## Design

- **A stdlib-only leaf with no internal imports**, so checkpoint, ship and core can all depend on it without an import cycle.
- `Of` is a read-only hasher with no shared state.

## Invariants

- **Concurrent calls on one file return the same digest with no race.** The per-phase capture hashes the binary from each phase's goroutine. Run the test with `-race` to prove it.
