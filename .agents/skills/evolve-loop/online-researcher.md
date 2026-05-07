# Accurate Online Researcher Protocol (2026 Standard)

This protocol defines how evolve-loop conducts online research. Phase 1 (RESEARCH) is the primary consumer — it runs the research loop every cycle. Builder uses this protocol reactively for unforeseen knowledge gaps during implementation.

## The Core Concept: Knowledge Capsules
Instead of reading the web directly in the middle of a build task, agents must perform research, distill the required knowledge into a dense **Knowledge Capsule**, and save it locally. The LLM simply retrieves the needed knowledge from the internet, stores the critical parts locally, and performs its tasks from the local cache. Future cycles read the capsule instead of searching the internet.

## Search Routing — Smart vs Default

Not all queries need the full 6-stage Smart Web Search pipeline. Route each query to the appropriate tool based on its complexity and the current token budget.

### Routing Decision Table

| Condition | Route to | Reason |
|-----------|----------|--------|
| **Phase 1 research** (gap analysis, concept cards) | Smart Web Search | Deep research needs intent classification, query transformation, iterative refinement |
| **`autoresearch` or `ultrathink` strategy** | **Smart Web Search** | **MANDATORY:** Overrides budget limits to guarantee out-of-the-box thinking and divergent research. |
| **Survey / Deep dive / Comparison** intent | Smart Web Search | Multi-angle queries, WebFetch extraction, and synthesis produce significantly better results |
| **Builder reactive lookup** (unforeseen API gap, error fix) | Default WebSearch | Quick single-query lookup; saves ~60% tokens vs Smart |
| **Budget LOW or EXHAUSTED** | Default WebSearch | Smart pipeline overhead exceeds value at low budgets |
| **Budget YELLOW** (context pressure) | Default WebSearch | Conserve tokens for build/audit phases (unless overridden by strategy) |
| **Factual / Artifact** intent (single-fact lookup) | Default WebSearch | One query suffices; no need for transformation pipeline |
| **Troubleshooting** (error string lookup) | Default WebSearch | Exact-quote search is already optimal; Smart adds minimal value |
| **How-to with known official docs** | Default WebSearch + single WebFetch | Skip transformation; go directly to the source |

### Cost Profile

| Approach | Typical Tokens | Typical Duration | Best For |
|----------|---------------|-----------------|----------|
| Smart Web Search (FULL) | ~40-45K | ~230s | Surveys, deep dives, architecture research, concept card generation |
| Smart Web Search (MEDIUM) | ~25-30K | ~150s | Moderate research with some depth needed |
| Default WebSearch (1-2 queries) | ~5-10K | ~30-60s | Quick factual lookups, error resolution, API checks |
| Default WebSearch (3-5 queries) | ~15-25K | ~90-150s | Broad landscape mapping, multi-topic quick scan |

### How to Use Default WebSearch

For quick lookups routed to Default WebSearch, call WebSearch directly (no smart-web-search.md pipeline):

1. **Formulate a keyword query** — add year for volatile topics, exact-quote error strings
2. **Execute 1-2 WebSearch calls** — one per distinct sub-topic
3. **Scan results** — extract the key fact or URL needed
4. **Optionally WebFetch** 1 URL if the snippet is insufficient
5. **Distill and cache** — follow the same Distill and Cache steps below

Do NOT apply the 6-stage pipeline (intent classification, T1-T6 transforms, reflection scoring, iterative refinement) for Default WebSearch queries. The overhead is not justified for simple lookups.

## Beyond-the-Ask Divergence Trigger

A structured provocation system that stimulates the LLM to think beyond the user's explicit request. Fires during Phase 1 research and Scout hypothesis generation to surface ideas the user didn't ask for but should consider.

**Research basis:** ProActLLM (arXiv:2410.12361) — proactive agents anticipate needs before expressed; ECIS 2024 — LLM-based divergent/convergent thinking in ideation; PROBE — proactive resolution of unspecified bottlenecks.

