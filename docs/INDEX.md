# Documentation Index

> Cross-document navigation for the evolve-loop documentation system. Maps concepts to their canonical (primary) document and secondary references.

## Core Concepts

| Concept | Primary Document | Secondary References |
|---------|-----------------|---------------------|
| Instinct lifecycle (creation → graduation → promotion) | [instincts.md](instincts.md) § Instinct Lifecycle Gates | [self-learning.md](self-learning.md) § f, [phase5-learn.md](../skills/evolve-loop/phase5-learn.md) § Instinct Graduation |
| Memory operations (dormant/decay/forgetting) | [instincts.md](instincts.md) § Memory Operations | [self-learning.md](self-learning.md) § 3-4, [ref-orchestrator-techniques.md](reference-orchestrator-techniques.md) § Instinct Forgetting |
| Instinct trust governance (external instincts) | [ref-orchestrator-techniques.md](reference-orchestrator-techniques.md) § Instinct Trust Governance | [instincts.md](instincts.md) § Lifecycle Gates |
| Gene templates (fix capsules) | [genes.md](genes.md) | [ref-orchestrator-techniques.md](reference-orchestrator-techniques.md) § Gene Self-Play |
| Skill building (instinct → skill graduation) | [skill-building.md](skill-building.md) | [self-learning.md](self-learning.md) |
| Memory hierarchy (6 layers) | [memory-hierarchy.md](memory-hierarchy.md) | [persistent-memory-architecture.md](persistent-memory-architecture.md) |

## Evaluation Frameworks

| Concept | Primary Document | Secondary References |
|---------|-----------------|---------------------|
| LLM-as-a-Judge (4D: Correctness, Completeness, Novelty, Efficiency) | [phase5-learn.md](../skills/evolve-loop/phase5-learn.md) § Self-Evaluation | [self-learning.md](self-learning.md) § b |
| Process rewards (per-step Builder confidence) | [phase5-learn.md](../skills/evolve-loop/phase5-learn.md) § Step-Level Process Rewards | [performance-profiling.md](performance-profiling.md) § processRewardsHistory |
| CLEAR framework (5D enterprise eval — reference only) | [enterprise-agent-evaluation.md](enterprise-agent-evaluation.md) | — |
| Eval Rigor Levels (Rigor-L0 through L3 — grader quality) | [adversarial-eval-coevolution.md](adversarial-eval-coevolution.md) § Composite Reward | [eval-grader-best-practices.md](eval-grader-best-practices.md) |
| Security Detection Layers (SecLayer-L1 through L4 — vuln detection) | [secure-code-generation.md](secure-code-generation.md) § Security Eval Grader | [ref-builder-techniques.md](reference-builder-techniques.md) § Secure Code Patterns |
| CSI (Coefficient of Self-Improvement) | [self-learning.md](self-learning.md) § h | [ref-orchestrator-techniques.md](reference-orchestrator-techniques.md) § CSI |
| Confidence-correctness alignment | [self-learning.md](self-learning.md) § i | [ref-orchestrator-techniques.md](reference-orchestrator-techniques.md) § Confidence-Correctness |

## Phase-Specific Agent Techniques

| Phase | Reference Document | Agent |
|-------|-------------------|-------|
| 1 — DISCOVER | [reference-scout-techniques.md](reference-scout-techniques.md) | Scout |
| 2 — BUILD | [reference-builder-techniques.md](reference-builder-techniques.md) | Builder |
| 3 — AUDIT | [reference-auditor-techniques.md](reference-auditor-techniques.md) | Auditor |
| 4-5 — SHIP/LEARN | [reference-orchestrator-techniques.md](reference-orchestrator-techniques.md) | Orchestrator/Operator |

## Security Documentation Map

