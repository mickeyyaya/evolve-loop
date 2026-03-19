# Token Optimization in Evolve Loop

How the evolve-loop minimizes token usage across agents and cycles.

## Summary Table

| Mechanism | Tokens Saved | When Active |
|-----------|-------------|-------------|
| Model Routing | ~30-60% cost reduction | Every agent invocation |
| KV-Cache Prefix Optimization | ~20-40% on repeated context | Cycle 2+ |
| Instinct Summary | ~10-20K per Builder run | When instincts exist |
| Plan Caching | ~30-50% on repeated task patterns | Structurally similar tasks |
| Incremental Scan | ~20-40K per Scout run | Cycle 2+ |
| Research Cooldown | ~15-30K per avoided search | Repeated queries within 12hr |
| Token Budget Schema | Prevents runaway costs | Every task and cycle |
| Auditor Adaptive Strictness | ~10-20K per routine audit | After 5 consecutive clean audits |

---

## Model Routing

The orchestrator selects the model for each agent invocation based on phase complexity:

| Phase | Default | Upgrade Condition | Downgrade Condition |
|-------|---------|-------------------|---------------------|
| Scout (DISCOVER) | sonnet | Deep research goal → opus | Cycle 2+ incremental scan → haiku |
| Builder (BUILD) | sonnet | M-complexity + 5+ files → opus | S-complexity inline tasks → haiku |
| Auditor (AUDIT) | sonnet | Security-sensitive changes → opus | Clean report, no risks → haiku |
| Operator (LEARN) | haiku | HALT conditions → sonnet | Standard post-cycle → haiku |
| Meta-cycle review | opus | Always | — |

The `repair` strategy always uses sonnet+ for Builder (accuracy over cost). The `innovate` strategy permits haiku for Auditor on style-only checks.

---

## KV-Cache Prefix Optimization

Layer 0 shared values (the team constitution in `memory-protocol.md`) are placed **first** in every agent context block. Because this section never changes between cycles, the Claude API can cache the KV activations for that prefix and reuse them across all agent calls in a session, maximizing cache hit rate and reducing prompt processing cost.

Rule: static, invariant content must appear before dynamic content (task details, workspace files) in the context block.

---

## Instinct Summary

Rather than loading all individual instinct YAML files (which grow over cycles), agents read the compact `instinctSummary` array stored inline in `state.json`. This array holds only the essential fields per instinct (id, title, confidence, key rule) — typically under 2K tokens regardless of how many instincts exist.

Agents only fall back to reading full YAML files when `instinctSummary` is empty or missing.

---

## Plan Caching

When a task is structurally similar to one solved in a prior cycle, the orchestrator matches against `state.json planCache` (similarity threshold > 0.7) and passes the cached template to Builder as `priorPlan`. The Builder adapts the template rather than designing from scratch.

Templates are stored after successful builds and pruned after 10 cycles with zero reuses. Reuse failures demote the template. This achieves ~30-50% cost reduction on repeated task patterns.

---

## Incremental Scan

On cycle 1, Scout performs a full codebase scan. On cycle 2+, Scout reads only the project digest (file list, recent changes, builder notes) instead of re-reading the entire codebase. This avoids redundant reads of files that have not changed, reducing Scout token usage by ~20-40K per cycle.

The Scout downgrade rule in model routing also allows haiku on incremental scans, compounding the savings.

---

## Research Cooldown

Web research queries are cached with a **12-hour TTL** in `state.json research.queries`. Before issuing any external search, agents check whether a recent result for the same query exists. If found and within TTL, the cached result is reused without a new API call.

This prevents duplicate research across cycles when the same topic recurs (e.g., best practices for a library, API documentation).

---

## Token Budget Schema

Two soft limits are enforced per run:

- **`tokenBudget.perTask`** (default 80,000): Maximum tokens a single Builder invocation should consume. Scout must break tasks likely to exceed this into smaller subtasks. Complexity M touching 10+ files is a red flag.
- **`tokenBudget.perCycle`** (default 200,000): Maximum tokens across all agents in one cycle. The orchestrator tracks cumulative usage and warns if exceeded.

These are soft limits — the orchestrator monitors and warns rather than hard-stopping. Consistent overruns trigger an Operator recommendation to reduce task sizing next cycle.

---

## Auditor Adaptive Strictness

The Auditor reads `auditorProfile` from `state.json` to skip redundant checklist sections for task types with a strong reliability track record:

- `consecutiveClean`: number of consecutive audits with no MEDIUM+ issues for a task type
- When `consecutiveClean >= 5`: Auditor runs reduced checklist (Security + Eval Gate only); Code Quality and Pipeline Integrity sections are skipped
- The orchestrator resets `consecutiveClean` to 0 after any WARN, FAIL, or MEDIUM+ finding

Exceptions: `harden` and `repair` strategies, and tasks touching agent or skill files, always receive the full checklist regardless of profile.

---

## Context Engineering Principles

These principles, drawn from Anthropic's context engineering best practices (2025), are already implemented across the evolve-loop pipeline. Naming them explicitly enables consistent application and audit.

### Static-Before-Dynamic Ordering

