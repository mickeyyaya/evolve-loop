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

The orchestrator selects the model tier for each agent invocation based on phase complexity. Tiers are provider-agnostic — see SKILL.md § Model Tier System for concrete model mappings per provider (Anthropic, Google, OpenAI, Mistral, DeepSeek, open-weight).

| Phase | Default | Upgrade Condition | Downgrade Condition |
|-------|---------|-------------------|---------------------|
| Scout (DISCOVER) | tier-2 | Cycle 1 or goal-directed (cycle ≤ 2) → tier-1 | Cycle 4+ with mature bandit data (3+ arms, pulls ≥ 3) → tier-3 |
| Builder (BUILD) | tier-2 | M + 5+ files → tier-1; audit retry (attempt ≥ 2) → tier-1 | S + plan cache hit → tier-3 |
| Auditor (AUDIT) | tier-2 | Security-sensitive changes → tier-1 | Clean report, no risks → tier-3 |
| Calibrate (Phase 0) | tier-3 | First calibration of session → tier-2 | Subsequent calibrations → tier-3 |
| Operator (LEARN) | tier-3 | Last cycle / fitness regression / meta-cycle → tier-2 | Standard post-cycle → tier-3 |
| Self-Evaluation | tier-2 (inline) | Audit retries / eval failures / miscalibration → tier-1 | All clean → tier-2 (inline) |
| Meta-cycle review | tier-1 | Always | — |

The `repair` strategy always uses tier-2+ for Builder (accuracy over cost). The `innovate` strategy permits tier-3 for Auditor on style-only checks. tier-1 routing targets decision points with multiplicative downstream impact — ~6.5% cost increase per 5-cycle session, offset by fewer wasted retries.

---

## KV-Cache Prefix Optimization

Layer 0 shared values (the team constitution in `memory-protocol.md`) are placed **first** in every agent context block. Because this section never changes between cycles, LLM APIs with prompt caching (e.g., prefix caching) can cache the KV activations for that prefix and reuse them across all agent calls in a session, maximizing cache hit rate and reducing prompt processing cost.

Rule: static, invariant content must appear before dynamic content (task details, workspace files) in the context block.

---

## Instinct Summary

Rather than loading all individual instinct YAML files (which grow over cycles), agents read the compact `instinctSummary` array stored inline in `state.json`. This array holds only the essential fields per instinct (id, title, confidence, key rule) — typically under 2K tokens regardless of how many instincts exist.

Agents only fall back to reading full YAML files when `instinctSummary` is empty or missing.

---

## Plan Caching

When a task is structurally similar to one solved in a prior cycle, the orchestrator matches against `state.json planCache` (similarity threshold > 0.7) and passes the cached template to Builder as `priorPlan`. The Builder adapts the template rather than designing from scratch.

Templates are stored after successful builds and pruned after 10 cycles with zero reuses. Reuse failures demote the template. This achieves ~30-50% cost reduction on repeated task patterns.

### Plan Cache Schema

Each entry in `state.json planCache` follows this structure:

```json
{
  "slug": "<task-slug that generated this template>",
  "taskType": "feature|stability|security|techdebt|performance",
  "filePatterns": ["src/**/*.ts", "tests/**/*.test.ts"],
  "approach": "<1-2 sentence approach summary>",
  "steps": ["<step 1>", "<step 2>", "..."],
  "cycle": "<N>",
  "successCount": 1,
  "lastUsedCycle": "<N>",
  "ttlCycles": 10
}
```

**Write-Back Protocol** — when Builder ships a task successfully:

1. Extract the `approach` and `steps` from `build-report.md`
2. Generalize file paths to glob patterns (e.g., `docs/foo.md` → `docs/*.md`)
3. Write the entry to `state.json planCache`
4. If a matching entry already exists (same `slug` or high similarity), increment `successCount` rather than creating a duplicate

**Similarity Matching Algorithm** — how the orchestrator selects a `priorPlan` template:

1. Compare `taskType` — exact match required (contributes 0.3 to composite score)
2. Compare `filePatterns` — Jaccard similarity of glob sets (contributes 0.3)
3. Compare `approach` — keyword overlap ratio between candidate and stored approach strings (contributes 0.4)
4. Composite score = `0.3 * taskType_match + 0.3 * filePatterns_jaccard + 0.4 * approach_overlap`
5. Threshold: composite score > 0.7 triggers template reuse; Builder receives the template as `priorPlan`

**Eviction Rules:**

- Templates idle for 10 cycles with zero reuses are pruned from `planCache`
- When a reused template leads to an Auditor FAIL verdict, `successCount` is decremented
- Templates reaching `successCount <= 0` are flagged for manual review or removed next cycle

---

## Incremental Scan

On cycle 1, Scout performs a full codebase scan. On cycle 2+, Scout reads only the project digest (file list, recent changes, builder notes) instead of re-reading the entire codebase. This avoids redundant reads of files that have not changed, reducing Scout token usage by ~20-40K per cycle.

