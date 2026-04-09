# Research Index

> Master index of all research documentation. Use to navigate the knowledge base.

---

## Summary Statistics

| Metric | Value |
|--------|-------|
| Total documents | 52 |
| Total lines | 10,128 |
| Categories | 10 |
| Subdirectories | `reference/` (4 docs), `incidents/` (3 docs) |

---

## By Category

### Agent Architecture

Role design, multi-agent coordination, and orchestration patterns.

| Doc | Key Topic | Lines |
|-----|-----------|-------|
| [agent-role-specialization.md](agent-role-specialization.md) | Single-responsibility agent personas and hierarchical architectures | 191 |
| [multi-agent-blackboard.md](multi-agent-blackboard.md) | Blackboard coordination pattern for shared structured state | 159 |
| [agent-orchestration-anti-patterns.md](agent-orchestration-anti-patterns.md) | Catalog of orchestration mistakes with detection heuristics | 98 |
| [agent-skill-composition.md](agent-skill-composition.md) | Combining atomic skills into complex agent behaviors | 196 |
| [self-evolving-tool-creation.md](self-evolving-tool-creation.md) | Agents detecting capability gaps and creating tools at runtime | 119 |
| [model-routing.md](model-routing.md) | 3-tier model abstraction and dynamic routing per phase | 86 |
| [emergent-agent-behaviors.md](emergent-agent-behaviors.md) | Taxonomy and containment of unexpected agent capabilities | 184 |

### Agent Safety & Security

Sandboxing, guardrails, governance, and reward hacking prevention.

| Doc | Key Topic | Lines |
|-----|-----------|-------|
| [agent-sandboxing-patterns.md](agent-sandboxing-patterns.md) | Isolation and permission models for agent containment | 142 |
| [agent-governance-compliance.md](agent-governance-compliance.md) | Governance frameworks, audit trails, regulatory readiness | 174 |
| [reward-hacking-prevention.md](reward-hacking-prevention.md) | Detecting and preventing specification gaming | 138 |
| [agent-interpretability.md](agent-interpretability.md) | Making agent decisions explainable with structured traces | 206 |
| [incidents/cycle-102-111.md](incidents/cycle-102-111.md) | Reward hacking incident during autonomous cycles | 38 |
| [incidents/cycle-132-141.md](incidents/cycle-132-141.md) | Orchestrator gaming incident and remediation | 302 |
| [incidents/gemini-forgery.md](incidents/gemini-forgery.md) | Multi-vector forgery attack and remediation | 188 |

### Agent Memory & Learning

Memory consolidation, instincts, genes, and knowledge distillation.

| Doc | Key Topic | Lines |
|-----|-----------|-------|
| [memory-consolidation-pipeline.md](memory-consolidation-pipeline.md) | Three-tier episodic/semantic/procedural memory model | 122 |
| [instincts.md](instincts.md) | Instinct system for learning actionable patterns per cycle | 220 |
| [genes.md](genes.md) | Reusable fix templates with executable steps and validation | 112 |
| [knowledge-distillation-agents.md](knowledge-distillation-agents.md) | Compressing reasoning traces into reusable compact formats | 139 |

### Agent Evaluation

Benchmarking, testing, output validation, and correctness verification.

| Doc | Key Topic | Lines |
|-----|-----------|-------|
| [agent-capability-benchmarking.md](agent-capability-benchmarking.md) | Multi-dimensional scoring to detect regressions | 185 |
| [agent-testing-frameworks.md](agent-testing-frameworks.md) | Systematic testing approaches for agent behavior | 188 |
| [agent-output-validation.md](agent-output-validation.md) | Deterministic and LLM-as-Judge validation layers | 172 |
| [eval-grader-best-practices.md](eval-grader-best-practices.md) | Designing bash eval graders that distinguish correct from incorrect | 197 |
| [code-correctness-verification.md](code-correctness-verification.md) | Verification techniques from unit tests to formal methods | 156 |
| [accuracy-self-correction.md](accuracy-self-correction.md) | Techniques for improving output accuracy and catching errors | 241 |
| [synthetic-data-generation.md](synthetic-data-generation.md) | Multi-agent pipelines for eval bootstrapping datasets | 171 |

### Agent Operations

Deployment, lifecycle, economics, and state persistence.

