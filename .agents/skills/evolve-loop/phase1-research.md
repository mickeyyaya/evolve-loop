---
name: evolve-loop/phase1-research
description: Phase 1 Research logic.
---

# Phase 1: RESEARCH

> Read this file at the start of every cycle. Proactive research loop that transforms evaluation signals into research questions, generates Concept Cards, and filters them through the Research Ledger.

Runs inline by the orchestrator (no separate agent).

## Token Budget

Max `tokenBudget.researchPhase` (25K). Terminate iteration if exceeded.

## Skip Conditions

- Lean mode active AND no benchmark weaknesses AND `researchAgenda.items` empty → skip, pass existing capsules
- Budget YELLOW → 1 iteration max, 1 query only

## Step 1: ORIENT — Read evaluation signals

Read from `state.json` (already loaded at cycle start):
- `benchmarkWeaknesses` → dimensions below 80
- `proposals[]` → pending proposals needing research backing
- `evalHistory` (last 5) → success rate trends
- `discoveryVelocity` → proposals/cycle trend
- `failedApproaches` → what to avoid
- `researchAgenda` → persistent research questions
- `researchLedger` → what worked/didn't from prior cycles

## Step 2: GAP ANALYSIS — Identify blind spots

1. For each benchmark dimension below 80: check `researchAgenda.capsuleIndex` for coverage
2. For each proposal without research backing: flag as gap
3. Identify dimensions with `diversityTracker.dimensionCoverage == 0` → auto-P0
4. Check diversity: if same dimension in `lastResearchedDimensions[-3:]`, block it
5. Output: ranked gap list → generate research agenda items if new

## Step 2.5: DIVERGENCE TRIGGER — Beyond-the-Ask provocations

Apply the Beyond-the-Ask Divergence Trigger from `online-researcher.md`:
1. Select 2 provocation lenses (1 random + 1 matched to weakest benchmark dimension)
2. Generate 1 provocation research query per lens
3. Pass selected lenses to Scout context for hypothesis generation

**Enhanced divergence via `/inspirer` (optional):** When `strategy == "innovate"` OR `discoveryVelocity.rolling3 < 0.5`, delegate to `/inspirer [goal] --depth QUICK --format evolve --lenses 3` for broader creative exploration with web-grounded research. Merge returned concept cards with standard gap-analysis cards (same +2 priority boost). Skip when `strategy == "repair"|"harden"`, lean mode active, or budget YELLOW/RED.

**Skip conditions:** lean mode active OR budget YELLOW → skip trigger (both standard and enhanced), use standard queries only.

## Step 3: RESEARCH — Execute 2-3 web queries + provocation queries

Follow `online-researcher.md` protocol with **search routing**:
- Generate standard queries from gap analysis using Signal-to-Question Mapping (see `online-researcher.md`)
- Generate provocation queries from Step 2.5 lenses (tagged `beyond-ask`)
- **Route each query** per the Search Routing table in `online-researcher.md`:
  - Survey/deep dive/comparison queries → **Smart Web Search** (`smart-web-search.md`)
  - Factual checks, simple gaps, budget-constrained → **Default WebSearch** (direct 1-2 queries)
  - Provocation queries (typically Survey intent) → **Smart Web Search**
- Write/update Knowledge Capsules to `.evolve/research/`
- Update `researchAgenda.capsuleIndex` with new capsule categorizations
- Score each query with Novelty/Relevance/Yield composite
- Tag beyond-ask concept cards with `"source": "beyond-ask"` and `"lens": "<lens-name>"`

## Step 4: CONCEPTUALIZE — Generate Concept Cards

For each research finding with composite >= 0.5:
- Create a Concept Card (schema in `online-researcher.md`)
- Score: feasibility, impact, novelty → composite
- Link to research agenda item and capsule

## Step 5: EVALUATE — Strict works/doesn't-work filter

Check each concept against `researchLedger.triedConcepts[]`:

| Ledger match | Action |
|-------------|--------|
| Similar concept with `DOESNT_WORK` | **Immediate DROP** — blocked by ledger |
| Similar concept with `WORKS` | **Boost** composite +0.1 |
| Similar concept with `INCONCLUSIVE` | Flag, no boost |
| No match | Score normally |

Apply diversity check: reject concepts clustering in same dimension as last 3 cycles.

**Binary verdict:** composite >= 0.6 AND not blocked → **KEEP**. Everything else → **DROP**.

## Step 6: DECIDE — Re-research or exit

- If ALL concepts DROPPED AND iteration < 2 → refine queries targeting different dimensions, go to Step 3
- Otherwise → exit with KEPT concepts

## Output

Write `$WORKSPACE_PATH/research-brief.md`:

```markdown
# Cycle {N} Research Brief

## Gap Analysis
| Gap | Dimension | Capsule Coverage | Priority |
|-----|-----------|-----------------|----------|

## Research Executed
| Query | Composite Score | Capsule Written |
|-------|----------------|-----------------|

## Concept Cards
| ID | Title | Feasibility | Impact | Novelty | Composite | Verdict |
|----|-------|------------|--------|---------|-----------|---------|

## Research Ledger Checks
| Concept | Similar in Ledger? | Ledger Verdict | Action |
|---------|-------------------|----------------|--------|

## Diversity Status
| Dimension | Coverage Count | Last Researched |
|-----------|---------------|-----------------|
```

Pass `conceptCandidates[]` (KEPT concepts only) to Scout context.

Update `state.json`: `researchAgenda` items, `capsuleIndex`, `diversityTracker.lastResearchedDimensions`.

**Phase gate:** `bash scripts/lifecycle/phase-gate.sh research-to-discover $CYCLE $WORKSPACE_PATH`
