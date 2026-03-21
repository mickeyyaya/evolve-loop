# Quality vs. Cost Tradeoffs in Evolve-Loop

<!-- challenge-token: cc369004556988c7 -->

A decision framework for understanding when token optimizations help quality, when they are neutral, and when they silently degrade the work the agents produce. Written for humans, not LLMs.

---

## 1. The Fundamental Tradeoff

Every token optimization is a bet: "I can remove this information without harming the output."

Sometimes that bet is obviously safe — removing duplicate tool descriptions doesn't affect the agent's reasoning. Sometimes it isn't — compressing a file that contains load-bearing implementation context can cause the Builder to miss a critical constraint.

The tension is asymmetric. Savings from optimization are immediate and visible (cheaper API bills, faster cycles). Quality loss is delayed and subtle (a build passes evals but produces brittle code; an audit misses a security issue). This lag makes it easy to over-optimize.

**When saving tokens does NOT hurt quality:**
- You are removing redundant structure, not information (duplicate headers, boilerplate)
- The removed content was irrelevant to the current phase (Scout history in Builder context)
- The downstream agent has access to the original if needed (full logs preserved)

**When saving tokens DOES hurt quality:**
- The compressed content contains a decision rationale the next agent needs
- The cheaper model receives a task that requires nuanced judgment
- The shorter context removes examples the agent was relying on for pattern-matching

---

## 2. Decision Matrix

For each optimization technique in the evolve-loop, here is its quality classification and the evidence behind it.

### Quality-Neutral (safe to apply unconditionally)

| Technique | Why No Quality Loss |
|-----------|-------------------|
| KV-cache prefix ordering | Identical tokens, just reordered for cache efficiency |
| Lazy tool loading | Removes unused tool descriptions — agents never needed them |
| Research cooldown deduplication | Reuses the same research result; no information lost |
| Prompt caching | Same content served from cache |

These are pure wins. There is no scenario where applying them makes output worse.

### Quality-Improving (save tokens AND improve output)

| Technique | Why Quality Improves | Evidence |
|-----------|---------------------|----------|
| Phase isolation (CoDA pattern) | Removes irrelevant Scout noise from Builder context — agent focuses on its actual task | CoDA arXiv:2512.12716: stable performance where monolithic baselines degraded |
| Dynamic turn budgets | Forces planning over trial-and-error; early exits prevent wasted effort on stuck builds | Turn-Control arXiv:2510.16786: solve rate maintained at 24-68% cost reduction |
| Graph-based file navigation | Agent reads 3-7 relevant files instead of 50+; focused context improves task relevance | RepoMaster arXiv:2505.21577: 95% token reduction, 110% improvement in submissions |
| Structured Distillation format | Distilled 4-field summaries are easier to search than raw logs | Structured Distillation arXiv:2603.13017: 96% retrieval quality at 11x compression |
| Instinct summary (compact array) | Compact instinct index reduces noise vs. loading all YAML files | Evolve-loop design |

The non-obvious finding: optimization can improve quality because noise is the enemy of attention. Removing irrelevant context helps the agent focus on what matters.

### Quality-Trading (save tokens, risk quality — requires guardrails)

| Technique | Quality Risk | Guardrail |
|-----------|-------------|-----------|
| Model downgrading (tier-2 → tier-3) | Cheaper model may miss subtle issues or produce fragile code | Requires `consecutiveClean >= 3` for same task type; automatic revert on any WARN or FAIL |
| Active context compression | Compressed summary may drop critical detail | Focus Agent: 96% retrieval quality maintained (arXiv:2601.07190); never compress load-bearing files |
| Memory distillation | 4-field summary loses nuance from raw cycle logs | Full cycle logs preserved in `.evolve/history/` for retrieval; distillation is an index, not a replacement |
| Adaptive audit strictness | Reduced checklist may miss edge cases | Sections D/E (eval integrity) never skipped; 20% of audits run full checklist regardless |
| Turn budget hard caps | Artificially stopping a build in progress | Extensions granted when measurable progress detected (eval delta > 0 or files changed) |

