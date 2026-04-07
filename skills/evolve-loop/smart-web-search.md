# Smart Web Search Protocol

> Intent-aware web search engine for LLM agents. Classifies user intent, transforms queries for maximum retrieval quality, executes iterative search-evaluate-refine loops, and produces grounded, cited responses. Works with WebSearch and WebFetch only. Usable standalone (`/smart-search`) or as a building block for other skills. Based on: Query2doc, Self-RAG, FLARE, ReAct, and Perplexity's RAG pipeline.

## Contents

- [Overview](#overview)
- [When to Use](#when-to-use)
- [Master Flow](#master-flow)
- [Reference (read on demand)](#reference-read-on-demand)

## Overview

Most LLM web search is naive: take the user question, pass it verbatim to a search API, summarize the first few results. This protocol replaces that with a 6-stage pipeline informed by retrieval-augmented generation research:

```
CLASSIFY → TRANSFORM → EXECUTE → EVALUATE → REFINE → SYNTHESIZE
```

Each stage has specific decision tables. Follow them in order. Do not skip stages.

## When to Use

**Use this protocol (deep research):**
- Surveys, deep dives, comparisons, or architecture research
- Phase 0.5 research producing concept cards
- User explicitly asks to search, research, or find information online
- Another skill delegates a complex search task to you

**Use Default WebSearch instead (quick lookup):**
- Factual single-answer lookups ("what is the API for X?")
- Troubleshooting error strings (exact-quote search is already optimal)
- Builder reactive lookups during implementation (API errors, config syntax)
- Token budget is LOW or EXHAUSTED
- Context budget pressure (YELLOW status)

See the Search Routing table in `online-researcher.md` for the full decision matrix.

**Do NOT use any web search:**
- Question is about the local codebase (use Grep/Glob instead)
- Question is purely mathematical or logical (answer directly)
- User explicitly says "don't search" or "from memory only"

## Master Flow

```
User Question
    │
    ▼
┌───────────────────────┐
│ Stage 1: CLASSIFY      │
│ Intent type + confidence│
└────────┬──────────────┘
         │
    ┌────┴────┐
    │HIGH +   │──── YES ──▶ Answer directly (no search)
    │stable?  │
    └────┬────┘
         │ NO
         ▼
┌───────────────────────┐
│ Check Cache            │──── HIT ──▶ Return cached answer
└────────┬──────────────┘
         │ MISS
         ▼
┌───────────────────────┐
│ Stage 2: TRANSFORM     │
│ T1→T2→T3→T4→T5→T6    │
│ Produces 1-3 queries   │
└────────┬──────────────┘
         │
         ▼
┌───────────────────────────────────┐
│ Stage 3: EXECUTE                   │
│ WebSearch (parallel if independent)│
│ WebFetch (if triggered)            │
└────────┬──────────────────────────┘
         │
         ▼
┌───────────────────────┐
│ Stage 4: EVALUATE      │
│ Score [IsRel][IsSup]   │
│ [IsUse][IsCur]         │
│ Discard composite < 0.4│
└────────┬──────────────┘
         │
    ┌────┴────────┐
    │>= 2 results │
    │composite >0.6│── YES ──▶ Stage 6: SYNTHESIZE
    └────┬────────┘
         │ NO
         ▼
┌───────────────────────┐
│ Stage 5: REFINE        │◀──┐
│ Reformulate query      │   │ Loop max 3x
│ Re-execute Stage 3+4   │───┘
└────────┬──────────────┘
         │ sufficient OR budget exhausted
         ▼
┌───────────────────────┐
│ Stage 6: SYNTHESIZE    │
│ Ground + cite + format │
│ Write to cache         │
└───────────────────────┘
```

## Reference (read on demand)

| File | When to read |
|------|-------------|
| [reference/search-stages.md](reference/search-stages.md) | Executing the pipeline — full Stage 1-6 protocol with intent classification, transforms, execution rules, scoring, refinement, and synthesis |
| [reference/search-integration.md](reference/search-integration.md) | Cache mechanics, budget tiers, operator syntax, integration API for callers, worked examples |
