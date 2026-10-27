# Control Flags Reference — `EVOLVE_*`

> The **[Generated Flag Index](#generated-flag-index)** below is projected from `go/internal/flagregistry` and regenerated with `evolve flags generate`. On 2026-10-27 the registry has **7 rows**. X18e deleted 5 rows (1 dead name and 4 test seams), and X18c deleted 13 rows of class (b) knobs. The registry is not the full inventory: [Classes of the readers outside the registry](#classes-of-the-readers-outside-the-registry) lists each name that production code reads. [ADR-0156](adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md) (I-16, component X18) removes the control flags. After X18, only the pipeline markers stay in the environment.
>
> The historical hand-maintained cluster tables were removed in the flag-consolidation campaign: they duplicated the registry and drifted stale (a `never_duplicate` violation). The campaign retires the scattered `os.Getenv` override surface — capabilities move into typed `.evolve/policy.json` config, profile SSOT, DI, or documented subprocess protocol. Background: `docs/research/flag-consolidation-campaign-2026-06-19.md`.

## Status Key

| Status | Meaning |
|--------|---------|
| active | Read in production code; do not remove without a deprecation window |
| internal | Set by the runner for subprocess injection (IPC/bootstrap); not operator-facing |
| deprecated | Still honored through a bridge; emits a stderr WARN; remove in a future cycle |

## Policy Config Blocks (non-flag, `.evolve/policy.json`)

Not every operator dial is an `EVOLVE_*` env flag — some are typed `.evolve/policy.json`
blocks (see [runtime-reference.md](../operations/runtime-reference.md) for the full table).
This section documents the closed-vocabulary fields those blocks use.

- `fleet.plan_source` (`.evolve/policy.json:fleet.plan_source`) — closed vocabulary: `triage` (the default, empty/absent maps here) or `manual`.
  - Any other value fails safe to `manual` (never passthrough, never the `triage` default). The system also surfaces a warning that names the rejected value.
  - Runtime meaning: `triage` is the ONLY value that arms native `evolve loop` wave dispatch (S2).
  - With `fleet.count>1`, `triage` makes each batch iteration a wave (one single-writer triage plan fanned out to disjoint lanes). `--max-cycles` counts waves, not lanes.
  - `manual` (or any fallback) keeps wave dispatch off. Multi-lane fan-out then happens only through an explicit `evolve fleet` invocation.
  - A wave whose triage plan commits zero lanes never consumes an iteration. It WARNs and falls through to the sequential path instead (D1 empty-plan guard).
- `fleet.landing` (`.evolve/policy.json:fleet.landing`) — closed vocabulary: `per-lane` (the default; empty/absent maps here) or `prefix-queue`.
  - Any other value fails safe to `per-lane` — **never** the composer. The system also surfaces a warning that names the rejected value. An operator typo must not silently route the main-push path through the unsoaked composer.
  - Runtime meaning: `per-lane` is today's behaviour. Every PASS lane independently fast-forward-merges and pushes main, and serializes on `.evolve/ship.lock`.
  - `prefix-queue` hands the SOLE main-push path to the single-writer prefix composer (`fleet.PrefixQueue`). PASS lanes enqueue instead of pushing.
  - The composer verifies composed queue PREFIXES (L1, L1+L2, L1+L2+L3) against the native gate set. The first failing prefix names the culprit positionally (Zuul NNFI), so the lanes behind it re-form without it. No bisection subsystem exists.
  - Window sizing is AIMD (start 3, +1 per green, halve on red, floor 1). Risk-tiered lanes take a solo slot. These are core / cross-cutting `TierIffy` lanes, and any lane that shares a touched file with one already composing.
  - Design: [ADR-0078](adr/0078-fleet-landing-prefix-queue.md). Implementation: `go/internal/fleet/prefixqueue.go`, `go/internal/phases/ship/planlanding.go`. Resolution: `go/internal/policy/policy.go`.
  - **Status (2026-10-09):** no production code calls the composer, so `prefix-queue` changes no behavior today. [ADR-0128](adr/0128-landing-queue-tiered-reverification.md) designs the durable queue that replaces it.
- `gates.repo_contract_gate` (`.evolve/policy.json:gates.repo_contract_gate`) — closed vocabulary: `enforce` (the default; empty/absent maps here) or `off`.
  - The system treats any other value as `enforce` and surfaces a warning that names the rejected value. A typo must not silently disable a red-main guard.
  - Runtime meaning: the ship-time repo-contract scanner pack. It holds the deterministic repo-wide guard suites that `repocontract.Packages()` lists. These are the catalog, profile and coherence scanners, the routing tests, and the whole-tree tests cheap enough for every ship.
  - `TestPackages_HoldEveryTestThatReadsTheWholeTree` requires each package whose tests read the tree to be in the pack or recorded outside it.
  - The pack runs in the lane worktree BEFORE the ship binds/pushes. A RED pack fails the ship closed with the dedicated `CodeRepoContractGate`. The lane FAILs honestly in place instead of redding main.
  - Per-cycle changed-scope testing structurally cannot catch this class. The suites scan repo-wide state (on-disk catalogs, tracked profiles, rendering parity) that a config-only diff never selects. Four live red-main incidents happened in the week of 2026-08-04.
  - The `enforce` default is a deliberate deviation from the shadow-first gate convention. The pack is existing deterministic repo tests with FP≈0 by construction (a failure here fails identically on main's next run).
  - The build handoff floor runs the same pack first (`core.RepoContractFloorChecks` over `ship.RunRepoContractPack`, 2026-09-29) and reads the same dial. `off` skips it there too, and any other word runs it. A named red goes back to the build correction instead of a failed ship.
  - The fixed pack runs only in evolve-loop's own module (`internal/repocontract.PackRuns`, one decision for ship and the floor). Elsewhere the system skips it with a note, and the backstops of the ship still run.
  - Implementation: `go/internal/phases/ship/repocontract.go`, `go/internal/core/repo_contract_floor.go`. Resolution: `go/internal/policy/policy.go` (`GatesConfig`).
- `gates.scan_cache_gate` (`.evolve/policy.json:gates.scan_cache_gate`) — closed vocabulary: `shadow` (the default; empty or absent maps here), `enforce` or `off`.
  - Any other value runs as `shadow`, and an unreadable policy gives the default.
  - The build floor and the ship scan key each package result. They write their greens to one verdict store, under the gate ids `build-floor` and `ship` (landing plan S5, [ADR-0134](adr/0134-fleet-landings-scale-through-content-keyed-verification-and-group-commit.md)).
  - At `shadow` the ship reads the store and signals the hits and the escapes. It runs every package and changes no verdict, exit code or order.
  - At `enforce` the ship skips each package with a green from any gate on the same key (plan [wave-latency-2026-10](../plans/wave-latency-2026-10.md) D6). The move to `enforce` needs one wave with 0 escapes and a security decision on the writers ([internal-testverdict.md](packages/internal-testverdict.md)).
  - `off` turns off both writers. Implementation: `go/internal/phases/ship/scanshadow.go` and `scanshadow_enforce.go`. Resolution: `policy.ScanCacheFor`.
- `gates.scan_cache_max_mb` and `gates.scan_cache_budget_seconds` (`.evolve/policy.json:gates.*`) — positive integers that bound the tree hash of the scan shadow. The compiled defaults are 2048 MB and 60 seconds, and a non-positive value takes the default. A tree over either bound makes the shadow degrade for that scan, with the reason in its summary. The ship scan never waits longer than the budget.
- `gc.testcache_ttl_days` and `gc.testcache_max_mb` (`.evolve/policy.json:gc.*`) — positive integers that bound the store of the scan shadow. The compiled defaults are 14 days and 256 MB, and a non-positive value takes the default. `evolve gc` removes the files past the age, then the oldest files until the store fits the cap. `evolve gc --dry-run` previews it. The ship path never trims the store.
- `boot.binary_refresh` (`.evolve/policy.json:boot.binary_refresh`) — closed vocabulary: `auto` (the default; empty/absent maps here) or `off`.
  - Any other value resolves to `auto`. The self-heal is integrity posture, so a typo must not silently disable it.
  - Runtime meaning: the boot-time binary staleness self-heal for the binary-lag class. In this class, fixes land on main but the loop keeps executing the binary it booted with. Cycles 1302–1309 ran a sentinel parser that the repo had already fixed at cycle-1301.
  - At boot, BEFORE recovery and the readiness gate, this check runs. When the running binary is behind plane HEAD by a delta that touches `go/`, the loop rebuilds with the canonical make target. The test uses the embedded build commit of the binary. Then the loop re-execs the fresh binary in place.
  - Every step is fail-open. Any failure WARNs and boots the old binary. A stale batch is yesterday's status quo, and a bricked loop is worse.
  - A consume-once marker file (`.evolve/boot-refresh-marker`) caps the heal at one try per target HEAD. A rebuild that does not change the stamp can therefore never re-exec forever.
  - `--resume` never refreshes (minimal-perturbation single-cycle protocol). The heal lands at the next fresh boot.
  - `off` is for deliberate old-binary pins (incident bisects).
  - Implementation: `go/cmd/evolve/cmd_loop_boot_refresh.go`. Resolution: `Policy.BootBinaryRefresh` (`go/internal/policy/policy.go`). Background: `docs/chronicle/2026-08-binary-lag.md`.

## Classes of the readers outside the registry

> Source: the audit of 2026-10-25 on `origin/main` at `603389ebd` ([research F5.1 to F5.6](../research/llm-isolation-2026-10.md#62-local-evidence-the-control-flags)). The fix of each class is component X18 ([ADR-0156](adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md) I-16, [spec §27](llm-isolation.md#27-no-ambient-control-flags-x18)). Until X18 lands, this list is the full inventory, and the generated index below is not.

The registry has 7 rows. The production code reads more names than that. Many names are built as `"EVOLVE_" + "X"`, so a literal grep misses them. Each reader was in one of five classes. Classes (d) and (e) are empty since X18e.

**(a) Pipeline markers.** The pipeline sets them for its own child processes. They stay, as one named Go list: `ipcenv.Markers()` in `go/internal/ipcenv/markers.go` (X18b). The list has 24 `EVOLVE_*` names and 9 `BRIDGE_*` names. Only its part `ipcenv.CellMarkers()` reaches an LLM cell.

| Group | Names | Reader |
|---|---|---|
| Roots | `EVOLVE_PROJECT_ROOT`, `EVOLVE_PLUGIN_ROOT` | many |
| Dispatch | `EVOLVE_DISPATCH_ID`, `EVOLVE_DISPATCH_DEPTH`, `EVOLVE_MSG_TOKEN`, `EVOLVE_PHASE` | `events/rootwire`, `subagent/recursion.go`, `agentmsg/boot`, `guards/phase.go` |
| Fleet and cycle (`ipcenv`) | `EVOLVE_FLEET`, `EVOLVE_FLEET_SCOPE`, `EVOLVE_FLEET_WIDTH`, `EVOLVE_FLEET_REACH_HOLDER`, `EVOLVE_WORKTREE_ROOT`, `EVOLVE_CYCLE_STATE_FILE`, `EVOLVE_TMUX_SOCKET` | `go/internal/ipcenv/ipcenv.go:5-18` |
| Fan-out worker | `EVOLVE_FANOUT_WORKER_TOKEN`, `EVOLVE_FANOUT_WORKER_NAME`, `EVOLVE_FANOUT_CYCLE`, `EVOLVE_FANOUT_WORKSPACE`, `EVOLVE_FANOUT_PARENT_AGENT`, `EVOLVE_FANOUT_WORKER_ARTIFACT`, `EVOLVE_FANOUT_CACHE_PREFIX_FILE` | `subagent/dispatchparallel.go:176-178`, `fanoutdispatch` |
| Protocol | `EVOLVE_BRIDGE_PIDFILE`, `EVOLVE_PHASE_RECOVERY_STAGE`, `EVOLVE_SHIP_RELEASE_NOTES`, `EVOLVE_CLI` | `bridge/engine.go:33`, `cli/phasecmd/phase_observer.go:155`, `ship/gitops.go:372`, `codeindex/usagelog/usagelog.go:28` |
| Bridge launch | `BRIDGE_CLI`, `BRIDGE_RUN_ID`, `BRIDGE_COMPLETION`, `BRIDGE_PERMISSION_MODE`, `BRIDGE_SESSION_NAME`, `BRIDGE_STREAM_OUTPUT`, `BRIDGE_REQUIRE_FULL`, `BRIDGE_REQUIRE_SANDBOX`, `BRIDGE_HUMAN_INPUT_FLAG` | `bridge/launch.go:275-297` |

**(b) Operator knobs.** They change the flow. X18c moved each one into a strict-decoded key of `.evolve/policy.json` or into a CLI parameter. The pipeline reads the key at each cycle start. No code reads a class (b) name from the environment now.

- `ipcenv.RetiredControls()` lists the former names.
- The cycle start prints one `[policy] WARN` for each retired entry with a set name. A family entry prints its pattern, for example `EVOLVE_*_CLI`. The WARN names the replacement.
- The WARN stays for one release.

| Former name | Replacement | Reader of the replacement |
|---|---|---|
| `EVOLVE_DYNAMIC_ROUTING`, `EVOLVE_ROUTING_MODE` | `router.dynamic_routing`, `router.routing_mode` | `config.Loader.ApplyRoutingKnobs`, through `router.ApplyPolicy` |
| `EVOLVE_MANDATORY_PHASES` | `mandatory_phases` (it adds phases to the spine; it cannot remove one) | `policy.Policy.MergeMandatory` |
| `EVOLVE_CONDITIONAL_MANDATORY`, `EVOLVE_MAX_OPTIONAL_INSERTIONS` | `router.conditional_mandatory` (a map of phase to expression), `router.max_optional_insertions` | `config.Loader.ApplyRoutingKnobs` |
| `EVOLVE_PHASE_IO` | `gates.phase_io` | `config.Loader.ApplyRoutingKnobs` |
| `EVOLVE_COMMIT_EVIDENCE` | none: no code consumed the stage, so X18c gave it no key | — |
| `EVOLVE_CLI_HEALTH` | `cli_health.enabled` (absent means on) | `bridgechain.CLIHealthOn`, `policy.CLIHealthConfig.On` |
| `EVOLVE_CLI_MAX_CONCURRENT_<FAM>` | `bridge.max_concurrent.<driver>` | `bridge.Deps.MaxConcurrent` |
| `EVOLVE_LANE` | the `--lane` parameter of `evolve worktree` | `runscope.ResolveLane` |
| `EVOLVE_INTENT_DELTA` | `workflow.intent_delta` | `intent.Config.DeltaMode` |
| `EVOLVE_<AGENT>_CLI`, `EVOLVE_<AGENT>_MODEL` | the `--cli <agent>=<driver>` and `--model <agent>=<tier>` parameters of `evolve loop`, or `pins.<phase>` | `cliroute.WithOverrides`, `llmroute.AgentOverride` |
| `EVOLVE_<AGENT>_SYSTEM_PROMPT`, `EVOLVE_SYSTEM_PROMPT` | `system_prompt` of the agent profile | `systemprompt.Resolve` |
| `EVOLVE_<AGENT>_PERMISSION_MODE` | `permission_mode` of the agent profile; `evolve plan-and-execute` sets `core.PhaseRequest.PermissionMode` | `phases/runner/routing.go` |
| `EVOLVE_PROFILE_DIR`, `EVOLVE_PERSONA_OVERRIDE` | the `--profile-dir` and `--persona-override` parameters of `evolve phases`; the ship lint reads `<root>/.evolve/profiles` | `phases/ship/commitgate.go` |
| `EVOLVE_GO_BIN` | `<root>/go/bin/evolve`, then `evolve` on `PATH` | `consensusdispatch`, `releasepipeline`, `rollback` |

**(c) Bypasses.** Each becomes an explicit CLI parameter that writes an audit record, or it goes (X18d).

| Names | Reader | Note |
|---|---|---|
| `EVOLVE_SANDBOX` | `config/env.go:19`, `looppreflight/looppreflight.go:240` | partly duplicates `sandbox.nested_fallback`; spec §7 refuses it in a phase |
| `EVOLVE_USE_PHASE_REGISTRY` | `config/config.go:47` | |
| `EVOLVE_OPERATOR_CONFIRM` | `continuation/authority.go:12` | the env form of `-operator` |
| `EVOLVE_SHIP_AUTO_CONFIRM` | `phases/ship/verify.go:183` | set for children by `releasepipeline` and `rollback` |
| `EVOLVE_PHASE_<X>_BIN` | `adapters/runner/runner.go:63-64` | |
| `BRIDGE_ALLOW_BYPASS`, `BRIDGE_DRY_RUN` | `bridge/launch.go:294-295` | |
| `BRIDGE_ALLOW_ANTHROPIC_BASE_URL` | `bridge/driver_claudetmux.go:28` | |
| `BRIDGE_ALLOW_OPENAI_API_KEY` | `bridge/driver_codextmux.go:26` | |
| `BRIDGE_HUMAN_SIMULATION` | `bridge/launch.go:48` | |

**(d) Dead names.** X18e deleted them, with their registry rows, their writers and the persona text that named them. No class (d) name is left. The plan row X18e in [llm-isolation-2026-10.md](../plans/llm-isolation-2026-10.md) lists each deleted name.

**(e) Test seams.** X18e moved each one behind an injected value. No production code reads a test seam from the environment:

| Former seam | The test sets now |
|---|---|
| the three directory overrides of `paths.Resolve` | the real roots `EVOLVE_PROJECT_ROOT` and `EVOLVE_PLUGIN_ROOT`, so each directory comes from the layout |
| the prompt directory override of `prompts.NewForProject` | an `agents/` directory in the project root |
| the binary overrides of `bridge.resolveBinary` | the field `bridge.Deps.Binaries` (CLI name to path); an E2E test puts the fake CLI first on `PATH` |
| the install and missing-tool switches of `evolve commit-gate run` | the fields `commitgate.Options.TestInstall` and `ForceMissing` |

`TestRemovedControlNames_HaveNoProductionRead` (`go/internal/guards/control_flags_test.go`) fails when production code reads one of these names again, however the code builds the name. `TestRetiredControls_HaveNoProductionRead` does the same for the class (b) names. `TestFlagRegistryRows_EachHasAProductionReader` fails on a registry row that no code reads.

The guard `TestNoControlFlagReadOutsideTheMarkerList` (`go/internal/guards/control_flags_test.go`) keeps this inventory true:

- It reads each non-test Go file and finds each read of an environment name. A read is a call of `os.Getenv`, `os.LookupEnv` or an `envchain` reader, or an index of a map whose variable or field name contains `env`. A function that passes its parameter to a read is a reader too.
- It folds a constant concatenation such as `"EVOLVE_" + "X"`, also through a constant of another package and a local variable. A part that is not constant becomes `*`, so `"EVOLVE_" + agent + "_CLI"` is `EVOLVE_*_CLI`.
- It fails on each `EVOLVE_*` or `BRIDGE_*` name that is not a marker or a control not yet moved. It also fails on a declared control that no code reads.
- `TestControlFlagLists_OnlyShrink` holds the ceiling of 10 controls not yet moved. X18c lowered it from 29.
- Limits: the guard finds a read by its form, not by its type.
  - A table entry counts as a read only when its field is `key`, as in `envDials` of `config/env.go`. The table of `ipcenv.RetiredControls` uses the field `Name`, because it holds no read.
  - A read whose name has no constant prefix is not judged.
  - An index of a map with another name, for example `vars["EVOLVE_X"]`, is not seen.
  - A scan of `os.Environ()` with `strings.HasPrefix` is not seen.
- The flag registry (`go/internal/flagregistry`) is the documented index of this file. `ipcenv` holds the classes that the code applies at run time. X18e adds the test that ties the two.

The paths in the tables are under `go/internal/` unless they start with `cmd/`. The names `EVOLVE_BYPASS_SHIP_GATE`, `EVOLVE_BYPASS_ROLE_GATE`, `EVOLVE_BYPASS_SHIP_VERIFY` and `EVOLVE_BYPASS_PREFIX_GATE` appear only in tests and in the CHANGELOG on `603389ebd`. They have no production reader, so they are not in a class.

<!-- GENERATED:flag-index BEGIN — do not edit by hand; run `evolve flags generate` -->

## Generated Flag Index

Complete flag index — generated from `go/internal/flagregistry` (SSOT). Edit the registry, then run `evolve flags generate`; do not edit this table by hand.

| Flag | Status | Kind | Default | Cluster | Purpose |
|------|--------|------|---------|---------|----------|
| `EVOLVE_CLI` | internal | — | — | — | Undocumented production reader (inventory 2026-06-11); classify when touched. |
| `EVOLVE_OPERATOR_CONFIRM` | internal | bool | unset (refuses) | Operator Authority | Non-interactive spelling of the `-operator` authority flag, which is the PRIMARY surface; the env path exists for a script or a session that cannot pass the flag (the EVOLVE_LANE precedent: flag primary, env retained for script compatibility). NOT a feature dial and NOT consolidatable into policy.json — it configures no behavior, it carries per-invocation CONSENT, and a persisted 'the operator agrees' value would permanently disarm the gate it guards, so the flag-reduction campaign metric (LiveFeatureFlags) correctly excludes it. Read by VALUE, never by mere presence (envchain.Bool): `1`/`true`/`yes`/`on` authorize; `0`, an unrecognized word, and an exported-but-empty value are all refusals, so a variable lingering in a lane's environment can never consent on the operator's behalf. Gates `evolve continuation release` (cycle-1684, ADR-0089): releasing a continuation binding erases the lineage the defect-ledger gate reads as anti-tamper evidence, and the phase guard denies in-process Agent/Task dispatch but says nothing about a Bash invocation of the evolve binary. |
| `EVOLVE_PLUGIN_ROOT` | active | — | — | Core Infrastructure (never consolidate) | Read-only plugin install location |
| `EVOLVE_PROJECT_ROOT` | active | — | — | Core Infrastructure (never consolidate) | Writable project directory (dual-root pattern) |
| `EVOLVE_SANDBOX` | active | — | — | Sandbox Cluster | Enable outer sandbox-exec/bwrap wrapper |
| `EVOLVE_TMUX_SOCKET` | internal | — | — | Bridge Cluster | IPC channel (F6, not an operator dial): the loop exports the per-run bridge tmux socket name (evolve-bridge-p<looppid>) so its bridge subprocesses, the reaper, and the orphan GC all target the same isolated socket — so an external `tmux kill-server` on one run's socket can't tear down another. Unset ⇒ the shared `evolve-bridge` default. Set by the loop (cmd_loop) and, for a test process that runs real tmux, by `tmuxtest.Main` (`evolve-bridge-t<testpid>`, a server of its own); never by the operator. |
| `EVOLVE_USE_PHASE_REGISTRY` | active | — | — | Dynamic Phase Routing (Go-native, v13.0.0 / PR #4 — default-off) | Set `0` to skip reading `phase-registry.json` (built-in defaults only) |

<!-- GENERATED:flag-index END -->