The Scout downgrade rule in model routing also allows tier-3 on incremental scans, compounding the savings.

---

## Research Cooldown

Web research queries are cached with a **12-hour TTL** in `state.json research.queries`. Before issuing any external search, agents check whether a recent result for the same query exists. If found and within TTL, the cached result is reused without a new API call.

This prevents duplicate research across cycles when the same topic recurs (e.g., best practices for a library, API documentation).

---

## Cross-Run Research Deduplication

**Problem:** When multiple parallel evolve-loop invocations share the same `state.json`, each run checks cooldowns independently. If two runs start within seconds of each other, both see expired cooldowns and issue the same queries — doubling research token costs for the same information.

**Protocol — query-level locking via state.json:**

Before issuing any web search, each run executes this protocol:

1. Read `state.json research.queries` with an OCC version check (read current `version`, compare after intended write).
2. Check if any existing entry matches the intended topic: keyword overlap ratio > 0.5 AND `issuedAt` within the last 12 hours.
3. **If match found** — skip the query and reuse cached `findings` from the matched entry. No API call is made.
4. **If no match** — write a placeholder entry with `"status": "pending"` and the current timestamp to `state.json` before issuing the query. Increment `version` atomically (OCC retry if version changed since step 1).
5. After the query completes, update the placeholder: replace `"status": "pending"` with `"status": "complete"` and write actual `findings`.
6. **Stale lock protection:** Other runs that see a `"pending"` entry wait up to 60 seconds (polling every 5s), then re-check. If still `"pending"` after 60 seconds, the waiting run issues the query independently and overwrites the stale placeholder.

**Pending entry schema:**
```json
{
  "topic": "<keyword summary>",
  "issuedAt": "<ISO-8601>",
  "status": "pending",
  "findings": null,
  "cycleNumber": <N>
}
```

**Expected savings:** Eliminates ~15-30K tokens per duplicate query avoided. With 3-4 queries per research phase, parallel runs save ~45-90K tokens per overlapping cycle.

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

All agent context blocks place invariant content (Layer 0 shared values, project context) before dynamic content (cycle-specific data, task objects). This maximizes KV-cache prefix hits — LLM APIs with prompt caching reuse activations for the shared prefix across all agent calls in a session. The KV-Cache Prefix Optimization section above is the evolve-loop's implementation of this principle.

### Just-In-Time Retrieval

Agents receive only the context they need for the current phase, not everything available:
- Scout reads `instinctSummary` (compact array) instead of all instinct YAML files
- Builder reads inline eval graders from scout-report instead of full eval files
- Operator reads `ledgerSummary` instead of full `ledger.jsonl`
- Incremental scan mode reads only `changedFiles` instead of the full codebase

The anti-pattern is "eager loading" — dumping all available context into every agent prompt. Each additional token of irrelevant context dilutes attention on relevant information and increases cost.

### Sub-Agent Compaction

When agents return results, the orchestrator extracts only the essential output (task list, verdict, score) rather than passing raw agent output to downstream agents. Target: each agent-to-agent handoff should carry under 2K tokens of context. The workspace file pattern (scout-report.md, build-report.md, audit-report.md) enforces this by giving each agent a structured, bounded output format.

### Context Window Management (Continuous Execution)

The evolve-loop runs **continuously through all requested cycles without stopping**. It never pauses to ask the user to resume.

After each cycle, a `handoff.md` checkpoint file is written with session state. This is a **safety checkpoint only** — if the session is externally interrupted (e.g., network drop, manual cancellation), a new session can read it to pick up where it left off.

```markdown
# Cycle Handoff — Cycle {N}

## Session State
- Cycles completed: <N> | Remaining: <M>
- Strategy: <strategy> | Goal: <goal or null>
- Benchmark: <overall>/100 (delta: +/-N from session start)

## This Cycle
- Tasks: <slug1> (PASS), <slug2> (PASS), <slug3> (FAIL)
- Audit iterations: <avg attempts per task>
- Instincts extracted: <N>

## Carry Forward
- Stagnation: <patterns or "none">
- Operator warnings: <list or "none">
- Research cooldowns: <expiry times>
- Next cycle priorities: <from operator brief>

## Cumulative Session Stats
- Total shipped: <N> | Failed: <N>
- Benchmark trajectory: <start> → <current>
- Active instincts: <count> (last extracted: cycle <N>)
```

This handoff serves as a **compaction anchor** — when the host LLM auto-compacts conversation history, this structured summary ensures cross-cycle continuity survives summarization. Under high context pressure, the orchestrator reduces token usage by relying on `instinctSummary` and `ledgerSummary` from state.json, keeping workspace files concise, trimming agent context to essential fields only, and activating lean mode (see Orchestrator Context Management below).

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

Hard turn caps in multi-turn agent loops are a blunt instrument. Research on SWE-bench shows that dynamic turn budgets achieve 24-68% cost reduction with minimal solve-rate impact, and an additional 12-24% savings beyond fixed limits when extensions are only granted on-demand (Turn-Control, arXiv:2510.16786).