| Doc | Key Topic | Lines |
|-----|-----------|-------|
| [agent-deployment-patterns.md](agent-deployment-patterns.md) | Blue-green, canary, rolling, shadow deployment strategies | 184 |
| [agent-lifecycle-management.md](agent-lifecycle-management.md) | Seven-stage lifecycle from creation to retirement | 201 |
| [agent-economics.md](agent-economics.md) | Cost modeling, ROI measurement, budget allocation | 280 |
| [agent-state-persistence.md](agent-state-persistence.md) | Checkpointing and state management for long-running agents | 202 |
| [agent-failure-tracing.md](agent-failure-tracing.md) | Debugging and tracing failures in multi-agent systems | 178 |
| [self-healing-agents.md](self-healing-agents.md) | Automatic recovery and self-repair patterns | 152 |
| [token-cost-optimization.md](token-cost-optimization.md) | Caching, model routing, speculative decoding cost reduction | 87 |

### Agent Engineering

Context engineering, prompt evolution, constrained decoding, and RAG.

| Doc | Key Topic | Lines |
|-----|-----------|-------|
| [context-engineering-patterns.md](context-engineering-patterns.md) | Five-strategy framework: selection, compression, ordering, isolation, format | 101 |
| [prompt-evolution-optimization.md](prompt-evolution-optimization.md) | Evolutionary search and meta-prompting for prompt improvement | 126 |
| [constrained-decoding-patterns.md](constrained-decoding-patterns.md) | Schema-constrained generation for structured agent outputs | 124 |
| [agentic-rag-patterns.md](agentic-rag-patterns.md) | Hierarchical retrieval strategies to minimize token cost | 142 |
| [long-context-agent-strategies.md](long-context-agent-strategies.md) | Utilizing 1M+ context windows effectively | 140 |
| [ai-code-review-agents.md](ai-code-review-agents.md) | AI-powered code review patterns and calibration | 207 |

### Agent Reasoning & Planning

Reasoning orchestration, workflow DAGs, and experiment loops.

| Doc | Key Topic | Lines |
|-----|-----------|-------|
| [reasoning-orchestration-patterns.md](reasoning-orchestration-patterns.md) | Selecting reasoning strategies per phase and complexity | 93 |
| [workflow-dag-patterns.md](workflow-dag-patterns.md) | DAG-based workflow orchestration and topology trade-offs | 206 |
| [autonomous-experiment-loops.md](autonomous-experiment-loops.md) | Hypothesis-driven experiment loop pattern | 180 |

### Agent Collaboration

HITL, collaboration games, interoperability, and IDE integration.

| Doc | Key Topic | Lines |
|-----|-----------|-------|
| [hitl-trust-calibration.md](hitl-trust-calibration.md) | Human-in-the-loop patterns and trust graduation over time | 185 |
| [agent-collaboration-games.md](agent-collaboration-games.md) | Game-theoretic framing for competitive/cooperative dynamics | 140 |
| [agent-interoperability-protocols.md](agent-interoperability-protocols.md) | Standardized agent-to-agent communication protocols | 179 |
| [agentic-ide-integration.md](agentic-ide-integration.md) | AI agent integration tiers with development environments | 161 |

### Meta & Reference

Roadmap, configuration, technique references, and performance profiling.

| Doc | Key Topic | Lines |
|-----|-----------|-------|
| [index.md](index.md) | Top-level documentation index | 33 |
| [configuration.md](configuration.md) | state.json configuration reference | 333 |
| [performance-profiling.md](performance-profiling.md) | Token spend measurement and cost bottleneck identification | 173 |
| [reference/scout-techniques.md](reference/scout-techniques.md) | Phase 2 (DISCOVER) techniques: task selection, difficulty estimation | 117 |
| [reference/scout-discovery.md](reference/scout-discovery.md) | Scout discovery and analysis phase guidelines | 40 |
| [reference/builder-techniques.md](reference/builder-techniques.md) | Phase 3 (BUILD) techniques: error recovery, structured output | 142 |
| [reference/auditor-techniques.md](reference/auditor-techniques.md) | Phase 4 (AUDIT) techniques: anti-conformity, threat detection | 141 |
| [reference/orchestrator-techniques.md](reference/orchestrator-techniques.md) | Phase 5-6 (SHIP/LEARN) techniques: memory, instinct lifecycle | 182 |
| [agent-config-versioning.md](agent-config-versioning.md) | Tracking agent configuration versions for traceability and rollback | 189 |

