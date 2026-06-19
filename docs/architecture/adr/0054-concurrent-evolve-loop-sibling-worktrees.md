# ADR-0054: Concurrent `evolve loop` — Sibling-Worktree Isolation Model

## Status

Accepted — Slices 2–5 implemented on branch `cycle-9f00fd60-5` (cycles 1–4); Slice 6 (this ADR + docs) completes the campaign.

- **Date:** 2026-06-19
- **Driver:** An operator request to run multiple independent `evolve loop` processes concurrently without collision — each loop owning its own git worktree, its own `.evolve/` state tree, and its own slice of host resources (tmux sessions, LLM CLI slots).
- **Evidence:** [concurrency-architecture-2026-06-18.md](../../knowledge-base/research/concurrency-architecture-2026-06-18.md) — implementation spec for Slices 2–6 (sibling-worktree architecture).
- **Relates to:** ADR-0049 (concurrent multi-cycle execution — fleet model, a DIFFERENT topology; see §Relationship to ADR-0049 below), ADR-0032 (swarm harness, intra-cycle parallelism).

## Problem

Two `evolve loop` processes launched from the SAME repository root collide: they share
`.evolve/state.json`, the same tmux session namespace, the same `~/.codex/config.toml`
read-merge-write path, and the same LLM CLI concurrency pool. There was no naming
mechanism to separate their artifacts, no reaper for orphaned sessions when one loop
is SIGKILL'd, and no cross-process admission control to cap concurrent CLI launches.

This is **not** the same problem as ADR-0049's intra-repo fleet concurrency (where M
cycles share one `.evolve/`). The sibling-worktree model is M independent loops each
with their own `.evolve/` — the collision surface is the **shared host runtime**
(tmux, ~/.codex, LLM CLI slots, ports), not the shared state directory.

## The Two-Layer Model

```
Layer 1: Namespace (per-worktree identity)
  ┌──────────────────────────────────────────────────┐
  │  internal/runscope  — stable branch + dir suffix │
  │  EVOLVE_LANE        — operator readable override  │
  │  runscope.Token()   — collision-safe per-run ID   │
  └──────────────────────────────────────────────────┘
            feeds into
Layer 2: Shared Host Runtime Guards
  ┌──────────────────────────────────────────────────┐
  │  internal/sessionreaper  — lease-gated orphan    │
  │                            session reaper (Tier 3)│
  │  internal/cliadmit       — cross-process LLM-CLI │
  │                            admission control      │
  └──────────────────────────────────────────────────┘
```

### Layer 1 — Namespace (`internal/runscope`)

Each `evolve loop` launched from a distinct git worktree receives a **collision-safe,
human-readable run scope token**: `cycle-<lane>-<N>` where `<lane>` is derived from
`sha256(worktree-root)[:8]` (the `runscope.EnvLane` default) or from the operator's
`EVOLVE_LANE` override. This token flows into:

- The tmux session name prefix (e.g. `evolve-bridge-cycle-campaign-3`).
- The workspace directory suffix under `.evolve/runs/`.
- The ledger `RunID` sidecar — stable for resume, unique across worktrees.

**Safety invariant:** correctness never depends on `EVOLVE_LANE` — the hash default is
collision-safe across distinct worktree roots. `EVOLVE_LANE` is readability-only.

### Layer 2 — Shared Host Runtime Guards

Two new packages close the host-runtime collision surface that Layer-1 naming alone
cannot solve:

#### `internal/sessionreaper` — Tier-3 Liveness Orphan Reaper

Tier-1 (per-launch `defer tmuxCleanup`) and Tier-2 (per-cycle `reapCycleSessions`)
already existed. The gap: a run SIGKILL'd mid-cycle leaks its tmux sessions; without a
Tier-3 reaper, `looppreflight` would only WARN about the leaked sessions — and a future
loop iteration might kill a live peer's sessions.

