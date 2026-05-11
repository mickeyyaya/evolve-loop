# Research Index

> Reference documents available to evolve-loop. The split:
>
> - **Active references** (5 files, in `docs/research/`) — cited by runtime
>   personas/skills/scripts; loaded into agent context during cycles.
> - **Archived references** (42 files, in `docs/private/research/`) — for
>   contributor reference; explicitly excluded from agent context via the
>   trust kernel (deny_subpaths + Layer-B filter). See
>   [docs/architecture/private-context-policy.md](architecture/private-context-policy.md)
>   for the convention.

---

## Summary Statistics

| Bucket | Path | Documents | LOC |
|---|---|---|---|
| Active | `docs/research/` | 5 | 1,220 |
| Archived | `docs/private/research/` | 42 | 7,737 |
| **Total available** | — | **47** | **8,957** |

---

## Active Reference Documents

These load into agent runtime context. Cited by the listed runtime artifact.

| Doc | Purpose | Used By |
|-----|---------|---------|
| [accuracy-self-correction.md](research/accuracy-self-correction.md) | CoT verification and anti-conformity checks | evolve-auditor.md |
| [eval-grader-best-practices.md](research/eval-grader-best-practices.md) | Eval grader precision and mutation resistance | eval-runner.md |
| [evaluator-research.md](research/evaluator-research.md) | Evaluator agent design rationale — 14 papers, 8 benchmarks | evaluator/SKILL.md |
| [performance-profiling.md](research/performance-profiling.md) | Token attribution and cost baselines | docs/index.md |
| [token-optimization-guide.md](research/token-optimization-guide.md) | Per-cycle token + cost optimization | docs/index.md |

---

## Archived Research (in `docs/private/research/`)

Restored verbatim from commit `35b31c4^` (cycle 13's parent). Cycle 13
correctly deleted these from runtime context per Liu et al. 2023 "Lost in
the Middle"; v9.1.x re-introduced them as developer-only reference under
`docs/private/`. Agents never see these during cycles. Contributors
read them directly.

Grouped by theme:

### Agent architecture & capabilities

| File | Topic |
|---|---|
| [agent-capability-benchmarking.md](private/research/agent-capability-benchmarking.md) | Capability measurement frameworks |
| [agent-role-specialization.md](private/research/agent-role-specialization.md) | Role decomposition patterns |
| [agent-skill-composition.md](private/research/agent-skill-composition.md) | Skill composition + selection |
| [agent-state-persistence.md](private/research/agent-state-persistence.md) | State models, durable execution patterns |
| [agent-testing-frameworks.md](private/research/agent-testing-frameworks.md) | Test harness patterns for agents |
| [emergent-agent-behaviors.md](private/research/emergent-agent-behaviors.md) | Emergence + unintended capabilities |
| [agent-lifecycle-management.md](private/research/agent-lifecycle-management.md) | Lifecycle stages + transitions |

### Multi-agent systems & coordination

| File | Topic |
|---|---|
| [agent-collaboration-games.md](private/research/agent-collaboration-games.md) | Multi-agent interaction games |
| [agent-orchestration-anti-patterns.md](private/research/agent-orchestration-anti-patterns.md) | Anti-patterns in orchestration |
| [multi-agent-blackboard.md](private/research/multi-agent-blackboard.md) | Blackboard / shared-state pattern |
| [reasoning-orchestration-patterns.md](private/research/reasoning-orchestration-patterns.md) | Reasoning chains across agents |

### Autonomous loops & self-improvement

| File | Topic |
|---|---|
| [autonomous-experiment-loops.md](private/research/autonomous-experiment-loops.md) | Self-improving loop protocols |
| [self-evolving-tool-creation.md](private/research/self-evolving-tool-creation.md) | Tool/gene library evolution |
| [self-healing-agents.md](private/research/self-healing-agents.md) | Recovery + self-repair |
| [prompt-evolution-optimization.md](private/research/prompt-evolution-optimization.md) | Promptbreeder-style evolution |

### Economics & deployment

| File | Topic |
|---|---|
| [agent-economics.md](private/research/agent-economics.md) | Unit economics, cost amplification |
| [agent-deployment-patterns.md](private/research/agent-deployment-patterns.md) | Production deployment shapes |
| [agent-config-versioning.md](private/research/agent-config-versioning.md) | Config versioning + migration |
| [token-cost-optimization.md](private/research/token-cost-optimization.md) | Token-budget patterns |

### Trust, safety, governance

| File | Topic |
|---|---|
| [agent-governance-compliance.md](private/research/agent-governance-compliance.md) | Compliance + governance frameworks |
| [agent-interpretability.md](private/research/agent-interpretability.md) | Interpretability techniques |
| [agent-output-validation.md](private/research/agent-output-validation.md) | Output validation strategies |
| [agent-sandboxing-patterns.md](private/research/agent-sandboxing-patterns.md) | Sandboxing approaches |
| [reward-hacking-prevention.md](private/research/reward-hacking-prevention.md) | Reward-hacking detection + prevention |
| [hitl-trust-calibration.md](private/research/hitl-trust-calibration.md) | Human-in-the-loop trust calibration |

### Memory, context, retrieval

| File | Topic |
|---|---|
| [memory-consolidation-pipeline.md](private/research/memory-consolidation-pipeline.md) | Memory consolidation across cycles |
| [agentic-rag-patterns.md](private/research/agentic-rag-patterns.md) | RAG patterns for agents |
| [context-engineering-patterns.md](private/research/context-engineering-patterns.md) | Context engineering techniques |
| [long-context-agent-strategies.md](private/research/long-context-agent-strategies.md) | Long-context utilization |
| [knowledge-distillation-agents.md](private/research/knowledge-distillation-agents.md) | Distillation for agent systems |

### Interfaces & ecosystem

| File | Topic |
|---|---|
| [agentic-ide-integration.md](private/research/agentic-ide-integration.md) | IDE integration patterns |
| [agent-interoperability-protocols.md](private/research/agent-interoperability-protocols.md) | A2A / MCP-style protocols |
| [agentic-systems-roadmap.md](private/research/agentic-systems-roadmap.md) | Ecosystem roadmap notes |
| [ai-code-review-agents.md](private/research/ai-code-review-agents.md) | Code review agent designs |
| [workflow-dag-patterns.md](private/research/workflow-dag-patterns.md) | Workflow DAG patterns |

### Code generation & refactoring

| File | Topic |
|---|---|
| [code-correctness-verification.md](private/research/code-correctness-verification.md) | Correctness verification |
| [constrained-decoding-patterns.md](private/research/constrained-decoding-patterns.md) | Constrained decoding |
| [refactoring-llm-research.md](private/research/refactoring-llm-research.md) | LLM-driven refactoring research |
| [refactoring-pipeline-architecture.md](private/research/refactoring-pipeline-architecture.md) | Refactoring pipeline shape |
| [refactoring-tools-landscape.md](private/research/refactoring-tools-landscape.md) | Tool landscape |
| [synthetic-data-generation.md](private/research/synthetic-data-generation.md) | Synthetic data techniques |
| [agent-failure-tracing.md](private/research/agent-failure-tracing.md) | Failure tracing + classification |
