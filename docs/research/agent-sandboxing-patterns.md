# Agent Sandboxing Patterns

> Reference document for agent isolation, sandboxing, and permission models.
> Apply defense-in-depth strategies to contain agent actions within defined
> boundaries across evolve-loop Scout, Builder, and Auditor phases.

## Table of Contents

1. [Three Isolation Axes](#three-isolation-axes)
2. [Capability-Based Permission Model](#capability-based-permission-model)
3. [Sandbox Implementation Strategies](#sandbox-implementation-strategies)
4. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
5. [Prior Art](#prior-art)
6. [Anti-Patterns](#anti-patterns)

---

## Three Isolation Axes

Enforce isolation along three independent axes. Layer all three for defense-in-depth.

| Axis | Scope | Mechanism | Example Controls |
|---|---|---|---|
| **Tooling** | Restrict which tools, APIs, and system calls an agent can invoke | Tool allowlists, API gateway policies, capability tokens | Scout: allow `Read`, `Grep`, `Glob`; deny `Write`, `Bash` destructive commands |
| **Host** | Isolate agent execution environment from host OS and other agents | Container namespaces, microVM boundaries, process sandboxes | Each agent runs in a separate Docker container or Firecracker microVM |
| **Network** | Control egress traffic, DNS resolution, and external API access | Egress filtering, DNS allowlists, proxy-based inspection | Allow `api.anthropic.com`; block all other outbound; deny raw socket access |

### Axis Interaction Matrix

| Combination | Risk Mitigated | Implementation Cost |
|---|---|---|
| Tooling only | Accidental misuse of privileged tools | Low |
| Tooling + Host | Tool misuse + container escape + filesystem tampering | Medium |
| Tooling + Host + Network | Full isolation: tool misuse, escape, data exfiltration | High |

---

## Capability-Based Permission Model

Assign each agent the minimum permission level required for its role. Never grant blanket admin access.

| Permission Level | Allowed Operations | Risk Level |
|---|---|---|
| **None** | No access to resource | Minimal |
| **Read-only** | Read files, query APIs, inspect state | Low |
| **Read-write** | Read + create/modify files and data | Medium |
| **Execute** | Read-write + run scripts, invoke tools, trigger builds | High |
| **Admin** | Full system access, modify permissions, override gates | Critical |

### Agent-to-Permission Mapping

| Agent | Permission Level | Justification |
|---|---|---|
| **Scout** | Read-only | Scout researches and analyzes; never modifies code or state |
| **Builder** | Read-write + Execute | Builder creates files, runs tests, invokes build tools |
| **Auditor** | Read-only | Auditor inspects artifacts and validates integrity; never modifies |
| **Orchestrator** | Execute | Orchestrator invokes agents and phase gates; no direct file writes |

### Permission Escalation Rules

| Rule | Description |
|---|---|
| Deny by default | Start with no permissions; grant explicitly per phase |
| Time-bound grants | Permissions expire at phase boundary |
| No self-escalation | An agent cannot modify its own permission set |
| Audit all escalations | Log every permission grant with timestamp and justification |

---

## Sandbox Implementation Strategies

| Strategy | Isolation Level | Startup Time | Resource Overhead | Pros | Cons | Use Cases |
|---|---|---|---|---|---|---|
| **Docker containers** | Process + filesystem namespace | 1-5s | Low-medium | Mature ecosystem, wide adoption, easy orchestration | Shared kernel, container escape risk | CI/CD pipelines, dev sandboxes, multi-agent workloads |
| **gVisor** | Application kernel in userspace | 2-5s | Medium | Strong syscall filtering, no kernel sharing | Incomplete syscall coverage, performance overhead | Untrusted workloads, multi-tenant platforms |
| **Firecracker microVMs** | Full VM-level isolation | <125ms | Low | Near-instant boot, minimal attack surface, strong isolation | Requires KVM, limited OS support | Serverless functions, high-security agent execution |
| **WASM sandboxes** | Language-level sandbox | <10ms | Minimal | Fast startup, deterministic execution, portable | Limited I/O capabilities, ecosystem maturity | Lightweight tool execution, plugin sandboxing |
| **OpenShell (NVIDIA)** | Container + policy engine | 2-10s | Medium | Built for AI agents, policy-driven access control | Early-stage, NVIDIA ecosystem dependency | GPU-accelerated agent workloads, research |
| **E2B sandbox** | Cloud microVM | 1-3s | Low (managed) | Purpose-built for AI code execution, managed service | Requires network access, vendor dependency | Rapid prototyping, hosted agent execution |

### Selection Criteria

| Factor | Docker | gVisor | Firecracker | WASM | OpenShell | E2B |
|---|---|---|---|---|---|---|
| Isolation strength | Medium | High | Very high | High | Medium-high | High |
| Startup latency | Medium | Medium | Low | Very low | Medium | Low |
| Ecosystem maturity | Very high | High | High | Medium | Low | Medium |
| AI-agent optimized | No | No | No | No | Yes | Yes |

---

## Mapping to Evolve-Loop

Map each evolve-loop mechanism to the sandboxing pattern it implements.

| Evolve-Loop Mechanism | Sandboxing Pattern | Isolation Axis | Description |
|---|---|---|---|
| **Git worktree isolation** | Code sandbox | Host | Each agent operates in a dedicated worktree; changes cannot leak across branches |
| **phase-gate.sh** | Permission boundary | Tooling | Deterministic script validates artifacts before phase transition; blocks unauthorized progression |
| **Canary files** | Tamper detection | Host | Sentinel files detect unauthorized filesystem modifications within agent workspaces |
| **Challenge tokens** | Integrity verification | Tooling | Random tokens injected into prompts verify agent actually executed (not fabricated output) |
| **Tool allowlists** | Capability restriction | Tooling | Per-agent tool configuration limits available APIs (Scout: read tools only) |
| **Workspace artifacts** | Audit trail | Tooling | scout-report.md, build-report.md, audit-report.md create verifiable execution records |

### Defense-in-Depth Stack

| Layer | Mechanism | Failure Mode Prevented |
|---|---|---|
| 1 | Tool allowlists | Agent invokes unauthorized tool |
| 2 | Git worktree isolation | Agent modifies files outside its workspace |
| 3 | phase-gate.sh validation | Agent skips required quality checks |
| 4 | Canary file monitoring | Agent tampers with sandbox boundaries |
| 5 | Challenge token verification | Agent fabricates execution without doing work |
| 6 | Artifact audit trail | Agent claims completion without evidence |

---

## Prior Art

| Source | Year | Key Contribution | Relevance |
|---|---|---|---|
| **OWASP AI Agent Security Top 10** | 2026 | Categorized top risks for autonomous AI agents including prompt injection, tool abuse, and sandbox escape | Defines threat model for agent sandboxing |
| **OpenShell (NVIDIA)** | 2025 | Policy-driven container sandbox for AI agent tool execution | Reference implementation for agent-aware sandboxing |
| **E2B Sandbox** | 2024 | Cloud microVM service purpose-built for AI code execution | Demonstrates managed sandbox-as-a-service pattern |
| **Anthropic Tool Use Safety Guidelines** | 2025 | Best practices for constraining tool invocations in Claude-based agents | Defines capability-based permission model for LLM agents |
| **Firecracker (AWS)** | 2018 | Lightweight microVM for serverless; sub-125ms boot time | Proves microVM isolation viable for ephemeral workloads |
| **gVisor (Google)** | 2018 | Application kernel intercepting syscalls in userspace | Demonstrates strong isolation without full VM overhead |
| **WASI (Bytecode Alliance)** | 2023 | WebAssembly System Interface for sandboxed I/O | Enables portable, capability-based sandboxing |

---

## Anti-Patterns

| Anti-Pattern | Description | Risk | Mitigation |
|---|---|---|---|
| **Over-permissioning** | Grant admin or execute permissions when read-only suffices | Agent modifies or deletes critical files; privilege escalation | Apply least-privilege; map each agent to minimum required permission level |
| **Sandbox escape via tool chaining** | Agent combines multiple low-privilege tools to achieve high-privilege effect | Circumvents individual tool restrictions; achieves unauthorized access | Validate tool call sequences; enforce invariants across tool chains, not just individual calls |
| **Shared-state leakage** | Multiple agents read/write the same mutable state (files, environment variables, databases) | Race conditions, data corruption, cross-agent information leakage | Isolate state per agent; use immutable handoffs between phases |
| **Missing egress controls** | Agent sandbox allows unrestricted outbound network access | Data exfiltration, command-and-control communication, dependency confusion attacks | Enforce DNS allowlists; proxy all outbound traffic; block raw sockets |
| **Static permission grants** | Permissions assigned once and never revoked or rotated | Stale permissions accumulate; compromised agent retains access indefinitely | Time-bound grants that expire at phase boundaries; re-authenticate per phase |
| **Trusting agent self-reports** | Accept agent's claim of task completion without independent verification | Agent fabricates output, skips steps, or hallucinates results | Require deterministic verification (phase-gate.sh, canary files, challenge tokens) |
| **Single-layer isolation** | Rely on only one isolation axis (e.g., tooling restrictions without host isolation) | Single point of failure; one bypass compromises entire system | Layer all three axes: tooling + host + network |
