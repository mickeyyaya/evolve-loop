# Accurate Online Researcher Protocol (2026 Standard)

This protocol defines how evolve-loop conducts online research. Phase 0.5 (RESEARCH) is the primary consumer — it runs the research loop every cycle. Builder uses this protocol reactively for unforeseen knowledge gaps during implementation.

## The Core Concept: Knowledge Capsules
Instead of reading the web directly in the middle of a build task, agents must perform research, distill the required knowledge into a dense **Knowledge Capsule**, and save it locally. The LLM simply retrieves the needed knowledge from the internet, stores the critical parts locally, and performs its tasks from the local cache. Future cycles read the capsule instead of searching the internet.

## The Research Workflow (Search-Distill-Cache)

When an agent encounters a knowledge gap (e.g., "How does the new Stripe v2 API work?" or "What are the latest 2026 Next.js routing patterns?"), follow this execution loop:

### 1. Search (via Smart Web Search)

Delegate ALL web search to the **Smart Web Search protocol** in `smart-web-search.md`. Do not call WebSearch or WebFetch directly — the protocol handles intent classification, query transformation, execution, result evaluation, and iterative refinement.

**Invocation:**
- `question`: The knowledge gap as a specific question
- `context`: The evolve-loop project context (domain, current benchmark weaknesses, what you already know)
- `budget`: Map from `tokenBudget.researchPhase` remaining (>15K → FULL, 10-15K → MEDIUM, 5-10K → LOW, <5K → EXHAUSTED)

**What you get back:** A grounded answer with inline citations, a source list, confidence level (HIGH/MEDIUM/LOW), and the queries that were executed.

**If confidence is LOW:** Consider a second invocation with a rephrased question or narrower scope before proceeding.

### 2. Distill

From the smart-web-search answer, extract ONLY:
- Facts, code snippets, and architectural constraints relevant to the current project context
- Anti-patterns and "don't do this" warnings
- Version-specific notes and compatibility requirements

Discard: marketing fluff, outdated tutorials, tangential information, generic advice not specific to the project.

If the search answer flagged conflicting sources, note the conflict in the capsule and prefer the higher-authority source.

### 3. Cache (Local Storage)

Write the distilled findings to a local Knowledge Capsule at `.evolve/research/<topic-slug>.md`:
```markdown
# Research: <Topic>
**Date:** <ISO-8601>
**Sources:** <URLs from smart-web-search source list>
**Confidence:** <HIGH|MEDIUM|LOW from smart-web-search>

## Key Constraints
- <Must-dos and anti-patterns>

## Code Patterns
- <Executable, concise snippets>
```
Once saved, proceed with the original task using the local capsule.

## Deduplication and Cache Invalidation
- **Cross-Run Research Deduplication (Query-Level Locking):** Before performing a web search, each run executes this protocol to prevent parallel runs from duplicating research tokens:
  1. Read `state.json research.queries` with an OCC version check.
  2. Check if any existing entry matches the intended topic (keyword overlap > 0.5 AND `issuedAt` within 12 hours).
  3. If match found: skip query and reuse `findings`.
  4. If no match: write placeholder entry `{"topic": "...", "status": "pending", "issuedAt": "<now>", "cycleNumber": <N>}` to `state.json` and atomically increment `version`.
  5. Wait logic: If an entry is `"pending"`, wait up to 60 seconds (polling every 5s). If still pending, overwrite the stale placeholder and search independently.
  6. After the query completes, update the placeholder to `"status": "complete"` and write the actual `findings`.
- If a capsule already exists at `.evolve/research/<topic-slug>.md`, use it.
- If the capsule is older than 30 days and the topic is volatile (e.g., latest frontend framework), invalidate the cache, re-research, and overwrite the file.
- For stable topics (e.g., POSIX shell standards), capsules never expire.

## Research Quality Scoring

After each web search query, score the result to decide whether to create a capsule:

| Dimension | Score | Criteria |
|-----------|-------|---------|
| **Novelty** | 0.0-1.0 | Not already covered in existing capsules or `research.queries` |
| **Relevance** | 0.0-1.0 | Directly applicable to the current goal or benchmark weakness |
| **Yield** | 0.0-1.0 | Contains actionable findings translatable into a concrete task |

**Composite** = mean(novelty, relevance, yield). If composite < 0.3, skip capsule creation. If composite > 0.7, tasks derived from this query get +1 priority boost.

Record scores in scout-report.md under the Research section:
```
| Query | Novelty | Relevance | Yield | Composite |
```

## Cross-Agent Integration
- **Phase 0.5 (RESEARCH):** Primary research consumer. Runs every cycle. Executes the Research Agenda Protocol (below) to generate queries from evaluation signals, then invokes `smart-web-search.md` for each query to produce Knowledge Capsules and create Concept Cards.
- **Scout (Phase 1):** No longer performs web research. Reads `research-brief.md` from Phase 0.5 and consumes `conceptCandidates` for task selection.
- **Builder (Phase 2):** Uses this protocol reactively if an unforeseen knowledge gap arises during implementation (e.g., an obscure API error). Invokes `smart-web-search.md` with budget LOW or MEDIUM, then caches the result as a Knowledge Capsule for future tasks.

