# internal/ipcenv

## Purpose

`internal/ipcenv` is the one home of the lane protocol keys: the `EVOLVE_` environment keys a parent evolve process sets for its children (`FleetKey`, `FleetScopeKey`, `FleetWidthKey`, `WorktreeRootKey`, `CycleStateFileKey`, `TmuxSocketKey`). They are not operator dials, so they have no `flagregistry` row, and the `flagreaders` and `envtaint` regression gates skip this directory. It also owns `Scrub`, the environment every `go test` a cycle spawns to judge the repository runs under.

## Design

- **`ProtocolKeys` is the set.** It returns a fresh slice of every key constant the package declares. Consumers that must never pass a lane key on refuse this set: the ACS suite refuses any of them in `acs.predicate_env` ([ADR-0114](../adr/0114-the-acs-verdict-is-always-written-and-complete.md) decision 5). `TestProtocolKeys_AreEveryKeyThePackageDeclaresAndEachIsScrubbed` parses the package's own constants, so a key added here joins the set the day it is added.
- **`Scrub` drops the whole `EVOLVE_` namespace**, not the listed keys, so a key nobody listed (a new one, or one owned elsewhere) cannot leak either. Only the key is inspected; a value that mentions the namespace survives.
- **`TmuxSocketKey`** (`EVOLVE_TMUX_SOCKET`) is the per-run bridge tmux socket the loop exports to its bridge subprocesses. `bridge.TmuxSocketEnv` is defined as this constant, so the key has one spelling. It keeps its `flagregistry` row (status internal, "IPC channel, not an operator dial") for now; the row predates the constant moving here.

## Invariants

- Every exported key constant is in `ProtocolKeys()`, and `Scrub` drops each (`TestProtocolKeys_AreEveryKeyThePackageDeclaresAndEachIsScrubbed`, `TestScrub_CoversEveryIPCKey`).
- `ProtocolKeys` never hands out its backing array (`TestProtocolKeys_ReturnsAFreshSlice`).
- Each constant's value is the wire contract (`TestIpcenv_ConstValues`, `TestTmuxSocketKey_IsTheBridgeSocketChannel`).

## Findings

- **2026-10-01, ADR-0114 re-review**: `acs.predicate_env` could name a lane key (`EVOLVE_FLEET`, the tmux socket) and re-admit it to every predicate past the scrub. The ACS suite needed the lane key set to refuse them, and the tmux socket key lived in `internal/bridge`; it moved here and `ProtocolKeys` was added.
