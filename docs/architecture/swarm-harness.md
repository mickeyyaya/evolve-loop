# Swarm Harness — Operator & Developer Guide

> Living guide for the multi-tmux-LLM-CLI subagent swarm harness. Design rationale: [ADR-0032](adr/0032-swarm-harness.md).
> Research record: [knowledge-base/research/swarm-concurrency-2026-05.md](../../knowledge-base/research/swarm-concurrency-2026-05.md).
>
> **Status:** v4 — COMPLETE + WIRED (HEAD `32bae0f`). The full pipeline is live: planner+validator,
> per-worker isolation (registry/reaper/provisioner), parallel dispatcher, merge-train (writer) +
> synthesis (reader) fan-in, and the `swarmRunner` Decorator wired into the orchestrator runners map
> (`cmd_cycle.go`: build=writer, scout=reader). Gated by `EVOLVE_SWARM_STAGE`: unset/`shadow` =
> byte-identical N=1 delegate; `advisory` = live dispatch, inner runner authoritative; `enforce` =
> swarm result is the phase output. **Orphan-on-cancel is now HARDENED:** the dispatcher pins a
> deterministic tmux session name (`swarm-c<cycle>-<workerID>` → `bridge.NamedSessionName`) and
> REGISTERS it BEFORE launch, so a worker cancelled mid-spawn is still reaped by name; headless
> workers create no session and are killed by ctx-cancel. Documented follow-ups (non-blocking):
> reader-enforce synthesis-to-one-artifact, per-worker port isolation for server-running writers.

## Session naming & teardown (orphan-on-cancel)

A tmux worker's session name is **pinned and pre-registered**, not discovered after the fact:

1. The dispatcher computes `sessionName = swarm-c<cycle>-<workerID>` — short (no validation
   overflow), and collision-free (worker IDs are validated unique within a plan), built from
   cycle+worker-id, NOT the task slug.
2. It registers a `SessionHandle{TmuxSession: bridge.NamedSessionName(sessionName), ...}` in the
   `SessionRegistry` (+ crash-safe manifest) **before** calling `Launch`.
3. `Launch` forwards `SessionName` via `core.BridgeRequest` → `--session-name` → the driver's
   `resolveSession` named-session path (a named session is *preserved* by the driver's own cleanup —
   the swarm owns teardown).
4. On dispatch scope exit (success, cancel, or fatal), `Reap` kills every registered session by its
   exact pinned name. A worker spawned-then-cancelled is therefore never an orphan.

`bridge.NamedSessionName(name)` (`"evolve-bridge-named-"+name`, truncated to 64) is the **single
source of truth** shared by `resolveSession` (creates the session) and the swarm reaper (kills it) —
they cannot drift. The crash-safe `evolve swarm reap` remains the backstop for a hard parent SIGKILL.

## What it is

