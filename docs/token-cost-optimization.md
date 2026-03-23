> **Token Cost Optimization** — Reference doc on reducing token costs and latency in agent pipelines. Covers caching, model routing, speculative decoding, and budget monitoring for evolve-loop cycles.

## Table of Contents

1. [Cost Reduction Techniques](#cost-reduction-techniques)
2. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
3. [Budget Monitoring](#budget-monitoring)
4. [Prior Art](#prior-art)
5. [Anti-Patterns](#anti-patterns)

---

## Cost Reduction Techniques

| Technique | Description | Expected Savings | Implementation Complexity |
|-----------|-------------|-----------------|--------------------------|
| Semantic caching | Cache LLM responses keyed by semantic similarity of prompts; serve cached results for near-duplicate queries | 90% token savings on repeated queries | Medium — requires embedding index and similarity threshold tuning |
| Prompt caching | Use provider-level cache_control (Anthropic) or cached_tokens (OpenAI) to avoid re-processing static prompt prefixes | 50-90% reduction on cached prefix tokens | Low — add cache_control breakpoints to system prompts |
| Output token reduction | Constrain response length via max_tokens, structured output schemas, and imperative instructions ("reply in under 200 tokens") | 30-60% latency reduction; proportional cost savings | Low — prompt engineering and max_tokens tuning |
| Speculative decoding | Use a smaller draft model to propose tokens; larger model verifies in parallel batches | 1.5-4x inference speedup with no quality loss | High — requires paired model deployment or provider support |
| Dynamic loop control | Monitor convergence signals mid-loop; terminate early when diminishing returns detected | 24% average cost reduction per pipeline run | Medium — instrument loop with quality-gate checks |
| Model routing | Route simple tasks to cheaper/faster models (Haiku); reserve expensive models (Opus) for complex reasoning | 50-70% cost reduction on routed tasks | Medium — build classifier or heuristic router |
| Context pruning | Strip irrelevant context before each LLM call; send only what the agent needs | 20-40% token savings per call | Medium — requires dependency analysis of prompt sections |
| Batch processing | Aggregate independent LLM calls into batched API requests where supported | 50% cost discount (Anthropic Batch API) | Low — restructure calls to use batch endpoints |

## Mapping to Evolve-Loop

| Evolve-Loop Feature | Optimization Technique | Where It Applies | Expected Impact |
|---------------------|----------------------|------------------|-----------------|
| Lean mode | Output token reduction + context pruning | Scout, Builder, Auditor agents | Reduce per-cycle token spend by 30-50% |
| Tier-based model routing | Model routing | Route Scout analysis to Haiku; keep Builder on Sonnet; escalate Auditor edge cases to Opus | 50-70% savings on Scout phase |
| AgentDiet compression | Context pruning + semantic summarization | Compress workspace artifacts before passing between agents | 20-40% fewer input tokens per agent handoff |
| Context budget estimation | Dynamic token budgeting | Pre-estimate token cost before each agent launch; abort if over budget | Prevent runaway cycles; enforce per-cycle caps |
| Early termination in Self-MoA | Dynamic loop control | Stop mixture-of-agents voting when consensus reached before all rounds complete | 24% average reduction in multi-agent reasoning cost |
| Prompt caching for system prompts | Prompt caching | Cache static portions of Scout, Builder, Auditor system prompts across cycles | 50-90% savings on system prompt tokens |
| Semantic result caching | Semantic caching | Cache Scout analysis for similar gene configurations; reuse Builder patterns for repeated task types | 90% savings on cache-hit cycles |
| Speculative pre-generation | Speculative decoding | Use Haiku as draft model for Builder code generation, verified by Sonnet | 1.5-2x speedup on code generation phase |

## Budget Monitoring

### Metrics to Track

| Metric | Granularity | Collection Method | Alert Threshold |
|--------|-------------|-------------------|-----------------|
| Input tokens per agent call | Per agent, per cycle | Log from API response `usage.input_tokens` | > 80% of model context window |
| Output tokens per agent call | Per agent, per cycle | Log from API response `usage.output_tokens` | > 4096 tokens (indicates unbounded generation) |
| Total tokens per cycle | Per cycle | Sum all agent calls in cycle | > 500K tokens per cycle |
| Cost per cycle (USD) | Per cycle | tokens * per-token rate | > $2.00 per cycle |
| Cache hit rate | Per agent type | Track cache hits / total calls | < 30% after warmup period |
| Cycles per hour | Pipeline-level | Timestamp delta between cycle completions | < 2 cycles/hour (indicates bottleneck) |
| Cost per task category | Per task type | Tag cycles by task type; aggregate cost | Any category > 3x median cost |

### Implementation Steps

| Step | Action | Tool |
|------|--------|------|
| 1 | Instrument all LLM calls to log `usage` fields | API wrapper / middleware |
| 2 | Write per-cycle cost to `workspace/metrics/cycle-N-cost.json` | Post-cycle hook |
| 3 | Aggregate metrics into rolling dashboard | `scripts/metrics-dashboard.sh` |
| 4 | Set alert thresholds in pipeline config | `genes/budget-genes.toml` |
| 5 | Auto-trigger lean mode when cycle cost exceeds threshold | Phase-gate check |
| 6 | Review weekly cost trends; adjust model routing tiers | Manual review |

## Prior Art

| Source | Key Contribution | Relevance to Evolve-Loop |
|--------|-----------------|--------------------------|
| AgentDiet (arXiv:2509.23586) | Systematic framework for compressing agent context without quality loss; reports 30-50% token reduction | Direct applicability to Scout/Builder/Auditor context handoffs |
| Speculative decoding (Leviathan et al., 2023) | Proved small draft models can accelerate large model inference 2-4x with identical output distribution | Apply to Builder code generation with Haiku-draft / Sonnet-verify |
| Anthropic prompt caching | Server-side caching of static prompt prefixes; 90% cost reduction on cached tokens; 5-minute TTL | Cache evolve-loop system prompts across rapid cycle execution |
| GPTCache (open-source) | Semantic similarity caching layer for LLM responses; pluggable embedding backends | Use as foundation for Scout analysis caching |
| OpenAI cached_tokens | Automatic prefix caching on repeated prompt structures; no code changes required | Reference implementation pattern for provider-agnostic cache abstraction |
| Karpathy autoresearch pattern | Agentic loop with built-in cost tracking and early termination on convergence | Model for dynamic loop control in Self-MoA |
| Codebase-Memory-MCP | MCP server for persistent codebase context; reduces re-indexing cost across sessions | Reduce Scout re-analysis cost on unchanged files |

## Anti-Patterns

| Anti-Pattern | Description | Consequence | Mitigation |
|-------------|-------------|-------------|------------|
| Over-caching stale results | Cache TTL too long; serve outdated analysis after codebase changes | Scout misses new bugs; Builder uses stale patterns | Invalidate cache on file change; use content-hash keys |
| Premature model downgrade | Route complex tasks to cheap models to save cost | Quality collapse; more cycles needed to fix errors | Validate routing classifier accuracy; keep fallback to stronger model |
| Ignoring output tokens | Optimize only input tokens; allow unbounded generation | Output tokens cost 3-5x more per token; latency spikes | Set max_tokens; use structured output schemas; audit response lengths |
| Context window waste | Send full file contents when only a function is needed | Hit context limits; pay for irrelevant tokens | Extract relevant snippets; use line-range reads; summarize large files |
| No budget caps | Run pipeline without per-cycle cost limits | Single runaway cycle consumes entire budget | Enforce hard caps in phase-gate; abort cycle on threshold breach |
| Cache without metrics | Enable caching but never measure hit rates | Unknown whether caching is effective; silent cache misses | Log cache hit/miss; alert on low hit rate; A/B test cache strategies |
| Monolithic prompts | Same large system prompt for all task types | Pay full prompt cost even for trivial tasks | Split prompts by task complexity; use tiered prompt templates |
| Sequential when parallel is possible | Run independent agent calls one at a time | 2-3x higher wall-clock time; no cost savings but higher latency | Identify independent calls; use parallel execution (TaskCreate) |
