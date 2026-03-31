---
name: evolve-loop/memory-protocol
description: "Shared memory protocol — defines all persistent state layers (JSONL ledger, workspace files, state.json, evals, instincts) and concurrency rules for the evolve-loop pipeline."
---

> Read this file when working with evolve-loop persistent state. Covers all memory layers, concurrency protocol, state.json schema, and instinct quality scoring.

## Contents
- [Layer 0: Shared Values](#layer-0-shared-values-core-rules) — behavioral rules, parallel coordination
- [Concurrency Protocol](#concurrency-protocol) — OCC writes, merge rules, run isolation
- [Layer 1: JSONL Ledger](#layer-1-jsonl-ledger-evolveledgerjsonl) — append-only structured log
- [Layer 2: Markdown Workspace](#layer-2-markdown-workspace-workspace_path) — agent-owned files, mailbox
- [Layer 3: Persistent State](#layer-3-persistent-state) — state.json schema, notes.md, project-digest
- [Layer 4: Eval State](#layer-4-eval-state) — eval definitions per task
- [Layer 5: Instincts](#layer-5-instincts) — personal instinct files
- [Layer 6: Experiment Journal](#layer-6-experiment-journal) — append-only Builder attempt log
- [Data Flow](#data-flow) — phase-by-phase pipeline diagram
- [Instinct Quality Scoring](#instinct-quality-scoring) — historicalSuccessRate extension

# Evolve Loop — Shared Memory Protocol

All files live under `.evolve/` in the **project directory** (not your CLI's global config directory).

## Layer 0: Shared Values (Core Rules)

Static team constitution. Place **first** in every agent context block for KV-cache optimization.

**Canonical source:** The full `sharedValues` JSON block is defined in [SKILL.md Shared Agent Values](SKILL.md). That is the single source of truth.

### Behavioral Rules

| Rule | Detail |
|------|--------|
| Immutability | Never mutate shared state directly; write new versions |
| Scope discipline | Each agent reads/writes only its owned workspace file |
| Blast radius awareness | Prefer S-complexity for hotspot files; edits < 30 lines |
| Learning mandate | When `instinctsExtracted == 0` for 2+ cycles, extract at least one before closing Phase 5 |

### Parallel Agent Coordination

- Agents must not write to each other's owned files
- `agent-mailbox.md` is the only shared write surface for cross-agent messages
- All agents read shared values first to align on core rules

## Concurrency Protocol

When multiple `/evolve-loop` invocations run in parallel, all shared state writes use **optimistic concurrency control (OCC)** via `version` field in `state.json`.

### OCC Write Protocol

1. **READ** `state.json`, note `version = V`
2. **Compute mutations** (new object — never mutate the read copy)
3. **WRITE** `state.json` with `version = V + 1`
4. **RE-READ** and verify `version == V + 1`
5. If conflict: re-read, rebase mutations using merge rules, retry with `new version + 1`. Max 3 retries, then HALT.

### Merge Rules

| Field | Strategy |
|-------|----------|
| `lastCycleNumber` | MAX of both values |
| `evaluatedTasks` | Union by slug (don't overwrite existing) |
| `evalHistory` | Append new cycle entry (keyed by unique cycle number) |
| `fitnessHistory` | Append new entry |
| `taskArms` | Sum `pulls` and `totalReward`, recompute `avgReward` |
| `instinctCount` | MAX |
| `instinctSummary` | Union by id |
| `mastery.consecutiveSuccesses` | Recompute from merged evalHistory tail |
| `research.queries` | Union by query string |
| `stagnation` | MAX of `nothingToDoCount` |
| `projectBenchmark` | First calibrator wins; skip if `lastCalibrated` < 1 hour ago |
| `researchAgenda.items` | Union by id (don't overwrite existing) |
| `researchAgenda.capsuleIndex` | Union by dimension key, dedupe slugs |
| `researchLedger.triedConcepts` | Union by id |
| `researchLedger.diversityTracker` | MAX per dimension count; concat lastResearchedDimensions |
| `skillInventory` | First calibrator wins; skip if `lastBuilt` < 1 hour ago |
| `skillEffectiveness` | Sum `invocations`, `hits`, `misses` per skill; recompute `hitRate` |
| `beyondAsk` | Sum `attempts`, `hits` per lens; recompute effectiveness |
| `version` | Always latest read value + 1 |

### Run Isolation

Each invocation generates `RUN_ID` (format: `run-<epoch-ms>-<random-4hex>`) and creates workspace at `.evolve/runs/$RUN_ID/workspace/`. Shared directories (`.evolve/evals/`, `.evolve/instincts/`, `.evolve/history/`) remain shared.

## Layer 1: JSONL Ledger (`.evolve/ledger.jsonl`)

Append-only structured log. Each agent appends one entry per invocation. Include `runId` for parallel run traceability.

```jsonl
{"ts":"<ISO-8601>","cycle":"<N>","runId":"<RUN_ID>","role":"<role>","type":"<type>","data":{...}}
```

Roles: `scout`, `builder`, `auditor`, `operator`, `eval`

## Layer 2: Markdown Workspace (`$WORKSPACE_PATH`)

Run-scoped workspace (`.evolve/runs/$RUN_ID/workspace/`). Each agent owns exactly one file. Shared `.evolve/workspace/` receives a copy after all cycles complete.

| File | Owner | Contains |
|------|-------|----------|
| `scout-report.md` | Scout | Discovery findings + selected task list |
| `build-report.md` | Builder | Implementation notes per task |
| `builder-notes.md` | Builder | Retrospective: fragility observations, approach surprises, recommendations. Persists across cycles. |
| `audit-report.md` | Auditor | Single-pass review + eval results |
| `operator-log.md` | Operator | Post-cycle health assessment |
| `session-summary.md` | Operator | Final cycle only: tasks shipped, features, fitness arc, synthesis |
| `agent-mailbox.md` | All agents | Cross-agent messages. Cleared of non-persistent messages in Phase 4. |

### Agent Mailbox Schema

```markdown
## Messages

| from | to | type | cycle | persistent | message |
|------|----|------|-------|------------|---------|
| scout | builder | hint | 5 | false | Prefer additive edits in phases.md |
| builder | auditor | flag | 5 | false | Eval grader path uses absolute path |
| auditor | scout | warn | 5 | true | phases.md Phase 4 is growing; consider splitting |
```

| Field | Values |
|-------|--------|
| `from` | `scout`, `builder`, `auditor`, `operator` |
| `to` | `scout`, `builder`, `auditor`, `operator`, `all` |
| `type` | `hint` (suggestion), `flag` (needs attention), `warn` (persistent), `info` (context) |
| `persistent` | `true` = survives Phase 4 cleanup; `false` = cleared each cycle |
| `message` | Free-text, under 100 chars |

Scout also appends a `decisionTrace` block listing evaluated candidates with `finalDecision` and `signals`. Consumed by Novelty Critic during meta-cycle.

**Orchestrator-written files:**

| File | Written by | Contains |
|------|-----------|----------|
| `research-brief.md` | Orchestrator (Phase 0.5) | Gap analysis + research findings + concept cards + keep/drop verdicts |
| `eval-report.md` | Orchestrator (eval-runner) | Eval gate results if run separately |
| `next-cycle-brief.json` | Operator | Guidance for next Scout: `weakestDimension`, `recommendedStrategy`, `taskTypeBoosts`, `avoidAreas`, `cycle`. Written to `$WORKSPACE_PATH/` and `.evolve/latest-brief.json`. |

## Layer 3: Persistent State

### `.evolve/state.json`

Cycle memory — avoids repeating searches, re-evaluating rejected tasks, or retrying failed approaches.

```json
{
  "lastUpdated": "2026-03-13T10:00:00Z",
  "lastCycleNumber": 0,
  "version": 0,
  "research": {
    "queries": [
      {"query": "AI agent orchestration best practices 2026", "date": "2026-03-13T10:00:00Z", "keyFindings": ["finding1"], "ttlHours": 12}
    ]
  },
  "evaluatedTasks": [
    {"slug": "fix-bug-x", "decision": "completed", "cycle": 1},
    {"slug": "add-feature-y", "decision": "rejected", "reason": "too complex", "revisitAfter": "2026-04-01"},
    {"slug": "add-feature-z", "decision": "deferred", "cycle": 3, "prerequisites": ["fix-bug-x"],
     "counterfactual": {"predictedComplexity": "M", "estimatedReward": 0.7, "alternateApproach": "extend existing config loader", "deferralReason": "blocked by missing test coverage"}}
  ],
  "failedApproaches": [
    {"feature": "real-time sync", "approach": "WebSocket", "error": "serverless incompatible",
     "reasoning": "WebSocket requires persistent connections but deployment uses serverless with ~10s timeout",
     "filesAffected": ["src/sync/ws-handler.ts"], "cycle": 3, "alternative": "SSE polling or Vercel AI SDK streaming",
     "errorCategory": "integration", "failedStep": "Step 3: Implement WebSocket handler"}
  ],
  "evalHistory": [
    {"cycle": 1, "verdict": "PASS", "checks": 9, "passed": 9, "failed": 0,
     "delta": {"tasksShipped": 3, "tasksAttempted": 3, "auditIterations": 1.0, "successRate": 1.0, "instinctsExtracted": 4, "stagnationPatterns": 0}}
  ],
  "instinctCount": 4,
  "operatorWarnings": [],
  "stagnation": {"nothingToDoCount": 0, "recentPatterns": []},
  "warnAfterCycles": 5,
  "tokenBudget": {"perTask": 80000, "perCycle": 200000, "researchPhase": 25000},
  "mastery": {"level": "novice|competent|proficient", "consecutiveSuccesses": 0},
  "ledgerSummary": {"totalEntries": 0, "cycleRange": [0, 0], "scoutRuns": 0, "builderRuns": 0, "totalTasksShipped": 0, "totalTasksFailed": 0, "avgTasksPerCycle": 0},
  "instinctSummary": [],
  "projectBenchmark": {
    "lastCalibrated": null, "calibrationCycle": 0, "overall": 0,
    "dimensions": {
      "documentationCompleteness": {"automated": 0, "llm": 0, "composite": 0},
      "specificationConsistency": {"automated": 0, "llm": 0, "composite": 0},
      "defensiveDesign": {"automated": 0, "llm": 0, "composite": 0},
      "evalInfrastructure": {"automated": 0, "llm": 0, "composite": 0},
      "modularity": {"automated": 0, "llm": 0, "composite": 0},
      "schemaHygiene": {"automated": 0, "llm": 0, "composite": 0},
      "conventionAdherence": {"automated": 0, "llm": 0, "composite": 0},
      "featureCoverage": {"automated": 0, "llm": 0, "composite": 0}
    },
    "history": [], "highWaterMarks": {}
  },
  "fitnessScore": 0.0,
  "fitnessHistory": [],
  "fitnessRegression": false,
  "discoveryVelocity": {
    "current": 0,
    "history": [{"cycle": 0, "proposalsGenerated": 0}],
    "rolling3": 0.0
  },
  "proposals": [],
  "researchAgenda": {
    "lastUpdated": null,
    "items": [
      {
        "id": "ra-001",
        "question": "string — the research question",
        "priority": "P0|P1|P2",
        "source": "benchmark-gap|proposal-backing|failure-pattern|velocity-decline",
        "sourceDetail": "string — what triggered this question",
        "originCycle": 0,
        "status": "open|in-progress|resolved|stale",
        "queries": [],
        "capsuleRefs": [],
        "conceptCards": [],
        "resolvedCycle": null
      }
    ],
    "capsuleIndex": {
      "documentationCompleteness": [],
      "specificationConsistency": [],
      "defensiveDesign": [],
      "evalInfrastructure": [],
      "modularity": [],
      "schemaHygiene": [],
      "conventionAdherence": [],
      "featureCoverage": []
    }
  },
  "researchLedger": {
    "triedConcepts": [
      {
        "id": "tc-001",
        "conceptTitle": "string",
        "researchSource": "ra-NNN",
        "capsuleRef": "string — capsule slug",
        "originCycle": 0,
        "implementedCycle": 0,
        "taskSlug": "string",
        "verdict": "WORKS|DOESNT_WORK|INCONCLUSIVE",
        "evidence": "string — e.g. 'dimension: 73 → 82 (+9)'",
        "benchmarkBefore": {},
        "benchmarkAfter": {},
        "keepOrDrop": "KEEP|DROP",
        "droppedReason": null
      }
    ],
    "diversityTracker": {
      "dimensionCoverage": {
        "documentationCompleteness": 0,
        "specificationConsistency": 0,
        "defensiveDesign": 0,
        "evalInfrastructure": 0,
        "modularity": 0,
        "schemaHygiene": 0,
        "conventionAdherence": 0,
        "featureCoverage": 0
      },
      "lastResearchedDimensions": []
    }
  },
  "promptVariants": []
}
```

### State.json Field Reference

| Field | Description |
|-------|-------------|
| `version` (default 0) | OCC counter. Incremented on every write. Detects parallel conflicts. |
| `lastCycleNumber` (default 0) | Last completed cycle number. Used for atomic allocation (parallel-safe). |
| `warnAfterCycles` (default 5) | Soft threshold — warn user when requesting this many cycles. |
| `mastery.level` | Difficulty graduation: `novice` (0-2, S only), `competent` (3-5, S+M), `proficient` (6+, S+M+L). |
| `mastery.consecutiveSuccesses` | Reset to 0 on audit failure, incremented on successful ship. |
| `taskArms` | Multi-armed bandit state. Per-type: `pulls`, `totalReward`, `avgReward`. Arms with `avgReward >= 0.8` and `pulls >= 3` get +1 priority boost. |
| `ledgerSummary` | Aggregated ledger stats so agents never read full ledger. Updated in Phase 4. |
| `instinctSummary` | Compact array of active instincts. Updated in Phase 5. |
| `fitnessScore` | Weighted average: `0.25*discover + 0.30*build + 0.20*audit + 0.15*ship + 0.10*learn`. |
| `fitnessHistory` | Rolling 3-entry array for trend detection. |
| `fitnessRegression` | `true` when score decreases 2 consecutive cycles. Operator HALT signal. |
| `evalHistory` | Trimmed to last 5 entries. Older data captured by `ledgerSummary`. |
| `projectBenchmark` | Persistent quality score from Phase 0. `highWaterMarks`: regression below `HWM - 10` triggers remediation. |
| `prerequisites` | Optional task slug array. Orchestrator auto-defers if unmet. |
| `counterfactual` | Lightweight what-if record on deferred tasks for retrospective accuracy. |
| `failedApproaches` | Structured reasoning: `error`, `reasoning`, `filesAffected`, `cycle`, `alternative`, `errorCategory` (planning/tool-use/reasoning/context/integration), `failedStep` (attributed step). |
| `discoveryVelocity` | Rolling 3-cycle average of proposals generated. `current`: this cycle's count, `history`: per-cycle entries, `rolling3`: 3-cycle average. Used for knowledge-complete convergence detection. |
| `proposals` | Array of discovery proposals from Builder findings. Each: `{title, source, confidence, category, cycle, unsolicited}`. Consumed by Scout for future task generation. |
| `promptVariants` | Prompt evolution experiments from meta-cycle. Each: `{agent, cycle, edit, baselineScore}`. Compared at next meta-cycle for auto-revert. |
| `researchAgenda` | Persistent research questions derived from evaluation signals. `items[]`: individual questions with priority, status, linked capsules and concepts. `capsuleIndex`: maps benchmark dimensions to Knowledge Capsule slugs for gap analysis. |
| `researchLedger` | Strict evaluation record of research-driven changes. `triedConcepts[]`: each concept with WORKS/DOESNT_WORK/INCONCLUSIVE verdict, benchmark before/after, KEEP/DROP decision. `diversityTracker`: prevents over-researching same dimension (max 3 consecutive, coverage counts). |

### `.evolve/notes.md` (shared, append under ship lock)

Cross-cycle context with rolling window. Each entry includes `runId`:
```markdown
## Cycle 8 (run-1710820800000-a3f1) — 2026-03-19
```

Every 5 cycles, entries older than 5 cycles compress into Summary section. Full history in `history/cycle-N/`.

### `.evolve/project-digest.md` (shared)

Project structure digest (~2-3KB). Generated cycle 1, regenerated every 10 cycles. Contains: directory tree, tech stack, conventions, recent git log. Scout reads this on cycle 2+ instead of full codebase scan.

### `.evolve/history/cycle-{N}/`

Archived workspace from each completed cycle.

## Layer 4: Eval State

### `.evolve/evals/<task-slug>.md`

Eval definitions created by Scout. Each defines code graders, regression evals, and acceptance checks. Used by Auditor and eval-runner.

## Layer 5: Instincts

### `.evolve/instincts/personal/`

YAML instinct files from Phase 5 learning pass. Start at confidence 0.5, increase with confirmation.

## Layer 6: Experiment Journal

### `$WORKSPACE_PATH/experiments.jsonl`

Append-only log of every Builder attempt. (Inspired by autoresearch's `results.tsv`.)

```jsonl
{"cycle":1,"task":"add-dark-mode","attempt":1,"verdict":"PASS","approach":"CSS custom properties with prefers-color-scheme","metric":"3/3 eval checks passed"}
{"cycle":2,"task":"fix-auth-bug","attempt":1,"verdict":"FAIL","approach":"patch session middleware","metric":"TypeError: session.destroy is not a function"}
{"cycle":2,"task":"fix-auth-bug","attempt":2,"verdict":"PASS","approach":"replace session middleware with cookie-based auth","metric":"2/2 eval checks passed"}
```

| Rule | Detail |
|------|--------|
| Write timing | Builder appends after each attempt (Phase 2) |
| Consumer | Scout reads to avoid re-proposing failed approaches |
| Truncation | Never — complete experiment history |
| Fields | `cycle`, `task` (slug), `attempt` (1-indexed), `verdict`, `approach` (1-sentence), `metric` |

## Data Flow

```
Phase 0: CALIBRATE (once per invocation)
Orchestrator -- benchmark-eval -> projectBenchmark + benchmark-report.md
              |
Phase 0.5: RESEARCH (every cycle)     v
Orchestrator -- gap analysis + web queries -> research-brief.md + conceptCandidates
              (reads: researchAgenda, researchLedger, benchmarkWeaknesses, proposals)
              (writes: researchAgenda, capsuleIndex, research-brief.md)
              |
Phase 1: DISCOVER              v
Scout --> scout-report.md + evals/<task>.md (reads research-brief + conceptCandidates)
              |
Phase 2: BUILD (per task)      v
Builder --> build-report.md
              |
Phase 3: AUDIT                 v
Auditor --> audit-report.md [GATE: MEDIUM+ blocks]
              |
Delta CHECK: BENCHMARK         v (after all tasks audited)
Orchestrator -- re-run relevant dimension checks [SOFT GATE: regression blocks]
              |
Phase 4: SHIP                  v (only if PASS + delta OK)
Orchestrator -- git commit + push
              |
Phase 5: LEARN                 v
Orchestrator -- instincts + archive
Operator --> operator-log.md (post-cycle, includes benchmark trend)
```

## Instinct Quality Scoring

Extends confidence-based lifecycle with `historicalSuccessRate` (inspired by EvolveR — arxiv 2510.16079).

### Schema Extension (per instinct YAML)

```yaml
historicalSuccessRate:
  applied: 5        # times referenced in a build
  passed: 4         # times build passed audit after applying
  rate: 0.80        # passed / applied
```

### Update Rules

| Phase | Action |
|-------|--------|
| Phase 2 (BUILD) | When Builder references instinct, increment `applied` |
| Phase 4 (SHIP) | After audit verdict, increment `passed` for applied instincts if task passed. Recompute `rate`. |
| Phase 5 (LEARN) | `rate < 0.5` and `applied >= 3` → candidate for retirement. `rate >= 0.8` and `applied >= 3` → +1 selection boost. |

Both `confidence` (pattern validity) and `rate` (practical build impact) inform instinct selection.
