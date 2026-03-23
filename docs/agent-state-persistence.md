# Agent State Persistence

> Reference document for checkpointing and state management in long-running agents.
> Use these patterns to preserve progress across failures, enable resumption from
> the last known-good state, and prevent data loss during multi-phase agent execution.

## Table of Contents

1. [State Categories](#state-categories)
2. [Checkpointing Strategies](#checkpointing-strategies)
3. [Resume-from-Failure](#resume-from-failure)
4. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
5. [Implementation Patterns](#implementation-patterns)
6. [Prior Art](#prior-art)
7. [Anti-Patterns](#anti-patterns)

---

## State Categories

Classify every piece of agent state by its lifetime and durability guarantees.

| Category | Lifetime | Storage | Survives Crash | Survives Restart | Examples |
|---|---|---|---|---|---|
| **Ephemeral** | Single function call | In-memory (variables, registers) | No | No | Loop counters, intermediate computations, scratch buffers |
| **Session** | Single agent invocation | Working directory, temp files | No | No | Workspace artifacts, draft outputs, in-progress edits |
| **Persistent** | Cross-session | Disk file (JSON, SQLite) | Yes | Yes | Configuration, cumulative metrics, agent state files |
| **Durable** | Permanent, append-only | Versioned log, immutable store | Yes | Yes | Audit ledgers, cycle history, decision records |

### Selection Criteria

| Factor | Prefer Ephemeral | Prefer Session | Prefer Persistent | Prefer Durable |
|---|---|---|---|---|
| **Cost of recomputation** | Cheap | Moderate | Expensive | Impossible to recreate |
| **Size** | Small (<1 KB) | Medium (<100 KB) | Any | Any |
| **Consistency requirement** | None | Within invocation | Across invocations | Across all time |
| **Regulatory / audit need** | No | No | No | Yes |

---

## Checkpointing Strategies

Select the checkpoint strategy based on execution characteristics and failure cost.

| Strategy | Trigger | Granularity | Overhead | Best For |
|---|---|---|---|---|
| **Phase-boundary** | Completion of a named phase (Scout, Builder, Auditor) | Coarse | Low | Multi-phase pipelines with clear boundaries |
| **Time-based** | Fixed interval (e.g., every 60 seconds) | Medium | Medium | Long-running single-phase tasks |
| **Event-triggered** | Specific events (file write, API call, tool invocation) | Fine | Variable | Tasks with expensive or non-idempotent operations |
| **Incremental** | Delta from previous checkpoint | Fine | Low per-checkpoint | Large state where full snapshot is expensive |
| **Full snapshot** | Any trigger above | Full state capture | High per-checkpoint | Small state or when incremental is unreliable |

### Checkpoint Contents

| Field | Required | Description |
|---|---|---|
| `checkpoint_id` | Yes | Unique identifier (UUID or monotonic counter) |
| `timestamp` | Yes | ISO 8601 creation time |
| `phase` | Yes | Current phase name at checkpoint time |
| `cycle` | Yes | Cycle number (for cyclic systems) |
| `state_hash` | Yes | SHA-256 of serialized state for integrity verification |
| `parent_id` | No | Previous checkpoint ID (enables chain validation) |
| `payload` | Yes | Serialized state data |
| `metadata` | No | Execution context (agent version, model, token usage) |

---

## Resume-from-Failure

Follow this sequence to recover from a crash or unexpected termination.

### Detection

| Signal | Detection Method | Indicates |
|---|---|---|
| Missing completion marker | Check for `phase_complete` flag in state file | Phase interrupted mid-execution |
| Stale lock file | Lock file older than max expected phase duration | Agent process died while holding lock |
| Inconsistent state hash | Recompute hash and compare to stored `state_hash` | Corrupted or partially written checkpoint |
| Gap in checkpoint chain | `parent_id` does not match previous checkpoint's `checkpoint_id` | Missing or lost intermediate checkpoint |

### Recovery Procedure

| Step | Action | Validation |
|---|---|---|
| 1 | Locate latest checkpoint file | File exists and is non-empty |
| 2 | Validate checkpoint integrity | Recompute `state_hash` and compare |
| 3 | Verify checkpoint chain | Walk `parent_id` links back to cycle start |
| 4 | Restore state from checkpoint payload | Deserialize and load into agent memory |
| 5 | Re-execute from interrupted phase | Start at the phase recorded in checkpoint |
| 6 | Write new checkpoint after successful recovery | Confirm new checkpoint passes validation |

### Idempotency Requirements

| Operation Type | Idempotent by Default | Make Idempotent By |
|---|---|---|
| File write (overwrite) | Yes | N/A |
| File write (append) | No | Track last written offset in checkpoint |
| API call (GET) | Yes | N/A |
| API call (POST/mutating) | No | Use idempotency keys, check prior results |
| Git commit | No | Check if commit with same message/diff exists |
| Tool invocation | Varies | Record tool call ID and result in checkpoint |

---

## Mapping to Evolve-Loop

Map each evolve-loop artifact to a state category and checkpointing role.

| Artifact | State Category | Checkpoint Role | Lifecycle |
|---|---|---|---|
| `state.json` | Persistent | Primary persistent state — stores cycle number, current phase, cumulative metrics | Updated at every phase boundary, survives restarts |
| Workspace files (`scout-report.md`, `build-report.md`, `audit-report.md`) | Session | Phase output checkpoints — each file marks completion of Scout, Builder, or Auditor | Created during cycle, archived to `history/` at cycle end |
| `ledger.jsonl` | Durable | Append-only audit log — immutable record of every cycle outcome | Append only, never modified or truncated |
| Handoff files (phase outputs consumed by next phase) | Session | Phase-boundary checkpoints — enable resume if next phase fails | Created by producer phase, consumed by consumer phase |
| `history/cycle-NNN/` | Durable | Cycle snapshots — complete record of all artifacts for a given cycle | Created at ship phase, immutable after creation |
| Gene/instinct files | Persistent | Evolved configuration — updated by successful cycles | Modified only during ship phase after Auditor approval |

### Phase-Boundary Checkpoint Flow

| Phase Transition | Checkpoint Written | Contents | Consumed By |
|---|---|---|---|
| Start -> Scout | `state.json` updated with `phase: scout` | Cycle number, selected task | Orchestrator (on failure recovery) |
| Scout -> Builder | `scout-report.md` written | Task analysis, implementation plan | Builder agent |
| Builder -> Auditor | `build-report.md` written | Changes made, files modified, test results | Auditor agent |
| Auditor -> Ship | `audit-report.md` written | Pass/fail verdict, scores, feedback | Ship phase (commit or rollback) |
| Ship -> Learn | Cycle archived to `history/` | All artifacts, ledger entry appended | Learn phase, future Scout cycles |

---

## Implementation Patterns

### Checkpoint Schema (JSON)

```json
{
  "checkpoint_id": "ckpt-20260324-143022-a7f3",
  "timestamp": "2026-03-24T14:30:22Z",
  "phase": "builder",
  "cycle": 142,
  "state_hash": "sha256:ab3f...9c12",
  "parent_id": "ckpt-20260324-142815-b2e1",
  "payload": {
    "task": "add-retry-logic",
    "files_modified": ["src/retry.ts"],
    "tests_passing": true
  },
  "metadata": {
    "agent_version": "1.0.0",
    "model": "claude-sonnet-4-5-20250514",
    "tokens_used": 12450
  }
}
```

### Validation on Resume

| Check | Implementation | On Failure |
|---|---|---|
| Schema validation | Validate checkpoint against JSON schema | Reject checkpoint, fall back to previous |
| Hash integrity | Recompute SHA-256 of `payload` field | Reject checkpoint, fall back to previous |
| Chain continuity | Verify `parent_id` links are unbroken | Log gap warning, resume from latest valid |
| Temporal ordering | Confirm timestamps are monotonically increasing | Reject out-of-order checkpoints |
| Phase consistency | Verify `phase` matches expected phase for cycle progress | Reset to last consistent phase boundary |

### Garbage Collection

| Policy | Rule | Rationale |
|---|---|---|
| Keep latest N checkpoints | Retain last 10 checkpoints per cycle | Balance between recovery depth and disk usage |
| Keep phase-boundary only | Delete intra-phase checkpoints after phase completes | Phase boundaries are sufficient for most recoveries |
| Archive completed cycles | Move cycle checkpoints to `history/` at cycle end | Preserve audit trail without cluttering workspace |
| TTL-based cleanup | Delete checkpoints older than 7 days (non-archived) | Prevent unbounded checkpoint accumulation |

---

## Prior Art

| System | State Model | Checkpoint Mechanism | Recovery Approach |
|---|---|---|---|
| **Apache Flink** | Distributed operator state | Asynchronous barrier snapshots (Chandy-Lamport algorithm) | Restore all operators from latest consistent snapshot; replay from source offsets |
| **Temporal Workflow** | Event-sourced workflow history | Every workflow step persisted as an event | Replay event history to reconstruct state; deterministic execution guarantees identical replay |
| **Redis (RDB + AOF)** | In-memory key-value store | RDB: periodic full snapshots; AOF: append-only command log | RDB: restore from snapshot; AOF: replay commands; combined: snapshot + partial AOF replay |
| **LangGraph** | Graph-based agent state | State persisted at each graph node transition | Resume from last completed node; re-execute current node with preserved state |
| **Kubernetes (etcd)** | Cluster configuration state | Raft consensus log + periodic snapshots | Restore from snapshot + replay raft log entries after snapshot |
| **Apache Spark** | RDD lineage graph | Lineage metadata (not data) as implicit checkpoint; explicit checkpoint to storage | Recompute lost partitions from lineage; restore from explicit checkpoint if available |

---

## Anti-Patterns

Avoid these common mistakes when implementing state persistence.

| Anti-Pattern | Problem | Fix |
|---|---|---|
| **Checkpoint bloat** | Store full state at every event, consuming gigabytes of disk | Use incremental checkpoints; garbage-collect old snapshots; compress payloads |
| **Inconsistent state** | Checkpoint written mid-mutation, capturing partial updates | Write checkpoints only at consistent boundaries (between phases, after commits) |
| **Missing validation on resume** | Resume from corrupted checkpoint causes cascading failures | Always validate hash, schema, and chain before restoring |
| **Over-persistence** | Persist ephemeral data (scratch variables, temp files) unnecessarily | Classify state by category; only persist what survives restart |
| **Implicit state** | Critical state lives only in environment variables or process memory | Make all resumable state explicit in checkpoint payload |
| **Checkpoint-resume asymmetry** | Checkpoint format diverges from resume parser over time | Use a single schema definition for both write and read paths |
| **Unbounded history** | Never delete old cycle snapshots, filling disk indefinitely | Apply TTL or retention-count policies; archive to cold storage |
| **Lock file leaks** | Agent dies without releasing lock; next run waits indefinitely | Use TTL-based locks; detect stale locks by timestamp comparison |
