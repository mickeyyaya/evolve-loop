# Agent Capability Benchmarking

> Reference document for measuring, tracking, and improving agent capabilities
> across evolve-loop cycles. Use multi-dimensional scoring to detect regressions,
> identify capability gaps, and guide task selection toward maximum impact.

## Table of Contents

1. [Three-Layer Evaluation Model](#three-layer-evaluation-model)
2. [Benchmark Landscape](#benchmark-landscape)
3. [Capability Vectors](#capability-vectors)
4. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
5. [Implementation Patterns](#implementation-patterns)
6. [Prior Art](#prior-art)
7. [Anti-Patterns](#anti-patterns)

---

## Three-Layer Evaluation Model

Evaluate agent capabilities at three distinct layers. Each layer builds on the one below.

| Layer | What It Measures | Example Metrics | Eval Frequency |
|-------|-----------------|-----------------|----------------|
| **Foundation** | Raw model abilities (reasoning, code generation, instruction following) | Pass@1 on coding problems, reasoning accuracy, instruction adherence rate | Per model change |
| **Component** | Tool use, memory recall, planning, self-correction in isolation | Tool call accuracy, context retrieval precision, plan step completion rate | Per agent update |
| **End-to-End** | Full task completion across Scout, Builder, Auditor pipeline | Task pass rate, cycle success rate, eval grader pass rate, regression count | Every cycle |

### Layer Dependencies

| Foundation Gap | Component Symptom | End-to-End Failure |
|---------------|-------------------|-------------------|
| Weak code generation | Builder produces non-compiling code | Task fails eval grader |
| Poor instruction following | Scout ignores task priority rules | Wrong task selected |
| Limited reasoning depth | Auditor misses regression | Shipped broken code |

---

## Benchmark Landscape

| Benchmark | Domain | Task Count | Metric | Relevance to Evolve-Loop |
|-----------|--------|-----------|--------|--------------------------|
| **SWE-bench** | GitHub issue resolution | 2,294 | % resolved | Builder code generation and debugging |
| **SWE-bench Pro** | Harder real-world SWE tasks | 500+ | % resolved | Builder on complex multi-file changes |
| **GAIA** | General AI assistants (multi-step reasoning) | 466 | Accuracy by level (1-3) | Scout research + planning capability |
| **Context-Bench** | Long-horizon context utilization | Varies | Recall accuracy at depth | Memory recall across long sessions |
| **tau-bench** | Tool-augmented task completion | 200+ | Task success rate | Tool use accuracy for all agents |
| **LiveCodeBench** | Competitive programming (contamination-free) | Rolling | Pass@1 | Foundation code generation ability |
| **FeatureBench** | Feature implementation from specs | 75+ | Feature correctness | End-to-end Builder capability |
| **ColBench** | Collaborative multi-agent coding | 100+ | Collaboration success rate | Multi-agent coordination patterns |

### Benchmark Selection Criteria

| Criterion | Weight | Rationale |
|-----------|--------|-----------|
| Contamination resistance | High | Avoid inflated scores from training data overlap |
| Task realism | High | Synthetic tasks diverge from real-world performance |
| Reproducibility | Medium | Deterministic grading enables trend tracking |
| Coverage breadth | Medium | Single-domain benchmarks miss capability gaps |

---

## Capability Vectors

Score each agent role across six dimensions. Use 0-100 scale per dimension.

### Dimension Definitions

| Dimension | Description | Measurement Method |
|-----------|-------------|-------------------|
| **Code Generation** | Produce correct, idiomatic code from specifications | Eval grader pass rate on Builder output |
| **Debugging** | Identify and fix defects in existing code | Fix rate on failed eval graders after retry |
| **Planning** | Decompose tasks into ordered, achievable steps | Scout task selection accuracy vs. actual difficulty |
| **Tool Use** | Select and invoke correct tools with valid parameters | Tool call success rate, unnecessary tool call count |
| **Memory Recall** | Retrieve relevant context from prior cycles | Instinct reuse rate, repeated mistake avoidance |
| **Self-Correction** | Detect own errors and recover without external input | Auditor catch rate, Builder self-fix before audit |

### Role-Dimension Matrix

Map expected capability emphasis per agent role.

| Dimension | Scout | Builder | Auditor |
|-----------|-------|---------|---------|
| Code Generation | Low | **Critical** | Medium |
| Debugging | Low | **High** | **Critical** |
| Planning | **Critical** | Medium | Low |
| Tool Use | **High** | **High** | Medium |
| Memory Recall | **Critical** | Medium | **High** |
| Self-Correction | Medium | **High** | **Critical** |

---

## Mapping to Evolve-Loop

### projectBenchmark as Capability Tracker

The `projectBenchmark` field in `state.json` tracks composite quality scores across dimensions.

| projectBenchmark Field | Capability Vector Mapping |
|----------------------|--------------------------|
| `dimensions.documentationCompleteness` | Planning (spec clarity) |
| `dimensions.testCoverage` | Code Generation + Debugging |
| `dimensions.codeQuality` | Code Generation + Self-Correction |
| `overall` | Weighted composite of all dimensions |
| `highWaterMarks` | Peak capability per dimension |

### evalHistory as Performance Over Time

| evalHistory Field | Tracking Purpose |
|-------------------|-----------------|
| `passRate` | End-to-End task success trend |
| `instinctsExtracted` | Memory Recall improvement signal |
| `filesChanged` | Code Generation volume proxy |
| `attemptsBeforePass` | Self-Correction efficiency |

### Mastery Levels as Capability Graduation

| Mastery Level | Threshold | Capability Implication |
|---------------|-----------|----------------------|
| `novice` | 0-2 consecutive successes | Agent still learning project patterns |
| `competent` | 3-4 consecutive successes | Reliable on standard tasks |
| `proficient` | 5+ consecutive successes | Ready for complex multi-file tasks |
| `expert` | 8+ consecutive successes + zero regressions | Eligible for autonomous mode expansion |

---

## Implementation Patterns

### Track Capability Dimensions Per Cycle

Record per-cycle capability scores in the ledger.

| Step | Action | Output |
|------|--------|--------|
| 1 | Scout completes task selection | Record Planning score (task difficulty vs. available capacity) |
| 2 | Builder completes implementation | Record Code Generation score (eval pass on first attempt) |
| 3 | Auditor completes review | Record Debugging score (issues caught), Self-Correction score (issues missed) |
| 4 | Phase-gate passes | Record Tool Use score (tool errors during cycle) |
| 5 | Learn phase completes | Record Memory Recall score (instinct reuse from prior cycles) |

### Regression Detection

| Signal | Detection Method | Response |
|--------|-----------------|----------|
| `projectBenchmark.overall` drops > 10 from high-water mark | Compare current vs. `highWaterMarks` in state.json | Trigger remediation task in next Scout cycle |
| `evalHistory` shows 2+ consecutive failures | Check `mastery.consecutiveSuccesses` reset | Reduce task complexity, increase Auditor scrutiny |
| Capability dimension drops while others hold | Compare per-dimension scores across last 5 cycles | Target specific agent role for prompt refinement |
| Tool error rate spikes | Count non-zero exit codes in ledger by tool name | Review tool definitions and parameter validation |

### Capability Gap Identification

| Gap Type | Detection | Remediation |
|----------|-----------|-------------|
| **Foundation gap** | Consistent failures across all roles on similar tasks | Switch model or add chain-of-thought scaffolding |
| **Component gap** | One role fails while others succeed on same project | Refine role-specific prompts and tool access |
| **Integration gap** | Individual roles pass but end-to-end fails | Improve handoff protocols between Scout, Builder, Auditor |
| **Memory gap** | Repeated mistakes despite prior instinct extraction | Strengthen memory consolidation, check instinct retrieval |

---

## Prior Art

| Project / Framework | Focus | Key Contribution |
|--------------------|-------|-----------------|
| **GAIA** (Mialon et al., 2023) | Multi-step reasoning with tools | Three-level difficulty taxonomy; measures real-world assistant capability |
| **Context-Bench** | Long-horizon context utilization | Measures retrieval accuracy at varying context depths; relevant to memory recall |
| **Anthropic Evals** | Model capability and safety evaluation | Structured eval framework with rubric-based grading; informs eval grader design |
| **OpenAI Evals Framework** | Community-driven model evaluation | Open eval registry pattern; demonstrates composable eval architecture |
| **SWE-bench** (Jimenez et al., 2024) | Real-world software engineering | Gold standard for code generation + debugging capability measurement |
| **MemRL / MemEvolve** | Memory-augmented reinforcement learning | Forced extraction patterns for stalled learning; adopted in evolve-loop phase 5 |
| **AgentBench** (Liu et al., 2023) | Multi-environment agent evaluation | Eight distinct environments; demonstrates multi-dimensional agent scoring |

---

## Anti-Patterns

| Anti-Pattern | Problem | Mitigation |
|-------------|---------|------------|
| **Single-metric evaluation** | One number hides capability gaps; agent appears strong while a critical dimension degrades | Track all six capability vectors independently; require minimum threshold per dimension |
| **Benchmark overfitting** | Optimizing for benchmark-specific patterns that do not transfer to real tasks | Rotate evaluation tasks; use project-specific evals alongside public benchmarks |
| **Comparing incompatible benchmarks** | Mixing scores from different benchmarks with different scales and task distributions | Normalize scores within each benchmark; never average across benchmark families |
| **Eval grader gaming** | Agent learns to satisfy grader syntax without producing correct output | Add semantic checks beyond pattern matching; use LLM-as-judge for subjective quality |
| **Ignoring capability regression** | Focusing only on new capabilities while existing ones silently degrade | Enforce high-water mark checks; halt on regression exceeding threshold |
| **Static capability model** | Assuming fixed capability profile; missing that capabilities shift with codebase growth | Re-calibrate projectBenchmark periodically; adjust mastery thresholds as project complexity increases |
| **Conflating speed with capability** | Treating faster completion as higher capability; missing quality trade-offs | Measure capability dimensions independently from cycle duration; track quality per token spent |