**The core problem:** Token usage in agentic loops grows quadratically with turn count (each turn adds to the context for all subsequent turns). A loop that runs 2x too many turns costs roughly 4x as many tokens on average.

### Per-Phase Turn Budgets

| Phase | Default Budget | Extension Policy |
|-------|---------------|-----------------|
| Scout | 5 turns | None — tight budget enforced; Scout must produce a task list within 5 turns |
| Builder | 10 turns | Dynamic extension: up to 5 additional turns granted when measurable progress is detected (eval delta > 0 or files changed) |
| Auditor | 3 turns | None — verdict must be reached within 3 turns; complexity escalates to tier-1 model instead |
| Operator | 2 turns | None — Operator writes state updates and brief; any deeper analysis deferred to meta-cycle |

The 75th percentile of historical usage is the recommended sweet spot for fixed limits (Turn-Control, arXiv:2510.16786). The budgets above are set at approximately the 75th percentile based on observed evolve-loop phase durations.

### Builder Dynamic Extension Mechanism

When Builder reaches turn 10, the orchestrator evaluates continuation criteria before granting additional turns:

1. **Progress check:** Has at least one file been modified since the last extension grant? If no files changed in the last 3 turns, the build is classified as stuck (see Early-Exit Detection below).
2. **Eval delta check:** If eval graders were run, did any grader status improve since the last turn? A flat or regressing eval score signals diminishing returns.
3. **Extension grant:** If both checks pass, up to 5 additional turns are granted (total cap: 15 turns). Extensions are non-renewable — if the builder reaches 15 turns it must report FAIL.
4. **Reason logging:** The orchestrator logs the extension grant reason in the ledger (`"type": "turn-extension"`) for retrospective analysis.

### Early-Exit Detection for Stuck Builds

Source: Fan et al., arXiv:2509.09853 (SWE-Effi, Sep 2025). Agents burn massive tokens on unsolvable problems without recognizing they are stuck. Token accumulation becomes a snowball: each additional turn re-encodes the full conversation history, compounding the cost with no marginal progress.

**Stuck build signals:**
- No file changes in the last 3 consecutive turns
- The same error or test failure repeating without variation across 2+ attempts
- Token spend exceeds 60K with zero eval graders passing

**On stuck detection:** The orchestrator terminates the build immediately (early exit), writes a FAIL report with `"stuckBuild": true`, and records the turn at which the stuck pattern was detected. The Scout picks this up next cycle and recommends a smaller or differently-scoped task.

**Pattern — marginal value gating:**

1. At each turn, estimate the probability that the current partial result is already sufficient (completion probability).
2. Compute the expected marginal value of one more turn: `E[value_gain] = completion_probability_delta * task_value`.
3. Stop early when `E[value_gain] < turn_cost` — i.e., when the expected improvement no longer justifies the token expenditure.

**Evolve-loop application:** The `tokenBudget.perTask` soft limit (80K tokens) and Scout's task-sizing rules act as a static approximation of this dynamic approach. The per-phase budgets above replace the single flat limit with phase-appropriate defaults backed by the Turn-Control research (arXiv:2510.16786).

---

## Orchestrator Context Management

### The Accumulation Problem

The orchestrator's conversation context grows ~40-60K tokens per cycle from agent prompts, results, file reads, and state updates. By cycle 6, this accumulates to ~300K+ tokens, causing progressive slowdown and increased cost. Symptoms: each cycle takes noticeably longer than the previous one, and token usage per cycle increases even for similar-complexity tasks.

### Lean Mode (cycles 4+)

After the first 3 cycles of an invocation, the orchestrator activates lean mode to cap per-cycle context growth:
- **State.json**: Read once at cycle start, not re-read before Phase 4
- **Agent results**: Use returned summaries instead of reading full workspace files
- **Scout report**: Extract task list from agent return value, not separate file read
- **Eval checksums**: Compute once, verify from memory
- **Benchmark delta**: Skip for S-complexity docs-only changes

Estimated savings: ~15-20K tokens per cycle (from ~50K to ~30K).

### Compaction Anchor Pattern

The `handoff.md` written after each cycle serves as a **compaction anchor** — a structured summary that preserves cross-cycle continuity when the host LLM auto-compacts conversation history. The handoff captures session state, cycle results, carry-forward context, and cumulative stats in a format that survives summarization without information loss.

### Recommended Batch Sizes

For optimal efficiency, run 5-7 cycles per invocation. Beyond 7 cycles, context accumulation begins to degrade performance even with lean mode active. For longer runs, start a new invocation that reads the previous handoff.md to resume.

---

For techniques that improve output accuracy and catch errors across these same agents and phases (chain-of-thought prompting, multi-stage verification, context alignment scoring, uncertainty acknowledgment), see `docs/accuracy-self-correction.md`.

For per-phase profiling and cost-bottleneck identification, see `docs/performance-profiling.md`.
