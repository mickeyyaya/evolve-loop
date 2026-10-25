# internal/ipcenv

## Purpose

`internal/ipcenv` is the one home of the lane protocol keys. These are the `EVOLVE_` environment keys that a parent evolve process sets for its children (`ProtocolKeys` lists them). They are not operator dials, so they have no `flagregistry` row, and the `flagreaders` and `envtaint` regression gates skip this directory. It also owns `Scrub`, the environment every `go test` a cycle spawns to judge the repository runs under.

## Design

- **`ProtocolKeys` is the set.** It returns a fresh slice of every key constant the package declares. Consumers that must never pass a lane key on refuse this set: the ACS suite refuses any of them in `acs.predicate_env` ([ADR-0114](../adr/0114-the-acs-verdict-is-always-written-and-complete.md) decision 5). `TestProtocolKeys_AreEveryKeyThePackageDeclaresAndEachIsScrubbed` parses the package's own constants, so a key added here joins the set the day it is added.
- **`Scrub` drops the whole `EVOLVE_` namespace**, not the listed keys, so a key nobody listed (a new one, or one owned elsewhere) cannot leak either. Only the key is inspected; a value that mentions the namespace survives (`FOO=EVOLVE_BAR` stays). Order is kept.
- **Why `Scrub` exists.** It gives the environment for any `go test` or `go run` that a cycle spawns to judge the repository. Examples are the ship repo-contract gate, the build floor and the phase-bindings self-check.
  - A lane exports its runtime state for the whole process (`EVOLVE_FLEET=1`, its own `EVOLVE_CYCLE_STATE_FILE`, `EVOLVE_TMUX_SOCKET`).
  - A child that inherits that state turns env-sensitive tests into false REDs. They pass in CI and in a clean shell. Examples: the "outside a cycle" tests of the guards, the lease fencing of cycle reset, and the fleet-off goldens of ship.
  - Everything outside the namespace (`PATH`, `HOME`, `GOFLAGS`, `GOTOOLCHAIN`) stays, so the toolchain still works.
  - A test that needs an `EVOLVE_` key sets it with `t.Setenv` inside the child, after the scrub.
- **`FleetWidthKey`** (`EVOLVE_FLEET_WIDTH`) advertises the effective lane width of the fleet supervisor to each launched cycle.
- **`FleetReachHolderKey`** (`EVOLVE_FLEET_REACH_HOLDER`) is set for a lane that only the optimistic admission rule admitted (ADR-0143 W1b). It names the lane in flight that the import-reach rule holds it behind. The orchestrator pool sets it, and the lane process reads it to report a composed-gate failure or a rebase conflict.
  - The orchestrator scales the ship-recovery budget of a contention-class error (`shipRecoveryBudget`: the larger of 2 and width plus 1). The width is the number of siblings that race for main.
  - `core.fleetWidthFromEnv` reads it from `CycleRequest.Env`, never from `os.Getenv`. So fleet siblings cannot leak their width into each other.
  - An absent, garbage or non-positive value means solo (1).
  - `lanebudget.Current` reads it from `os.Getenv`, because the budget belongs to the process that runs the `go test`. Each lane is a separate process, so it reads its own width ([internal-lanebudget.md](internal-lanebudget.md)).
- **`CycleStateFileKey`** (`EVOLVE_CYCLE_STATE_FILE`) names the own per-run cycle-state file of a fleet lane (`runs/cycle-N/cycle-state.json`). So two concurrent lanes never share `<evolveDir>/cycle-state.json`. It applies only to the evolve directory that holds it (`paths.CycleStateFileFor`).
- **`TmuxSocketKey`** (`EVOLVE_TMUX_SOCKET`) is the per-run bridge tmux socket the loop exports to its bridge subprocesses. `bridge.TmuxSocketEnv` is defined as this constant, so the key has one spelling. It keeps its `flagregistry` row (status internal, "IPC channel, not an operator dial") for now; the row predates the constant moving here.

- **`MsgTokenKey`** (`EVOLVE_MSG_TOKEN`) is the message token of one dispatch ([internal-agentmsg.md](internal-agentmsg.md)). `boot.Env` puts it in the environment of the dispatch. `Scrub` never forwards it.
- **`DispatchDepthKey`** (`EVOLVE_DISPATCH_DEPTH`) and **`PhaseKey`** (`EVOLVE_PHASE`) are the markers of an agent cell ([llm-isolation.md](../llm-isolation.md) §7.1). The bridge sets both on each LLM child. `DispatchDepth` reads the depth and gives 0 for an absent, junk or negative value. `subagent.ReadDispatchDepth` calls it.
- **`DispatchIDKey`** (`EVOLVE_DISPATCH_ID`) is the tag of one dispatch. The bridge sets it in the environment of the CLI it starts, and each descendant inherits it. The bridge at dispatch end and `evolve gc` read it from the environment of other processes to prove ownership ([internal-proctree.md](internal-proctree.md)). No Go code reads it from its own environment.

## Invariants

- Every exported key constant is in `ProtocolKeys()`, and `Scrub` drops each (`TestProtocolKeys_AreEveryKeyThePackageDeclaresAndEachIsScrubbed`, `TestScrub_CoversEveryIPCKey`).
- `ProtocolKeys` never hands out its backing array (`TestProtocolKeys_ReturnsAFreshSlice`).
- The namespace rule needs no second list: every key lives under `EVOLVE_`, so a key added tomorrow is scrubbed the day it is added (`TestScrub_CoversEveryIPCKey`). Only the key is inspected (`TestScrub_DropsEveryEvolveKeyAndKeepsTheToolchain`).
- The width is read from the request environment and falls back to solo on garbage (`TestShipRecovery_ContentionBudgetScalesWithFleetWidth`). That `fleetWidthFromEnv` never reads `os.Getenv` has no test, so that half is unpinned.
- Each constant's value is the wire contract (`TestIpcenv_ConstValues`, `TestTmuxSocketKey_IsTheBridgeSocketChannel`).

## Findings

- **2026-10-01, ADR-0114 re-review**: `acs.predicate_env` could name a lane key (`EVOLVE_FLEET`, the tmux socket) and re-admit it to every predicate past the scrub. The ACS suite needed the lane key set to refuse them, and the tmux socket key lived in `internal/bridge`; it moved here and `ProtocolKeys` was added.