All agent context blocks place invariant content (Layer 0 shared values, project context) before dynamic content (cycle-specific data, task objects). This maximizes KV-cache prefix hits — the Claude API caches activations for the shared prefix and reuses them across all agent calls in a session. The KV-Cache Prefix Optimization section above is the evolve-loop's implementation of this principle.

### Just-In-Time Retrieval

Agents receive only the context they need for the current phase, not everything available:
- Scout reads `instinctSummary` (compact array) instead of all instinct YAML files
- Builder reads inline eval graders from scout-report instead of full eval files
- Operator reads `ledgerSummary` instead of full `ledger.jsonl`
- Incremental scan mode reads only `changedFiles` instead of the full codebase

The anti-pattern is "eager loading" — dumping all available context into every agent prompt. Each additional token of irrelevant context dilutes attention on relevant information and increases cost.

### Sub-Agent Compaction

When agents return results, the orchestrator extracts only the essential output (task list, verdict, score) rather than passing raw agent output to downstream agents. Target: each agent-to-agent handoff should carry under 2K tokens of context. The workspace file pattern (scout-report.md, build-report.md, audit-report.md) enforces this by giving each agent a structured, bounded output format.

### Context Window Management (Stop-Hook Pattern)

The evolve-loop uses a **60% capacity threshold** to prevent context exhaustion mid-cycle. After each cycle completes, the orchestrator assesses context window usage:

- **Below 60%:** Continue to next cycle normally.
- **At or above 60%:** Write a `handoff.md` file and stop gracefully.

The `handoff.md` file carries all context needed to resume in a fresh session:

```markdown
# Cycle Handoff — Cycle {N}

## Session State
- Cycles completed this session: <count>
- Strategy: <current strategy>
- Goal: <goal or null>
- Remaining cycles: <endCycle - currentCycle>

## Key Context to Carry Forward
- Active stagnation patterns: <list>
- Unresolved operator warnings: <list>
- Last delta metrics: <summary>

## Resume Command
`/evolve-loop <remaining cycles> [strategy] [goal]`
```

This enables **indefinite runtime** across sessions. The next `/evolve-loop` invocation reads `handoff.md` during initialization and applies the carried-forward context. The key insight: it's better to stop cleanly at 60% and resume with full context capacity than to push to 90%+ and risk mid-cycle truncation where partial work is lost.

### Minimal Non-Overlapping Tool Sets

Each agent receives only the tools it needs. Scout gets Read/Grep/Glob/Bash/WebSearch/WebFetch (discovery tools). Builder gets Read/Write/Edit/Bash/Grep/Glob (implementation tools). Auditor gets Read/Grep/Glob/Bash (review tools). Operator gets Read/Grep/Glob (assessment tools). No agent receives tools it doesn't use — this reduces tool-description overhead in the prompt and prevents misuse.

---

## Agentic Plan Caching (APC) — Research Baseline

Source: "Agentic Plan Caching: Test-Time Memory for Fast and Cost-Efficient LLM Agents" (NeurIPS 2025).

Benchmark results on standard agentic benchmarks:
- **50.31% cost reduction** vs. no caching baseline
- **27.28% latency reduction**
- **96.61% performance retention** (task solve rate nearly unchanged)

How APC works (two-step template extraction from execution logs):

1. **Rule-based filter** — strips verbose chain-of-thought reasoning and agent scratchpad from prior execution traces, keeping only the structural plan skeleton.
2. **Lightweight LLM pass** — removes context-specific entities (file names, variable values, cycle numbers), producing a reusable template that generalises across structurally similar tasks.

Relevance to evolve-loop: The `planCache` mechanism in `state.json` (similarity threshold > 0.7, template pruning after 10 zero-reuse cycles) is the evolve-loop's implementation of this pattern. The NeurIPS 2025 results provide an external benchmark for expected savings — the 30-50% estimate in the Plan Caching section above is conservative relative to the paper's 50.31% figure.

---

## Dynamic Turn Limits

Hard turn caps in multi-turn agent loops are a blunt instrument. A probability-based approach cuts costs ~24% while maintaining solve rates.

**The core problem:** Token usage in agentic loops grows quadratically with turn count (each turn adds to the context for all subsequent turns). A loop that runs 2x too many turns costs roughly 4x as many tokens on average.

**Pattern — marginal value gating:**

1. At each turn, estimate the probability that the current partial result is already sufficient (completion probability).
2. Compute the expected marginal value of one more turn: `E[value_gain] = completion_probability_delta * task_value`.
3. Stop early when `E[value_gain] < turn_cost` — i.e., when the expected improvement no longer justifies the token expenditure.

**Evolve-loop application:** The `tokenBudget.perTask` soft limit (80K tokens) and Scout's task-sizing rules act as a static approximation of this dynamic approach. A future improvement would track per-turn token spend and emit a stop signal when marginal progress (measured by eval score delta) falls below a configurable threshold.

For techniques that improve output accuracy and catch errors across these same agents and phases (chain-of-thought prompting, multi-stage verification, context alignment scoring, uncertainty acknowledgment), see `docs/accuracy-self-correction.md`.
