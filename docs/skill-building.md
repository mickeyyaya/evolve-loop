# Skill Building Lifecycle

How patterns discovered during evolve-loop cycles graduate from raw observations into durable, reusable skills. This document is the canonical reference for the full lifecycle.

## Overview

The evolve-loop implements a confidence-gated graduation pipeline:

```
Observation → Instinct (0.5) → Confirmed (0.8+) → Policy (0.9+) → Skill/Gene
```

Each stage requires measurable evidence before promotion. This prevents premature generalization while ensuring valuable patterns don't stay trapped at low confidence.

## Stage 1: Observation (Cycle N)

During Phase 5 LEARN, the orchestrator reads all workspace files and identifies patterns:

- **Successful patterns** — approaches that worked and why
- **Failed patterns** — approaches that failed and root causes
- **Domain knowledge** — codebase-specific conventions discovered
- **Process insights** — task sizing, eval effectiveness observations

Not every observation becomes an instinct. The entropy gating check prevents duplicate or low-information instincts: if a new observation is >90% similar to an existing instinct, the existing instinct's confidence is incremented instead.

## Stage 2: Instinct (confidence 0.5)

A pattern observed once. Written to `.evolve/instincts/personal/` as YAML:

```yaml
- id: inst-001
  pattern: "god-file-extraction"
  description: "When a markdown file exceeds 800 lines, extract the largest self-contained section"
  confidence: 0.5
  source: "cycle-1/split-phases-learn-phase"
  type: "technique"
  category: "procedural"
```

**Categories:**
- **Episodic** (anti-pattern, successful-pattern) — things that happened
- **Semantic** (convention, architecture, domain) — knowledge about the codebase
- **Procedural** (process, technique) — how to do things

**Forcing functions for confidence growth:**
- Each time an agent cites the instinct in `instinctsApplied`, confidence increases by +0.05
- Instincts not cited for 5+ cycles decay by -0.1 per consolidation pass
- Below 0.3 → archived as stale

## Stage 3: Confirmed Instinct (confidence 0.8+)

The pattern has been validated across multiple cycles. At this threshold:

- Agents in `instinctSummary` read and may apply it during task selection and building
- The Scout applies instinct-driven priority boosts
- The pattern is eligible for global promotion (copied to `~/.evolve/instincts/personal/`)

**What 0.8 means:** The pattern has been cited by agents in at least 6 independent cycles (0.5 + 6 × 0.05 = 0.8). This is enough evidence that it's a genuine pattern, not a one-off observation.

## Stage 4: Orchestrator Policy (confidence 0.9+)

The pattern is so reliable it becomes a named rule in `SKILL.md` under "Orchestrator Policies." Examples:

| Policy | Origin | Confidence |
|--------|--------|------------|
| Inline S-complexity tasks | inst-007 | 0.9 |
| Grep-based evals | inst-004 | 0.9 |

Policies are hard rules the orchestrator enforces without discretion. They bypass the instinct lookup — the rule is baked into the orchestration logic.

**Promotion criteria:**
- Confidence ≥ 0.9 (at least 8 independent citations)
- Pattern is general (not specific to one task type or file)
- The orchestrator can enforce it deterministically

## Stage 5: Skill or Gene (optional)

For patterns that are complex enough to warrant their own file:

- **Gene** — a structured fix template with selector, steps, and validation. Written to `.evolve/genes/`. Genes describe *how to fix* with executable steps.
- **Skill** — a full agent instruction set. Only created when a pattern is complex enough to warrant multi-step guidance.

Most instincts never reach this stage. The majority stabilize as confirmed instincts or orchestrator policies.

## Memory Consolidation

Every 3 cycles (or when instinctCount > 20), the orchestrator consolidates:

1. **Cluster** — merge instincts with >85% semantic similarity
2. **Archive** — move merged originals to `.evolve/instincts/archived/` with `supersededBy`
3. **Decay** — reduce confidence of uncited instincts by -0.1
4. **Prune** — archive instincts below 0.3 confidence

This prevents instinct bloat while preserving provenance. Archived instincts are never deleted.

## Anti-Patterns

- **Premature graduation** — promoting a 0.5 instinct to policy because it "feels right" (requires evidence)
- **Confidence inflation** — citing an instinct without actually applying it (inflation without signal)
- **Instinct hoarding** — keeping 50+ instincts active without consolidation (dilutes attention)
- **Stale policies** — policies that reference removed features or outdated conventions
- **Over-specificity** — instincts tied to a single file path instead of a generalizable pattern

## Lifecycle Metrics

Track these to assess skill building health:

| Metric | Healthy Range | Signal |
|--------|--------------|--------|
| Instincts extracted per cycle | 1-5 | 0 for 2+ cycles = extraction stall |
| Avg confidence of active instincts | 0.5-0.8 | >0.9 everywhere = not enough new learning |
| Consolidation ratio (merged/total) | 10-30% | >50% = too many near-duplicates |
| Policy count | 3-8 | >10 = review for staleness |
| Decay rate (archived per consolidation) | 1-3 | >5 = instincts not being applied |
