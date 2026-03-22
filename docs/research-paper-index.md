# Research Paper Index

Comprehensive index of all AI research papers cited in the evolve-loop skill. Each entry maps a paper to its integration cycle, target files, and the specific technique adopted.

## Papers by Integration Cycle

### Cycle 139 — Self-Improvement Foundations

| Paper | arXiv | Technique Adopted | Target File | Status |
|-------|-------|-------------------|-------------|--------|
| Darwin Godel Machine (DGM) | arXiv:2505.22954 | Failure Pattern Analysis with root cause categories | `docs/self-learning.md` | PROVISIONAL |
| ACE: Agentic Context Engineering | arXiv:2510.04618 | Strategy Playbook with GRC pipeline, anti-collapse safeguards | `skills/evolve-loop/phase5-learn.md` | PROVISIONAL |
| DAAO: Difficulty-Aware Orchestration | arXiv:2509.11079 | Continuous 1-10 difficulty scoring, cost-performance feedback | `docs/self-learning.md` | PROVISIONAL |

### Cycle 140 — Evaluation and Audit Quality

| Paper | arXiv | Technique Adopted | Target File | Status |
|-------|-------|-------------------|-------------|--------|
| AgentPRM: Process Reward Models | arXiv:2502.10325 | Step-level trajectory scoring (4-phase decomposition) | `skills/evolve-loop/phase5-learn.md` | PROVISIONAL |
| Free-MAD: Consensus-Free Debate | arXiv:2509.11035 | Anti-conformity audit protocol, split-role adversarial check | `docs/accuracy-self-correction.md` | PROVISIONAL |
| HiPRAG: Hierarchical Process Rewards | arXiv:2510.07794 | Per-query research quality scoring (novelty/relevance/yield) | `agents/evolve-scout.md` | PROVISIONAL |

### Cycle 141 — Planning and Optimization

| Paper | arXiv | Technique Adopted | Target File | Status |
|-------|-------|-------------------|-------------|--------|
| GoalAct: Hierarchical Planning | arXiv:2504.16563 | Goal-continuity milestone tracking, branch trap detection | `docs/self-learning.md` | PROVISIONAL |
| AutoPDL: Prompt Optimization | arXiv:2504.04365 | Prompt variant tracking via experiments.jsonl | `docs/self-learning.md` | PROVISIONAL |
| ARTEMIS: Evolutionary Agent Config | arXiv:2512.09108 | configVariant field for joint config optimization | `docs/self-learning.md` | PROVISIONAL |

### Cycle 142 — Error Recovery and Memory

| Paper | arXiv | Technique Adopted | Target File | Status |
|-------|-------|-------------------|-------------|--------|
| AgentDebug: Error Taxonomy | arXiv:2509.25370 | 5-dimension failure classification for targeted retries | `docs/accuracy-self-correction.md` | PROVISIONAL |
| Agent Memory Surveys | arXiv:2505.00675, arXiv:2603.07670 | Instinct forgetting protocol with causal review gate | `docs/instincts.md` | PROVISIONAL |
| SWE-CI Benchmark | March 2026 | Mandatory regression evals, EvoScore-inspired decay | `docs/security-considerations.md`, `skills/evolve-loop/eval-runner.md` | PROVISIONAL |

### Cycle 143 — Tool Learning and Governance

| Paper | arXiv | Technique Adopted | Target File | Status |
|-------|-------|-------------------|-------------|--------|
| Tool-R0: Self-Evolving Tool Use | arXiv:2602.21320 | Gene self-play evolution, adversarial curriculum | `docs/genes.md` | PROVISIONAL |
| Agent Skills Governance | arXiv:2602.12430 | Trust tier framework for instinct global promotion | `docs/security-considerations.md` | PROVISIONAL |
| Safiron: Pre-Execution Guardrails | arXiv:2510.09781 | Deferred — pre-execution risk classification | — | DEFERRED |

### Cycle 144 — Evaluation and Budget Optimization

| Paper | arXiv | Technique Adopted | Target File | Status |
|-------|-------|-------------------|-------------|--------|
| AgentAssay: Regression Testing | arXiv:2603.02601 | Three-valued eval verdicts, behavioral fingerprinting, SPRT | `docs/accuracy-self-correction.md` | PROVISIONAL |
| BATS: Budget-Aware Tool Scaling | arXiv:2511.17006 | budgetRemaining injection, explore/exploit switching | `docs/performance-profiling.md` | PROVISIONAL |

### Cycle 145 — Uncertainty, Structured Output, Documentation

| Paper | arXiv | Technique Adopted | Target File | Status |
|-------|-------|-------------------|-------------|--------|
| Agentic Uncertainty Quantification (AUQ) | arXiv:2601.15703, arXiv:2602.05073 | Action-conditional gating, UAM/UAR dual-process | `docs/accuracy-self-correction.md` | PROVISIONAL |
| DCCD: Draft-Conditioned Constrained Decoding | arXiv:2603.03305 | Two-stage draft-then-constrain for structured output | `docs/performance-profiling.md` | PROVISIONAL |
| DocAgent: Multi-Agent Documentation | arXiv:2504.08725 | Topological code processing, 3D eval framework | `docs/research-paper-index.md` | DEFERRED |

### Pre-Session (Cycles 16-19) — Foundational Methods

| Paper | arXiv | Technique Adopted | Target File | Status |
|-------|-------|-------------------|-------------|--------|
| Stepwise Confidence Scoring | arXiv:2511.07364 | Per-evidence mini-scores before aggregate | `docs/self-learning.md` | VALIDATED |
| EvolveR Experience Scoring | arXiv:2510.16079 | Experience-weighted task selection | `docs/self-learning.md` | VALIDATED |
| MUSE Memory Categories | MUSE framework | Functional memory category taxonomy | `docs/self-learning.md` | VALIDATED |
| CSI Metric | Karpathy/GVU | Coefficient of Self-Improvement rolling window | `docs/self-learning.md` | VALIDATED |
| Confidence-Correctness Alignment | arXiv:2603.06604 | Calibration error detection, recalibration trigger | `docs/self-learning.md` | VALIDATED |
| Self-Evolving Agent Taxonomy | arXiv:2507.21046 | Four-stage evolution lifecycle mapping | `docs/self-learning.md` | VALIDATED |

## Status Legend

| Status | Meaning |
|--------|---------|
| **VALIDATED** | Target dimension improved ≥+2 from adoption baseline |
| **PROVISIONAL** | Not yet demonstrated ≥+2 improvement; re-evaluated at each meta-cycle |
| **DEFERRED** | Paper noted but not yet integrated into the skill |

## Research Coverage Map

| Domain | Papers | Key Gaps |
|--------|--------|----------|
| Self-improvement | DGM, ARTEMIS, Tool-R0 | Agent variant branching (DGM Phase 6) |
| Context engineering | ACE, ACON (deferred) | Context compression implementation |
| Evaluation | AgentPRM, HiPRAG, SWE-CI, AgentAssay | Formal verification (VERINA, deferred) |
| Orchestration | DAAO, GoalAct | Multi-agent coordination protocols |
| Robustness | Free-MAD, AgentDebug, AUQ | Adversarial testing frameworks |
| Memory | Memory Surveys, MUSE | Causal memory retrieval |
| Security | Agent Skills, Safiron (deferred) | Runtime guardrail integration |
| Optimization | AutoPDL, BATS, DCCD, Stepwise Scoring | Prompt caching strategies |
| Documentation | DocAgent (deferred) | Topological code processing |
