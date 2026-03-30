# Smart Web Search Protocol

> Intent-aware web search engine for LLM agents. Classifies user intent, transforms
> queries for maximum retrieval quality, executes iterative search-evaluate-refine
> loops, and produces grounded, cited responses. Works with WebSearch and WebFetch
> only. Usable standalone (`/smart-search`) or as a building block for other skills.
> Based on: Query2doc, Self-RAG, FLARE, ReAct, and Perplexity's RAG pipeline.

## Contents

- [Overview](#overview)
- [Stage 1: Intent Classification](#stage-1-intent-classification)
  - [Intent Type Table](#intent-type-table)
  - [Confidence Gate](#confidence-gate)
- [Stage 2: Query Transformation Pipeline](#stage-2-query-transformation-pipeline)
  - [T1 Affirmative Rewrite](#t1-affirmative-rewrite)
  - [T2 Query2doc Expansion](#t2-query2doc-expansion)
  - [T3 Decomposition](#t3-decomposition)
  - [T4 Operator Injection](#t4-operator-injection)
  - [T5 Orthogonal Angles](#t5-orthogonal-angles)
  - [T6 Domain Scoping](#t6-domain-scoping)
- [Stage 3: Search Execution](#stage-3-search-execution)
  - [Execution Rules](#execution-rules)
  - [WebFetch Extraction Prompts](#webfetch-extraction-prompts)
- [Stage 4: Result Evaluation](#stage-4-result-evaluation)
  - [Reflection Scoring](#reflection-scoring)
  - [Conflict Resolution](#conflict-resolution)
- [Stage 5: Iterative Refinement](#stage-5-iterative-refinement)
  - [Refinement Decision Table](#refinement-decision-table)
  - [FLARE Mid-Generation Retrieval](#flare-mid-generation-retrieval)
  - [Iteration Protocol](#iteration-protocol)
- [Stage 6: Synthesis and Grounding](#stage-6-synthesis-and-grounding)
  - [Grounding Rules](#grounding-rules)
  - [Output Format](#output-format)
- [Cache Protocol](#cache-protocol)
  - [Cache Mechanics](#cache-mechanics)
  - [Topic Volatility](#topic-volatility)
  - [Evolve-Loop Integration](#evolve-loop-integration)
- [Budget Management](#budget-management)
  - [Budget Tiers](#budget-tiers)
  - [Budget Tracking](#budget-tracking)
- [Operator Syntax Reference](#operator-syntax-reference)
  - [Operator Table](#operator-table)
  - [Auto-Applied Operators](#auto-applied-operators)
- [Integration API](#integration-api)
  - [Input Contract](#input-contract)
  - [Output Contract](#output-contract)
  - [Calling From Other Skills](#calling-from-other-skills)
- [Master Flow](#master-flow)
- [Worked Examples](#worked-examples)
  - [Example 1: Troubleshooting](#example-1-troubleshooting)
  - [Example 2: Survey](#example-2-survey)
  - [Example 3: How-To](#example-3-how-to)

---

## Overview

Most LLM web search is naive: take the user question, pass it verbatim to a search API, summarize the first few results. This protocol replaces that with a 6-stage pipeline informed by retrieval-augmented generation research:

```
CLASSIFY → TRANSFORM → EXECUTE → EVALUATE → REFINE → SYNTHESIZE
```

Each stage has specific decision tables. Follow them in order. Do not skip stages.

**When to use this protocol (deep research):**
- Surveys, deep dives, comparisons, or architecture research
- Phase 0.5 research producing concept cards
- User explicitly asks to search, research, or find information online
- Another skill delegates a complex search task to you

**When to use Default WebSearch instead (quick lookup):**
- Factual single-answer lookups ("what is the API for X?")
- Troubleshooting error strings (exact-quote search is already optimal)
- Builder reactive lookups during implementation (API errors, config syntax)
- Token budget is LOW or EXHAUSTED
- Context budget pressure (YELLOW status)

See the Search Routing table in `online-researcher.md` for the full decision matrix.

**When NOT to use any web search:**
- Question is about the local codebase (use Grep/Glob instead)
- Question is purely mathematical or logical (answer directly)
- User explicitly says "don't search" or "from memory only"

---

## Stage 1: Intent Classification

Before executing any search, classify the question. This determines the entire downstream strategy.

### Intent Type Table

| Intent Type | Signal Words / Patterns | Strategy | Max Queries |
|---|---|---|---|
| **Factual** | "what is", "when did", "who", "define", "meaning of" | Single precise query with exact terms | 1 |
| **How-to** | "how to", "configure", "set up", "implement", "create" | Affirmative rewrite + official docs `site:` filter | 2 |
| **Comparison** | "vs", "compare", "difference between", "which is better", "or" | Parallel queries — one per option in same context | 2-3 |
| **Troubleshooting** | Error strings, stack traces, "not working", "fails", "bug" | Exact-quote the error + contextual description query | 2 |
| **Survey** | "best practices", "latest", "modern", "2025/2026", "state of the art" | Multi-query with orthogonal angles | 3 |
| **Deep dive** | "explain", "internals", "architecture", "how does X work under the hood" | Query + WebFetch the most authoritative URL | 2 + 1 fetch |
| **Artifact** | "find the repo", "official docs for", "API reference", "GitHub" | Direct `site:`-scoped search | 1 |

**Classification rules:**
1. If the question matches multiple types, choose the **most specific** one
2. If a stack trace or error message is present, always classify as **Troubleshooting**
3. If the question contains "vs" or "compare", always classify as **Comparison**
4. Default to **How-to** if ambiguous between How-to and Survey

### Confidence Gate

Before searching, self-assess: "Can I answer this reliably from training data?"

| Confidence | Topic Stability | Action |
|---|---|---|
| HIGH (>0.9) | Stable (algorithms, POSIX, math, RFCs) | Answer directly. Append: `[answered from training data — not web-verified]` |
| HIGH (>0.9) | Volatile (frameworks, APIs, pricing, security) | Search anyway — training data may be stale |
| MEDIUM (0.5-0.9) | Any | Search to verify. Use results to confirm or correct |
| LOW (<0.5) | Any | Mandatory search. Do not attempt answer without retrieval |

**Stable topics** (skip search when HIGH confidence): mathematical theorems, POSIX/ANSI standards, TCP/IP/HTTP protocol specs, language specifications (ECMAScript, Python PEP), algorithms and data structures, design patterns (GoF).

**Volatile topics** (always search even at HIGH confidence): framework versions and APIs, security advisories and CVEs, pricing and availability, cloud provider features, package versions and compatibility, anything with a year in the query.

---

## Stage 2: Query Transformation Pipeline

Apply these 6 transforms in order. Not all transforms apply to every intent type — see the applicability column.

### T1 Affirmative Rewrite

Convert the question into a statement that describes what the answer document would say. Search engines match documents, not questions — answer-bearing documents are written in affirmative form.

| Before (Question) | After (Affirmative) |
|---|---|
| "How do I configure CORS in Express?" | "Configure CORS middleware in Express.js" |
| "What causes ECONNREFUSED?" | "ECONNREFUSED connection refused root cause and fix" |
| "Why is my Docker build slow?" | "Docker build optimization slow layer caching" |
| "When was React 19 released?" | "React 19 release date announcement" |

**Applicability:** All intent types. Always apply T1 first.

### T2 Query2doc Expansion

Generate a 2-3 sentence pseudo-document describing what the ideal search result would contain. Extract the most specific technical terms from this pseudo-document and append them to the query.

**Process:**
1. Imagine the perfect answer document. What would its first paragraph say?
2. Write that paragraph mentally (do not output it)
3. Extract 3-5 specific technical terms that distinguish this topic from related ones
4. Append these terms to the T1 output

| Topic | Pseudo-Document Key Terms | Appended to Query |
|---|---|---|
| Express CORS config | `cors npm package`, `origin`, `credentials`, `preflight` | "Configure CORS middleware Express.js cors npm origin credentials preflight" |
| PostgreSQL ECONNREFUSED | `pg_hba.conf`, `listen_addresses`, `systemctl`, `5432` | "ECONNREFUSED PostgreSQL pg_hba.conf listen_addresses 5432" |
| React server components | `RSC`, `use client`, `streaming`, `server-only` | "React server components RSC use client streaming server-only" |

**Applicability:** All intent types except **Artifact** (artifact queries are already specific).

### T3 Decomposition

Split compound questions into independent sub-queries when the question spans multiple topics or requires multi-hop reasoning.

**When to decompose:**
- Question contains AND/OR joining distinct topics
- Answer requires information from 2+ unrelated domains
- Question has a comparative structure with 3+ options

**When NOT to decompose:**
- Question is a single atomic fact
- Sub-topics are tightly coupled (searching together gets better results)
- Budget is LOW (decomposition multiplies queries)

| Compound Question | Sub-Queries |
|---|---|
| "Compare Redis and Memcached for caching and explain their clustering" | Q1: "Redis vs Memcached caching performance benchmarks" Q2: "Redis cluster architecture sharding" Q3: "Memcached consistent hashing distributed" |
| "Set up ESLint with TypeScript and Prettier" | Q1: "ESLint TypeScript configuration @typescript-eslint" Q2: "ESLint Prettier integration eslint-config-prettier" |

**Applicability:** **Comparison**, **Survey**, complex **How-to**. Skip for **Factual**, **Artifact**, simple **Troubleshooting**.

### T4 Operator Injection

Add search operators based on intent type. These dramatically improve precision.

| Intent Type | Operators to Add | Example |
|---|---|---|
| **Troubleshooting** | Exact-quote the error string | `"ECONNREFUSED 127.0.0.1:5432"` |
| **How-to** | `after:2024` for framework topics | `Configure Tailwind v4 Vite after:2024` |
| **Artifact** | `site:` for known domain | `site:github.com tokio-rs/axum` |
| **Survey** | No `site:` (want breadth), add year | `LLM caching best practices 2026` |
| **Deep dive** | `site:` for authoritative source if known | `site:v8.dev garbage collection internals` |
| **Factual** | Exact-quote proper nouns | `"React 19" release date` |
| **Comparison** | Group with OR if single query | `(Redis OR Memcached) session store performance` |

**Applicability:** All intent types. Always apply T4.

### T5 Orthogonal Angles

For survey-type queries, generate 2-3 queries that approach the topic from completely different angles. This maximizes coverage and prevents echo-chamber results.

| Survey Topic | Angle 1 (Technical) | Angle 2 (Practical) | Angle 3 (Critical) |
|---|---|---|---|
| "Modern auth patterns" | "OAuth 2.1 PKCE implementation 2026" | "Passkey WebAuthn adoption production" | "Session vs token authentication tradeoffs security" |
| "LLM caching" | "Semantic cache embedding similarity LLM" | "Prompt caching KV cache implementation" | "LLM cache invalidation stale response problems" |
| "React state management" | "Zustand Jotai signal-based state 2026" | "TanStack Query server state vs client state" | "React state management over-engineering antipattern" |

**Applicability:** **Survey** only. Skip for all other intent types.

### T6 Domain Scoping

Map the query topic to high-signal domains. Use this to mentally prioritize results, and for `site:` operators when the intent type calls for it.

| Topic Category | Tier 1 (Prefer) | Tier 2 (Accept) | Tier 3 (Deprioritize) |
|---|---|---|---|
| Language / Framework | Official docs site | github.com, stackoverflow.com | Tutorial blogs, w3schools |
| DevOps / Infra | Cloud provider docs, official tool docs | github.com, engineering blogs | Medium listicles |
| Academic / Research | arxiv.org, scholar.google.com, acm.org | Official project pages | Blog summaries of papers |
| Security | NVD, official advisories, OWASP | Security researcher blogs | News aggregators |
| API Reference | Official API docs | SDK GitHub repos | Third-party wrappers |
| General Tech | news.ycombinator.com, official blogs | Reputable tech publications | SEO-farm content |

**Applicability:** All intent types. Informs result evaluation in Stage 4.

---

## Stage 3: Search Execution

### Execution Rules

| Rule | Detail |
|---|---|
| Results per query | Request top 3-5 results. Do not process 10+ results per query. |
| Parallel queries | Execute independent sub-queries in a single turn (multiple `WebSearch` calls) |
| Sequential queries | Execute dependent queries after evaluating prior results |
| WebFetch triggers | Use when: (a) snippet is promising but insufficient, (b) intent is Deep Dive, (c) official docs URL identified, (d) need to verify a specific claim |
| WebFetch limit | Max 3 WebFetch calls per search session |
| WebSearch limit | Max 5 WebSearch calls per search session |
| Fail-open | If WebSearch returns no results, broaden the query (remove operators, simplify terms) |
| Fail-closed for WebFetch | If WebFetch fails (403, timeout, paywall), fall back to snippet-only synthesis. Do not retry authenticated URLs. |

### WebFetch Extraction Prompts

Always provide a focused extraction prompt to WebFetch. Never fetch a raw page without guidance.

| Intent Type | Prompt Template |
|---|---|
| **How-to** | "Extract step-by-step instructions for {topic}. Include code examples, required dependencies, and common pitfalls. Ignore navigation, ads, and boilerplate." |
| **API Reference** | "Extract the API signature, parameters, return type, and usage example for {function/endpoint}. Include version-specific notes." |
| **Troubleshooting** | "Extract the root cause explanation and fix for this error: {error}. Include version-specific notes and any prerequisite configuration." |
| **Comparison** | "Extract the key differences, tradeoffs, and recommendation for {A} vs {B} in context of {use case}." |
| **Deep dive** | "Extract the architectural explanation of {topic}. Include diagrams descriptions, key components, data flow, and performance characteristics." |
| **Survey** | "Extract the main recommendations, patterns, and anti-patterns for {topic}. Include any benchmarks or metrics cited." |

---

## Stage 4: Result Evaluation

### Reflection Scoring

After retrieving results, score each result on 4 dimensions (inspired by Self-RAG reflection tokens). This is a mental evaluation — do not output scores to the user.

| Dimension | Tag | Score Range | Criteria |
|---|---|---|---|
| **Relevance** | [IsRel] | 0.0 - 1.0 | Does this result directly answer the query? Not tangentially related — directly. |
| **Supported** | [IsSup] | 0.0 - 1.0 | Are the claims in this result backed by evidence, code, or citations? Or is it opinion/speculation? |
| **Useful** | [IsUse] | 0.0 - 1.0 | Does this provide actionable information beyond what we already know from other results or training data? |
| **Current** | [IsCur] | 0.0 - 1.0 | Is the information current? For volatile topics: within 2 years = 1.0, 2-4 years = 0.5, older = 0.2. For stable topics: always 1.0. |

**Composite = mean(IsRel, IsSup, IsUse, IsCur)**

| Composite | Action |
|---|---|
| >= 0.7 | **HIGH quality** — use as primary source, cite prominently |
| 0.4 - 0.69 | **MEDIUM quality** — use as supporting source, cross-reference |
| < 0.4 | **LOW quality** — discard. Do not cite. Do not use in synthesis. |

### Conflict Resolution

| Scenario | Resolution |
|---|---|
| Two results contradict each other | Prefer: official docs > primary source (paper, RFC) > blog > forum. Flag the conflict in output. |
| All results are LOW quality (< 0.4) | Trigger Stage 5 refinement loop |
| Result is pre-2024 on a volatile topic | Downweight [IsCur] by 0.3. Note staleness if used. |
| Result is from a known unreliable domain | Set [IsSup] = 0.2 regardless of content quality |
| Multiple results agree | Boost confidence — corroboration increases [IsSup] for all agreeing results by 0.1 |

---

## Stage 5: Iterative Refinement

### Refinement Decision Table

After Stage 4 evaluation, decide whether to refine or proceed to synthesis.

| Condition | Action | Max Additional Iterations |
|---|---|---|
| All results composite < 0.4 | Reformulate: broaden terms, remove operators, try completely different angle | 2 |
| Partial answer found, specific gaps remain | Generate follow-up query targeting the gap: "I know X, but need Y" → search for Y | 2 |
| Conflicting results, no authoritative source found | Add `site:` for official docs domain. Or WebFetch the most credible URL. | 1 |
| >= 2 results with composite > 0.6 | **Exit loop** → proceed to Stage 6 synthesis | 0 |
| Budget exhausted (see Budget Management) | **Force exit** → synthesize with best available. Note LOW confidence. | 0 |

### FLARE Mid-Generation Retrieval

When generating a response and encountering a claim you are uncertain about:

1. Write the tentative sentence containing the uncertain claim
2. Identify the specific uncertain span (e.g., "the default timeout is 30 seconds")
3. Use that span as a search query: `"default timeout 30 seconds" {technology}`
4. If the search confirms: keep the sentence
5. If the search contradicts: replace with the retrieved fact and cite it
6. If the search finds nothing: mark with `[unverified]` or omit

**Apply FLARE only when:** confidence on a specific claim is LOW and the claim is material to the answer. Do not FLARE on every sentence — that wastes budget.

### Iteration Protocol

```
iteration = 0
sufficient = false

WHILE iteration < 3 AND NOT sufficient:
    IF iteration == 0:
        Execute transformed queries from Stage 2
    ELIF iteration == 1:
        Reformulate based on what was missing or wrong
        Try: remove operators, broaden terms, add context
    ELIF iteration == 2:
        Try completely different angle or domain
        Try: different vocabulary, adjacent topic, official source

    Execute Stage 3 (Search)
    Execute Stage 4 (Evaluate)

    IF count(results with composite > 0.6) >= 2:
        sufficient = true
    ELIF budget_exhausted:
        sufficient = true  # forced exit

    iteration += 1

IF NOT sufficient:
    Synthesize with disclaimer: "Limited results found. Confidence: LOW."
```

---

## Stage 6: Synthesis and Grounding

### Grounding Rules

These rules are non-negotiable. Every search response must follow them.

| Rule | Detail |
|---|---|
| **Cite every factual claim** | Use inline citations: "Express.js supports CORS via middleware [1]." Number sequentially. |
| **No hallucinated facts** | If a fact was not retrieved AND you are not confident from training data, do not state it. Say "no authoritative source found" or omit. |
| **Mark training-data claims** | If you supplement retrieved results with training-data knowledge, mark it: "[from training data — not web-verified]" |
| **Source list at end** | Numbered list of all cited URLs with title and domain. |
| **Code snippets attributed** | If code comes from a specific source, cite it. If synthesized from multiple sources, note "adapted from [1], [2]". |
| **Confidence indicator** | HIGH: 3+ corroborating sources. MEDIUM: 1-2 sources. LOW: no direct source or conflicting results. |
| **Conflict transparency** | If sources disagree, say so explicitly. Do not silently pick one side. |
| **Recency note** | If the most recent source is > 1 year old on a volatile topic, note: "Most recent source is from {date}. Current behavior may differ." |

### Output Format

```markdown
## Answer

[Grounded response with inline citations [1][2]...]

[If applicable: code examples with attribution]

## Sources
1. [Title](URL) — domain.com, YYYY-MM-DD (if available)
2. [Title](URL) — domain.com
...

## Search Metadata
- **Intent:** {classified intent type}
- **Queries executed:** {count}
- **Iterations:** {count}
- **Confidence:** HIGH | MEDIUM | LOW
- **Cache:** HIT | MISS
```

**When called as a building block** (from another skill): Return the same structure but skip the `## Search Metadata` section unless the caller requests it.

---

## Cache Protocol

### Cache Mechanics

| Aspect | Rule |
|---|---|
| Cache location | `.smart-search-cache/` in project root (gitignored) |
| Cache key | SHA-256 of normalized query: lowercase, remove stop words ("the", "a", "is", "how", "do", "I"), sort remaining terms alphabetically |
| Cache format | `{first-8-chars-of-hash}.md` containing: query, timestamp, TTL, results, scores, synthesized answer |
| Cache hit | Read cached file → check TTL → if valid, return cached answer. Skip Stages 2-5. |
| Cache miss | Execute full pipeline → write cache on completion |
| Max cache size | 50 files. When exceeded, delete the oldest file (by timestamp in the file). |

### Topic Volatility

| Volatile (60 min TTL) | Stable (7 day TTL) |
|---|---|
| Framework versions, APIs under active development | Algorithms, data structures, design patterns |
| Security advisories, CVEs | Programming language specifications |
| Pricing, availability, service status | Mathematical concepts |
| "Latest", "2025", "2026", "new", "updated" in query | Protocols (TCP, HTTP/2, gRPC spec) |
| Package compatibility and changelogs | POSIX, SQL, RFC standards |
| Cloud provider feature availability | Foundational CS concepts |

**Classification rule:** If the query contains a year, "latest", "new", "updated", "current", or references a specific version — treat as volatile regardless of topic.

### Evolve-Loop Integration

When this protocol is invoked from `online-researcher.md` or Phase 0.5:

1. Execute the full 6-stage pipeline as normal
2. In addition to `.smart-search-cache/`, also write a Knowledge Capsule to `.evolve/research/<topic-slug>.md` using the standard capsule format:
   ```markdown
   # Research: <Topic>
   **Date:** <ISO-8601>
   **Sources:** <URLs>

   ## Key Constraints
   - <Must-dos and anti-patterns>

   ## Code Patterns
   - <Executable, concise snippets>
   ```
3. Register the query in `state.json.research.queries` per the memory-protocol OCC rules
4. Score the query using the research quality dimensions (Novelty/Relevance/Yield) as defined in `online-researcher.md`

---

## Budget Management

### Budget Tiers

| Level | WebSearch Calls | WebFetch Calls | Max Iterations | When to Use |
|---|---|---|---|---|
| **FULL** | 5 | 3 | 3 | Default for standalone invocation |
| **MEDIUM** | 3 | 1 | 2 | When called as a building block with moderate token budget |
| **LOW** | 2 | 0 | 1 | When token budget is tight. Single query, no WebFetch. |
| **EXHAUSTED** | 0 | 0 | 0 | Answer from cache or training data only |

### Budget Tracking

Track these counters throughout the search session:

| Counter | Incremented When | Budget Check |
|---|---|---|
| `searchCalls` | Each `WebSearch` invocation | If `searchCalls >= tier.maxSearch`: stop searching |
| `fetchCalls` | Each `WebFetch` invocation | If `fetchCalls >= tier.maxFetch`: no more fetches |
| `iterations` | Each refinement loop pass | If `iterations >= tier.maxIter`: force synthesis |

**Evolve-loop budget mapping:** When invoked from the research phase, map `tokenBudget.researchPhase` remaining:
- \> 15K tokens remaining → FULL
- 10K-15K → MEDIUM
- 5K-10K → LOW
- < 5K → EXHAUSTED

**Beast mode:** If budget is EXHAUSTED but the question is critical (user explicitly asked, or the search is blocking implementation), note the constraint and provide the best answer possible from training data with appropriate caveats.

---

## Operator Syntax Reference

### Operator Table

| Operator | Syntax | Purpose | Example |
|---|---|---|---|
| Exact phrase | `"phrase here"` | Match exact string — ideal for error messages, API names | `"ECONNREFUSED 127.0.0.1"` |
| Site filter | `site:domain.com` | Restrict to specific domain — ideal for official docs | `site:docs.python.org asyncio` |
| Title filter | `intitle:keyword` | Match keyword in page title | `intitle:"migration guide" Next.js 15` |
| File type | `filetype:ext` | Find specific file types | `filetype:yaml kubernetes deployment` |
| Recency | `after:YYYY` | Restrict to recent results | `React server components after:2025` |
| Exclusion | `-term` | Exclude irrelevant results | `Python async -tutorial -beginner` |
| OR grouping | `(A OR B)` | Search for either term | `(Redis OR Valkey) session store` |
| Wildcard | `*` | Match any word in a phrase | `"how to * CORS in Express"` |

### Auto-Applied Operators

These operators are automatically injected based on intent type in T4:

| Intent | Auto Operators | Rationale |
|---|---|---|
| **Troubleshooting** | `"exact error string"` | Error messages are unique identifiers — exact match finds fixes |
| **How-to** (volatile topic) | `after:2024` | Framework tutorials go stale quickly |
| **Artifact** | `site:{known domain}` | Direct navigation to known sources |
| **Survey** | Year in query (e.g., `2026`) but NO `site:` | Surveys need breadth — site restriction kills diversity |
| **Deep dive** | `site:{authoritative domain}` if known | Deep dives need depth from one good source |
| **Factual** | `"proper nouns"` in quotes | Exact match on names, versions, dates |
| **Comparison** | `(A OR B)` grouping or parallel queries | Find head-to-head comparisons in single results |

---

## Integration API

### Input Contract

When another skill invokes this protocol, it provides:

| Field | Required | Type | Default | Description |
|---|---|---|---|---|
| `question` | Yes | string | — | The raw user question or knowledge gap |
| `context` | No | string | null | Additional context: project type, framework, constraints |
| `budget` | No | enum | FULL | FULL / MEDIUM / LOW / EXHAUSTED |
| `cache_ns` | No | string | "default" | Cache namespace prefix — allows caller-specific cache isolation |
| `skip_cache` | No | bool | false | Force fresh search even if cache hit exists |

### Output Contract

The protocol returns:

| Field | Always Present | Type | Description |
|---|---|---|---|
| `answer` | Yes | string | Grounded response with inline citations |
| `sources` | Yes | list | URLs with titles, used in citations |
| `confidence` | Yes | enum | HIGH / MEDIUM / LOW |
| `intent` | Yes | string | Classified intent type |
| `queries_used` | Yes | list | Actual search queries executed |
| `cache_hit` | Yes | bool | Whether the answer came from cache |

### Calling From Other Skills

To use smart-web-search from another skill:

```
When you need to search the web, follow the Smart Web Search protocol:
1. Read smart-web-search.md
2. Provide: question, context (optional), budget (optional)
3. Execute the 6-stage pipeline
4. Use the returned answer, sources, and confidence in your workflow
```

For quick integration, a skill can simply add:

> For web search, apply the Smart Web Search protocol in `smart-web-search.md`.
> Provide the question and any relevant context. Use the returned grounded answer.

---

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

---

## Worked Examples

### Example 1: Troubleshooting

**User question:** "I'm getting `ECONNREFUSED 127.0.0.1:5432` when connecting to PostgreSQL"

| Stage | Action |
|---|---|
| **1. Classify** | Intent: **Troubleshooting** (error string present). Confidence: MEDIUM (common error, but causes vary). |
| **2. Transform** | T1: "ECONNREFUSED 127.0.0.1 5432 PostgreSQL connection refused fix" T4: Add exact quote: `"ECONNREFUSED 127.0.0.1:5432"` Query 1: `"ECONNREFUSED 127.0.0.1:5432" PostgreSQL` Query 2: `PostgreSQL connection refused 5432 pg_hba.conf listen_addresses fix` |
| **3. Execute** | WebSearch both queries in parallel. Top results: StackOverflow answer, PostgreSQL wiki, DigitalOcean tutorial. |
| **4. Evaluate** | SO answer [IsRel=0.9, IsSup=0.8, IsUse=0.8, IsCur=0.7] = 0.8. PG wiki [0.7, 0.9, 0.6, 1.0] = 0.8. Both HIGH quality. |
| **5. Refine** | 2 results with composite > 0.6 → skip refinement. |
| **6. Synthesize** | Grounded answer listing 4 common causes (service not running, wrong port, pg_hba.conf, listen_addresses) with citations [1][2]. Confidence: HIGH. |

### Example 2: Survey

**User question:** "What are the best practices for LLM caching in 2026?"

| Stage | Action |
|---|---|
| **1. Classify** | Intent: **Survey** ("best practices", "2026"). Confidence: LOW (rapidly evolving field). |
| **2. Transform** | T1: "LLM caching best practices 2026" T5 orthogonal angles: Q1: "Semantic cache embedding similarity LLM API 2026" Q2: "Prompt caching KV cache Claude OpenAI implementation" Q3: "LLM cache invalidation stale response detection" |
| **3. Execute** | WebSearch all 3 queries in parallel. 9 results total. WebFetch the most authoritative (Redis blog on semantic caching). |
| **4. Evaluate** | Redis blog [0.9, 0.8, 0.9, 1.0] = 0.9. Anthropic prompt caching docs [0.8, 0.9, 0.7, 1.0] = 0.85. 3 other results > 0.6. |
| **5. Refine** | 5 results with composite > 0.6 → skip refinement. |
| **6. Synthesize** | Comparison table: semantic caching vs prompt caching vs KV-cache. Recommendations per use case. 5 citations. Confidence: HIGH. |

### Example 3: How-To

**User question:** "How to configure Tailwind CSS v4 with Vite?"

| Stage | Action |
|---|---|
| **1. Classify** | Intent: **How-to** ("how to", "configure"). Confidence: MEDIUM (v4 is new, training data may be stale). |
| **2. Transform** | T1: "Configure Tailwind CSS v4 Vite project setup" T2: Extract terms: `@tailwindcss/vite`, `postcss`, `tailwind.config` T4: Add `after:2025` (volatile framework) T6: Prefer `site:tailwindcss.com` Query 1: `site:tailwindcss.com Tailwind CSS v4 Vite installation after:2025` Query 2: `Tailwind CSS v4 Vite @tailwindcss/vite postcss configuration 2025 2026` |
| **3. Execute** | WebSearch Query 1. Top result: official Tailwind docs. WebFetch the official docs URL with prompt: "Extract step-by-step Vite installation for Tailwind v4. Include code examples and required dependencies." |
| **4. Evaluate** | Official docs [1.0, 1.0, 0.9, 1.0] = 0.975. |
| **5. Refine** | 1 result but from official docs with composite 0.975 → sufficient (authoritative single source). |
| **6. Synthesize** | Step-by-step guide from official docs. Code snippets attributed to [1]. Confidence: HIGH. |
