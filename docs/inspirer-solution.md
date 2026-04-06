# Inspirer Skill — Solution & Design Document

> Design document for the `/inspirer` skill — a standalone creative divergence engine grounded in data-driven research. Records the design rationale, architecture decisions, and relationship to existing evolve-loop mechanisms.

## Table of Contents

1. [Problem Statement](#problem-statement)
2. [Design Rationale](#design-rationale)
3. [Architecture](#architecture)
4. [Relationship to Existing Mechanisms](#relationship-to-existing-mechanisms)
5. [The 12 Provocation Lenses](#the-12-provocation-lenses)
6. [Research Grounding Protocol](#research-grounding-protocol)
7. [Scoring System](#scoring-system)
8. [Evolve-Loop Integration](#evolve-loop-integration)
9. [File Inventory](#file-inventory)
10. [References](#references)

---

## Problem Statement

The evolve-loop has powerful creativity mechanisms — 10 provocation lenses, Smart Web Search, Concept Cards with feasibility/impact/novelty scoring, and a Research Ledger that tracks what works and doesn't. But these mechanisms are **locked inside the evolve-loop cycle**:

| Mechanism | Where It Lives | Limitation |
|-----------|---------------|-----------|
| Provocation lenses | Phase 0.5 Step 2.5 | Only fires during evolve-loop cycles |
| Smart Web Search | Phase 0.5 Step 3 | Only accessible to evolve-loop orchestrator |
| Concept Cards | Phase 0.5 Step 4 | Output only consumed by Scout agent |
| Research Ledger | Phase 5 feedback | Only tracks evolve-loop cycle outcomes |

**The gap:** A developer asking "how should I approach X?" cannot access these creative thinking tools without running a full evolve-loop cycle. The mechanisms are excellent but trapped.

## Design Rationale

### Core Principle: Creative divergence grounded in evidence

Brainstorming without research produces vague suggestions. Research without creative framing produces obvious answers. The inspirer forces creative questions first (via provocation lenses), then grounds every idea in web research.

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Standalone skill** | Works without evolve-loop infrastructure. Any Claude Code user can invoke `/inspirer` on any topic. |
| **Reuse existing lenses** | The 10 lenses from `online-researcher.md` are proven. Add 2 new ones for non-codebase topics instead of reinventing. |
| **Depth control** | Users explicitly trade creativity depth for token cost (QUICK 20K / STANDARD 40K / DEEP 60K). |
| **Research requirement** | Every idea MUST have web research backing. No research = auto-drop. This is what separates inspirer from generic brainstorming. |
| **Actionable output** | Inspiration Cards include implementation sketches, risks, and next steps — not just idea titles. |
| **Loose coupling** | Evolve-loop integration via `--format evolve` flag, not tight coupling to `state.json`. |

## Architecture

Six-stage pipeline: **FRAME → DIVERGE → RESEARCH → SCORE → CONVERGE → DELIVER**

```
Input: "How should we handle X?"
         │
    ┌────▼────┐
    │  FRAME  │  Parse topic, classify domain, detect constraints
    └────┬────┘
         │
    ┌────▼─────┐
    │ DIVERGE  │  Apply 3-5 provocation lenses → divergent research questions
    └────┬─────┘  (each lens produces 1-2 questions that push thinking sideways)
         │
    ┌────▼─────┐
    │ RESEARCH │  Web search each question — every idea needs evidence
    └────┬─────┘  (Smart Web Search for deep, Default for quick)
         │
    ┌────▼────┐
    │  SCORE  │  Inspiration Cards: feasibility × impact × novelty
    └────┬────┘  (extended Concept Cards with implementation sketches)
         │
    ┌────▼─────┐
    │ CONVERGE │  Rank, diversity filter (max 2/lens), select top N
    └────┬─────┘
         │
    ┌────▼─────┐
    │ DELIVER  │  Inspiration Report / brief table / evolve-loop JSON
    └──────────┘
```

### What Makes This Different From Generic Brainstorming

| Aspect | Generic Brainstorm | `/inspirer` |
|--------|-------------------|-------------|
| **Input** | "Give me ideas for X" | Structured problem frame with domain classification |
| **Divergence** | Free association | 12 named provocation lenses with domain affinity |
| **Grounding** | None — relies on LLM knowledge | Every idea requires web research backing |
| **Scoring** | Subjective ("I like this one") | Quantitative: feasibility × impact × novelty |
| **Output** | Idea list | Scored cards with implementation sketches, risks, next steps |
| **Filtering** | None | KEEP/DROP verdicts; diversity filter; composite threshold |

## Relationship to Existing Mechanisms

### What We Reuse (Not Reinvent)

| Mechanism | Source | How Inspirer Uses It |
|-----------|--------|---------------------|
| 10 provocation lenses | `online-researcher.md` lines 56-67 | Ported to `reference/provocation-lenses.md`, extended with 2 new lenses |
| Smart Web Search | `smart-web-search.md` | Invoked directly for STANDARD/DEEP depth — no modification |
| Concept Card schema | `online-researcher.md` lines 256-283 | Extended into Inspiration Card (adds oneLiner, implementationSketch, risks, nextSteps) |
| Search routing | `online-researcher.md` lines 8-44 | Reuse routing decision table (Smart vs Default based on intent) |
| Scoring rubric | `online-researcher.md` composite scoring | Extended with 5-point granularity per dimension |

### What's New

| Addition | Why |
|----------|-----|
| 2 new lenses (Constraint Flip, Audience Shift) | The original 10 are codebase-oriented. General topics need lenses for business constraints and user perspective. |
| Domain classification | Routes lens selection to the most effective lenses per topic type |
| Depth control (QUICK/STANDARD/DEEP) | Evolve-loop has a fixed token budget per cycle. Standalone use needs explicit depth control. |
| 3 output formats (full/brief/evolve) | Evolve-loop only needs JSON concept cards. Humans need readable reports. |
| Implementation sketches in cards | Concept Cards are abstract. Inspiration Cards include 3-5 concrete implementation steps. |

## The 12 Provocation Lenses

| # | Lens | Question | Origin |
|---|------|---------|--------|
| 1 | Inversion | "What if we did the opposite?" | evolve-loop |
| 2 | Analogy | "What from an adjacent domain?" | evolve-loop |
| 3 | 10x Scale | "What breaks at 10x?" | evolve-loop |
| 4 | Removal | "What if we deleted this?" | evolve-loop |
| 5 | User-Adjacent | "What problem comes next?" | evolve-loop |
| 6 | First Principles | "Why does this exist?" | evolve-loop |
| 7 | Composition | "Combine two unrelated things?" | evolve-loop |
| 8 | Failure Mode | "How would this fail silently?" | evolve-loop |
| 9 | Ecosystem | "What external tool makes this obsolete?" | evolve-loop |
| 10 | Time Travel | "What will we wish we had in 3 months?" | evolve-loop |
| 11 | **Constraint Flip** | "What if the biggest constraint were removed?" | **NEW** |
| 12 | **Audience Shift** | "What if the user were someone different?" | **NEW** |

Research basis: ProActLLM (arXiv:2410.12361) — proactive agents that anticipate needs; ECIS 2024 — LLM-based divergent/convergent thinking; PROBE — proactive resolution of unspecified bottlenecks.

## Research Grounding Protocol

**Critical rule:** Ideas without at least 1 supporting research result score 0.0 on feasibility and are auto-dropped. No research = no recommendation.

| Depth | Search Method | Max Queries | Token Budget |
|-------|-------------|------------|-------------|
| QUICK | Default WebSearch | 3-5 | ~20K |
| STANDARD | Smart Web Search + Default mix | 5-8 | ~40K |
| DEEP | Smart Web Search for all | 8-12 | ~60K |

For each research result, the skill captures: source URL, key finding, relevance score, and recency. Technology topics penalize results > 2 years old.

## Scoring System

**Inspiration Card** = Concept Card + actionable fields:

| Field | Purpose |
|-------|---------|
| `oneLiner` | 1-sentence pitch — why this matters |
| `implementationSketch` | 3-5 concrete steps to start |
| `risks` | What could go wrong |
| `nextSteps` | Immediate actions |
| `feasibility` | 0.0-1.0 — can we actually do this? |
| `impact` | 0.0-1.0 — how much does this matter? |
| `novelty` | 0.0-1.0 — how fresh is this thinking? |

**Composite:** `(feasibility + impact + novelty) / 3`
**Verdict:** `composite >= 0.5` AND `researchBacking >= 1` → KEEP

## Evolve-Loop Integration

### Phase 0.5 Delegation (Optional)

When the evolve-loop orchestrator detects creativity is needed:

| Condition | Action |
|-----------|--------|
| `strategy == "innovate"` | Delegate to `/inspirer [goal] --depth QUICK --format evolve` |
| `discoveryVelocity.rolling3 < 0.5` | Delegate (stagnation needs fresh ideas) |
| `strategy == "repair"` or `"harden"` | Skip (focused strategies don't need divergence) |
| Budget YELLOW/RED or lean mode | Skip (conserve tokens) |

Returned concept cards merge with standard gap-analysis cards and flow to Scout with +2 priority boost.

## File Inventory

| File | Purpose | Lines |
|------|---------|-------|
| `skills/inspirer/SKILL.md` | Main skill definition | ~350 |
| `skills/inspirer/reference/provocation-lenses.md` | 12 lenses with examples and domain affinity matrix | ~200 |
| `skills/inspirer/reference/scoring-rubric.md` | Inspiration Card schema and scoring criteria | ~120 |
| `skills/inspirer/reference/worked-examples.md` | 3 end-to-end pipeline examples | ~200 |
| `skills/evolve-loop/phases.md` (modified) | Phase 0.5 delegation hook | +3 lines |
| `.claude-plugin/plugin.json` (modified) | Skill registration | +1 line |

## References

| Source | Key Contribution |
|--------|-----------------|
| ProActLLM (arXiv:2410.12361) | Proactive agents that anticipate needs before expressed |
| ECIS 2024 — LLM Ideation | LLM-based divergent/convergent thinking in creative ideation |
| PROBE | Proactive resolution of unspecified bottlenecks |
| evolve-loop `online-researcher.md` | Source of the 10 provocation lenses and Concept Card schema |
| evolve-loop `smart-web-search.md` | 6-stage intent-aware search pipeline reused for research grounding |
