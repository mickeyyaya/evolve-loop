# Performance Profiling in Evolve Loop

How to measure token spend per phase, identify cost bottlenecks, and track efficiency trends over cycles.

---

## Per-Phase Token Measurement

Each agent phase produces a structured output file. Token spend per phase is inferred from ledger entries tagged with the agent role:

```bash
# Token spend by agent role for a single cycle
jq -r 'select(.cycle == 9) | [.role, .data.tokensUsed // "n/a"] | @tsv' \
  .evolve/ledger.jsonl
```

Key ledger fields for profiling:

| Field | Description |
|-------|-------------|
| `role` | Agent phase: `scout`, `builder`, `auditor`, `operator` |
| `data.tokensUsed` | Tokens consumed by this invocation (if instrumented) |
| `data.filesChanged` | Files touched — proxy for context load |
| `data.attempts` | Retry count — each retry doubles token cost for that phase |
| `ts` | Timestamp — latency between phase completions |

When `tokensUsed` is not logged directly, estimate spend from:
- Scout: `filesAnalyzed * avgTokensPerFile` + web research calls
- Builder: file read tokens + write tokens + test run overhead
- Auditor: build-report.md size + files read in verification pass
- Operator: ledger summary size + state.json read

---

## Cost-Bottleneck Identification

### Which Agent Consumes the Most Tokens

Typical distribution in a balanced cycle:

| Phase | Share of Cycle Tokens | Primary Driver |
|-------|-----------------------|----------------|
| Scout | 25-40% | Full scan (cycle 1) or research calls |
| Builder | 40-55% | File reads, implementation, test runs |
| Auditor | 10-20% | build-report.md + targeted file verification |
| Operator | 5-10% | ledger.jsonl summary + state.json update |

Builder is the dominant consumer in most cycles. Scout becomes the bottleneck on cycle 1 (full scan) or when research cooldown expires and new web queries fire.

### Signals of Runaway Cost

Watch for these patterns in ledger entries:

- **`data.attempts > 1`** — each Builder retry re-reads all context; 3 retries can triple phase cost
- **`data.filesChanged > 10`** — high file count signals a task was undersized (complexity M or L disguised as S)
- **Scout scan mode = full on cycle 2+** — incremental scan should activate; if not, check `state.json projectDigest`
- **Auditor reads > 5 files** — routine audits should converge on build-report.md + 1-2 changed files; broad reads indicate high-risk changes
- **`tokenBudget.perCycle` exceeded** — Operator logs a warning; consecutive overruns trigger task-sizing recommendation next cycle

```bash
# Find cycles where Builder retried
jq -r 'select(.role == "builder" and .data.attempts > 1) | [.cycle, .data.task, .data.attempts] | @tsv' \
  .evolve/ledger.jsonl
```

---

## Cycle-Level Telemetry Patterns

### Using ledger.jsonl

The ledger is the primary telemetry source. Each entry is a JSONL record appended at phase completion.

```bash
# Cost trend: count build entries per cycle
jq -r 'select(.type == "build") | [.cycle, .data.status, .data.filesChanged] | @csv' \
  .evolve/ledger.jsonl

# Identify cycles with the most retries (highest token waste)
jq -r 'select(.role == "builder") | {cycle: .cycle, task: .data.task, attempts: (.data.attempts // 1)}' \
  .evolve/ledger.jsonl | jq -s 'sort_by(-.attempts) | .[0:5]'
```

### Using processRewardsHistory

The Operator appends `processRewardsHistory` entries to `state.json` after each cycle (last 5 cycles retained). This stores **per-step Builder confidence cross-validated against Auditor findings** — not named dimension scores. See [phase5-learn.md § Step-Level Process Rewards](../skills/evolve-loop/phase5-learn.md) for the canonical schema.

```bash
# Extract step-level confidence vs auditor mismatch trend
jq '.processRewardsHistory | map({cycle: .cycle, steps: [.steps[] | select(.auditorIssue)]})' \
  .evolve/state.json
```

Key profiling signals derived from processRewardsHistory:
- **Overconfident steps** — `builderConfidence` high but `auditorIssue: true` → Builder miscalibration
- **Underconfident steps** — `builderConfidence` low but `auditorIssue: false` → unnecessary caution
- **Systematic weakness** — same step type flagged across 2+ cycles → extract procedural instinct

Meta-cycle reads `processRewardsHistory` to identify systematic Builder weaknesses. Note: the per-cycle self-evaluation uses 4 LLM-as-a-Judge dimensions (Correctness, Completeness, Novelty, Efficiency) — these are distinct from the step-level process rewards.

---

## Token Budget Relationship

Two soft limits govern per-run spending (defined in `state.json tokenBudget`):

