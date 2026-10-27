# internal/runscope

## Purpose

`internal/runscope` is the single source of truth for every run-, cycle- and lane-scoped name that the concurrent loop emits. That covers git worktree branches and directories, which are global to one shared `.git` object store. It also covers tmux session prefixes, which are global to one tmux server. One immutable Value Object replaces the branch-minting sites in `core`, `swarm` and `cmd`. It also replaces the parallel token schemes that drifted.

## Design

- **`runscope` composes two leaf primitives and re-implements neither.** The lane is the 8-hex SHA-256 of `internal/projecthash`. The run token is `internal/sessionrecord.RunScopeToken`.
- **The dependency points one way.** `runscope` depends on both primitives, never the reverse. `core`, `swarm`, `cmd/evolve` and `internal/bridge` can all import `runscope` without an import cycle. `swarm` gains no direct `core` dependency. `sessionrecord` stays the leaf that the observer and the bridge share.
- **The git branch and worktree directory namespace is the only collision surface.** It is global across the sibling worktrees of one object store. The lane is the discriminator. It is stable for one worktree and across resume. A resumed cycle then reuses its branch and its warm worktree.
- **The pattern is the pattern of GitLab `CI_CONCURRENT_ID`, Jenkins `@N` and Temporal.** The path is stable, the lane is bounded and the run id is in a sidecar. An id in the path breaks resume and the warm cache.
- **The per-invocation ULID `RunID` is in no path.** It lives only in the ledger, the `run.json` sidecar and the tmux session names. `RunID` can be empty where only a lane and a cycle exist, such as the stateless `evolve worktree` CLI.
- **`RunScope` carries three coordinates:** the lane, the `RunID` and the cycle. Every name is a pure projection of them. Two equal values give byte-identical names.
- **`ResolveLane` has a fixed order.** An explicit value (the `--lane` flag) wins. Then the hash-of-root default (`LaneFromRoot`) applies. No code reads `EVOLVE_LANE` since X18c ([ADR-0156](../adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md) I-16), so the package has no `EnvLane` constant.
- **The `--lane` value only makes a lane readable.** An example is `--lane campaign`. Correctness never depends on it, because the hash default is collision-safe across distinct roots.
- **Name shapes.** `CycleBranch` is `cycle-<lane>-<N>`. It is both the `-B` branch and the directory leaf, so the in-process provisioner and the `evolve worktree` CLI agree. `WorktreeDir` is `<base>/cycle-<lane>-<N>`. `IntegrationBranch` adds `-integration`. `WorkerBranch` adds `-<workerID>`.
- **`SessionPrefix` is `RunToken()+"-"` when `RunID` is set, else empty.** For an empty `RunID` it equals the legacy session name.

## Invariants

- **Distinct roots get distinct lanes, and one root keeps its lane.** Two concurrent sibling worktrees must not collide on the shared object store. Path variants of one directory (trailing slash, relative form) give the same lane. `TestResolveLane_DistinctRootsDistinctLanes` and `TestResolveLane_StableAcrossResume` pin this.
- **The lane is byte-compatible with the shipped hotfix token.** `absClean` makes the root absolute and clean before the hash, as `gitexec.WorktreeToken` did. The first 4 bytes of the SHA-256, in hex, equal `projecthash.Compute`. The existing `cycle-<lane>-N` worktrees of running loops stay valid.
- **`absClean` falls back to the cleaned input.** `filepath.Abs` fails only when the working directory is unreadable. `TestLaneFromRoot_EqualsHotfixToken` pins the compatibility.
- **A lane override is a git-ref-safe token.** `sanitizeLane` keeps `[A-Za-z0-9_-]`, with no leading or trailing `-`. It drops `.` on purpose. An override then cannot make the forms that a dot allows and git refs forbid: `..`, a trailing `.lock` and a leading `.`.
- **An override with nothing safe left becomes empty.** `ResolveLane` then falls through to the next source. `TestResolveLane_OverridePrecedence` and `TestRunScope_BranchNamesRefSafe` pin this.
- **`WorkspacePath` carries no lane.** It is `<root>/.evolve/runs/cycle-<N>`. The `.evolve/` of each worktree isolates state physically. The `run.json` symlink and the tmux registry path match the bare `cycle-<N>` form across the worktree boundary. It mirrors `core.RunWorkspacePath`, so callers have one naming entry point. `TestRunScope_WorkspacePathStableNoLane` pins this.
- **`RunToken` equals the bridge token.** It is `"r"+runID[:8]`, from `sessionrecord.RunScopeToken`. It is byte-identical to the token that the bridge mints and the observer asserts. `TestRunScope_RunTokenMatchesLegacy` pins this.
