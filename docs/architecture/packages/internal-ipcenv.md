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
- **`Markers` is the one list of the pipeline markers** (class (a) of [control-flags.md](../control-flags.md#classes-of-the-readers-outside-the-registry), X18b of [llm-isolation.md](../llm-isolation.md) §27). It holds 24 `EVOLVE_*` names and 9 `BRIDGE_*` names. Each consumer imports the list, and no consumer keeps a copy:
  - `CellMarkers` is the part that reaches an LLM cell. `bridge.Deps.cellEnv` passes only these names from the `EVOLVE_` space. A token, a dispatch tag and a launch argument stay out (`TestCellMarkers_AreTheMarkersThatReachAnLLMCell`).
  - `UnmovedControls` lists the bypasses (class (c)) that X18d has not moved yet. A pattern with `*` names a family, for example `EVOLVE_PHASE_*_BIN`. The list can only become shorter: `TestControlFlagLists_OnlyShrink` holds a ceiling of 10. X18c moved the 19 class (b) names out of it.
  - `RetiredControls` lists the class (b) names that X18c moved, each with its replacement. `RetiredInUse` returns each entry whose name is set with a value, once. `warnRetiredControls` of `cmd/evolve` prints one `[policy] WARN` for each at the cycle start. The table field is `Name`, not `key`, because the table holds no read.
  - `Carried` accepts a marker or a control that is not yet moved. `carriedEnv` of `cmd/evolve` uses it for the `EVOLVE_*` names, so a cycle gets no other `EVOLVE_*` name and no `BRIDGE_*` name of the operator shell.
  - The guard `TestNoControlFlagReadOutsideTheMarkerList` (`internal/guards/control_flags_test.go`) reads each non-test Go file. It finds each read of an environment name, folds the constant concatenations, and fails on a name outside these lists. It also fails on a declared control that no code reads, so a moved name leaves the list. Since X18e no test seam is allowed.
  - The list has no exported constants. An exported constant of this package must be in `ProtocolKeys`, and the new names are not lane protocol keys.
- **`DispatchIDKey`** (`EVOLVE_DISPATCH_ID`) is the tag of one dispatch. The bridge sets it in the environment of the CLI it starts, and each descendant inherits it. The bridge at dispatch end and `evolve gc` read it from the environment of other processes to prove ownership ([internal-proctree.md](internal-proctree.md)). No Go code reads it from its own environment.

## Invariants

- Every exported key constant is in `ProtocolKeys()`, and `Scrub` drops each (`TestProtocolKeys_AreEveryKeyThePackageDeclaresAndEachIsScrubbed`, `TestScrub_CoversEveryIPCKey`).
- `ProtocolKeys` never hands out its backing array (`TestProtocolKeys_ReturnsAFreshSlice`).
- `Markers` is class (a) of control-flags.md, and each protocol key is a marker (`TestMarkers_AreClassAOfTheControlFlagInventory`). `Markers`, `CellMarkers` and `UnmovedControls` give a fresh slice (`TestMarkerLists_ReturnFreshSlices`).
- `Carried` refuses an unlisted name such as `EVOLVE_BYPASS_SHIP_GATE`, and no control is also a marker (`TestCarried_IsAMarkerOrAControlNotYetMoved`, `TestUnmovedControls_AreWellFormedAndMatchNoMarker`).
- A retired name is not carried and matches no marker, and `RetiredControls` gives a fresh slice (`TestRetiredControls_AreNoLongerCarriedAndMatchNoMarker`). An empty or a valueless name gives no WARN (`TestRetiredInUse_NamesEachRetiredControlThatIsSetOnceWithItsReplacement`).
- The namespace rule needs no second list: every key lives under `EVOLVE_`, so a key added tomorrow is scrubbed the day it is added (`TestScrub_CoversEveryIPCKey`). Only the key is inspected (`TestScrub_DropsEveryEvolveKeyAndKeepsTheToolchain`).
- The width is read from the request environment and falls back to solo on garbage (`TestShipRecovery_ContentionBudgetScalesWithFleetWidth`). That `fleetWidthFromEnv` never reads `os.Getenv` has no test, so that half is unpinned.
- Each constant's value is the wire contract (`TestIpcenv_ConstValues`, `TestTmuxSocketKey_IsTheBridgeSocketChannel`).

## Findings

- **2026-10-01, ADR-0114 re-review**: `acs.predicate_env` was able to name a lane key (`EVOLVE_FLEET`, the tmux socket) and admit it again to each predicate after the scrub. The ACS suite needed the set of lane keys to refuse them. The tmux socket key was in `internal/bridge`. It moved here, and `ProtocolKeys` was added.