- **`perTask` (default 80,000 tokens):** Maximum for a single Builder invocation. Tasks projected to exceed this must be split by Scout into subtasks. Complexity M touching 10+ files is the key red flag.
- **`perCycle` (default 200,000 tokens):** Maximum across all four agent phases in one cycle. The Operator monitors cumulative spend and emits a budget warning in `operator-log.md` when exceeded.

Typical spend breakdown per cycle: Scout 10-60K (incremental vs. full scan), Builder 40-80K, Auditor 10-20K, Operator 5-10K. Consistent `perCycle` overruns indicate task sizing is too aggressive or model routing is not downgrading as expected.

---

## Model Routing Cost Impact

Model selection is the highest-leverage cost control. The evolve-loop uses a 3-tier abstraction (see SKILL.md § Model Tier System) so routing works across any LLM provider. Approximate cost ratios relative to tier-2:

| Tier | Relative Cost | Typical Phase |
|------|--------------|---------------|
| tier-3 | ~0.1-0.3x | Operator (standard), Scout (cycle 4+ with mature bandit), Auditor (clean streak), Calibrate (subsequent) |
| tier-2 | 1x | Builder (default), Scout (default), Auditor (default), Self-Eval (standard cycles) |
| tier-1 | ~3-5x | Scout (cycle 1, goal-directed), Builder (M + 5+ files, audit retry ≥ 2), Self-Eval (problem cycles), Meta-cycle review |

### When to Route to tier-3

- Scout on cycle 4+ with mature bandit data (3+ arms, pulls ≥ 3) — selection is data-driven
- Builder on S-complexity tasks with plan cache hit — execution-only, plan is proven
- Auditor after 5 consecutive clean audits (`consecutiveClean >= 5` in `auditorProfile`)
- Operator on all standard post-cycle updates
- Calibrate on subsequent calibrations (anchored by prior scores)

### When to Route to tier-1

- Scout on cycle 1 or goal-directed cycle ≤ 2 — strategic foundation sets session trajectory
- Builder on audit retry attempt ≥ 2 — design mistake needs deeper reasoning
- Self-Evaluation on problem cycles (audit retries, eval failures, miscalibration > 0.15)
- Meta-cycle review — always uses deep reasoning

tier-1 is justified at decision points with multiplicative downstream impact. A cycle routing Scout and Auditor to tier-3 while keeping Builder on tier-2 saves approximately 40-60% of total cycle cost versus all-tier-2 routing.

## Structured Output via Draft-Conditioned Decoding (DCCD-Inspired)

DCCD (arXiv:2603.03305) demonstrates that constraining LLM output to a schema during generation causes a "projection tax" — feasible mass drops, confidence falls 39%, and accuracy degrades up to 24pp. The fix: **two-stage draft-then-constrain** — let the model reason freely first, then project the draft onto the required schema.

**Application to Auditor verdicts:** The Auditor currently produces structured verdicts inline. DCCD suggests:
1. **Draft stage:** Auditor reasons freely about the build quality (no format constraints)
2. **Constrain stage:** Extract verdict fields (PASS/WARN/FAIL, severity, findings) from the draft into the structured audit-report format

This preserves reasoning quality while ensuring structured output. The same pattern applies to Scout's JSON decision traces and Builder's structured build-report tables.

---

## Budget-Aware Agent Scaling (BATS-Inspired)

BATS (arXiv:2511.17006) demonstrates that injecting continuous `budgetRemaining` signals into agent context produces strictly superior cost-performance Pareto frontiers compared to budget-unaware agents. Agents without budget awareness spend resources on low-value actions early and run out before high-value actions.

**Budget tracker injection:** Pass `budgetRemaining` to Builder and Scout context at each phase:

```json
{"budgetRemaining": {"tokens": 150000, "phase": "build", "strategy": "explore"}}
```

**Explore/exploit switching heuristic:**

| Budget Remaining | Strategy | Agent Behavior |
|-----------------|----------|----------------|
| >60% of perCycle | **Explore** | Full codebase scan, multiple approach candidates, broader research |
| 30-60% | **Prioritize** | Focus on highest-confidence approach, skip low-yield research |
| <30% | **Exploit** | Commit to current approach, no new exploration, minimize file reads |

**Integration with existing tokenBudget:** The orchestrator already tracks `tokenBudget.perTask` and `perCycle` in state.json. BATS adds dynamic strategy adaptation: after each agent invocation, recompute `budgetRemaining` and update the strategy field passed to the next agent. This prevents the common failure mode where early cycles consume 80% of budget on exploration, leaving insufficient tokens for later implementation.

For token reduction mechanisms including KV-cache prefix optimization, plan caching, and incremental scan, see `docs/token-optimization.md`.
