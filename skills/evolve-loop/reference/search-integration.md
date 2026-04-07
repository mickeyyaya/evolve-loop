# Smart Web Search — Cache, Budget, Operators, Integration

> Read this file when caching results, managing budget tiers, looking up operator syntax, integrating from another skill, or studying worked examples.

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