A reusable primitive that lets one phase dispatch **N heterogeneous workers** (each its own CLI/model)
that collaborate on a partitioned task, then reduces their results to ONE phase output. It preserves
evolve-loop's single-writer invariants: parallelism lives only in disjoint worktrees, and the only
serialized section is the merge. It **falls back to N=1** (today's behavior) on any planning/validation
failure.

## The one rule to remember: writer vs reader

- **WRITER swarm** (build): workers WRITE code → partitions MUST be completely disjoint by file
  ownership. A non-disjoint plan is **rejected → N=1**. Fan-in is a serialized git merge-train.
- **READER swarm** (scout/audit): workers only READ → overlap is allowed (wastes tokens, never
  corrupts). Fan-in is summary synthesis, no git.

If you remember nothing else: **a writer swarm only runs on a provably disjoint partition; otherwise it
is a single writer.**

## Components (`go/internal/swarm/`)

| File | Responsibility |
|---|---|
| `types.go` | Pure value types: `Mode`, `WorkerSpec`, `SwarmPlan`, `ValidationResult`, `Conflict`. |
| `parse.go` | `ParsePlan(artifact)` — extracts the `{"swarm_plan": {...}}` JSON from `swarm-plan.md`. |
| `partition.go` | `Validate(plan)` — the mode-aware safety gate (writer strict-disjoint → reject-to-N=1; reader lenient). |
| `topo.go` | `TopoOrder(workers)` — Kahn's algorithm over the `depends_on` DAG → serialized merge order. |
| `dispatcher.go` _(v3)_ | Structured-concurrency fan-out (semaphore + WaitGroup; blocks until all reaped). |
| `registry.go` _(v2)_ | `SessionRegistry`: in-mem + on-disk manifest of live sessions. |
| `mergetrain.go` _(v4)_ | Serialized dev→integration merge, gated on each worker's acceptance check. |
| `reaper.go` _(v2)_ | Orphan sweep from the manifest (crash recovery). |

The planner phase is `go/internal/phases/swarmplan/` (a `runner.BaseRunner` clone of buildplanner);
the persona is `agents/evolve-swarm-planner.md`; the profile is `.evolve/profiles/swarm-planner.json`.

## Control flow (end-to-end, target state v4)

```
orchestrator reaches build → swarmRunner.Run (Decorator wrapping the build runner)
  1. swarm-plan phase emits swarm-plan.md  → swarm.ParsePlan
  2. swarm.Validate(plan):
       fallback/overlap/cyclic-DAG → return inner.Run (N=1, byte-identical to today)
       OK → continue
  3. provision cycle-<N>-integration + per-worker cycle-<N>-w<i> worktrees; register sessions
  4. Dispatcher: launch N workers in parallel (own branch+worktree+sandbox+tmux); reap all on scope exit
  5. MergeTrain (writers): topo order; merge each dev branch → integration; gate on acceptance;
     conflict → re-dispatch authoring worker once → else FAIL
     | synthesis (readers): fold summaries into one artifact
  6. aggregate N PhaseResponse → ONE; orchestrator writes ONE ledger entry (WorkerCount/Workers)
  7. ship sees the integration worktree
```

## Configuration

| Env var | Default | Effect |
|---|---|---|
| `EVOLVE_SWARM_STAGE` | `shadow` | Rollout dial: `shadow`/off (no dispatch) → `advisory` (dispatch, non-authoritative) → `enforce` (merge-train authoritative). |
| `EVOLVE_SWARM_PLANNER` | off | Legacy `=1` form that enables the `swarm-plan` phase (maps to the stage via config.Load). |
| `EVOLVE_SWARM_CONCURRENCY` _(v3)_ | `2` | Max workers dispatched at once (semaphore cap). |
| `EVOLVE_SWARM_BUDGET_USD` _(v3)_ | unset | Cumulative USD ceiling across all workers in a swarm; abort remaining on breach. |

## On-disk manifest schema _(v2)_

`<evolveDir>/runs/cycle-<N>/.swarm/sessions.json`, atomic-written on every register/unregister:

```json
{ "cycle": 153, "phase": "build", "pid": 12345, "updated": "<ISO-8601>",
  "sessions": [ { "worker_id": "w0", "tmux_session": "evolve-bridge-c153-build-w0-pid12345-...",
    "pgid": 67890, "worktree": ".../.evolve/worktrees/cycle-153-w0", "branch": "cycle-153-w0",
    "started_at": "<ISO-8601>", "status": "live|reaped" } ] }
```

## `evolve swarm` commands _(v2)_

- `evolve swarm status [--cycle N]` — list live/orphaned sessions from the manifest.
- `evolve swarm reap [--cycle N]` — kill orphaned sessions (pgroup → tmux kill-session → confirm),
  prune their worktrees, mark reaped. Idempotent; safe to run anytime; also run as an `evolve loop`
  preflight.

## Debugging runbook

**A worker seems stuck / the swarm hangs.**
1. `evolve swarm status --cycle N` (v2) or inspect `.swarm/sessions.json` for live sessions.
2. List its tmux session: `tmux list-sessions | grep '^evolve-bridge-c<N>-'`. Attach read-only:
   `tmux attach -t <session> -r` to see what the CLI is doing.
3. Read the worker's artifacts under `<workspace>` and its dev branch:
   `git -C <project> log --oneline cycle-<N>-w<i>` and `git -C <worktree> status`.
4. Check the phase-observer/stall detector (`EVOLVE_OBSERVER_*`) — a stalled worker is SIGTERM'd at
   `EVOLVE_OBSERVER_STALL_S`.

**Orphaned sessions after a crash (parent SIGKILL'd).**
- `evolve swarm reap --cycle N` (v2). It reads the manifest and kills by recorded pgid + tmux session.
- **NEVER** `pkill -f codex` on a shared host — it matches other sessions' MCP servers whose PATH
  contains the macOS cryptex `codex.system` string. Always target the exact `evolve-bridge-c<N>-`
  prefix or recorded pgids.

**A writer swarm unexpectedly ran as N=1.**
- Expected when the partition wasn't disjoint. Read `swarm-plan.md` and the validator reason in the
  ledger / shadow log: an overlap → `Conflicts` lists the file(s) claimed by multiple workers; a cyclic
  `depends_on` → "invalid merge DAG". Fix the planner's partition or accept N=1.

**A merge-train step failed _(v4)_.**
- The failing worker's dev branch text-merged but broke its acceptance check, or had a real conflict.
  Integration was left at the last-good tip (`git merge --abort`). Inspect `cycle-<N>-integration` and
  the worker's branch; the authoring worker is re-dispatched once before the swarm FAILs.

## Invariants (do not break)
- **Workers never call `ledger.Append`** — the hash chain is single-writer; the orchestrator writes ONE
  entry per swarm. Pass results back by channel, not the Ledger.
- **Only the merge-train touches the integration index**, one worker at a time (no concurrent `git merge`).
- **`git worktree add` is serialized** (shared `.git/worktrees/`); only the builds run parallel.
- **Validation is pure** (`swarm.Validate`/`TopoOrder`/`ParsePlan` do no I/O) — keep it that way so it
  stays exhaustively unit-testable.
