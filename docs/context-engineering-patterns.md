# Context Engineering Patterns

> Reference document for the five-strategy context engineering framework:
> selection, compression, ordering, isolation, and format optimization.
> Apply these strategies to minimize token usage and maximize agent effectiveness
> across evolve-loop cycles.

## Table of Contents

1. [The Five Strategies](#the-five-strategies)
2. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
3. [Implementation Patterns](#implementation-patterns)
4. [Prior Art](#prior-art)
5. [Anti-Patterns](#anti-patterns)

---

## The Five Strategies

| Strategy | Definition | Key Principle | When to Apply |
|---|---|---|---|
| **Selection** | Choose which context to include based on relevance to the current task | Include only what the agent needs; exclude everything else | Before constructing any prompt |
| **Compression** | Reduce token count while preserving semantic information | Summarize, deduplicate, and strip boilerplate | When context exceeds budget or between phase handoffs |
| **Ordering** | Place most important context first; arrange static before dynamic | Static fields first for KV-cache reuse; critical info at top | When assembling multi-section prompts |
| **Isolation** | Separate concerns so each agent/phase receives only its own context | One agent, one responsibility, one context scope | When designing multi-agent pipelines |
| **Format Optimization** | Use tables, structured data, and concise formats over prose | Tables compress 2-3x vs prose at equivalent information density | When encoding reference data, reports, or checklists |

---

## Mapping to Evolve-Loop

| Strategy | Scout Action | Builder Action | Auditor Action |
|---|---|---|---|
| **Selection** | Load per-phase context matrix from `phases.md`; include only scout-relevant genes and instincts | Receive scout-report plus build-relevant genes; exclude audit criteria | Receive build-report plus audit rubric; exclude scout search context |
| **Compression** | Produce a compressed scout-report summary; use AgentDiet trajectory compression | Consume compressed scout-report; emit concise build-report | Consume compressed build-report; emit structured audit verdict |
| **Ordering** | Place static system prompt and instincts before dynamic codebase search results | Place static gene definitions before dynamic code diffs | Place static rubric before dynamic test output |
| **Isolation** | Own `scout-report.md` exclusively; never read build or audit artifacts | Own `build-report.md` exclusively; read scout-report read-only | Own `audit-report.md` exclusively; read build-report read-only |
| **Format** | Emit tables in scout-report for task candidates and risk assessments | Emit structured JSON in ledger entries; use code blocks for diffs | Emit structured pass/fail tables in audit-report |

---

## Implementation Patterns

### Context Budget Estimation

| Metric | Target | Calculation |
|---|---|---|
| System prompt (static) | <4,000 tokens | Measure once; cache across cycles |
| Phase instructions | <2,000 tokens | Per-phase slice from `phases.md` |
| Workspace artifacts | <6,000 tokens | scout-report + build-report combined |
| Dynamic codebase context | <8,000 tokens | File reads + search results |
| Total per-agent budget | <20,000 tokens | Sum of above; trigger lean mode if exceeded |

### Lean Mode Activation

| Trigger | Action |
|---|---|
| Context exceeds 80% of budget | Strip examples and verbose descriptions |
| Context exceeds 90% of budget | Compress all artifacts to summary-only form |
| Context exceeds 95% of budget | Drop lowest-priority context sections entirely |

### Handoff Files Between Phases

| Handoff | File | Owner | Format | Max Size |
|---|---|---|---|---|
| Scout to Builder | `scout-report.md` | Scout | Markdown tables | 3,000 tokens |
| Builder to Auditor | `build-report.md` | Builder | Markdown + code blocks | 4,000 tokens |
| Auditor to Orchestrator | `audit-report.md` | Auditor | Structured verdict table | 2,000 tokens |

### Compressed State Summaries

| Summary Type | Purpose | Content |
|---|---|---|
| `instinctSummary` | Carry forward learned instincts without full history | Top instincts ranked by activation count |
| `ledgerSummary` | Provide cycle history without loading full ledger | Last 5 cycle outcomes, cumulative metrics |
| `geneSummary` | Pass active gene configuration compactly | Gene names + current values, no descriptions |

---

## Prior Art

| Source | Contribution | Key Insight |
|---|---|---|
| Anthropic Context Engineering Guide | Defined context engineering as distinct from prompt engineering | Context is the entire information environment, not just the prompt |
| CEMM (arXiv:2603.09619) | Context Engineering for Memory Management in agents | Structured memory retrieval outperforms naive RAG by 40%+ |
| AgentDiet (arXiv:2509.23586) | Trajectory compression for multi-step agents | Compress intermediate reasoning traces by 60% with <5% quality loss |
| OpenAI Prompt Caching | Static prefix caching for cost reduction | Place stable content first; dynamic content last; save 50% on cached prefixes |
| Karpathy Autoresearch | Autonomous research agent patterns | Iterative context refinement across research cycles |

---

## Anti-Patterns

| Anti-Pattern | Description | Consequence | Fix |
|---|---|---|---|
| Context Overload | Include entire codebase or all project docs in every prompt | Token budget blown; critical info lost in noise | Apply selection strategy; use per-phase context matrix |
| Stale Context Pollution | Carry forward outdated artifacts from previous cycles | Agent acts on obsolete information; regressions | Expire workspace files each cycle; regenerate from source |
| Premature Compression | Compress context before the agent has processed it | Loss of critical details needed for decision-making | Compress only at phase boundaries, never within a phase |
| Ignoring KV-Cache | Place dynamic content before static content in prompts | Cache miss on every request; increased latency and cost | Reorder: static system prompt first, dynamic context last |
| Monolithic Context | Send identical context to all agents regardless of role | Wasted tokens; agent confusion from irrelevant info | Apply isolation strategy; scope context per agent role |
| Format Bloat | Use verbose prose where tables or structured data suffice | 2-3x token waste; harder for agents to parse | Convert reference data to tables; use JSON for structured output |
