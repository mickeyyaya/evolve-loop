# Multi-Agent Blackboard Architecture

> Reference document for the blackboard coordination pattern in multi-agent systems.
> Use this pattern when agents must share structured state without direct message passing.
> Apply to evolve-loop to formalize the existing proto-blackboard into typed, access-controlled slots.

## Table of Contents

1. [Blackboard Architecture](#blackboard-architecture)
2. [Typed Slot Design](#typed-slot-design)
3. [Access Control](#access-control)
4. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
5. [Implementation Patterns](#implementation-patterns)
6. [Prior Art](#prior-art)
7. [Anti-Patterns](#anti-patterns)

---

## Blackboard Architecture

| Aspect | Blackboard | Message Passing | File-Based Coordination |
|---|---|---|---|
| **Communication model** | Shared central repository; agents read/write typed slots | Point-to-point or pub/sub channels between agents | Agents read/write files on disk with conventions |
| **Coupling** | Low — agents depend on slot schema, not on each other | Medium — sender must know receiver or topic | Low — agents depend on file paths and formats |
| **State visibility** | Global — any authorized agent can inspect any slot | Local — only participants in a channel see messages | Global — any agent with filesystem access can read |
| **Conflict resolution** | Built-in via versioning, OCC, or merge rules | Handled by message ordering (queues, sequence numbers) | Manual — file locks, last-write-wins, or OCC on version fields |
| **Scalability** | Bounded by slot count and read/write frequency | Scales with channel count; can fan out | Scales with filesystem I/O; degrades with many small files |
| **Best for** | Shared-state problems where multiple agents contribute partial solutions | Event-driven pipelines with clear producer/consumer roles | Simple orchestration with few agents and infrequent updates |

**Core principle:** The blackboard is a structured, typed, access-controlled shared memory. Agents act as independent knowledge sources that monitor the blackboard for relevant changes and contribute results back.

---

## Typed Slot Design

| Slot Type | Mutability | Lifecycle | Schema Requirements | Example |
|---|---|---|---|---|
| **Fact** | Immutable once written | Permanent — persists across cycles | `{ key, value, source, timestamp, hash }` | Eval score, git commit SHA, benchmark result |
| **Hypothesis** | Versioned — each write creates a new version | Retained until confirmed or refuted | `{ key, value, version, author, confidence, timestamp }` | Scout task proposal, strategy recommendation |
| **Task** | Claimable — transitions through states | Created → Claimed → Completed/Failed | `{ id, description, state, claimedBy, cycle, attempts }` | Build task, eval task, refactor task |
| **Signal** | Ephemeral — consumed on read or after TTL | Expires after one cycle or fixed duration | `{ type, payload, timestamp, ttl }` | "Audit triggered", "convergence reached", challenge token |

### Slot State Transitions

| Slot Type | Valid Transitions |
|---|---|
| **Fact** | `empty → written` (no further changes) |
| **Hypothesis** | `proposed → revised → confirmed` or `proposed → revised → refuted` |
| **Task** | `open → claimed → in-progress → completed` or `open → claimed → in-progress → failed → open` |
| **Signal** | `emitted → consumed` or `emitted → expired` |

---

## Access Control

| Agent Role | Facts | Hypotheses | Tasks | Signals |
|---|---|---|---|---|
| **Orchestrator** | Read + Write | Read + Write | Read + Write + Claim | Read + Write |
| **Scout** | Read | Read + Write | Read + Write (propose only) | Read |
| **Builder** | Read | Read | Read + Claim + Update | Read + Write |
| **Auditor** | Read + Write (eval results) | Read + Confirm/Refute | Read + Update (pass/fail) | Read + Write |
| **Phase-Gate Script** | Read + Write (authoritative) | Read | Read + Update (state transitions) | Write (gate signals) |

### Permission Rules

| Rule | Description |
|---|---|
| **Write-once facts** | Only the originating agent or phase-gate script may write a fact; no overwrites permitted |
| **Hypothesis ownership** | Any agent may propose; only Auditor or Orchestrator may confirm/refute |
| **Task claiming** | Use optimistic concurrency control (OCC) — read version, write with expected version, retry on conflict |
| **Signal consumption** | First reader consumes; implement via atomic read-and-delete or TTL expiry |
| **Escalation** | Write failures after 3 retries escalate to Orchestrator via signal |

---

## Mapping to Evolve-Loop

| Blackboard Concept | Current Evolve-Loop Artifact | Slot Type | Notes |
|---|---|---|---|
| Central repository | `.evolve/` directory | — | Root namespace for all shared state |
| Shared facts | `state.json` | Fact | Cycle number, mastery scores, eval history, strategy — OCC via `version` field |
| Task registry | `state.json.evaluatedTasks` | Task | Scout writes proposals; Builder claims via OCC; Auditor marks pass/fail |
| Event log | `ledger.jsonl` | Fact (append-only) | Hash-chained entries — tamper-evident record of all cycle events |
| Agent context | Workspace files (`scout-report.md`, `build-report.md`, `audit-report.md`) | Hypothesis | Versioned per cycle; represent agent conclusions subject to review |
| Ephemeral coordination | Challenge tokens, `forceFullAudit` flag | Signal | Generated per cycle by Orchestrator; consumed by agents; not persisted beyond cycle |
| Agent mailbox | `builder-notes.md`, `notes.md` | Hypothesis | Proto-blackboard — unstructured text that Builder and Scout read/write across cycles |
| Access control | Phase-gate scripts (`phase-gate.sh`, `verify-eval.sh`) | — | Enforce write permissions; only scripts update authoritative fields in `state.json` |

### Gap Analysis

| Gap | Current State | Blackboard Improvement |
|---|---|---|
| Untyped workspace files | Free-form markdown with implicit schema | Define typed slot schemas; validate on write |
| No slot-level notifications | Agents poll or rely on Orchestrator to pass context | Add signal slots that trigger agent activation |
| Manual conflict resolution | OCC on `state.json` version field only | Extend OCC to all mutable slots; add retry + escalation |
| No read tracking | No record of which agent read which slot | Add read receipts for audit trail |

---

## Implementation Patterns

### Slot Schema

| Field | Type | Required | Description |
|---|---|---|---|
| `slotId` | string | Yes | Unique identifier (e.g., `fact:eval:cycle-142`) |
| `slotType` | enum | Yes | `fact`, `hypothesis`, `task`, `signal` |
| `key` | string | Yes | Logical name (e.g., `evalScore`, `buildTask-3`) |
| `value` | any | Yes | Payload — structure varies by slot type |
| `version` | integer | Yes | Monotonically increasing; used for OCC |
| `author` | string | Yes | Agent role that wrote the slot |
| `timestamp` | ISO 8601 | Yes | Write time |
| `prevHash` | string | No | Hash of previous version (for chained slots) |
| `ttl` | integer | No | Seconds until expiry (signals only) |

### Conflict Resolution

| Strategy | When to Use | Mechanism |
|---|---|---|
| **Optimistic Concurrency Control** | Task claiming, state updates | Read version V, write with `expectedVersion: V`, reject if mismatch |
| **Last-Writer-Wins** | Low-contention hypothesis updates | Timestamp comparison; latest write prevails |
| **Merge** | Composite slots (e.g., aggregated scores) | Combine values using domain-specific merge function |
| **Escalate** | Unresolvable conflicts after max retries | Emit signal to Orchestrator; block until resolved |

### Notification on Slot Changes

| Pattern | Description | Trade-off |
|---|---|---|
| **Polling** | Agent checks blackboard at fixed intervals | Simple; wastes cycles when no changes |
| **Watch + Callback** | Register interest in slot keys; trigger on write | Efficient; requires notification infrastructure |
| **Event Log Tailing** | Tail `ledger.jsonl` for new entries matching slot patterns | Reuses existing infra; slight latency |
| **Signal Slot** | Writer emits ephemeral signal; interested agents consume | Decoupled; signal may expire before consumption |

---

## Prior Art

| Source | Key Contribution | Relevance to Evolve-Loop |
|---|---|---|
| **Hearsay-II** (Erman et al., 1980) | Original blackboard architecture for speech recognition; multiple knowledge sources contribute to shared hypothesis levels | Foundation model — demonstrates how heterogeneous agents converge on a solution via shared state |
| **BB1** (Hayes-Roth, 1985) | Blackboard with explicit control knowledge — meta-level reasoning about which knowledge source to activate | Maps to Orchestrator's strategy selection and convergence detection |
| **arXiv:2510.01285** — LLM Blackboard Systems | Applies blackboard pattern to LLM multi-agent collaboration; typed knowledge slots with confidence scores | Directly applicable — validates typed slots and confidence-based hypothesis management for LLM agents |
| **arXiv:2505.18279** — Multi-Agent Coordination Patterns | Survey of coordination mechanisms for LLM agent teams; compares blackboard, message passing, and hierarchical control | Provides taxonomy for choosing coordination pattern based on task structure |
| **SLSA / in-toto** (Supply chain integrity) | Tamper-evident provenance logs with hash chains | Already adopted in evolve-loop's `ledger.jsonl`; validates hash-chain approach for blackboard event logs |

---

## Anti-Patterns

| Anti-Pattern | Symptom | Fix |
|---|---|---|
| **Untyped slots** | Agents write arbitrary data; consumers fail on unexpected schema | Define and validate slot schemas; reject malformed writes |
| **Write conflicts** | Multiple agents overwrite the same slot without coordination | Enforce OCC on all mutable slots; use version fields |
| **Stale reads** | Agent acts on outdated slot data; produces incorrect results | Add version checks before acting; re-read after delays |
| **Blackboard bloat** | Accumulated slots consume excessive context window tokens | Implement TTL for signals; archive old facts; compact hypotheses on confirmation |
| **God slot** | Single slot accumulates all state; becomes a monolithic config object | Decompose into fine-grained typed slots with clear ownership |
| **Phantom dependencies** | Agent assumes a slot exists but no agent writes it | Document slot producers/consumers in access control table; validate at startup |
| **Silent consumption** | Signal consumed without acknowledgment; sender cannot verify delivery | Add read receipts or acknowledgment signals |
| **Slot name collisions** | Two agents use the same slot key for different purposes | Use namespaced keys (e.g., `scout:hypothesis:task-42`, `auditor:fact:eval-142`) |
