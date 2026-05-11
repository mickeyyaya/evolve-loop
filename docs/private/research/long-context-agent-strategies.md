> **Long-Context Agent Strategies** — Reference doc on utilizing 1M+ context windows
> effectively for agents. Covers context degradation, mitigation strategies,
> budget frameworks, and mapping to evolve-loop phases.

## Table of Contents

1. [Context Degradation Problem](#context-degradation-problem)
2. [Mitigation Strategies](#mitigation-strategies)
3. [Context Budget Framework](#context-budget-framework)
4. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
5. [Prior Art](#prior-art)
6. [Anti-Patterns](#anti-patterns)

---

## Context Degradation Problem

Models advertise 1M-2M token windows but effective performance degrades well before those limits. Understand these failure modes before designing long-context agent pipelines.

| Failure Mode | Description | Onset Point | Observable Symptoms |
|---|---|---|---|
| **Context rot** | Accumulated stale, contradictory, or redundant information pollutes the context over many turns | 30-50% of window capacity | Agent contradicts earlier instructions; outputs drift from task |
| **Lost-in-the-middle** | Information placed in the middle of long contexts receives less attention than content at the beginning or end | Any context length >4K tokens | Agent ignores mid-context instructions; follows only preamble and recent content |
| **Attention dilution** | As context grows, per-token attention weight decreases, reducing recall precision on any single fact | 60-80% of window capacity | Agent gives vague summaries instead of precise answers; misses specific details |
| **Recency bias** | Model over-weights recently added tokens relative to earlier, equally important content | Proportional to context length | Agent prioritizes latest information even when earlier context is more relevant |
| **Instruction amnesia** | System instructions and role definitions lose influence as conversation context grows | 40-60% of window capacity | Agent stops following formatting rules, constraints, or role boundaries |
| **Hallucination amplification** | Longer contexts increase probability of hallucinated facts, especially when context contains near-duplicate information | 50-70% of window capacity | Agent fabricates details that plausibly fit the context but are not present in it |

---

## Mitigation Strategies

| Strategy | Mechanism | Token Savings | Quality Impact | Implementation Complexity |
|---|---|---|---|---|
| **Hierarchical summarization** | Progressively compress older context into summaries at multiple granularity levels (turn-level, phase-level, session-level) | 60-80% on older context | Preserves key decisions and outcomes; loses verbatim details | Medium — requires summary generation at phase boundaries |
| **Prompt compression (AgentDiet)** | Apply trained compression model to reduce prompt tokens while preserving semantic content; remove filler words, redundant phrasing, and boilerplate | 30-50% across all prompts | Minimal quality loss on factual tasks; slight degradation on style-sensitive tasks | Medium — integrate compression model as preprocessing step |
| **Chain-of-Agents segmented processing** | Partition long context into segments; assign each segment to a worker agent; synthesize results in a manager agent | Scales linearly — each worker sees only its segment | Maintains per-segment precision; synthesis quality depends on manager prompt | High — requires multi-agent orchestration and segment boundary design |
| **Retrieval-augmented context** | Replace bulk context with targeted retrieval from vector store or index; inject only relevant chunks per query | 70-90% reduction vs full-context approach | High precision on retrieved chunks; misses cross-document relationships | Medium — requires embedding pipeline and index maintenance |
| **Context window management** | Track token budget explicitly; evict lowest-relevance content when approaching capacity; use sliding window with anchored prefix | 40-60% via eviction | Preserves most-relevant content; risks losing needed-but-infrequent details | Low — implement token counter and eviction policy |
| **Static prefix caching** | Place invariant instructions at prompt start; use provider cache_control to avoid re-processing | 50-90% on cached prefix | Zero quality impact — identical processing to uncached | Low — restructure prompt ordering and add cache breakpoints |
| **Context checkpointing** | Save compressed state at phase boundaries; restore from checkpoint instead of replaying full history | 70-85% on cross-phase handoffs | Preserves essential state; loses exploratory dead-ends (often desirable) | Medium — define checkpoint schema and serialization |

---

## Context Budget Framework

Allocate the context window into three tiers based on change frequency and cache efficiency.

| Tier | Content Type | % of Budget | Change Frequency | Cache Strategy | Examples |
|---|---|---|---|---|---|
| **Static prefix** | System instructions, role definitions, tool schemas, project rules | 10-15% | Never within session | Provider-level prompt cache (cache_control breakpoints) | CLAUDE.md, instincts, gene definitions, tool descriptions |
| **Semi-stable** | Session goals, accumulated summaries, reference docs, phase artifacts | 20-30% | Once per phase or cycle | Application-level cache; invalidate on phase transition | instinctSummary, ledgerSummary, scout-report, build-report |
| **Dynamic** | Current task context, recent conversation turns, live code diffs, error output | 40-55% | Every turn | No cache; fresh each invocation | User messages, tool outputs, code under edit, test results |
| **Reserved headroom** | Buffer for model output generation and safety margin | 10-15% | N/A | N/A | Keep free to avoid truncation and degradation in output quality |

### Budget Allocation Rules

| Rule | Rationale |
|---|---|
| Never exceed 80% of advertised context window for input | Degradation accelerates in the last 20%; reserve for output and headroom |
| Place static prefix first, always | Maximizes KV-cache hit rate; anchors instructions against amnesia |
| Compress semi-stable tier at every phase boundary | Prevents context rot from accumulating stale phase artifacts |
| Evict dynamic content oldest-first, relevance-weighted | Recency bias means oldest dynamic content contributes least |
| Track token count explicitly before each LLM call | Invisible budget overruns cause silent quality degradation |
| Set hard ceiling per tier; reject overflow | Prevents any single tier from crowding out others |

---

## Mapping to Evolve-Loop

### Lean Mode as Context Management

| Lean Mode Feature | Context Strategy | Mechanism |
|---|---|---|
| Compressed prompts | Prompt compression | Strip boilerplate and verbose instructions; use imperative shorthand |
| instinctSummary | Hierarchical summarization | Compress full instincts file into ranked summary for agent injection |
| ledgerSummary | Hierarchical summarization | Compress full ledger into key metrics and recent cycle outcomes |
| Phase-scoped context | Context window management | Each agent (Scout, Builder, Auditor) receives only its phase-relevant context |
| Context checkpoint at phase gate | Context checkpointing | phase-gate.sh triggers state serialization between Scout, Builder, Auditor |

### Per-Phase Context Selection Matrix

| Context Element | Scout | Builder | Auditor | Rationale |
|---|---|---|---|---|
| System instructions + instincts | Full | Full | Full | All agents need role and project rules |
| instinctSummary (lean mode) | Compressed | Compressed | Compressed | Reduce instinct tokens by 60-70% |
| ledgerSummary | Include | Exclude | Include | Scout needs history for task selection; Auditor needs it for regression check; Builder does not |
| Gene definitions | Include | Exclude | Include | Scout selects genes; Auditor validates gene compliance; Builder works from task spec |
| scout-report.md | Exclude | Include | Include | Builder implements from scout report; Auditor validates against it |
| build-report.md | Exclude | Exclude | Include | Auditor reviews build output |
| Source code under edit | Exclude | Include | Include | Builder writes code; Auditor reviews it |
| Test output | Exclude | Include | Include | Builder fixes failures; Auditor verifies pass |
| Previous cycle audit | Include | Exclude | Exclude | Scout uses prior audit to select next task |

### AgentDiet Compression Integration

| Integration Point | Compression Target | Expected Savings |
|---|---|---|
| Pre-Scout prompt assembly | instincts + ledger + gene definitions | 30-40% token reduction |
| Pre-Builder prompt assembly | scout-report + source code context | 20-30% token reduction |
| Pre-Auditor prompt assembly | scout-report + build-report + test output | 25-35% token reduction |
| Cross-cycle handoff | Full cycle artifacts compressed to cycle summary | 70-80% reduction per completed cycle |

### Context Checkpoint and Handoff

| Checkpoint | Trigger | State Captured | State Excluded |
|---|---|---|---|
| Post-Scout | phase-gate.sh Scout pass | Task selection rationale, file targets, risk assessment | Exploratory search results, rejected candidates |
| Post-Builder | phase-gate.sh Builder pass | Code changes (diff), test results, build status | Intermediate attempts, reverted changes |
| Post-Auditor | phase-gate.sh Auditor pass | Audit verdict, score, issues found, cycle summary | Verbose analysis reasoning, redundant re-listings |
| Post-Ship | Successful merge/commit | One-line cycle outcome for ledger entry | All phase artifacts (archived to disk, evicted from context) |

---

## Prior Art

| Source | Key Contribution | Relevance to Long-Context Agents |
|---|---|---|
| **Chroma context-rot research** | Empirical measurement of model degradation as context fills; identified nonlinear quality drop after 60% capacity | Provides data-backed rationale for the 80% budget ceiling and headroom reservation |
| **Chain-of-Agents (ICLR 2026)** | Multi-agent architecture where worker agents process context segments independently, then a manager synthesizes | Directly applicable to evolve-loop's Scout/Builder/Auditor pipeline; each agent handles a context segment |
| **AgentDiet (arXiv:2509.23586)** | Trained prompt compression model achieving 30-50% token reduction with minimal quality loss on agent benchmarks | Integration candidate for lean mode prompt assembly; compress instincts, reports, and reference docs |
| **Anthropic prompt caching** | Provider-level KV-cache reuse for static prompt prefixes via cache_control breakpoints | Foundational to the static prefix tier; reduces cost and latency on invariant system instructions |
| **Google Gemini 1M/2M context** | Largest commercially available context windows; research on attention patterns at extreme lengths | Validates lost-in-the-middle effect at scale; informs ordering strategy (important content at edges) |
| **Karpathy autoresearch loop** | Autonomous research agent with iterative context accumulation and compression | Pattern for evolve-loop's multi-cycle context growth; demonstrates need for periodic compression |
| **OpenClaw compaction** | Context compaction technique that preserves decision-relevant information while discarding exploration traces | Aligns with checkpoint strategy of excluding exploratory dead-ends and rejected candidates |
| **Codebase-Memory-MCP** | MCP server providing persistent codebase memory across sessions via semantic retrieval | Complementary to retrieval-augmented context tier; provides cross-session memory without context window cost |

---

## Anti-Patterns

| Anti-Pattern | Description | Consequence | Mitigation |
|---|---|---|---|
| **Context stuffing** | Include all available information in every prompt regardless of relevance | Premature degradation; wasted tokens; attention dilution across irrelevant content | Apply per-phase context selection matrix; include only what the agent needs |
| **Ignoring degradation signals** | Assume model performance is constant across context window; never monitor output quality | Silent quality regression; agent produces increasingly unreliable outputs as context grows | Track token budget explicitly; monitor output quality metrics; compress at phase boundaries |
| **No budget tracking** | Never count tokens; discover budget overrun only when API rejects the request or output truncates | Unpredictable failures; truncated outputs missing critical content | Implement token counter; set hard ceilings per tier; alert before capacity threshold |
| **Monolithic prompts** | Single massive prompt containing all instructions, context, history, and data with no structure | Impossible to cache; no eviction granularity; lost-in-the-middle on all interior content | Decompose into tiered sections; use structured format (tables, headers); enable selective caching |
| **Replaying full history** | Pass entire conversation history to every agent call instead of compressed checkpoints | Exponential context growth; context rot from stale turns; unnecessary cost | Use context checkpoints; compress at phase boundaries; evict resolved conversation turns |
| **Uniform compression** | Apply same compression ratio to all content regardless of importance or recency | Critical instructions compressed away; trivial content preserved at full fidelity | Weight compression by relevance; protect static prefix from compression; compress oldest content most aggressively |
| **Skipping phase-gate compression** | Transition between Scout, Builder, and Auditor without compressing intermediate artifacts | Cumulative context bloat across phases; Builder inherits Scout's full exploratory context | Run context checkpoint at every phase-gate.sh invocation; serialize only essential state |