## Research Agenda Protocol

Phase 0.5 uses the Research Agenda (`state.json.researchAgenda`) to generate targeted queries. This protocol transforms evaluation signals into research questions.

### Signal-to-Question Mapping

| Evaluation Signal | Research Question Template | Priority |
|---|---|---|
| `benchmarkWeaknesses[dim].score < 60` | "Best practices for {dimension} in {projectContext.domain} 2026" | P0 |
| `benchmarkWeaknesses[dim].score < 80` | "Advanced techniques for {dimension} improvement" | P1 |
| `proposals[].source == "builder-discovery"` | "State of the art for {proposal.category}: {proposal.title}" | P1 |
| `failedApproaches[].errorCategory` repeats 3+ times | "Alternative approaches to {errorCategory} in {domain}" | P0 |
| `discoveryVelocity.rolling3 < 0.5` | "Emerging patterns in {projectContext.domain} {year}" | P2 |
| `evalHistory[].delta.successRate < 0.7` for 2+ cycles | "Common failure modes in {attempted task types}" | P1 |
| `researchAgenda.items[].status == "open"` | Use the agenda item's pre-formulated query | P0-P2 |

### Gap Analysis (before generating queries)

1. Read all 8 benchmark dimensions from `projectBenchmark.dimensions`
2. For each dimension below 80: check `capsuleIndex` for existing coverage
3. For each proposal in `state.json.proposals`: check if supporting research exists
4. Identify blind spots: dimensions with 0 capsules and no recent queries
5. Output: ranked list of gaps → generates research agenda items

### Capsule Index Management

When creating or referencing a capsule, categorize it in `researchAgenda.capsuleIndex`:
- Match capsule topic to benchmark dimension(s)
- Before creating a capsule, check index for existing coverage (deduplication)
- Max 3 capsules per dimension before forced switch to underserved dimensions

## Concept Card Schema

Phase 0.5 converts research findings into structured implementation ideas.

```json
{
  "id": "cc-NNN",
  "title": "string — concrete implementation idea",
  "targetFiles": ["path/to/file"],
  "complexity": "S|M",
  "researchBacking": ["capsule-slug"],
  "agendaItemId": "ra-NNN",
  "feasibility": 0.0-1.0,
  "impact": 0.0-1.0,
  "novelty": 0.0-1.0,
  "composite": 0.0-1.0
}
```

**Scoring rubric:**

| Dimension | 0.0 | 0.5 | 1.0 |
|-----------|-----|-----|-----|
| **Feasibility** | Requires major new infrastructure | Modifies 3-5 existing files | Modifies 1-2 files, clear pattern |
| **Impact** | No benchmark dimension affected | Addresses P1 gap | Addresses P0 gap, >= 5 point improvement expected |
| **Novelty** | Repeat of existing capsule knowledge | Extends known approach | Entirely new technique/pattern |

**Composite** = mean(feasibility, impact, novelty)

**Verdict rules (strict, binary):**

| Composite | Similar concept in researchLedger? | Verdict |
|-----------|-----------------------------------|---------|
| >= 0.6 | No prior attempt | **KEEP** — pass to Scout as conceptCandidate |
| >= 0.6 | Prior attempt WORKS | **KEEP** + boost (validated pattern) |
| >= 0.6 | Prior attempt DOESNT_WORK | **DROP** — log "blocked by ledger" |
| < 0.6 | Any | **DROP** — insufficient quality |

## Research Ledger Integration

Before generating Concept Cards, Phase 0.5 MUST check `researchLedger.triedConcepts[]`:

1. For each potential concept, search `triedConcepts` for similar titles (keyword overlap > 0.5)
2. If match found with `verdict: "DOESNT_WORK"` → **immediate drop**, do not create concept card
3. If match found with `verdict: "WORKS"` → **boost** composite by +0.1 (capped at 1.0)
4. If match found with `verdict: "INCONCLUSIVE"` → flag for re-evaluation, no boost

### Diversity Enforcement

Check `researchLedger.diversityTracker` before executing queries:

| Rule | Check | Action if violated |
|------|-------|--------------------|
| No dimension researched 3 cycles in a row | `lastResearchedDimensions[-3:]` all same | Block that dimension, research next-priority |
| Underserved dimensions get P0 | `dimensionCoverage[dim] == 0` | Auto-promote to P0 regardless of signal |
| Max capsules per dimension | `capsuleIndex[dim].length >= 3` | Deprioritize, switch to underserved |

### Verdict Rules (Phase 5 writes, Phase 0.5 reads)

| Condition | Verdict | Keep/Drop |
|-----------|---------|-----------|
| Benchmark dimension improved >= 3 points | **WORKS** | KEEP, boost similar +1 |
| Benchmark unchanged or declined after implementation | **DOESNT_WORK** | DROP, block similar concepts |
| Eval PASS on first attempt | **WORKS** | KEEP |
| Eval FAIL after 2+ retries | **DOESNT_WORK** | DROP, record failure pattern |
| Shipped but no measurable improvement | **INCONCLUSIVE** | Keep 1 more cycle, then DROP if still no signal |
