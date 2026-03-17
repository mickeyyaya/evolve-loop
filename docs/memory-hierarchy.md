# Memory Hierarchy

## Overview

The evolve-loop uses a six-layer memory hierarchy. Each layer serves a different temporal scope and access pattern — from static rules loaded once per session to full cycle archives written at completion. Together, they give agents the right information at the right time without redundant reads or state collisions.

```
Layer 0: Shared Values    ← static, prefix-cached, read-only
Layer 1: JSONL Ledger     ← append-only, structured, permanent
Layer 2: Markdown Workspace ← per-cycle, one file per agent
Layer 3: state.json       ← persistent config + metrics
Layer 4: Instincts        ← extracted patterns with confidence
Layer 5: History Archives ← immutable cycle snapshots
```

---

## Memory Layers

### Layer 0: Shared Values

**Path:** `skills/evolve-loop/memory-protocol.md` (top section)

Static behavioral rules shared by all agents: immutability, scope discipline, blast-radius awareness, and the learning mandate. This section is placed **first** in every agent context block — because it never changes between cycles, the LLM's KV-cache retains it as a warm prefix, eliminating redundant token processing on every invocation.

Agents must not write to this layer. It is the team constitution.

### Layer 1: JSONL Ledger

**Path:** `.evolve/ledger.jsonl`

Append-only structured log. Each agent appends exactly one entry per invocation:
```jsonl
{"ts":"<ISO-8601>","cycle":<N>,"role":"<role>","type":"<type>","data":{...}}
```

The ledger is never truncated. It is the permanent audit trail. Agents do not read the full ledger directly — instead, `state.json` carries a `ledgerSummary` with aggregated stats so agents get the signal without the I/O cost.

### Layer 2: Markdown Workspace

**Path:** `.evolve/workspace/`

Human-readable files overwritten each cycle. Each agent owns exactly one file:

| File | Owner |
|------|-------|
| `scout-report.md` | Scout |
| `build-report.md` | Builder |
| `builder-notes.md` | Builder (persists across cycles) |
| `audit-report.md` | Auditor |
| `operator-log.md` | Operator |
| `agent-mailbox.md` | All agents (shared write surface) |

Agents read upstream workspace files and write only their own. `agent-mailbox.md` is the exception — all agents may append to it for cross-agent coordination.

### Layer 3: state.json

**Path:** `.evolve/state.json`

Persistent configuration and aggregated metrics that survive across cycles. Key fields:

- `research` — cached query results with a 12-hour TTL
- `evalHistory` — per-cycle pass/fail metrics (last 5 entries)
- `failedApproaches` — structured records of what was tried and why it failed
- `instinctSummary` — compact array of all active instincts (agents read this instead of YAML files)
- `planCache` — reusable plan templates for recurring task types
- `taskArms` — multi-armed bandit state for task-type selection
- `processRewards` / `processRewardsHistory` — per-phase quality scores
- `mastery` — difficulty graduation level (novice / competent / proficient)
- `tokenBudget` — per-task and per-cycle token caps
- `fileExplorationMap` — last-touched cycle per file for novelty scoring

### Layer 4: Instincts

**Path:** `.evolve/instincts/personal/`

YAML files extracted during Phase 5 learning. Each instinct captures a reusable pattern with a confidence score (starts at 0.5, increases when confirmed across cycles). Instincts fall into three types: episodic, semantic, and procedural.

- **Episodic** — what happened: specific observations from a cycle ("phases.md above 700 lines causes merge conflicts")
- **Semantic** — how things work: generalizations about the system ("append-only files are safer than in-place rewrites")
- **Procedural** — how to do things: step-by-step templates ("to add a new eval grader: write test-f check first, then grep-c check")

High-confidence instincts (0.9+) graduate to orchestrator policy — they become standing rules applied without explicit lookup.

### Layer 5: History Archives

**Path:** `.evolve/history/cycle-{N}/`

Immutable snapshots of the workspace directory taken at the end of each completed cycle. Agents do not read these during normal operation — they are the audit trail and the source data for meta-cycle retrospectives.

---

## Memory Types

| Type | Definition | Source | Example |
|------|-----------|--------|---------|
| **Episodic** | What happened in a specific cycle | Build/audit reports | "Task A failed due to absolute path mismatch in cycle 12" |
| **Semantic** | How the system works | Instinct extraction | "KV-cache hits require a stable prefix in Layer 0" |
| **Procedural** | How to perform a task | Gene capsules, instincts | "When adding a doc: write eval graders first, then content" |

---

## Abstraction Pathway

Raw observations travel up the abstraction ladder each cycle:

```
Episodic observation (what happened this cycle)
      ↓  Phase 5 extraction
Instinct (pattern with confidence score)
      ↓  confidence ≥ 0.9
Semantic knowledge (stable system understanding)
      ↓  repeated confirmation
Procedural template (reusable step sequence)
      ↓  plan cache / gene capsule
Plan cache entry (reuse in future cycles)
```

Promotion to each higher level requires repeated confirmation. A single-cycle observation becomes an instinct. An instinct confirmed across three cycles becomes semantic knowledge. A procedural pattern confirmed across five cycles becomes a gene capsule for direct reuse.

---

## Consolidation

Every 3 cycles the orchestrator runs a memory consolidation pass:

1. **Cluster** — group instincts with high semantic similarity
2. **Merge** — combine related instincts into a single higher-confidence entry
3. **Decay** — reduce confidence of instincts uncited for 3+ cycles
4. **Archive** — move stale (confidence < 0.3) instincts to history
5. **Compress** — rewrite the `notes.md` Summary section to a fixed-size paragraph

Consolidation prevents unbounded growth and keeps the instinct set focused on patterns that remain relevant.

---

## Agent Access Matrix

| Layer | Scout | Builder | Auditor | Operator | Orchestrator |
|-------|-------|---------|---------|----------|--------------|
| 0: Shared Values | read | read | read | read | read |
| 1: JSONL Ledger | append | append | append | append | append |
| 2: Workspace | read + write `scout-report.md` | read + write `build-report.md` | read + write `audit-report.md` | read + write `operator-log.md` | read all |
| 3: state.json | read + write | read | read | read + write | read + write |
| 4: Instincts | read (`instinctSummary`) | read (`instinctSummary`) | read | read | write (Phase 5) |
| 5: History | none | none | none | read (meta-cycle) | write (Phase 4) |

---

## Cross-Agent Memory Sharing

Agents share state through two channels:

**Workspace files (async, one-way):** Each agent reads the upstream file written by the previous agent. Scout writes `scout-report.md`; Builder reads it. Builder writes `build-report.md`; Auditor reads it. This is the primary coordination path.

**Agent mailbox (direct messaging):** `agent-mailbox.md` is the only file all agents may append to. Agents post structured rows with `from`, `to`, `type`, `cycle`, `persistent`, and `message` fields. Non-persistent messages are cleared in Phase 4; persistent messages survive across cycles. The mailbox replaces ad-hoc inline notes and gives inter-agent communication an inspectable record.

Example flow for a cross-agent hint:
```
Scout appends to mailbox: { from: scout, to: builder, type: hint, message: "phases.md is a hotspot — keep diff under 30 lines" }
Builder reads mailbox at Phase 2 start → applies constraint
Builder appends: { from: builder, to: auditor, type: flag, message: "eval graders use absolute paths — verify before running" }
Auditor reads mailbox at Phase 3 start → adjusts verification order
```
