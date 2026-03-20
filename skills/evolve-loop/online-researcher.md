# Accurate Online Researcher Protocol (2026 Standard)

This protocol defines how evolve-loop agents (Scout, Builder) should conduct online research to maximize accuracy, efficiency, and context-window utilization. The internet is the infinite source of knowledge, but reading raw web pages repeatedly is slow, prone to hallucination, and token-expensive. 

## The Core Concept: Knowledge Capsules
Instead of reading the web directly in the middle of a build task, agents must perform research, distill the required knowledge into a dense **Knowledge Capsule**, and save it locally. The LLM simply retrieves the needed knowledge from the internet, stores the critical parts locally, and performs its tasks from the local cache. Future cycles read the capsule instead of searching the internet.

## The Research Workflow (Plan-Route-Act-Verify)

When an agent encounters a knowledge gap (e.g., "How does the new Stripe v2 API work?" or "What are the latest 2026 Next.js routing patterns?"), follow this execution loop:

### 1. Plan (Query Transformation)
- Do not search for the raw question.
- Formulate 2-3 specific, orthogonal search queries. Use **Hypothetical Document Embeddings (HyDE)** strategy: think about what the *answer* document would look like and search for those terms.

### 2. Route & Act (Targeted Retrieval)
- Use your web search tool to execute the queries.
- Fetch the top 2-3 most relevant URLs. Do not read 10+ pages; prioritize high-signal domains (official docs, GitHub issues, authoritative blogs).

### 3. Verify & Distill
- Extract ONLY the facts, code snippets, and architectural constraints relevant to the current project context.
- Discard marketing fluff, outdated tutorials, and irrelevant tangents.
- If the retrieved information conflicts, verify against a secondary source or explicit official documentation.

### 4. Cache (Local Storage)
- Write the distilled findings to a local markdown file: `.evolve/research/<topic-slug>.md`.
- Format the capsule exactly like this:
  ```markdown
  # Research: <Topic>
  **Date:** <ISO-8601>
  **Sources:** <URLs>
  
  ## Key Constraints
  - <Must-dos and anti-patterns>
  
  ## Code Patterns
  - <Executable, concise snippets>
  ```
- Once saved, proceed with the original task using the local capsule.

## Deduplication and Cache Invalidation
- Before performing a web search, always check if `.evolve/research/<topic-slug>.md` already exists.
- If the capsule is older than 30 days and the topic is volatile (e.g., latest frontend framework), invalidate the cache, re-research, and overwrite the file.
- For stable topics (e.g., POSIX shell standards), capsules never expire.

## Cross-Agent Integration
- **Scout (Phase 1):** Uses this protocol to scope complex tasks. If a task requires external knowledge, Scout performs the research and creates the capsule so the Builder doesn't have to spend its token budget on web searching.
- **Builder (Phase 2):** Uses this protocol if an unforeseen knowledge gap arises during implementation (e.g., an obscure API error) and caches the solution for future tasks.