---

## Cross-Reference Map

Documents that reference other docs in this knowledge base.

| Source Doc | References |
|------------|------------|
| [accuracy-self-correction.md](accuracy-self-correction.md) | agent techniques, eval graders |
| [agent-deployment-patterns.md](agent-deployment-patterns.md) | architecture references |
| [agent-governance-compliance.md](agent-governance-compliance.md) | audit trail references |
| [agent-interoperability-protocols.md](agent-interoperability-protocols.md) | protocol specs |
| [agent-lifecycle-management.md](agent-lifecycle-management.md) | phase references |
| [agentic-rag-patterns.md](agentic-rag-patterns.md) | token optimization |
| [configuration.md](configuration.md) | models-quickstart, architecture |
| [eval-grader-best-practices.md](eval-grader-best-practices.md) | configuration, phases |
| [genes.md](genes.md) | instincts.md |
| [incidents/cycle-132-141.md](incidents/cycle-132-141.md) | cycle-102-111.md, research references |
| [incidents/gemini-forgery.md](incidents/gemini-forgery.md) | cycle-132-141.md, adversarial eval |
| [index.md](index.md) | reference/* docs |
| [instincts.md](instincts.md) | genes.md, configuration.md |
| [memory-consolidation-pipeline.md](memory-consolidation-pipeline.md) | instincts.md, genes.md |
| [model-routing.md](model-routing.md) | configuration.md |
| [performance-profiling.md](performance-profiling.md) | configuration.md, token optimization |
| [prompt-evolution-optimization.md](prompt-evolution-optimization.md) | eval graders |
| [reference/builder-techniques.md](reference/builder-techniques.md) | scout-techniques.md |
| [reference/orchestrator-techniques.md](reference/orchestrator-techniques.md) | instincts.md, genes.md |

### Refactoring

Automated refactoring research, tool landscape, and pipeline architecture.

| Doc | Key Topic | Lines |
|-----|-----------|-------|
| [refactoring-llm-research.md](refactoring-llm-research.md) | LLM refactoring studies (arXiv 2024-2025), RefactoringMirror pattern, safety stats, model comparison | 574 |
| [refactoring-tools-landscape.md](refactoring-tools-landscape.md) | Tool catalog (SonarQube, ESLint, jscpd, knip, dep-cruiser), 66-technique catalog, detection algorithms | 361 |
| [refactoring-pipeline-architecture.md](refactoring-pipeline-architecture.md) | AST transformation tools (OpenRewrite, Rector, tree-sitter, jscodeshift), pipeline design, anti-patterns | 451 |

---

## Reading Order

Recommended sequence for newcomers to the evolve-loop knowledge base.

| Step | Doc | Rationale |
|------|-----|-----------|
| 1 | [index.md](index.md) | Orient with the top-level documentation structure |
| 2 | [configuration.md](configuration.md) | Understand the runtime configuration model |
| 3 | [model-routing.md](model-routing.md) | Learn the 3-tier model abstraction |
| 4 | [agent-role-specialization.md](agent-role-specialization.md) | Understand how agent roles are designed |
| 5 | [instincts.md](instincts.md) | Learn the learning mechanism |
| 6 | [genes.md](genes.md) | Understand reusable fix templates |
| 7 | [memory-consolidation-pipeline.md](memory-consolidation-pipeline.md) | See how knowledge persists across cycles |
| 8 | [reference/scout-techniques.md](reference/scout-techniques.md) | Phase 2 techniques |
| 9 | [reference/builder-techniques.md](reference/builder-techniques.md) | Phase 3 techniques |
| 10 | [reference/auditor-techniques.md](reference/auditor-techniques.md) | Phase 4 techniques |
| 11 | [reference/orchestrator-techniques.md](reference/orchestrator-techniques.md) | Phase 5-6 techniques |
| 12 | [eval-grader-best-practices.md](eval-grader-best-practices.md) | Understand evaluation quality |
| 13 | [reward-hacking-prevention.md](reward-hacking-prevention.md) | Learn safety guardrails |
| 14 | [incidents/cycle-102-111.md](incidents/cycle-102-111.md) | Study real failure case |
| 15 | [incidents/cycle-132-141.md](incidents/cycle-132-141.md) | Study systemic gaming incident |
| 16 | [agent-economics.md](agent-economics.md) | Understand cost modeling |
