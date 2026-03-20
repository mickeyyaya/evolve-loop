# Parallel Safety

The evolve-loop supports multiple concurrent invocations against the same repository. This document consolidates the coordination mechanisms that prevent data races and ensure consistency.

## Coordination Mechanisms

### 1. Optimistic Concurrency Control (OCC)

`state.json` uses a `version` field as an optimistic lock. Every write follows this protocol:

1. Read state.json, note `version = V`
2. Make changes to the in-memory copy
3. Write state.json with `version = V + 1`
4. Re-read and verify `version == V + 1`
5. If mismatch (another run wrote first) → re-read, re-apply changes, retry

OCC protects: cycle number allocation, task claiming, eval history updates, instinct summary changes, and all other state mutations.

See [memory-protocol.md](../skills/evolve-loop/memory-protocol.md) § Concurrency Protocol for the full specification.

### 2. Ship Lock (`.evolve/.ship-lock`)

The SHIP phase (Phase 4) is inherently serial — only one run can push to git at a time. A filesystem lock ensures this:

```bash
# Acquire (mkdir is atomic on POSIX)
mkdir .evolve/.ship-lock

# Release (after push completes)
rm -rf .evolve/.ship-lock
```

- **Timeout:** 60 seconds max wait
- **Stale lock detection:** Locks older than 5 minutes are broken automatically
- **Scope:** Covers `git pull --rebase`, `git push`, and version bump operations

### 3. Run Isolation

Each invocation operates in its own workspace directory (`$WORKSPACE_PATH`), preventing workspace file conflicts between concurrent runs. Run-local files (scout-report, build-report, etc.) never collide.

See [run-isolation.md](run-isolation.md) for the full isolation model.

### 4. Task Claiming

Before building, the orchestrator claims each task via OCC to prevent two parallel runs from building the same task:

1. Read `state.json.evaluatedTasks`
2. Filter out tasks already claimed (`decision: "selected"` or `"completed"`)
3. Write remaining tasks with `decision: "selected"` and the current `runId`
4. Verify via OCC protocol

### 5. Append-Only Logs

`ledger.jsonl` and `experiments.jsonl` use append-only writes. Concurrent appends are safe on POSIX filesystems for line-oriented data.

## Shared vs Isolated Resources

| Resource | Concurrency Model | Conflict Risk |
|----------|-------------------|---------------|
| `state.json` | OCC (version field) | Low — retries resolve |
| `ledger.jsonl` | Append-only | None |
| Workspace files | Run-isolated | None |
| `.evolve/evals/` | One writer per cycle | Low |
| `.evolve/instincts/` | Serial within LEARN | Low |
| `latest-brief.json` | Last-writer-wins | Acceptable (advisory) |
| Git push | Ship lock | None (serialized) |

## When Things Go Wrong

- **OCC conflict:** Automatic retry (max 3). If persistent, likely a tight parallel loop — increase cycle interval.
- **Stale ship lock:** Auto-broken after 5 minutes. If recurring, check for crashed sessions.
- **Duplicate task claim:** OCC prevents this. If a task appears twice in evaluatedTasks, the second run's claim will fail the version check.

See [architecture.md](architecture.md) § Shared Memory Architecture for the broader context.