`ReapOrphans(ctx, evolveDir string, o Options) (Report, error)`:
- Walks `<evolveDir>/runs/*/tmux-sessions.jsonl` (the session registries).
- For each run's directory, reads the per-run `.lease` heartbeat.
- If `runlease.Fresh(lease, now, ttl)` → **SKIP** (live peer — the core safety invariant).
- Else `swarm.ReapRunSessions(registryPath, o.Kill)`.

Wired in `looppreflight` (replaces the former glob-WARN) and exposed via
`evolve swarm reap-orphans [--dry-run]` as an operator backstop.

**Safety invariant:** a run whose lease is Fresh is **never reaped**, regardless of the
reaper's caller or any other condition. This is the point of the design: the `.lease`
file is the sole liveness oracle; the tmux registry is the address book.

#### `internal/cliadmit` — Cross-Process LLM-CLI Admission Control

M concurrent loops all hammering the same `codex`/`claude`/`agy` binary exhaust the
per-CLI rate limit. No cross-process cap existed; the `swarm/dispatcher.go` semaphore
is in-process only.

`Acquire(ctx, cli string, max int, ttl time.Duration) (release func(), err error)`:
- A flock'd holder-set JSON under `$XDG_RUNTIME_DIR/evolve/cli-<name>.slots`.
- On Acquire: lock → prune holders whose heartbeat is older than `ttl` (crashed-holder
  self-healing via lease-as-liveness) → admit if `len < max`, else block-with-backoff.
- `max <= 0` → **unbounded** → byte-identical to today (safe default, no behavior change).
- Dial: `EVOLVE_CLI_MAX_CONCURRENT_<CLI>` (e.g. `EVOLVE_CLI_MAX_CONCURRENT_CODEX=2`).
- A failed acquire **degrades gracefully**: proceed uncapped + WARN (admission control
  must never block a phase outright).

Hooked in `internal/bridge/driver_tmux_repl.go` just before session creation
(`release, err := cliadmit.Acquire(...); defer release()`).

## The `runscope` Value Object

`internal/runscope` exports:

```go
// Token is the stable, collision-safe per-worktree run-scope string.
// Carries: branch prefix + lane + monotonic counter.
type Token string

// EnvLane reads EVOLVE_LANE or derives the lane from sha256(worktreeRoot)[:8].
func EnvLane(worktreeRoot string) string

// New mints a Token for a given lane and run counter.
func New(lane string, n int) Token
```

The token is a pure value object: no I/O, no global state, trivially testable. It is
the single source of truth for naming across all subsystems (tmux, workspace, ledger).

## Slice-by-Slice Delivery Summary

| Cycle | Slice | Slug | Scope | Status |
|-------|-------|------|-------|--------|
| Slice 1 (pre-campaign) | 1 | `runscope` | Layer-1 value object; `internal/runscope` | SHIPPED (base) |
| 1 | 2 | `codex-pretrust-concurrent-regression` | Regression test for the `~/.codex/config.toml` RMW flock fix | SHIPPED |
| 2 | 3 | `sessionreaper-orphan-reap` | `internal/sessionreaper` Tier-3 reaper; looppreflight wiring | SHIPPED |
| 3 | 4 | `cliadmit-cross-process-admission` | `internal/cliadmit` admission control; driver hook | SHIPPED |
| 4 | 5 | `fleet-soak-invariants` | `evolve fleet soak --count N` system-level proof of Slices 1–4 | SHIPPED |
| 5 | 6 | `concurrent-loop-adr-docs` | This ADR + runtime-reference.md update + flag registry rows | SHIPPED |

## Prior Art

The sibling-worktree model is the 2026 industry standard for concurrent CI/CD agents
sharing a single repository. Key precedents:

