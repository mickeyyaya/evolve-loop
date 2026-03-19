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

### Using processRewards

The Operator writes `processRewards` into `state.json` after each cycle. This array tracks per-metric scores (efficiency, modularity, accuracy) over time and is the primary input for strategy adjustment.

```bash
# Extract efficiency score trend
jq '.processRewards | map({cycle: .cycle, efficiency: .scores.efficiency})' \
  .evolve/state.json
```

Efficiency score components relevant to profiling:
- **tokenEfficiency** — eval score per 1K tokens spent (higher is better)
- **retryRate** — fraction of build phases requiring retries
- **auditPassRate** — fraction of builds passing audit on first submission

A falling `tokenEfficiency` score over 3+ cycles is the strongest signal to reduce task complexity or enable more aggressive model downgrading.

---

## Token Budget Relationship

Two soft limits govern per-run spending (defined in `state.json tokenBudget`):

- **`perTask` (default 80,000 tokens):** Maximum for a single Builder invocation. Tasks projected to exceed this must be split by Scout into subtasks. Complexity M touching 10+ files is the key red flag.
- **`perCycle` (default 200,000 tokens):** Maximum across all four agent phases in one cycle. The Operator monitors cumulative spend and emits a budget warning in `operator-log.md` when exceeded.

Typical spend breakdown per cycle: Scout 10-60K (incremental vs. full scan), Builder 40-80K, Auditor 10-20K, Operator 5-10K. Consistent `perCycle` overruns indicate task sizing is too aggressive or model routing is not downgrading as expected.

---

## Model Routing Cost Impact

Model selection is the highest-leverage cost control. Approximate cost ratios (relative to sonnet):

| Model | Relative Cost | Typical Phase |
|-------|--------------|---------------|
| haiku | ~0.04x | Operator (standard), Scout (incremental cycle 2+), Auditor (clean streak) |
| sonnet | 1x | Builder (default), Scout (default), Auditor (default) |
| opus | ~3-5x | Builder (M-complexity, 5+ files), Auditor (security-sensitive), Scout (deep research) |

### When to Route to Haiku

- Scout on cycle 2+ incremental scan (no research calls, no new domain detection needed)
- Builder on S-complexity inline documentation tasks (this task is an example)
- Auditor after 5 consecutive clean audits (`consecutiveClean >= 5` in `auditorProfile`)
- Operator on all standard post-cycle updates

Opus is justified when Builder touches agent orchestration logic (phases.md, SKILL.md), Auditor finds security-sensitive changes, or the `repair` strategy is active. Meta-cycle review always uses opus. A cycle routing Scout and Auditor to haiku while keeping Builder on sonnet saves approximately 40-60% of total cycle cost versus all-sonnet routing.

For token reduction mechanisms including KV-cache prefix optimization, plan caching, and incremental scan, see `docs/token-optimization.md`.
