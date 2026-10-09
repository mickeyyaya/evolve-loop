# Documentation Index

> These are the reference documents that the evolve-loop agents and skills read during operation.

## Agent Technique References

| Phase | Document | Agent |
|-------|----------|-------|
| 1 — DISCOVER | [reference/scout-techniques.md](reference/scout-techniques.md) | Scout |
| 2 — BUILD | [reference/builder-techniques.md](reference/builder-techniques.md) | Builder |
| 3 — AUDIT | [reference/auditor-techniques.md](reference/auditor-techniques.md) | Auditor |
| 4-5 — SHIP/LEARN | [reference/orchestrator-techniques.md](reference/orchestrator-techniques.md) | Orchestrator/Operator |

## Core References

| Document | Purpose |
|----------|---------|
| [genes.md](reference/genes.md) | Gene/capsule fix template format and usage |
| [instincts.md](reference/instincts.md) | Instinct lifecycle, graduation, and memory operations |
| [model-routing.md](reference/model-routing.md) | The tier ladder and the `cli_routing` table |
| [configuration.md](reference/configuration.md) | Configuration schema and domain detection |
| [reference/scout-discovery.md](reference/scout-discovery.md) | Codebase scans and hotspot detection |
| [accuracy-self-correction.md](private/research/archived-2026-05-19/accuracy-self-correction.md) | CoT verification and anti-conformity checks |
| [performance-profiling.md](private/research/archived-2026-05-19/performance-profiling.md) | Token attribution and cost baselines |
| [eval-grader-best-practices.md](eval-grader-best-practices.md) | Eval grader precision and mutation resistance |

## Architecture

| Document | Purpose |
|----------|---------|
| [phase-architecture.md](architecture/phase-architecture.md) | Per-phase deep-dive: Calibrate → Intent → Scout → Builder → Auditor → Ship → Learn |
| [phase-architecture-citations.md](architecture/phase-architecture-citations.md) | Public-paper citations behind the design of each phase |
| [platform-compatibility.md](architecture/platform-compatibility.md) | Cross-CLI support matrix + adapter contract |
| [tri-layer.md](architecture/tri-layer.md) | Skill/Persona/Command layered orchestration model |
| [capability-schema.md](architecture/capability-schema.md) | The schema of the adapter capability manifest + a guide to author the manifest for a new CLI |
| [intent-phase.md](architecture/intent-phase.md) | Intent capture phase + AwN classifier specification |
| [sequential-write-discipline.md](architecture/sequential-write-discipline.md) | Parallelization discipline rule (`parallel_eligible`) + concurrency cap (default 2 since v8.55.0) — when a role can fan out, and how many workers run at once |
| [build-explanation-contract.md](architecture/build-explanation-contract.md) | Builder-authored rationale deliverable, provenance binding, and Audit/Ship/Retro verification lifecycle |
| [review-loop-and-quality-index.md](architecture/review-loop-and-quality-index.md) | The code-review phase and the shared quality index (ADR-0124, in shadow since v22.27.0) |
| [convergence-policy.md](architecture/convergence-policy.md) | The rule that makes each loop that repeats converge or escalate (ADR-0126, in shadow since v22.27.0) |
| [event-channels.md](architecture/event-channels.md) | The push-only publish/subscribe protocol: channel logs, cursors, the kernel wake and `evolve events` subscriptions (ADR-0127, proposed) |
| [fleet-landing-queue.md](architecture/fleet-landing-queue.md) | The landing queue for concurrent fleet lanes: the overlap proof, the tiers T1 to T4, the scoped gates and the review (ADR-0128, proposed) |

## Release & Operations

| Document | Purpose |
|----------|---------|
| [publishing-releases.md](guides/publishing-releases.md) | Push/tag/release/propagate vocabulary + the pipeline that heals itself |
| [release-archive.md](operations/release-archive.md) | Per-version implementation notes (v8.21–current) |
| [release-notes/index.md](operations/release-notes/index.md) | Per-version release-notes index |

## Research Notes

| Document | Purpose |
|----------|---------|
| [evaluator-research.md](private/research/archived-2026-05-19/evaluator-research.md) | Evaluator agent design rationale |
| [token-optimization-guide.md](private/research/archived-2026-05-19/token-optimization-guide.md) | Per-cycle token + cost optimization |
| [event-notification-protocol-2026-10.md](research/event-notification-protocol-2026-10.md) | Research for ADR-0127: kernel file notification, the cost of local IPC, and topic and channel prior art |
| [concurrent-cycle-landing-2026-10.md](research/concurrent-cycle-landing-2026-10.md) | Research for ADR-0128: the rebase census, the cycle 1843 finding, why the prefix queue is off, prior art and the cost model |
| [research-index.md](research-index.md) | Full research-paper index |
| [research/](research-index.md) | Merged research tree (2026-08-05): research packages + single-file notes from the former `kb/` and `knowledge-base/research/` roots |
| [chronicle/README.md](chronicle/README.md) | Engineering chronicle — workstream-level narratives (problem / approaches / decision / results / retro) |

## Reports

| Document | Purpose |
|----------|---------|
| [code-review-simplify-solution.md](reports/code-review-simplify-solution.md) | Code-review-simplify integration solution |
| [inspirer-solution.md](reports/inspirer-solution.md) | Inspirer agent integration solution |

## Incident Reports

| Report | Summary |
|--------|---------|
| [incidents/cycle-102-111.md](incidents/cycle-102-111.md) | Reward hacking through tautological evals |
| [incidents/cycle-132-141.md](incidents/cycle-132-141.md) | Orchestrator gaming — skipped agents, fabricated cycles |
| [incidents/gemini-forgery.md](incidents/gemini-forgery.md) | Cross-platform audit forgery |

## Architecture Decision Records

| ADR | Title | Cycle | Status |
|-----|-------|-------|--------|
| [ADR 0001](adr/0001-plugin-dir-resolution.md) | Plugin-Dir Resolution | 23 | Accepted |
| [ADR 0002](adr/0002-disable-slash-commands-semantics.md) | Disable-Slash-Commands Semantics | 23 | Accepted |
| [ADR 0003](adr/0003-layer-3-persona-split-pattern.md) | Layer-3 Persona Split Pattern | 24 | Accepted |
| [ADR 0004](adr/0004-context-md-adoption.md) | CONTEXT.md Canonical Glossary Adoption | 24 | Accepted |
| [ADR 0005](adr/0005-tsc-application.md) | TSC Application to Persona Files | 24–26 | Accepted |
| [ADR 0006](adr/0006-layer-p-memo-handoff-template.md) | Layer-P Memo Phase Contract | 24–25 | Accepted |
| [ADR 0007](adr/0007-inbox-injection-protocol.md) | Inbox-Injection Protocol | 27 | Accepted |
| [ADR 0009](adr/0009-phase-handoff-schemas.md) | Phase Handoff Schemas (C2) | 63 | Accepted |