### Provocation Lenses

Each cycle, select **2 lenses** (1 random + 1 matched to weakest benchmark dimension). Apply them after gap analysis to generate divergent research queries and hypotheses.

| Lens | Provocation Question | Best For |
|------|---------------------|----------|
| **Inversion** | "What if we did the exact opposite of the current approach?" | Breaking assumptions, finding anti-patterns |
| **Analogy** | "What would this look like if borrowed from {adjacent domain}?" | Cross-pollination, novel techniques |
| **10x Scale** | "What breaks if this needs to handle 10x the current load/complexity?" | Revealing hidden bottlenecks |
| **Removal** | "What if we deleted this entirely — what would we lose and gain?" | Simplification, dead code discovery |
| **User-Adjacent** | "What problem is the user going to hit NEXT after this one is solved?" | Proactive suggestions, workflow gaps |
| **First Principles** | "Why does this exist? What fundamental constraint requires it?" | Challenging inherited complexity |
| **Composition** | "What if we combined two unrelated features/modules into one?" | Emergent capabilities, DRY opportunities |
| **Failure Mode** | "How would this fail silently? What's the worst undetected failure?" | Resilience, observability gaps |
| **Ecosystem** | "What tool/library/pattern exists externally that makes this obsolete?" | Adoption opportunities, wheel reinvention |
| **Time Travel** | "What will we wish we had built 3 months from now?" | Forward-looking architecture |

### Lens Selection Protocol

```
# 1. Random lens (ensures diversity)
RANDOM_LENS = lenses[RANDOM % len(lenses)]

# 2. Dimension-matched lens (targeted)
weakest = argmin(projectBenchmark.dimensions)
MATCHED_LENS = dimensionToLens[weakest]
```

**Dimension-to-Lens mapping:**

| Weakest Dimension | Matched Lens | Rationale |
|-------------------|-------------|-----------|
| documentationCompleteness | User-Adjacent | What docs will users need next? |
| specificationConsistency | First Principles | Why do specs diverge? |
| defensiveDesign | Failure Mode | What fails silently? |
| evalInfrastructure | Inversion | What if evals tested the opposite? |
| modularity | Removal | What can we delete to simplify? |
| schemaHygiene | Composition | What schemas overlap? |
| conventionAdherence | Analogy | How do best-in-class projects handle this? |
| featureCoverage | Ecosystem | What external tools solve this? |

### Trigger Execution (Phase 1, after gap analysis)

1. **Select lenses** — 1 random + 1 dimension-matched (deduplicate if same)
2. **Generate provocation queries** — for each lens, produce 1 research question using the provocation template applied to the current project context and goal
3. **Route to appropriate search tool** — provocation queries are typically Survey/Deep dive intent → route to **Smart Web Search** per the Search Routing table
4. **Score findings** — same Novelty/Relevance/Yield scoring as standard research
5. **Tag outputs** — all concept cards from provocation queries get tagged `"source": "beyond-ask"` and `"lens": "<lens-name>"`
6. **Apply +1 priority boost** to beyond-ask concepts (proactive insights are high-value)

### Trigger Execution (Scout, hypothesis generation)

Scout applies the **same 2 lenses** selected in Phase 1 when generating hypotheses (step 6). For each lens:
1. Apply the provocation question to the codebase findings from discovery
2. Generate 1 hypothesis tagged `"source": "beyond-ask"`, `"lens": "<lens-name>"`
3. Beyond-ask hypotheses with confidence >= 0.6 (lower threshold than standard 0.7) auto-promote to task candidates

### Tracking and Feedback

Phase 6 (LEARN) tracks beyond-ask outcomes separately:

| Metric | How |
|--------|-----|
| **beyond-ask hit rate** | % of beyond-ask proposals that get selected as tasks within 3 cycles |
| **beyond-ask value** | benchmark delta from shipped beyond-ask tasks vs standard tasks |
| **lens effectiveness** | per-lens hit rate — lenses below 10% hit rate after 10 cycles get replaced |

