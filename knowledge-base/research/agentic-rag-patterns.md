# Agentic RAG Patterns

> Reference document for retrieval-augmented generation patterns in agent systems.
> Apply hierarchical retrieval strategies to minimize token cost and maximize
> information relevance across evolve-loop Scout, Builder, and Auditor phases.

## Table of Contents

1. [RAG Technique Taxonomy](#rag-technique-taxonomy)
2. [Hierarchical Retrieval Interface](#hierarchical-retrieval-interface)
3. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
4. [Implementation Patterns](#implementation-patterns)
5. [Prior Art](#prior-art)
6. [Anti-Patterns](#anti-patterns)

---

## RAG Technique Taxonomy

| Technique | Mechanism | Latency | Token Cost | Strengths | Weaknesses |
|---|---|---|---|---|---|
| **Keyword Search** | BM25 / TF-IDF over document index | Low | Minimal | Fast, deterministic, no embedding infra | Misses synonyms, no semantic understanding |
| **Semantic Search** | Dense vector embedding + ANN lookup | Medium | Low-medium | Captures meaning, handles paraphrases | Requires embedding model, index maintenance |
| **Chunk-Read** | Retrieve fixed-size text chunks around matches | Medium | Medium | Simple implementation, good for code | Arbitrary chunk boundaries split context |
| **GraphRAG** | Entity-relationship graph traversal + community summaries | High | High | Captures cross-document relationships | Expensive graph construction, complex pipeline |
| **RAPTOR** | Recursive abstractive tree of summaries over corpus | High | High | Multi-level abstraction, global reasoning | Slow index build, summary drift risk |
| **LightRAG** | Lightweight graph-based retrieval with dual-level keys | Medium | Medium | Simpler than GraphRAG, fast updates | Less expressive than full graph traversal |
| **A-RAG (Agentic RAG)** | Agent decides what to retrieve, when, and how deeply | Variable | Variable | Adaptive, budget-aware, multi-hop reasoning | Requires orchestration, risk of over-retrieval |

---

## Hierarchical Retrieval Interface

Use a three-level retrieval hierarchy. Route queries to the cheapest level that satisfies accuracy requirements.

| Level | Name | Mechanism | Latency | Token Budget | Use When |
|---|---|---|---|---|---|
| **L1** | Keyword | Grep, glob, BM25 over file names and summaries | <1s | <500 tokens | Task exists in known location; need file path or summary |
| **L2** | Semantic | Embedding search or targeted file read | 1-5s | 500-5000 tokens | Need specific content from partially known location |
| **L3** | Deep-Read | Full file parse, multi-file scan, codebase traversal | 5-30s | 5000-50000 tokens | Need comprehensive understanding across multiple files |

### Escalation Rules

| Condition | Action |
|---|---|
| L1 returns zero results | Escalate to L2 |
| L1 returns ambiguous results (>10 candidates) | Escalate to L2 with narrower query |
| L2 returns insufficient context for decision | Escalate to L3 |
| Token budget exhausted | Stop retrieval, proceed with available context |
| Confidence threshold met | Stop retrieval at current level |

---

## Mapping to Evolve-Loop

### Scout Phase

| Retrieval Step | Level | Source | Purpose |
|---|---|---|---|
| Read project-digest | L1 | `workspace/project-digest.md` | Identify codebase structure and recent changes |
| Search for task candidates | L1 | Grep across `src/`, `docs/`, `tests/` | Find files related to potential improvements |
| Read specific files | L2 | Targeted file reads from L1 hits | Understand code context for task scoping |
| Deep-scan codebase | L3 | Multi-file traversal across modules | Discover cross-cutting concerns and dependencies |

### Builder Phase

| Retrieval Step | Level | Source | Purpose |
|---|---|---|---|
| Read instinctSummary | L1 | `workspace/instinct-summary.md` | Load compressed behavioral guidance |
| Read full instinct files | L2 | `docs/reference/instincts.md`, individual instinct files | Get detailed implementation guidance |
| Read gene definitions | L2 | `docs/reference/genes.md`, relevant gene files | Understand constraints and quality criteria |
| Scan implementation context | L3 | Source files referenced in scout-report | Deep understanding for code changes |

### Auditor Phase

| Retrieval Step | Level | Source | Purpose |
|---|---|---|---|
| Read eval definitions | L1 | `workspace/eval-graders.md` | Load pass/fail criteria for current cycle |
| Read audit rubric | L2 | `docs/eval-grader-best-practices.md` | Understand scoring methodology |
| Verify implementation | L3 | Changed files from build-report | Validate code correctness against requirements |

---

## Implementation Patterns

### When to Use Each Level

| Scenario | Recommended Level | Rationale |
|---|---|---|
| Check if a file exists | L1 | Glob or `test -f` suffices |
| Find function definition | L1 | Grep for function name |
| Understand function behavior | L2 | Read the file containing the function |
| Understand cross-module interaction | L3 | Trace calls across multiple files |
| Validate architectural compliance | L3 | Scan multiple modules for pattern adherence |
| Load cached summary | L1 | Read pre-computed digest |

### Budget-Aware Retrieval

| Strategy | Implementation | Token Savings |
|---|---|---|
| **Progressive disclosure** | Start at L1, escalate only on insufficient results | 40-70% vs always using L3 |
| **Cached summaries** | Pre-compute digests; read summary before full file | 50-80% on repeated access |
| **Selective inclusion** | Include only relevant sections from retrieved files | 30-60% vs full file inclusion |
| **Retrieval budget cap** | Set max tokens per retrieval phase; stop when exhausted | Prevents runaway token usage |
| **Result deduplication** | Deduplicate overlapping chunks before injection | 10-30% on multi-query retrieval |

### Caching Retrieved Context

| Cache Type | Scope | TTL | Invalidation |
|---|---|---|---|
| **Project digest** | Entire codebase summary | Per cycle | Regenerate at cycle start |
| **File content cache** | Individual file reads | Per phase | Invalidate on file modification |
| **Instinct summary** | Compressed behavioral rules | Cross-cycle | Invalidate on instinct mutation |
| **Embedding index** | Vector store of codebase | Daily | Rebuild on significant code changes |

---

## Prior Art

| Reference | Year | Key Contribution | Relevance to Evolve-Loop |
|---|---|---|---|
| Lewis et al., "Retrieval-Augmented Generation for Knowledge-Intensive NLP Tasks" | 2020 | Introduced RAG paradigm: retrieve-then-generate | Foundation for all retrieval-augmented agent patterns |
| arXiv:2501.09136 — Agentic RAG Survey | 2025 | Taxonomy of agentic retrieval: single-agent, multi-agent, graph-based | Directly informs A-RAG technique selection |
| arXiv:2602.03442 — Advanced RAG Techniques | 2026 | Benchmarks on hierarchical retrieval and budget-aware strategies | Validates L1/L2/L3 escalation approach |
| GraphRAG (Microsoft) | 2024 | Entity-relationship graphs + community summaries for global queries | Applicable to cross-module dependency analysis |
| RAPTOR (Stanford) | 2024 | Recursive tree of summaries enabling multi-level abstraction | Informs project-digest hierarchical compression |
| LightRAG | 2024 | Dual-level retrieval keys on lightweight graph structure | Middle ground between keyword search and full GraphRAG |
| Anthropic prompt caching | 2024 | Cache static prompt prefixes for KV-cache reuse | Enables cost reduction on repeated instinct/gene loading |

---

## Anti-Patterns

| Anti-Pattern | Description | Consequence | Mitigation |
|---|---|---|---|
| **Over-retrieval** | Retrieve everything "just in case" without relevance filtering | Token budget exhaustion, context dilution, slower responses | Set retrieval budget caps; use progressive L1-L2-L3 escalation |
| **Stale index** | Use cached embeddings or summaries after codebase has changed | Retrieve outdated or incorrect context; hallucinate based on stale data | Invalidate caches at cycle boundaries; timestamp all cached artifacts |
| **Retrieval without validation** | Inject retrieved content without verifying relevance or accuracy | Pollute agent context with irrelevant or contradictory information | Score retrieved chunks for relevance; discard below threshold |
| **Ignoring retrieval costs** | Treat all retrieval as free; always use L3 deep-read | Unnecessary token spend, slower cycle times, budget overrun | Track token cost per retrieval; prefer cheapest sufficient level |
| **Retrieval-generation mismatch** | Retrieve at wrong granularity for the generation task | Too coarse loses detail; too fine loses global context | Match retrieval level to task complexity |
| **Single-hop limitation** | Retrieve once and generate; never follow up with clarifying retrieval | Miss information requiring multi-step reasoning | Use A-RAG pattern: retrieve, reason, retrieve again if needed |
| **Embedding monoculture** | Use only semantic search; ignore keyword and structural retrieval | Miss exact-match queries where keyword search excels | Combine keyword (L1) and semantic (L2) retrieval in hybrid pipeline |
