# Documentation Index

> Reference documents that the evolve-loop agents and skills read during operation.

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
| [model-routing.md](reference/model-routing.md) | Tier-based model selection rules |
| [configuration.md](reference/configuration.md) | Configuration schema and domain detection |
| [reference/scout-discovery.md](reference/scout-discovery.md) | Codebase scanning and hotspot detection |
| [accuracy-self-correction.md](research/accuracy-self-correction.md) | CoT verification and anti-conformity checks |
| [performance-profiling.md](research/performance-profiling.md) | Token attribution and cost baselines |
| [eval-grader-best-practices.md](research/eval-grader-best-practices.md) | Eval grader precision and mutation resistance |

## Architecture

| Document | Purpose |
|----------|---------|
| [phase-architecture.md](architecture/phase-architecture.md) | Per-phase deep-dive: Calibrate → Intent → Scout → Builder → Auditor → Ship → Learn |
| [phase-architecture-citations.md](architecture/phase-architecture-citations.md) | Public-paper citations behind each phase's design |
| [platform-compatibility.md](architecture/platform-compatibility.md) | Cross-CLI support matrix + adapter contract |
| [tri-layer.md](architecture/tri-layer.md) | Skill/Persona/Command layered orchestration model |
| [capability-schema.md](architecture/capability-schema.md) | Adapter capability manifest schema + authoring guide for new CLIs |
| [intent-phase.md](architecture/intent-phase.md) | Intent capture phase + AwN classifier specification |
| [sequential-write-discipline.md](architecture/sequential-write-discipline.md) | Parallelization discipline rule (`parallel_eligible`) + concurrency cap (default 2 since v8.55.0) — when may a role fan out, and how many workers run at once |

## Release & Operations

| Document | Purpose |
|----------|---------|
| [release-protocol.md](release/release-protocol.md) | Push/tag/release/propagate vocabulary + self-healing pipeline |
| [release-archive.md](release/release-archive.md) | Per-version implementation notes (v8.21–current) |
| [release-notes/index.md](release-notes/index.md) | Per-version release-notes index |

## Research Notes

| Document | Purpose |
|----------|---------|
| [evaluator-research.md](research/evaluator-research.md) | Evaluator agent design rationale |
| [token-optimization-guide.md](research/token-optimization-guide.md) | Per-cycle token + cost optimization |
| [research-index.md](research-index.md) | Full research-paper index |

## Reports

| Document | Purpose |
|----------|---------|
| [code-review-simplify-solution.md](reports/code-review-simplify-solution.md) | Code-review-simplify integration solution |
| [inspirer-solution.md](reports/inspirer-solution.md) | Inspirer agent integration solution |

## Incident Reports

| Report | Summary |
|--------|---------|
| [incidents/cycle-102-111.md](incidents/cycle-102-111.md) | Reward hacking via tautological evals |
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