Store in `state.json.beyondAsk`:
```json
{
  "beyondAsk": {
    "lensHistory": [{"cycle": N, "lenses": ["inversion", "ecosystem"], "conceptsGenerated": 2, "conceptsKept": 1}],
    "hitRate": 0.0-1.0,
    "lensEffectiveness": {"inversion": {"attempts": 5, "hits": 2}, ...}
  }
}
```

### Output in Research Brief

Add a `## Beyond-the-Ask Provocations` section to `research-brief.md`:

```markdown
## Beyond-the-Ask Provocations
| Lens | Provocation | Finding | Concept Card? |
|------|------------|---------|---------------|
| Inversion | "What if eval graders tested failure instead of success?" | ... | cc-042 |
| Ecosystem | "What external tool makes X obsolete?" | ... | DROPPED (low feasibility) |
```

---

## The Research Workflow (Search-Distill-Cache)

When an agent encounters a knowledge gap (e.g., "How does the new Stripe v2 API work?" or "What are the latest 2026 Next.js routing patterns?"), follow this execution loop:

### 1. Search (route per Search Routing table above)

**For deep research (Smart Web Search):** Delegate to the **Smart Web Search protocol** in `smart-web-search.md`. The protocol handles intent classification, query transformation, execution, result evaluation, and iterative refinement.

**Invocation (Smart):**
- `question`: The knowledge gap as a specific question
- `context`: The evolve-loop project context (domain, current benchmark weaknesses, what you already know)
- `budget`: Map from `tokenBudget.researchPhase` remaining (>15K → FULL, 10-15K → MEDIUM, 5-10K → LOW, <5K → EXHAUSTED)

**What you get back:** A grounded answer with inline citations, a source list, confidence level (HIGH/MEDIUM/LOW), and the queries that were executed.

**For quick lookups (Default WebSearch):** Call WebSearch directly with 1-2 keyword queries. Add year filters for volatile topics. Extract the needed fact from result snippets. Optionally WebFetch 1 URL for more detail.

**If confidence is LOW (either route):** Consider a second invocation with a rephrased question or narrower scope before proceeding.

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
- **Phase 1 (RESEARCH):** Primary research consumer. Runs every cycle. Executes the Research Agenda Protocol (below) to generate queries from evaluation signals, then routes to **Smart Web Search** (for surveys, deep dives, concept card research) or **Default WebSearch** (for factual checks, simple gap fills) per the Search Routing table above.
- **Scout (Phase 2):** No longer performs web research. Reads `research-brief.md` from Phase 1 and consumes `conceptCandidates` for task selection.
- **Builder (Phase 3):** Uses this protocol reactively if an unforeseen knowledge gap arises during implementation (e.g., an obscure API error). Routes to **Default WebSearch** (1-2 direct queries) unless the gap is a complex architecture question requiring depth. Caches the result as a Knowledge Capsule for future tasks.

## Research Agenda Protocol

Phase 1 uses the Research Agenda (`state.json.researchAgenda`) to generate targeted queries. This protocol transforms evaluation signals into research questions.

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

Phase 1 converts research findings into structured implementation ideas.

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

Before generating Concept Cards, Phase 1 MUST check `researchLedger.triedConcepts[]`:

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

### Verdict Rules (Phase 6 writes, Phase 1 reads)

| Condition | Verdict | Keep/Drop |
|-----------|---------|-----------|
| Benchmark dimension improved >= 3 points | **WORKS** | KEEP, boost similar +1 |
| Benchmark unchanged or declined after implementation | **DOESNT_WORK** | DROP, block similar concepts |
| Eval PASS on first attempt | **WORKS** | KEEP |
| Eval FAIL after 2+ retries | **DOESNT_WORK** | DROP, record failure pattern |
| Shipped but no measurable improvement | **INCONCLUSIVE** | Keep 1 more cycle, then DROP if still no signal |