| Concern | Document |
|---------|----------|
| Pipeline integrity (eval tamper, state corruption) | [security-considerations.md](security-considerations.md) |
| Pre-execution enforcement (AgentSpec, AEGIS) | [runtime-guardrails.md](runtime-guardrails.md) |
| Code-level security (SecLayer-L1 through L4) | [secure-code-generation.md](secure-code-generation.md) |
| Audit-phase security checks | [ref-auditor-techniques.md](reference-auditor-techniques.md) § Threat Taxonomy, Runtime Guardrail |
| Anti-gaming (reward hacking, defense layers) | [agentic-reward-hacking.md](agentic-reward-hacking.md) |
| Anti-conformity audit (Free-MAD) | [accuracy-self-correction.md](accuracy-self-correction.md) § Anti-Conformity |

## Accuracy & Self-Correction

| Concept | Primary Document | Secondary References |
|---------|-----------------|---------------------|
| Anti-conformity (Free-MAD) | [accuracy-self-correction.md](accuracy-self-correction.md) § Anti-Conformity | [ref-auditor-techniques.md](reference-auditor-techniques.md) § Anti-Conformity Check |
| Error taxonomy (AgentDebug 5D) | [accuracy-self-correction.md](accuracy-self-correction.md) § Agent Error Taxonomy | [ref-builder-techniques.md](reference-builder-techniques.md) § Targeted Error Recovery |
| Uncertainty gating (AUQ) | [accuracy-self-correction.md](accuracy-self-correction.md) § Uncertainty | [ref-builder-techniques.md](reference-builder-techniques.md) § Uncertainty Gating |
| Multi-agent reflection (MAR) | [adversarial-eval-coevolution.md](adversarial-eval-coevolution.md) § MAR | [accuracy-self-correction.md](accuracy-self-correction.md) § Anti-Conformity |

## Token Optimization & Cost

| Concept | Primary Document | Secondary References |
|---------|-----------------|---------------------|
| Token reduction mechanisms (9 techniques) | [token-optimization.md](token-optimization.md) | — |
| BATS budget-aware scaling | [ref-builder-techniques.md](reference-builder-techniques.md) § Budget-Aware | [performance-profiling.md](performance-profiling.md) § Budget-Aware, [token-optimization.md](token-optimization.md) |
| Model routing (tier system) | [model-routing.md](model-routing.md) | [performance-profiling.md](performance-profiling.md) § Model Routing |
| Plan template caching | [self-learning.md](self-learning.md) § d | [token-optimization.md](token-optimization.md) |
| Performance profiling | [performance-profiling.md](performance-profiling.md) | — |

## Multi-Agent Coordination

| Concept | Primary Document | Secondary References |
|---------|-----------------|---------------------|
| Topology (sequential, parallel, DAG) | [multi-agent-coordination.md](multi-agent-coordination.md) | [ref-orchestrator-techniques.md](reference-orchestrator-techniques.md) § Multi-Agent Coordination |
| Agent observability (logging, tracing) | [agent-observability.md](agent-observability.md) | — |
| Island model (parallel evolution) | [island-model.md](island-model.md) | — |
| Operator brief | [operator-brief.md](operator-brief.md) | — |

## Research & Incidents

| Concept | Primary Document |
|---------|-----------------|
| Research paper index (40+ papers) | [research-paper-index.md](research-paper-index.md) |
| Incident: cycles 102-111 (reward hacking) | [incident-report-cycle-102-111.md](incident-report-cycle-102-111.md) |
| Incident: cycles 132-141 (orchestrator gaming) | [incident-report-cycle-132-141.md](incident-report-cycle-132-141.md) |
| Incident: Gemini forgery | [incident-report-gemini-forgery.md](incident-report-gemini-forgery.md) |

## Architecture & Configuration

| Concept | Primary Document |
|---------|-----------------|
| System architecture | [architecture.md](architecture.md) |
| Configuration schema | [configuration.md](configuration.md) |
| Domain adapters | [domain-adapters.md](domain-adapters.md) |
| Platform compatibility | [platform-compatibility.md](platform-compatibility.md) |
| Run isolation | [run-isolation.md](run-isolation.md) |
| Parallel safety | [parallel-safety.md](parallel-safety.md) |
| Writing new agents | [writing-agents.md](writing-agents.md) |