| System | Mechanism | Analogue here |
|--------|-----------|---------------|
| **GitLab Runner** | `CI_CONCURRENT_ID` slot index injected per job | `EVOLVE_LANE` + `runscope.Token()` |
| **Jenkins** | `@N` workspace suffix (e.g. `workspace@2`) per executor slot | `cycle-<lane>-<N>` branch/dir naming |
| **GitHub Actions / Buildkite** | Stable `runs/<id>/` path + run ID in sidecar; ID NOT in the path segment | `run.json` + ledger `RunID`; stable `cycle-N` workspace |
| **Temporal / Apache Airflow** | `run_id` as identity key in sidecar; workspace path is stable + slot-indexed | Same: `run.json` carries `RunID`, workspace path stable |
| **Bazel / Nix** | ID in path (ephemeral, never resumed) | Explicitly rejected: ID-in-path breaks `--resume` and warm caches |

The convergent principle across all surveyed systems: **stable path + run ID in the sidecar**
for resumable workspaces; ID-in-path only for ephemeral sandboxes.

## Relationship to ADR-0049 (Fleet Model)

ADR-0049 describes the **fleet model**: M cycles running concurrently inside ONE shared
`.evolve/` directory (one global ledger, one state file, one project lock scoped per-run
via S6's floor removal). Its concurrency surface is intra-repo state isolation.

This ADR describes the **sibling-worktree model**: M independent `evolve loop` processes
each running in their own git worktree with their own `.evolve/`. There is no shared
state file or ledger — the collision surface is purely the **shared host runtime**.

| Dimension | ADR-0049 (Fleet) | ADR-0054 (Sibling-Worktree) |
|-----------|------------------|-----------------------------|
| `.evolve/` directories | 1 shared | N independent |
| Concurrency surface | state.json, ledger, project lock | tmux, ~/.codex, LLM CLI slots |
| Loop topology | one shared loop spawns M cycles | M independent loops, each full-stack |
| Namespace need | per-run RunID within one dir | per-worktree lane + token |
| Applicable to fleet? | native | yes — `cycle-<lane>-N` is collision-free under fleet too |

The two models are **complementary, not competing.** The sibling-worktree model's Layer-1
naming (`cycle-<lane>-N`) is already collision-safe in the fleet topology (same root →
distinct N), so adopting fleet later requires no Layer-1 rework.

## Decisions

| # | Decision | Resolution |
|---|----------|-----------|
| D1 | **Workspace identity** | Stable `cycle-N` path + RunID in sidecar/ledger; lane index in session name. ID NOT in the path. |
| D2 | **Session liveness oracle** | `.lease` heartbeat is the SOLE liveness signal; tmux registry is the address book. Reaper NEVER uses session names alone. |
| D3 | **Admission control failure mode** | Failed `cliadmit.Acquire` ALWAYS degrades to uncapped + WARN; never blocks a phase. |
| D4 | **`EVOLVE_REAP_ORPHANS` role** | Documentation / operator opt-out dial; does NOT gate `sessionreaper` core logic. The reaper is unconditionally wired in looppreflight. |

## Consequences

**Positive:**
- M independent `evolve loop` processes can run concurrently without tmux session
  collisions, codex config corruption, or CLI rate-limit cascades.
- The safety invariant (never reap a live peer) is enforced by the lease oracle, not
  by naming convention — naming bugs cannot cause a reaper false-positive.
- `max <= 0` (the default) is byte-identical to the pre-concurrency behavior: operators
  adopt incrementally.

**Negative / residual risk:**
- Ports and `/tmp` paths are out of scope (rarely collide in the sibling model; noted in
  the spec as an ADR-only footnote for now).
- `EVOLVE_CLI_MAX_CONCURRENT_<CLI>` requires per-CLI tuning; the safe default (unbounded)
  does not cap anything.

## Flags Introduced

| Flag | Default | Purpose |
|------|---------|---------|
| `EVOLVE_LANE` | hash-of-worktree-root | Human-readable lane override for the run-scope token |
| `EVOLVE_REAP_ORPHANS` | `1` (active) | Operator opt-out for session reaping (`0` disables); does NOT gate `sessionreaper` core logic |
| `EVOLVE_CLI_MAX_CONCURRENT_<CLI>` | `0` (unbounded) | Per-CLI cross-process admission cap (e.g. `EVOLVE_CLI_MAX_CONCURRENT_CODEX=2`) |