These techniques require active monitoring. They should not be applied and forgotten.

---

## 3. Quality Signals to Watch

If you apply quality-trading optimizations, these metrics will surface degradation before it becomes a crisis.

**Primary signals** (check after every cycle that uses a new optimization):

- **`fitnessScore`** — composite quality metric in `state.json`. A drop of more than 2 points in a single cycle is a warning. A drop of more than 5 points means pause and investigate.
- **`projectBenchmark.overall`** — project-level quality score. Any regression from cycle-over-cycle baseline while a cost optimization is active is suspicious.
- **`auditorProfile.consecutiveClean`** — resets to 0 on any WARN or FAIL. If this counter stops growing after a model downgrade, the downgrade is hurting quality.

**Secondary signals** (check weekly or when primary signals trigger):

- **Eval first-attempt failure rate** — tracked implicitly by Auditor retry counts in the ledger. If retry count rises after applying an optimization, the optimization is probably the cause.
- **Audit WARN rate** — count of WARN verdicts per cycle. A rising WARN rate after a tier-3 downgrade is a direct signal that the cheaper model is producing lower-quality work.
- **Benchmark regression** — if `projectBenchmark.overall` drops more than 3 points across any 5-cycle window, the orchestrator suspends all tier-3 Builder routing until recovery.

---

## 4. When to Reverse an Optimization

Reverting a cost-saving measure is not a failure — it is the system working as designed. These are the concrete triggers:

**Reverse model downgrading (tier-3 → tier-2) immediately when:**
- Any Auditor WARN or FAIL verdict occurs for a task type that was using tier-3
- Eval first-attempt failure rate exceeds 33% for the task type
- `consecutiveClean` drops below 3 after any new cycle

**Reverse active context compression when:**
- A build fails because the Builder "forgot" a constraint it had previously seen (root-cause: compressed summary dropped load-bearing detail)
- Auditor notes "inconsistent with earlier decision" more than once in a 3-cycle window

**Reverse adaptive audit strictness when:**
- Any security-sensitive file is changed (always triggers full checklist regardless)
- A `harden` or `repair` strategy cycle is active
- A bug is discovered in production that an audit check would have caught

**Reverse turn budget caps when:**
- Builder consistently reports FAIL at turn limit with no "stuck" signals (progress was happening but was cut off)
- Eval graders were improving turn-over-turn at the point of termination

**General trigger:** If you apply an optimization and `fitnessScore` drops in 2 consecutive cycles, revert the optimization and observe whether the score recovers. If it does, the optimization was the cause.

---

## 5. The Golden Rule

> **Optimize the container, not the content.**

Remove redundant *structure* — duplicate tool descriptions, stale conversation history, boilerplate formatting, re-read file contents that haven't changed. Do not remove *information* that the current phase needs to make correct decisions.

**Container examples** (safe to optimize):
- The Scout's raw file-scan output after the Builder has the scout-report.md summary
- Tool schemas for tools the current agent will never invoke
- Conversation history from phases that have already completed
- Repeated context blocks that appear in every cycle without changing

**Content examples** (never optimize away):
- The eval graders a Builder needs to verify its own output
- The error message from a failed test that the Builder is trying to fix
- The acceptance criteria in the task spec
- The quality guardrail rules that govern when a model downgrade is safe

When in doubt, ask: "If this token is missing, can the agent still make the right decision?" If no, it is content. Keep it.

---

## Research References

The quantitative claims in this document come from published papers applied during cycles 112-114. For full citations and implementation details, see `docs/research-applied-context-optimization.md`.

- **Focus Agent** (arXiv:2601.07190): 96% retention on structured distillation
- **Structured Distillation** (arXiv:2603.13017): 96% retrieval quality at 11x compression
- **Turn-Control** (arXiv:2510.16786): solve rate maintained at 24-68% cost reduction
- **MasRouter** (arXiv:2502.11133): 52-70% cost reduction via model routing with quality maintained

See also `docs/token-optimization.md` for implementation details and `docs/performance-profiling.md` for profiling methodology.
