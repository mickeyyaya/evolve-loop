# Agent Interoperability Protocols

> Reference document for standardized agent-to-agent communication protocols.
> Use this guide to select, implement, and enforce typed contracts between agents.
> Apply to evolve-loop to formalize phase boundaries as protocol-compliant handoffs.

## Table of Contents

1. [Protocol Landscape](#protocol-landscape)
2. [Typed Phase Contracts](#typed-phase-contracts)
3. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
4. [Implementation Patterns](#implementation-patterns)
5. [Prior Art](#prior-art)
6. [Anti-Patterns](#anti-patterns)

---

## Protocol Landscape

| Dimension | MCP (Model Context Protocol) | A2A (Agent-to-Agent) | ACP (Agent Communication Protocol) | ANP (Agent Network Protocol) |
|---|---|---|---|---|
| **Origin** | Anthropic (2024) | Google DeepMind (2025) | IBM Research (2025) | Community / W3C-adjacent (2025) |
| **Transport** | JSON-RPC over stdio, SSE, or HTTP | HTTP/REST + Server-Sent Events | HTTP/REST + WebSocket | HTTP/REST + DIDComm |
| **Message format** | JSON-RPC 2.0 with typed tool schemas | JSON with AgentCard + Task objects | JSON-LD with capability descriptors | JSON-LD with verifiable credentials |
| **Discovery** | Client-configured server list; no built-in registry | `/.well-known/agent.json` AgentCard endpoint | Capability-based registry with semantic matching | Decentralized DID-based agent registry |
| **Authentication** | Delegated to transport (API keys, OAuth) | OAuth 2.0 with push-based token exchange | mTLS + capability tokens | DID-based mutual authentication |
| **State model** | Stateless tool calls; client manages context | Stateful tasks with lifecycle (submitted, working, done) | Stateful sessions with capability negotiation | Stateless messages with correlation IDs |
| **Streaming** | SSE for server-to-client notifications | SSE for partial task results | WebSocket for bidirectional streaming | Not specified; transport-agnostic |
| **Multi-agent** | Hub-and-spoke (client calls multiple servers) | Peer-to-peer with task delegation | Peer-to-peer with capability routing | Mesh network with relay nodes |
| **Best for** | Tool integration; extending a single agent | Cross-platform agent collaboration | Enterprise agent orchestration | Open, decentralized agent networks |

### Protocol Selection Criteria

| Criterion | Recommendation |
|---|---|
| Single agent extending capabilities | Use MCP — lowest friction, widest adoption |
| Two known agents collaborating | Use A2A — built-in task lifecycle and streaming |
| Enterprise fleet with capability routing | Use ACP — semantic matching and mTLS |
| Open network with untrusted agents | Use ANP — DID-based identity and verification |
| Evolve-loop internal coordination | Use file-based contracts (see below) — zero-infrastructure overhead |

---

## Typed Phase Contracts

Define explicit input/output schemas at every phase boundary. Each contract specifies the required fields, types, and validation rules.

### Scout to Builder Contract

| Field | Type | Required | Description |
|---|---|---|---|
| `task_id` | string | yes | Unique identifier for the cycle task |
| `task_description` | string | yes | Human-readable task summary |
| `task_type` | enum(`feat`, `fix`, `refactor`, `perf`, `docs`) | yes | Classification driving Builder strategy |
| `target_files` | string[] | yes | File paths the Builder must modify |
| `acceptance_criteria` | string[] | yes | Testable conditions for Auditor to verify |
| `context_refs` | string[] | no | Paths to related docs, prior reports, or code |
| `priority` | enum(`critical`, `high`, `medium`, `low`) | no | Urgency signal for scheduling |
| `scout_confidence` | number (0-1) | no | Scout self-assessed confidence in the task |

### Builder to Auditor Contract

| Field | Type | Required | Description |
|---|---|---|---|
| `task_id` | string | yes | Must match Scout's task_id |
| `files_changed` | string[] | yes | Paths actually modified by Builder |
| `test_results` | object | yes | Test runner output (pass count, fail count, coverage) |
| `build_status` | enum(`success`, `partial`, `failed`) | yes | Overall build outcome |
| `commit_sha` | string | no | Git SHA if Builder committed |
| `implementation_notes` | string | no | Builder rationale and design decisions |
| `risks_identified` | string[] | no | Known risks for Auditor to evaluate |

### Auditor to Orchestrator Contract

| Field | Type | Required | Description |
|---|---|---|---|
| `task_id` | string | yes | Must match upstream task_id |
| `verdict` | enum(`pass`, `fail`, `pass_with_warnings`) | yes | Gate decision |
| `checks_run` | object[] | yes | Array of {name, result, detail} check records |
| `blocking_issues` | string[] | no | Issues that caused a `fail` verdict |
| `warnings` | string[] | no | Non-blocking concerns |
| `score` | number (0-100) | no | Composite quality score |

---

## Mapping to Evolve-Loop

Map evolve-loop's existing coordination mechanisms to formal protocol concepts.

| Protocol Concept | Evolve-Loop Mechanism | File / Location |
|---|---|---|
| **Phase contract** | `handoff-*.json` files passed between phases | `workspace/handoff-scout.json`, `workspace/handoff-builder.json` |
| **Message passing** | Agent mailbox pattern via task artifacts | `workspace/*.md` (scout-report, build-report, audit-report) |
| **Service discovery** | Agent role declarations in CLAUDE.md | `CLAUDE.md`, `agents/*.md` |
| **Shared values** | Protocol baseline enforced across all agents | `docs/instincts.md`, `docs/genes.md` |
| **Event log** | Ledger recording all phase transitions | `ledger.jsonl` |
| **Authentication** | Not applicable (single-machine, single-user) | N/A |
| **Schema registry** | Contract definitions in reference docs | `docs/reference/` |
| **Health check** | Phase-gate script validating artifacts | `scripts/phase-gate.sh` |

### Handoff Flow

```
Scout ──handoff-scout.json──▶ phase-gate.sh ──▶ Builder
Builder ──handoff-builder.json──▶ phase-gate.sh ──▶ Auditor
Auditor ──audit-report.md──▶ phase-gate.sh ──▶ Orchestrator (ship/learn)
```

Enforce: every arrow passes through `phase-gate.sh`. No direct agent-to-agent calls bypass the gate.

---

## Implementation Patterns

### Schema Validation at Phase Boundaries

| Step | Action | Tool |
|---|---|---|
| 1 | Define JSON Schema for each contract | `schemas/handoff-scout.schema.json` |
| 2 | Validate handoff file against schema before phase transition | `ajv` or `jsonschema` CLI |
| 3 | Reject transition on validation failure | `phase-gate.sh` exits non-zero |
| 4 | Log validation result to ledger | Append to `ledger.jsonl` |

### Contract Versioning

| Strategy | Description | When to Use |
|---|---|---|
| **Additive-only** | Add new optional fields; never remove or rename existing fields | Default strategy for all contracts |
| **Version header** | Include `"contract_version": "1.2"` in every handoff | When breaking changes are unavoidable |
| **Dual-read** | Consumer reads both v1 and v2 fields during migration window | During version transitions |
| **Sunset period** | Deprecate old fields for N cycles before removal | Minimum 10 cycles before removal |

### Backward Compatibility Rules

| Rule | Rationale |
|---|---|
| New fields must be optional with defaults | Older producers must not break newer consumers |
| Never change a field's type | Type changes break all existing validators |
| Never remove a required field | Removal breaks all existing consumers |
| Use `additionalProperties: true` in schemas | Allow forward-compatible extensions |
| Pin `contract_version` in every handoff | Enable consumers to branch on version |

### Error Handling at Boundaries

| Error Type | Response | Recovery |
|---|---|---|
| Missing required field | Reject handoff; return to previous phase | Producer agent re-runs with error context |
| Type mismatch | Reject handoff; log schema violation | Producer agent corrects output format |
| Unknown fields | Accept handoff; log warning | No action required (forward compatibility) |
| Stale contract version | Accept if within sunset period; reject otherwise | Producer agent upgrades to current schema |

---

## Prior Art

| Protocol / Framework | Origin | Year | Key Contribution | Relevance to Evolve-Loop |
|---|---|---|---|---|
| **MCP** | Anthropic | 2024 | Standardized tool integration for LLM agents via JSON-RPC | Foundation for agent-tool communication; usable as transport |
| **A2A** | Google DeepMind | 2025 | Task-based agent collaboration with AgentCard discovery | Task lifecycle model maps to evolve-loop phases |
| **ACP** | IBM Research | 2025 | Capability-based routing with semantic matching | Inspiration for role-based agent dispatch |
| **FIPA-ACL** | FIPA/IEEE | 2002 | Formal agent communication language with performatives | Foundational semantics (request, inform, propose, refuse) |
| **Contract-Net Protocol** | Smith (1980) | 1980 | Task announcement, bidding, and awarding among agents | Model for Scout task selection and Builder assignment |
| **KQML** | DARPA | 1993 | Knowledge Query and Manipulation Language for agents | Early typed message envelope design |
| **Blackboard Architecture** | Erman et al. | 1980 | Shared-state coordination via typed knowledge sources | Evolve-loop workspace as proto-blackboard (see `multi-agent-blackboard.md`) |

---

## Anti-Patterns

| Anti-Pattern | Description | Consequence | Mitigation |
|---|---|---|---|
| **Untyped messages** | Pass free-form text or unstructured JSON between agents | Silent failures, misinterpretation, debugging difficulty | Define JSON Schema for every handoff; validate at phase gate |
| **Version drift** | Producer and consumer use different contract versions without negotiation | Deserialization errors, missing fields, corrupt state | Pin contract_version; enforce sunset periods; validate on read |
| **Chatty protocols** | Agents exchange many small messages per phase transition | Token waste, latency, context window bloat | Batch all phase data into a single handoff document |
| **Tight coupling via shared state** | Agents read/write the same mutable file without coordination | Race conditions, lost updates, non-reproducible runs | Use immutable handoff files; each phase writes new artifacts |
| **Implicit contracts** | Phase expectations live only in agent prompts, not in schemas | Contract violations undetectable until runtime failure | Extract contracts to explicit schema files; test with fixtures |
| **Missing phase gate** | Agents hand off directly without validation checkpoint | Invalid data propagates downstream; audit trail gaps | Route every transition through `phase-gate.sh` |
| **Monolithic handoff** | Single handoff file contains data for all downstream phases | Unnecessary coupling; changes to one phase break all consumers | Scope each handoff to exactly one phase boundary |
| **Silent field deprecation** | Remove fields without warning or migration path | Downstream agents crash or produce incorrect results | Announce deprecation in contract changelog; enforce sunset period |
