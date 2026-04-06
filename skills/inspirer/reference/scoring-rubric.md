# Scoring Rubric — Inspiration Cards

> Detailed scoring criteria, card schema, and worked examples for the `/inspirer` skill's Stage 4 (SCORE).

## Inspiration Card Schema

```json
{
  "id": "insp-NNN",
  "title": "<concise idea title — max 10 words>",
  "oneLiner": "<1-sentence pitch — why this matters to the user>",
  "lens": "<provocation lens that generated this>",
  "researchBacking": [
    {
      "source": "<URL>",
      "title": "<source title>",
      "finding": "<key finding — 1-2 sentences>",
      "relevance": 0.0
    }
  ],
  "implementationSketch": [
    "<step 1: concrete action>",
    "<step 2: concrete action>",
    "<step 3: concrete action>"
  ],
  "risks": [
    "<risk 1: what could go wrong>",
    "<risk 2: what could go wrong>"
  ],
  "nextSteps": [
    "<action 1: what to do immediately>",
    "<action 2: what to do immediately>"
  ],
  "feasibility": 0.0,
  "impact": 0.0,
  "novelty": 0.0,
  "composite": 0.0,
  "verdict": "KEEP|DROP"
}
```

## Scoring Dimensions

### Feasibility (Can we actually do this?)

| Score | Label | Criteria |
|-------|-------|----------|
| 0.0-0.2 | Impossible | Requires technology that doesn't exist; fundamental unsolved problem |
| 0.3-0.4 | Speculative | Major unknowns; no proven implementations found in research |
| 0.5-0.6 | Challenging | Achievable but requires significant effort or expertise |
| 0.7-0.8 | Practical | Achievable with known tech, moderate effort, some precedent |
| 0.9-1.0 | Straightforward | Proven patterns exist; well-documented; low execution risk |

### Impact (How much does this matter?)

| Score | Label | Criteria |
|-------|-------|----------|
| 0.0-0.2 | Negligible | No measurable improvement; cosmetic only |
| 0.3-0.4 | Minor | Nice-to-have; improves a secondary metric slightly |
| 0.5-0.6 | Moderate | Meaningful improvement to a key metric or user experience |
| 0.7-0.8 | Significant | Major improvement; addresses a top-3 pain point |
| 0.9-1.0 | Transformative | 10x improvement; game-changer for the domain |

### Novelty (How fresh is this thinking?)

| Score | Label | Criteria |
|-------|-------|----------|
| 0.0-0.2 | Standard | Already common practice; first page of any tutorial |
| 0.3-0.4 | Incremental | Minor twist on well-known approach |
| 0.5-0.6 | Fresh | Combines known ideas in a non-obvious way |
| 0.7-0.8 | Innovative | Applies technique from a different domain; cross-pollination |
| 0.9-1.0 | Original | Genuinely new approach; no direct precedent found |

## Composite Calculation

```
composite = (feasibility + impact + novelty) / 3
```

## Verdict Rules

| Condition | Verdict |
|-----------|---------|
| `composite >= 0.5` AND `researchBacking.length >= 1` | **KEEP** |
| `composite >= 0.5` AND `researchBacking.length == 0` | **DROP** (no evidence) |
| `composite < 0.5` | **DROP** (below threshold) |

## Worked Scoring Example

**Idea:** "Use event sourcing with CQRS for user activity tracking"
- **Lens:** 10x Scale
- **Research:** Found Uber engineering blog showing 50x throughput improvement; EventStoreDB benchmarks
- **Feasibility:** 0.7 (proven pattern, but requires architectural shift)
- **Impact:** 0.8 (solves the scaling bottleneck directly)
- **Novelty:** 0.5 (known pattern, but novel application to this specific problem)
- **Composite:** (0.7 + 0.8 + 0.5) / 3 = **0.67** → **KEEP**

**Idea:** "Quantum computing for faster database queries"
- **Lens:** Constraint Flip
- **Research:** No practical implementations found; quantum DB is 5-10 years away
- **Feasibility:** 0.1 (technology doesn't exist for production use)
- **Impact:** 0.9 (would be transformative if possible)
- **Novelty:** 0.8 (genuinely different approach)
- **Composite:** (0.1 + 0.9 + 0.8) / 3 = **0.60** → technically KEEP but feasibility below 0.3 should trigger caution flag
