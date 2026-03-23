# Autonomous Experiment Loops

> Read this file to understand the hypothesis-driven experiment loop pattern and how it maps to evolve-loop phases.

## Contents

- [The Autonomous Experiment Loop Pattern](#the-autonomous-experiment-loop-pattern) — Core cycle and flow diagram
- [Mapping to Evolve-Loop Phases](#mapping-to-evolve-loop-phases) — Phase-by-phase correspondence
- [Prior Art](#prior-art) — Research references and benchmarks
- [Implementation Patterns](#implementation-patterns) — Apply the pattern in each agent
- [Anti-Patterns](#anti-patterns) — Common failure modes to avoid

---

## The Autonomous Experiment Loop Pattern

The autonomous experiment loop is a self-improving cycle where an AI agent formulates hypotheses, runs experiments, measures outcomes, extracts learnings, and repeats. Each iteration refines the agent's understanding and improves subsequent cycles.

### Core Cycle

| Step | Action | Output |
|------|--------|--------|
| 1. Hypothesize | Formulate a testable prediction with expected outcome | Hypothesis statement + predicted metric |
| 2. Experiment | Implement the change with instrumented metrics collection | Code change + metric instrumentation |
| 3. Measure | Collect actual metrics and compare to predicted values | Metric delta (predicted vs actual) |
| 4. Learn | Analyze results, update priors, extract transferable patterns | Learning entry with confidence adjustment |
| 5. Repeat | Carry forward learnings into the next hypothesis | Updated prior knowledge for next cycle |

### Flow Diagram

```
  ┌─────────────────────────────────────────────┐
  │                                             │
  ▼                                             │
┌──────────────┐    ┌──────────────┐    ┌───────┴──────┐
│  HYPOTHESIZE │───▶│  EXPERIMENT  │───▶│   MEASURE    │
│              │    │              │    │              │
│ - Testable   │    │ - Implement  │    │ - Collect    │
│   prediction │    │ - Instrument │    │   metrics    │
│ - Expected   │    │ - Execute    │    │ - Compare to │
│   metric     │    │              │    │   predicted  │
└──────────────┘    └──────────────┘    └──────┬───────┘
                                               │
                                               ▼
                                        ┌──────────────┐
                                        │    LEARN     │
                                        │              │
                                        │ - Analyze    │
                                        │ - Update     │
                                        │   priors     │
                                        │ - Extract    │
                                        │   patterns   │
                                        └──────┬───────┘
                                               │
                                               ▼
                                        ┌──────────────┐
                                        │   REPEAT     │
                                        │              │
                                        │ - Next cycle │
                                        │ - Carry      │
                                        │   learnings  │
                                        └──────────────┘
```

---

## Mapping to Evolve-Loop Phases

| Loop Phase | Evolve-Loop Phase | Agent | Key Actions |
|------------|-------------------|-------|-------------|
| Hypothesize | DISCOVER | Scout | Formulate testable predictions; define expected metric deltas; specify falsification criteria |
| Experiment | BUILD | Builder | Implement the change with instrumented metrics; collect baseline measurements before modification |
| Measure | AUDIT | Auditor | Collect post-change metrics; compare actual vs predicted outcomes; flag deviations > threshold |
| Learn | LEARN | Operator | Consolidate findings into learning entries; update priors for future hypothesis generation |
| Repeat | Next cycle | All | Carry forward learnings; adjust hypothesis generation based on accumulated evidence |

### Phase Transition Requirements

| Transition | Gate Condition | Failure Action |
|------------|---------------|----------------|
| Hypothesize → Experiment | Hypothesis is testable and falsifiable | Reject; reformulate with measurable criteria |
| Experiment → Measure | Implementation complete; metrics instrumented | Block; instrument metrics before proceeding |
| Measure → Learn | Metrics collected; comparison computed | Block; collect missing metrics |
| Learn → Repeat | Learning entry written; priors updated | Block; write learning entry |

---

## Prior Art

| Project | Author / Source | Key Contribution | Scale |
|---------|----------------|------------------|-------|
| autoresearch | Karpathy | 630-line Python script running autonomous research loops | 700 experiments in 2 days; 20 optimizations discovered |
| Darwin-Godel Machine | Self-improving agent systems research | Agents that modify their own code through evolutionary search | Theoretical framework for open-ended self-improvement |
| SICA | Self-Improving Coding Agent | Coding agent that iterates on its own prompts and strategies | Demonstrated measurable improvement over successive iterations |
| SWE-EVO | Software evolution benchmark | Benchmark for evaluating AI-driven software evolution | Standardized evaluation of autonomous code improvement |

### Key Lessons from Prior Art

| Lesson | Source | Application to Evolve-Loop |
|--------|--------|---------------------------|
| Keep experiment scripts small and self-contained | autoresearch (630 lines) | Scope each cycle to a single testable hypothesis |
| Run many small experiments, not few large ones | autoresearch (700 experiments) | Prefer S/M complexity tasks over L/XL |
| Self-modification requires strong guardrails | Darwin-Godel Machine | Use phase-gate.sh at every transition |
| Measure everything, trust nothing | SICA | Instrument all changes with eval graders |

---

## Implementation Patterns

### Scout: Generate Testable Hypotheses

| Requirement | Description |
|-------------|-------------|
| Prediction | State the expected outcome as a measurable metric |
| Baseline | Record current metric value before the experiment |
| Falsification | Define what result would disprove the hypothesis |
| Scope | Limit to one variable per experiment |

Example hypothesis format:

```
Hypothesis: Adding input validation to phase-gate.sh will reduce
  false-pass rate from 12% to <5%.
Predicted metric: false-pass rate delta = -7% or more
Falsification: If false-pass rate remains >5%, hypothesis is rejected.
```

### Builder: Instrument Experiments

| Requirement | Description |
|-------------|-------------|
| Baseline capture | Record pre-change metric values in build-report |
| Metric hooks | Add measurement points at key execution paths |
| Isolation | Change only the variable under test; minimize side effects |
| Rollback plan | Document how to revert if experiment causes regression |

### Auditor: Compare Predicted vs Actual

| Requirement | Description |
|-------------|-------------|
| Metric collection | Gather all instrumented metrics post-change |
| Delta computation | Calculate actual - predicted for each metric |
| Deviation threshold | Flag results where actual deviates >20% from predicted |
| Verdict linkage | Tie audit verdict to experiment outcome (confirmed / refuted / inconclusive) |

### Operator: Extract Learnings

| Requirement | Description |
|-------------|-------------|
| Result classification | Mark hypothesis as confirmed, refuted, or inconclusive |
| Prior update | Adjust confidence in related hypotheses based on result |
| Pattern extraction | Identify reusable patterns from confirmed hypotheses |
| Negative result capture | Record what did NOT work with equal rigor |

---

## Anti-Patterns

| Anti-Pattern | Description | Mitigation |
|--------------|-------------|------------|
| Unfalsifiable hypotheses | Predictions that cannot be disproved by any metric outcome | Require explicit falsification criteria in every hypothesis |
| Metric gaming (Goodhart's law) | Optimizing for the metric rather than the underlying goal | Use multiple orthogonal metrics; rotate primary metric periodically |
| Experiment scope creep | Changing multiple variables in a single experiment cycle | Enforce one-variable-per-cycle rule; reject multi-variable tasks at Scout phase |
| Ignoring negative results | Discarding experiments that disprove the hypothesis | Require learning entries for all outcomes; weight negative results equally |
| Confirmation bias | Interpreting ambiguous results as confirming the hypothesis | Set deviation thresholds before experiment; use automated comparison |
| Missing baselines | Running experiments without recording pre-change metrics | Block BUILD → AUDIT transition if baseline metrics are absent |
| Phantom improvements | Claiming improvement without statistical significance | Require minimum sample size or repeated measurements |

---

## Quick Reference

| Term | Definition |
|------|-----------|
| Hypothesis | A testable prediction about the effect of a specific change |
| Baseline | The metric value measured before the experiment |
| Delta | The difference between actual and predicted metric values |
| Prior | Accumulated knowledge that informs future hypothesis generation |
| Falsification | Evidence that disproves a hypothesis |
| Experiment cycle | One complete pass through hypothesize → experiment → measure → learn |
