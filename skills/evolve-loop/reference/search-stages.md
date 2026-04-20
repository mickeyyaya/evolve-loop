---
name: reference
description: Reference doc.
---

# Smart Web Search — Detailed Stages

> Read this file when executing the 6-stage search pipeline. Contains all stage protocols, decision tables, and transformation rules.

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
