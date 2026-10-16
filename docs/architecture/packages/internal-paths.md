# internal/paths

## Purpose

`internal/paths` resolves the canonical `.evolve/*` filesystem layout that the whole Go pipeline honors. One `Layout` value is built at the entry of a command. It is passed by value to each call site that needs a path. It replaced an env-resolution block that was copied three times in the subagent command, and about ten inline `filepath.Join(..., ".evolve", ...)` strings.

## Design

- **Resolution order per field**, highest priority first:
  - the matching `EVOLVE_*_OVERRIDE` env var;
  - a path derived from `PluginRoot` (profiles, adapters, capability) or `ProjectRoot` (state, ledger);
  - the cwd, when the env is empty (`ProjectRoot` only).
- **Roots.** `ProjectRoot` is the project directory of the user, the parent of `.evolve/`. It comes from `EVOLVE_PROJECT_ROOT` or the cwd. `PluginRoot` is where the plugin is installed. It equals `ProjectRoot` in a checkout and differs for an installed plugin. It comes from `EVOLVE_PLUGIN_ROOT`, else from `ProjectRoot`.
- **Fields.**
  - `EvolveDir` is `<ProjectRoot>/.evolve`.
  - `StateFile` is `<EvolveDir>/state.json`, the persistent batch-scope state.
  - `CycleStateFile` is `<EvolveDir>/cycle-state.json`, the transient per-cycle state.
  - `LedgerFile` is `<EvolveDir>/ledger.jsonl`, the hash-chained audit trail (override `EVOLVE_LEDGER_OVERRIDE`).
  - `ProfilesDir` is `<PluginRoot>/.evolve/profiles` (override `EVOLVE_PROFILES_DIR_OVERRIDE`).
  - `AdaptersDir` is `<PluginRoot>/adapters` (override `EVOLVE_ADAPTERS_DIR_OVERRIDE`).
- **`Resolve` takes the env lookup and the cwd as parameters.** Production passes `os.Getenv`, or calls `ResolveFromEnv`. `ResolveFromEnv` resolves a failing `os.Getwd` to `""` to keep the signature pure. A caller that must surface a cwd failure calls `Resolve` with an explicit cwd.
- **Single homes.** `EvolveDirOf` is the intended home of the `<root>/.evolve` join. Other call sites in `cmd/` and `internal/` still spell the join inline. They migrate here when they are touched.
- **`PolicyPath` and `LoopStopPath`.** `PolicyPath` is the home of the policy file, which the loop wave and chain engines read. `LoopStopPath` is the home of the brake marker that `evolve loop-stop` writes.
- **`RunWorkspace` is the one home of `<root>/.evolve/runs/cycle-<N>`** ([ADR-0103](../adr/0103-component-breakdown-program.md) unit 09). `core.RunWorkspacePath` projects it for the orchestrator. The defect ledger sits below `core` and reads it directly.
- **`CycleStateFileFor` is the one rule for the cycle-state file of an evolve dir.** A fleet lane names its own per-run file in its evolve dir through `ipcenv.CycleStateFileKey`. Each process that the lane spawns inherits it, test binaries included. Thus the override applies only to the evolve dir that holds it. The override is an absolute path, and a relative one counts as outside.
- **Test seam.** `absFn` lets tests reach the `filepath.Abs` error branch. That call fails only when `os.Getwd` fails, for example when the cwd is deleted or unmounted.

## Invariants

- **`CapabilityDir` never honors `EVOLVE_ADAPTERS_DIR_OVERRIDE`.** It mirrors the bash `REAL_ADAPTERS_DIR` invariant. Capability manifests thus reflect the real install and not a test sentinel dir. `TestResolve_CapabilityDirAlwaysPluginRoot` pins this.
- **`Resolve` is total.** It never returns an error and never reads the filesystem. The cycle-state scope check makes a relative evolve dir absolute against the process cwd. The caller checks existence and permissions.
- **A `Layout` is bound to the cwd at construction.** When the cwd is empty and no override is set, relative fields capture the working directory at that time. A caller must not call `os.Chdir` between `Resolve` and a filesystem write that uses a `Layout` field. If it does, the write lands relative to the new cwd.
- **A project root is made absolute at each entrypoint (`AbsoluteRoot`).** A relative root breaks path-crossing contracts. A worktree-phase agent has the worktree as cwd. The in-process bridge has the main repo as cwd. They resolve the same relative artifact path to different absolute locations. That caused an artifact timeout.
- **`AbsoluteRoot` fails loud.** When `filepath.Abs` fails, it warns through `warn` (when `warn` is not nil) and returns the path unchanged. To carry on silently reproduces the timeout. `label` names the source of the path (for example `--project-root`) in the warning.

## Findings

- The artifact timeout that `AbsoluteRoot` guards against was `ExitArtifactTimeout` in cycle 119.
