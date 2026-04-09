---
name: inspirer
description: Use when the user invokes /inspirer or asks to brainstorm creatively, think outside the box, explore unconventional approaches, break out of stagnation, or generate research-backed ideas with provocation lenses
argument-hint: "[topic/question] [--depth QUICK|STANDARD|DEEP] [--lenses N] [--format full|brief|evolve]"
---

> Think outside the box, backed by evidence. 12 provocation lenses, web-grounded research, scored and filtered recommendations.

## Contents
- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Stage 1: FRAME](#stage-1-frame)
- [Stage 2: DIVERGE](#stage-2-diverge)
- [Stage 3: RESEARCH](#stage-3-research)
- [Stage 4: SCORE](#stage-4-score)
- [Stage 5: CONVERGE](#stage-5-converge)
- [Stage 6: DELIVER](#stage-6-deliver)
- [Depth Control](#depth-control)
- [Evolve-Loop Integration](#evolve-loop-integration)
- [Output Schema](#output-schema)
- [Reference](#reference-read-on-demand)

## Quick Start

```bash
# Basic — think creatively about a topic
/inspirer "How should we handle real-time sync in a serverless app?"

# Control depth (QUICK = fast ideation, DEEP = thorough exploration)
/inspirer "What features increase user retention?" --depth DEEP

# Specify number of lenses (default: 4 for STANDARD)
/inspirer "Multi-agent coordination patterns" --lenses 5

# Output for evolve-loop consumption
/inspirer "Improve eval infrastructure" --format evolve --depth QUICK
```

**Parse arguments:**
- First quoted string or remaining text → `topic`
- `--depth QUICK|STANDARD|DEEP` → research depth (default: `STANDARD`)
- `--lenses N` → number of provocation lenses (default: per depth level)
- `--format full|brief|evolve` → output format (default: `full`)

## Architecture

Six-stage pipeline: **FRAME → DIVERGE → RESEARCH → SCORE → CONVERGE → DELIVER**

```
Input: User's topic/question
         │
    ┌────▼────┐
    │  FRAME  │  Parse topic, classify domain, detect constraints
    └────┬────┘
         │
    ┌────▼─────┐
    │ DIVERGE  │  Apply 3-5 provocation lenses → divergent questions
    └────┬─────┘
         │ 1-2 research questions per lens
    ┌────▼─────┐
    │ RESEARCH │  Web search each question (Smart or Default routing)
    └────┬─────┘
         │ Research findings with sources
    ┌────▼────┐
    │  SCORE  │  Create Inspiration Cards, score feasibility × impact × novelty
    └────┬────┘
         │ Scored cards with KEEP/DROP verdicts
    ┌────▼─────┐
    │ CONVERGE │  Rank by composite, diversity filter, select top 5-8
    └────┬─────┘
         │
    ┌────▼─────┐
    │ DELIVER  │  Output as Inspiration Report / brief table / evolve JSON
    └──────────┘
```

**Why this pipeline?** Creative divergence without research produces brainstorming fluff. Research without creative framing produces obvious answers. This pipeline forces creative questions first, then grounds every idea in evidence.

## Stage 1: FRAME

Parse the user's topic and establish context for lens selection.

| Step | Action |
|------|--------|
| 1. Parse topic | Extract the core question or challenge |
| 2. Classify domain | `code-architecture`, `product-strategy`, `technical-research`, `process-improvement`, `general` |
| 3. Detect constraints | Implicit limits (technology, team size, timeline, budget) mentioned in the topic |
| 4. Check evolve-loop context | If invoked from evolve-loop, read `benchmarkWeaknesses` and `failedApproaches` from context. Otherwise skip. |

**Output:** Problem Frame object:
```json
{
  "topic": "<parsed question>",
  "domain": "<classified domain>",
  "constraints": ["<constraint 1>", "<constraint 2>"],
  "evolveContext": null | { "weaknesses": [...], "failedApproaches": [...] }
}
```

## Stage 2: DIVERGE

Apply provocation lenses to the Problem Frame. Each lens generates 1-2 divergent research questions that push thinking beyond the obvious.

### Lens Selection

| Depth | Lenses | Selection Method |
|-------|--------|-----------------|
| QUICK | 3 | 1 random + 2 domain-matched |
| STANDARD | 4 | 1 random + 3 domain-matched |
| DEEP | 5 | 1 random + 4 domain-matched |

**Domain-to-lens matching:** See [reference/provocation-lenses.md](reference/provocation-lenses.md) for the full affinity matrix. Each domain has 4-5 high-affinity lenses ranked by relevance.

### The 12 Provocation Lenses

| # | Lens | Provocation Question |
|---|------|---------------------|
| 1 | **Inversion** | "What if we did the exact opposite?" |
| 2 | **Analogy** | "What would this look like borrowed from {adjacent domain}?" |
| 3 | **10x Scale** | "What breaks at 10x the current load/complexity?" |
| 4 | **Removal** | "What if we deleted this entirely?" |
| 5 | **User-Adjacent** | "What problem will the user hit NEXT?" |
| 6 | **First Principles** | "Why does this exist? What fundamental constraint requires it?" |
| 7 | **Composition** | "What if we combined two unrelated things?" |
| 8 | **Failure Mode** | "How would this fail silently?" |
| 9 | **Ecosystem** | "What external tool/pattern makes this obsolete?" |
| 10 | **Time Travel** | "What will we wish we had in 3 months?" |
| 11 | **Constraint Flip** | "What if the biggest constraint were removed entirely?" |
| 12 | **Audience Shift** | "What if the primary user were someone completely different?" |

Lenses 1-10 are from the evolve-loop research protocol. Lenses 11-12 are added for general-purpose topics beyond codebase analysis.

### Divergent Question Generation

For each selected lens:
1. Apply the provocation question to the Problem Frame
2. Generate 1-2 concrete, searchable research questions
3. Tag each question with `lens` and `domain`

**Example:** Topic = "How to handle real-time sync in serverless?"
- Inversion lens → "What architectures deliberately avoid real-time sync and still succeed?"
- Ecosystem lens → "What managed services handle real-time sync so we don't build it?"
- 10x Scale lens → "What real-time sync approaches handle 10M+ concurrent connections?"

## Stage 3: RESEARCH

Ground every divergent question in web research. Route queries based on depth.

| Depth | Routing | Max Queries | Max WebFetch |
|-------|---------|------------|-------------|
| QUICK | Default WebSearch (1-2 queries per question) | 5 total | 2 |
| STANDARD | Smart Web Search for complex, Default for simple | 8 total | 4 |
| DEEP | Smart Web Search for all questions | 12 total | 6 |

**Smart Web Search** protocol: Use `smart-web-search.md` 6-stage pipeline (intent classification → query transformation → execution → evaluation → refinement → synthesis).

**Default WebSearch**: Direct 1-2 keyword queries with year filter for volatile topics.

**For each research result, capture:**
- Source URL and title
- Key finding (1-2 sentences)
- Relevance to the divergent question (0.0-1.0)
- Recency (penalize results > 2 years old for technology topics)

**Critical rule:** Ideas without at least 1 supporting research result are scored 0.0 on feasibility and auto-dropped in Stage 4. No research = no recommendation.

## Stage 4: SCORE

Convert research-backed ideas into **Inspiration Cards** — extended Concept Cards with actionable detail.

### Inspiration Card Schema

```json
{
  "id": "insp-NNN",
  "title": "<concise idea title>",
  "oneLiner": "<1-sentence pitch — why this matters>",
  "lens": "<which provocation lens generated this>",
  "researchBacking": [
    {"source": "<URL>", "finding": "<key finding>", "relevance": 0.0}
  ],
  "implementationSketch": [
    "<step 1>", "<step 2>", "<step 3>"
  ],
  "risks": ["<risk 1>", "<risk 2>"],
  "nextSteps": ["<immediate action 1>", "<immediate action 2>"],
  "feasibility": 0.0,
  "impact": 0.0,
  "novelty": 0.0,
  "composite": 0.0,
  "verdict": "KEEP|DROP"
}
```

### Scoring Rubric

| Dimension | 0.0-0.2 | 0.3-0.5 | 0.6-0.8 | 0.9-1.0 |
|-----------|---------|---------|---------|---------|
| **Feasibility** | Requires tech that doesn't exist | Major unknowns, high risk | Achievable with known tech + moderate effort | Straightforward, proven patterns |
| **Impact** | Negligible improvement | Nice-to-have | Meaningful improvement to key metric | Transformative, 10x improvement |
| **Novelty** | Already standard practice | Minor twist on existing | Fresh combination of known ideas | Genuinely new approach |

**Composite:** `composite = (feasibility + impact + novelty) / 3`

**Verdict:** `composite >= 0.5` AND `researchBacking.length >= 1` → **KEEP**. Otherwise → **DROP**.

See [reference/scoring-rubric.md](reference/scoring-rubric.md) for detailed examples.

## Stage 5: CONVERGE

Filter and rank KEPT cards into the final recommendation set.

| Step | Action |
|------|--------|
| 1. Remove DROP cards | Only KEEP cards proceed |
| 2. Diversity filter | Max 2 cards per lens (prevents one lens dominating) |
| 3. Rank by composite | Highest composite first |
| 4. Select top N | QUICK: top 3-5, STANDARD: top 5-8, DEEP: top 8-12 |
| 5. Cluster by theme | Group related cards (e.g., "scaling" cluster, "simplification" cluster) |

## Stage 6: DELIVER

Output in the requested format.

### Format: `full` (default)

Human-readable Inspiration Report:

```markdown
# Inspiration Report: <topic>

## Problem Frame
- **Domain:** <domain>
- **Constraints:** <constraints>
- **Lenses applied:** <lens1>, <lens2>, <lens3>

## Top Recommendations

### 1. <title> (composite: 0.XX)
> <oneLiner>

**Lens:** <which lens>
**Evidence:** <source> — "<key finding>"

**Implementation sketch:**
1. <step 1>
2. <step 2>
3. <step 3>

**Risks:** <risk 1>, <risk 2>
**Next steps:** <action 1>, <action 2>

---
### 2. <title> ...

## Research Sources
| # | Source | Finding | Used By |
|---|--------|---------|---------|

## Dropped Ideas (for transparency)
| Idea | Lens | Composite | Drop Reason |
|------|------|-----------|-------------|
```

### Format: `brief`

Compact table:

```markdown
| # | Idea | Lens | Composite | One-Liner | Next Step |
|---|------|------|-----------|-----------|-----------|
```

### Format: `evolve`

JSON compatible with evolve-loop Scout task selection:

```json
{
  "conceptCandidates": [
    {
      "id": "insp-001",
      "title": "...",
      "targetFiles": ["..."],
      "complexity": "S|M",
      "feasibility": 0.0,
      "impact": 0.0,
      "novelty": 0.0,
      "composite": 0.0,
      "source": "inspirer",
      "lens": "<lens-name>",
      "researchBacking": ["<capsule-ref>"]
    }
  ]
}
```

## Depth Control

| Depth | Lenses | Queries | Token Budget | Duration | Best For |
|-------|--------|---------|-------------|----------|----------|
| **QUICK** | 3 | 3-5 | ~20K | ~30-60s | Fast ideation, time-constrained brainstorming |
| **STANDARD** | 4 | 5-8 | ~40K | ~2-3 min | Balanced creativity + research depth |
| **DEEP** | 5 | 8-12 | ~60K | ~4-6 min | Architecture decisions, strategy sessions, thorough exploration |

Default: **STANDARD**

## Evolve-Loop Integration

When invoked from within the evolve-loop pipeline, the inspirer provides enhanced creative divergence.

### Phase 1 Delegation

The orchestrator can delegate to inspirer at Step 2.5 (DIVERGENCE TRIGGER):

**Trigger conditions** (ALL must be true):
- `strategy == "innovate"` OR `discoveryVelocity.rolling3 < 0.5`
- Budget is GREEN (not YELLOW/RED)
- Lean mode is NOT active
- Strategy is NOT `repair` or `harden`

**Invocation:** `/inspirer [goal] --depth QUICK --format evolve --lenses 3`

**Result:** Returned concept cards merge with standard gap-analysis cards and flow to Scout with +2 priority boost (same as research-backed concepts).

### Standalone Use

Outside evolve-loop, the skill requires no pipeline infrastructure. It uses WebSearch and WebFetch tools directly.

## Reference (read on demand)

| File | When to Read |
|------|-------------|
| [reference/provocation-lenses.md](reference/provocation-lenses.md) | Deep lens descriptions, domain affinity matrix, examples |
| [reference/scoring-rubric.md](reference/scoring-rubric.md) | Detailed scoring criteria, card schema, worked scoring examples |
| [reference/worked-examples.md](reference/worked-examples.md) | 3 end-to-end pipeline examples |
