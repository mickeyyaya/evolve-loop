---
name: evolve-loop/memory-protocol
description: "Shared memory protocol — defines all persistent state layers (JSONL ledger, workspace files, state.json, evals, instincts) and concurrency rules for the evolve-loop pipeline."
---

# Evolve Loop — Shared Memory Protocol

All files live under `.evolve/` in the **project directory** (not your CLI's global config directory).

## Layer 0: Shared Values (Core Rules)

Static team constitution. All agents must follow these rules at all times. Place this section **first** in every agent context block for KV-cache optimization — it never changes between cycles, maximizing cache hit rate.

### Behavioral Rules

- **Immutability** — never mutate shared state directly; always write new versions (new files, new JSON objects, new lines)
- **Scope discipline** — each agent reads/writes only its owned workspace file; do not modify files owned by other agents
- **Blast radius awareness** — prefer S-complexity changes for hotspot files (phases.md, memory-protocol.md, SKILL.md); any edit to a hotspot must touch fewer than 30 lines
- **Learning mandate** — when `instinctsExtracted == 0` for 2+ consecutive cycles, the orchestrator must extract at least one instinct before closing Phase 5

### Parallel Agent Coordination

When concurrent agents share the same workspace, shared values act as the coordination layer:
- Agents must not write to each other's owned files
- The mailbox (`agent-mailbox.md`) is the only shared write surface for cross-agent messages
- All agents always read shared values first to align on core rules before acting

## Concurrency Protocol

When multiple `/evolve-loop` invocations run in parallel, all shared state writes use **optimistic concurrency control (OCC)** via a `version` field in `state.json`.

### OCC Write Protocol

Every `state.json` write follows this sequence:

1. **READ** `state.json`, note `version = V`
2. **Compute mutations** (create a new object — never mutate the read copy)
3. **WRITE** `state.json` with `version = V + 1`
4. **Immediately RE-READ** and verify `version == V + 1`
5. If `version != V + 1` (conflict — another run wrote between steps 1 and 3):
   a. RE-READ the new state
   b. REBASE mutations onto the new state using merge rules below
   c. RETRY with `new version + 1`
   d. Max 3 retries, then HALT with conflict error

### Merge Rules

When a conflict is detected, merge fields as follows:

| Field | Merge Strategy |
|-------|---------------|
| `lastCycleNumber` | MAX of both values |
| `evaluatedTasks` | Union by slug (don't overwrite existing entries) |
| `evalHistory` | Append new cycle entry (keyed by unique cycle number) |
| `fitnessHistory` | Append new entry |
| `taskArms` | Sum `pulls` and `totalReward`, recompute `avgReward` |
| `instinctCount` | MAX |
| `instinctSummary` | Union by id |
| `mastery.consecutiveSuccesses` | Recompute from merged evalHistory tail |
| `research.queries` | Union by query string |
| `stagnation` | MAX of `nothingToDoCount` |
| `projectBenchmark` | First calibrator wins; skip if `lastCalibrated` < 1 hour ago |
| `version` | Always use the latest read value + 1 |

### Run Isolation

Each invocation generates a unique `RUN_ID` (format: `run-<epoch-ms>-<random-4hex>`) and creates its own workspace directory at `.evolve/runs/$RUN_ID/workspace/`. Shared directories (`.evolve/evals/`, `.evolve/instincts/`, `.evolve/history/`) remain shared. Agent context blocks receive `workspacePath` and `runId` so all file reads/writes are scoped to the run directory.

## Layer 1: JSONL Ledger (`.evolve/ledger.jsonl`)

Structured, append-only log. Each agent appends one entry per invocation. Include `runId` for parallel run traceability.

```jsonl
{"ts":"<ISO-8601>","cycle":<N>,"runId":"<RUN_ID>","role":"<role>","type":"<type>","data":{...}}
```

Roles: `scout`, `builder`, `auditor`, `operator`, `eval`

## Layer 2: Markdown Workspace (`$WORKSPACE_PATH`)

Run-scoped workspace (`.evolve/runs/$RUN_ID/workspace/`). Each agent owns exactly one file. The shared `.evolve/workspace/` directory receives a copy of the final workspace after all cycles complete (backward compatibility).

| File | Owner | Contains |
|------|-------|----------|
| `scout-report.md` | Scout | Discovery findings + selected task list |
| `build-report.md` | Builder | Implementation notes per task |
| `builder-notes.md` | Builder | Retrospective: file fragility observations, approach surprises, and recommendations for the next Scout run. Read by Scout in incremental mode as `builderNotes`. Persists across cycles (not cleared between cycles) so Phase 1 of the next cycle can always read the latest notes. |
| `audit-report.md` | Auditor | Single-pass review + eval results |
| `operator-log.md` | Operator | Post-cycle health assessment |
| `session-summary.md` | Operator | Written on the final cycle only. Summarizes total tasks shipped, key features, fitness arc, and a 3-sentence synthesis of the session. |
| `agent-mailbox.md` | All agents | Cross-agent messages for the current cycle. Cleared of non-persistent messages in Phase 4. |

### Agent Mailbox Schema (`workspace/agent-mailbox.md`)

Agents post and read structured messages here to coordinate across phases within a cycle.

```markdown
## Messages

| from | to | type | cycle | persistent | message |
|------|----|------|-------|------------|---------|
| scout | builder | hint | 5 | false | Prefer additive edits in phases.md — file has high blast radius |
| builder | auditor | flag | 5 | false | Eval grader path uses absolute path — verify before running |
| auditor | scout | warn | 5 | true | phases.md Phase 4 is growing; consider splitting next cycle |
```

Fields:
- `from`: sending agent (`scout`, `builder`, `auditor`, `operator`)
- `to`: recipient agent (`scout`, `builder`, `auditor`, `operator`, `all`)
- `type`: `hint` (suggestion), `flag` (needs attention), `warn` (persistent concern), `info` (context)
- `cycle`: cycle number when the message was posted
- `persistent`: `true` = survives Phase 4 cleanup; `false` = cleared each cycle
- `message`: free-text content (keep under 100 chars)

Scout also appends a `decisionTrace` block — a workspace-only field (not persisted to `state.json`) listing all evaluated candidate tasks with their `finalDecision` and `signals` array. Consumed by the Novelty Critic during meta-cycle analysis to detect selection bias.

**Orchestrator-written files** (not agent-owned):
| File | Written by | Contains |
|------|-----------|----------|
| `eval-report.md` | Orchestrator (eval-runner) | Eval gate results if run separately |
| `next-cycle-brief.json` | Operator | Structured guidance for next Scout run: `weakestDimension`, `recommendedStrategy`, `taskTypeBoosts`, `avoidAreas`, `cycle`. Written to both `$WORKSPACE_PATH/` (run-local) and `.evolve/latest-brief.json` (shared, last-writer-wins). Scout reads own run's brief first, falls back to shared `latest-brief.json`. |

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
      {
        "query": "AI agent orchestration best practices 2026",
        "date": "2026-03-13T10:00:00Z",
        "keyFindings": ["finding1", "finding2"],
        "ttlHours": 12
      }
    ]
  },
  "evaluatedTasks": [
    {"slug": "fix-bug-x", "decision": "completed", "cycle": 1},
    {"slug": "add-feature-y", "decision": "rejected", "reason": "too complex", "revisitAfter": "2026-04-01"},
    {
      "slug": "add-feature-z", "decision": "deferred", "cycle": 3,
      "prerequisites": ["fix-bug-x"],
      "counterfactual": {
        "predictedComplexity": "M",
        "estimatedReward": 0.7,
        "alternateApproach": "extend existing config loader instead of new abstraction",
        "deferralReason": "blocked by missing test coverage in config module"
      }
    }
  ],
  "failedApproaches": [
    {
      "feature": "real-time sync",
      "approach": "WebSocket",
      "error": "serverless incompatible",
      "reasoning": "WebSocket requires persistent connections, but the deployment target (Vercel) uses serverless functions with ~10s timeout. The connection drops before any useful data can stream.",
      "filesAffected": ["src/sync/ws-handler.ts", "src/api/stream.ts"],
      "cycle": 3,
      "alternative": "SSE polling or Vercel's AI SDK streaming"
    }
  ],
  "evalHistory": [
    {
      "cycle": 1,
      "verdict": "PASS",
      "checks": 9,
      "passed": 9,
      "failed": 0,
      "delta": {
        "tasksShipped": 3,
        "tasksAttempted": 3,
        "auditIterations": 1.0,
        "successRate": 1.0,
        "instinctsExtracted": 4,
        "stagnationPatterns": 0
      }
    }
  ],
  "instinctCount": 4,
  "operatorWarnings": [],
  "stagnation": {
    "nothingToDoCount": 0,
    "recentPatterns": [
      {
        "type": "same-file-churn|same-error-repeat|diminishing-returns",
        "description": "brief description of the pattern",
        "cycleRange": [3, 5],
        "detectedAt": "2026-03-13T10:00:00Z"
      }
    ]
  },
  "warnAfterCycles": 5,
  "tokenBudget": {
    "perTask": 80000,
    "perCycle": 200000
  },
  "synthesizedTools": [
    {"name": "validate-yaml", "path": ".evolve/tools/validate-yaml.sh", "purpose": "Validate YAML syntax", "cycle": 7, "useCount": 0}
  ],
  "planCache": [
    {
      "slug": "add-dark-mode",
      "taskType": "feature",
      "filePatterns": ["src/**/*.css", "src/components/**/*.tsx"],
      "approach": "Add CSS custom properties for theme, toggle component",
      "steps": ["Define CSS variables", "Create ThemeToggle", "Add localStorage persistence"],
      "cycle": 3,
      "successCount": 2
    }
  ],
  "mastery": {
    "level": "novice|competent|proficient",
    "consecutiveSuccesses": 0
  },
  "processRewards": {
    "discover": 0.0,
    "build": 0.0,
    "audit": 0.0,
    "ship": 0.0,
    "learn": 0.0,
    "skillEfficiency": 0.0
  },
  "ledgerSummary": {
    "totalEntries": 0,
    "cycleRange": [0, 0],
    "scoutRuns": 0,
    "builderRuns": 0,
    "totalTasksShipped": 0,
    "totalTasksFailed": 0,
    "avgTasksPerCycle": 0
  },
  "instinctSummary": [],
  "projectBenchmark": {
    "lastCalibrated": null,
    "calibrationCycle": 0,
    "overall": 0,
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
    "history": [],
    "highWaterMarks": {}
  },
  "processRewardsHistory": [
    {"cycle": 1, "discover": 1.0, "build": 1.0, "audit": 1.0, "ship": 1.0, "learn": 0.8, "skillEfficiency": 1.0}
  ],
  "fitnessScore": 0.0,
  "fitnessHistory": [],
  "fitnessRegression": false
}
```

**Rules:**
- Read at the start of every cycle
- Research queries have a 12hr TTL — skip if still fresh
- Rejected tasks have optional `revisitAfter` — skip until date passes
- Deferred tasks may include a `counterfactual` annotation: `{predictedComplexity, estimatedReward, alternateApproach, deferralReason}`. This lightweight what-if record enables retrospective accuracy checks — did the actual outcome match the prediction when the task was eventually completed?
- Failed approaches logged with structured reasoning: `error` (what happened), `reasoning` (why it failed), `filesAffected` (blast radius), `cycle` (when), `alternative` (what to try instead)
- Completed tasks are never re-proposed
- `prerequisites`: optional array of task slugs that must be `decision: "completed"` before a dependent task is eligible for building. Set when the Scout proposes a task with explicit dependencies. The orchestrator checks this field in Phase 1 and auto-defers any task whose prerequisites are unmet
- `version` (default 0): optimistic concurrency control counter. Incremented on every state.json write. Used to detect parallel write conflicts (see Concurrency Protocol above)
- `lastCycleNumber` (default 0): the last completed cycle number — used for atomic cycle number allocation (parallel-safe)
- `warnAfterCycles` (default 5): soft threshold — orchestrator warns user when requesting this many cycles in a single invocation
- `mastery.level`: difficulty graduation — `novice` (0-2 successes, S only), `competent` (3-5, S+M), `proficient` (6+, S+M+L). Updated in Phase 4 after each successful ship
- `mastery.consecutiveSuccesses`: reset to 0 on any audit failure, incremented on each successful ship
- `processRewards`: latest cycle's per-phase scores (0.0-1.0) for quick access. Updated in Phase 4
- `taskArms`: multi-armed bandit state for task type selection. Tracks per-type reward history so the Scout can bias selection toward historically successful task types. Schema:
  ```json
  "taskArms": {
    "feature":     {"type": "feature",     "pulls": 8, "totalReward": 7, "avgReward": 0.875},
    "stability":   {"type": "stability",   "pulls": 4, "totalReward": 3, "avgReward": 0.75},
    "security":    {"type": "security",    "pulls": 2, "totalReward": 2, "avgReward": 1.0},
    "techdebt":    {"type": "techdebt",    "pulls": 3, "totalReward": 2, "avgReward": 0.667},
    "performance": {"type": "performance", "pulls": 1, "totalReward": 0, "avgReward": 0.0}
  }
  ```
  Fields per arm: `type` (task category), `pulls` (times selected), `totalReward` (cumulative shipped successes), `avgReward` (totalReward / pulls, or 0 if pulls=0). Updated in Phase 4 after each task outcome. Arms with `avgReward >= 0.8` and `pulls >= 3` earn a +1 priority boost in Scout task selection (see SKILL.md Bandit Task Selection).
- `processRewardsHistory`: rolling 3-entry array of per-cycle process rewards for trend detection. Each entry includes `cycle` and all dimension scores. Kept to last 3 entries — older data is in `evalHistory`. Enables distinguishing sustained degradation from one-off dips. Schema:
  ```json
  "processRewardsHistory": [
    {"cycle": 17, "discover": 1.0, "build": 1.0, "audit": 1.0, "ship": 1.0, "learn": 0.8, "skillEfficiency": 1.0},
    {"cycle": 18, "discover": 1.0, "build": 1.0, "audit": 1.0, "ship": 1.0, "learn": 0.8, "skillEfficiency": 1.0}
  ]
  ```
- `pendingImprovements`: array of auto-generated remediation tasks triggered when processRewards dimensions fall below 0.7 for 2+ consecutive cycles. Scout treats these as high-priority task candidates. Schema:
  ```json
  "pendingImprovements": [
    {"dimension": "learn", "score": 0.5, "sustained": true, "suggestedTask": "extract instincts from recent cycles", "cycle": 19, "priority": "high"}
  ]
  ```
- `planCache`: reusable plan templates for recurring task types. Entries include `slug`, `taskType`, `filePatterns`, `approach`, `steps`, `cycle`, `successCount`
- `crossoverLog`: list of crossover-generated task slugs with lineage and outcome. Schema:
  ```json
  "crossoverLog": [
    {"slug": "add-themed-sync", "parents": ["add-dark-mode", "add-real-time-sync"], "cycle": 5, "selected": true, "outcome": "PASS|FAIL|deferred|null"}
  ]
  ```
  Fields: `slug` (offspring task slug), `parents` (array of two parent planCache slugs), `cycle` (cycle proposed), `selected` (whether it was chosen for building), `outcome` (result if built, else `null`)
- `synthesizedTools`: tools generated by Builder capability gap detection. Entries include `name`, `path`, `purpose`, `cycle`, `useCount`
- `fileExplorationMap`: rolling map of `{filePath: lastTouchedCycle}` tracking which files were modified each cycle. Updated in Phase 4 after each successful ship. Entries older than 10 cycles are pruned. Used by Scout to compute novelty scores — files with `lastTouchedCycle <= currentCycle - 3` are considered under-explored.
- `ledgerSummary`: aggregated stats from ledger.jsonl so agents never need to read the full ledger. Updated in Phase 4. Contains `totalEntries`, `cycleRange`, `scoutRuns`, `builderRuns`, `totalTasksShipped`, `totalTasksFailed`, `avgTasksPerCycle`
- `instinctSummary`: compact array of all active instincts with `id`, `pattern`, `confidence`, `type`, and optional `graduated` flag. Updated in Phase 5 after instinct extraction. Agents read this instead of all instinct YAML files
- `fitnessScore`: composite score (0.0-1.0) computed as weighted average of processRewards: `0.25*discover + 0.30*build + 0.20*audit + 0.15*ship + 0.10*learn`. Updated in Phase 4. A single "did the project get better?" signal inspired by autoresearch's monotonic fitness gate
- `fitnessHistory`: rolling array of last 3 fitnessScores for trend detection. Schema: `[{"cycle": N, "score": 0.85}, ...]`
- `fitnessRegression`: boolean flag set to `true` when fitnessScore decreases for 2 consecutive cycles. Operator reads this as a HALT-worthy signal
- `evalHistory` is trimmed to the last 5 entries in state.json — older data is captured by `ledgerSummary`
- `projectBenchmark`: persistent project-level quality score computed during Phase 0 (CALIBRATE). Contains `lastCalibrated` (ISO timestamp), `calibrationCycle` (cycle number when last calibrated), `overall` (0-100 average), `dimensions` (per-dimension `{automated, llm, composite}` scores), `history` (last 5 calibration snapshots for trend analysis), and `highWaterMarks` (per-dimension highest composite score — once a dimension hits 80+, regression below `HWM - 10` triggers mandatory remediation). Phase 0 runs once per invocation, not per cycle. The delta check between Phase 3 and Phase 4 uses `projectBenchmark.dimensions` as the baseline for regression detection

### `.evolve/notes.md` (shared, append under ship lock)

Cross-cycle context with a rolling window structure. Appends happen during Phase 4 under the ship lock, so no concurrent write risk. Each cycle entry includes the `runId` in its header for traceability:

```markdown
## Cycle 8 (run-1710820800000-a3f1) — 2026-03-19
```

Rolling window structure:

```markdown
# Evolve Loop Cross-Cycle Notes

## Summary (cycles 1 through N-5)
<rewritten fixed-size paragraph, ~500 bytes: total tasks, key milestones, active deferred items>

## Recent Cycles
<full detail for last 5 cycles only>
```

Every 5 cycles (aligned with meta-cycle), entries older than 5 cycles are compressed into the Summary section. Full history is preserved in `history/cycle-N/` archives.

### `.evolve/project-digest.md` (shared)

Project structure digest (~2-3KB) generated on cycle 1 and regenerated every 10 cycles. Contains: directory tree with file sizes, tech stack, conventions, and recent git log. Scout reads this on cycle 2+ instead of full codebase scan. Stored at the shared `.evolve/` level (read-mostly cache). Any run can refresh it; stale reads are acceptable since it's a cache.

### `.evolve/history/cycle-{N}/`

Archived workspace from each completed cycle.

## Layer 4: Eval State

### `.evolve/evals/<task-slug>.md`

Eval definitions created by the Scout. Each file defines code graders, regression evals, and acceptance checks. Used by the Auditor and eval-runner.

## Layer 5: Instincts

### `.evolve/instincts/personal/`

Instinct files extracted during Phase 5 learning pass. YAML format with confidence scoring.

Instincts start at confidence 0.5 and increase when confirmed across multiple cycles.

## Layer 6: Experiment Journal

### `$WORKSPACE_PATH/experiments.jsonl`

Append-only log of every Builder attempt (pass or fail). Inspired by autoresearch's `results.tsv` — logs ALL experiments, not just successes.

```jsonl
{"cycle":1,"task":"add-dark-mode","attempt":1,"verdict":"PASS","approach":"CSS custom properties with prefers-color-scheme","metric":"3/3 eval checks passed"}
{"cycle":2,"task":"fix-auth-bug","attempt":1,"verdict":"FAIL","approach":"patch session middleware","metric":"TypeError: session.destroy is not a function"}
{"cycle":2,"task":"fix-auth-bug","attempt":2,"verdict":"PASS","approach":"replace session middleware with cookie-based auth","metric":"2/2 eval checks passed"}
```

**Rules:**
- Builder appends one entry after each attempt (Phase 2)
- Scout reads this log to avoid re-proposing approaches that already failed
- Never truncated — serves as complete experiment history
- Fields: `cycle`, `task` (slug), `attempt` (1-indexed), `verdict` (PASS/FAIL), `approach` (1-sentence summary), `metric` (eval result or error message)

## Data Flow

```
Phase 0: CALIBRATE (once per invocation)
Orchestrator ── benchmark-eval → projectBenchmark + benchmark-report.md
              |
Phase 1: DISCOVER              v
Scout ──→ scout-report.md + evals/<task>.md (reads benchmarkWeaknesses)
              |
Phase 2: BUILD (per task)      v
Builder ──→ build-report.md
              |
Phase 3: AUDIT                 v
Auditor ──→ audit-report.md [GATE: MEDIUM+ blocks]
              |
Δ CHECK: BENCHMARK DELTA       v (after all tasks audited)
Orchestrator ── re-run relevant dimension checks [SOFT GATE: regression blocks]
              |
Phase 4: SHIP                  v (only if PASS + delta OK)
Orchestrator ── git commit + push
              |
Phase 5: LEARN                 v
Orchestrator ── instincts + archive
Operator ──→ operator-log.md (post-cycle, includes benchmark trend)
```

## Instinct Quality Scoring

Extends the confidence-based instinct lifecycle (see `docs/self-learning.md`) with a dynamic `historicalSuccessRate` field inspired by EvolveR experience curation (arxiv 2510.16079). While `confidence` tracks belief strength via confirmation/contradiction, `historicalSuccessRate` tracks empirical build outcomes when the instinct was applied.

### Schema Extension (per instinct YAML)

```yaml
historicalSuccessRate:
  applied: 5        # times instinct was referenced in a build
  passed: 4         # times the build passed audit after applying this instinct
  rate: 0.80        # passed / applied (0.0 if applied == 0)
```

### Update Rules

- **Phase 2 (BUILD):** When Builder references an instinct, increment `applied` for that instinct.
- **Phase 4 (SHIP):** After audit verdict, increment `passed` for each applied instinct if the task passed. Recompute `rate = passed / applied`.
- **Phase 5 (LEARN):** Instincts with `rate < 0.5` and `applied >= 3` are candidates for retirement or revision. Instincts with `rate >= 0.8` and `applied >= 3` earn a selection boost (+1 priority when Scout picks instincts to apply).
- Both `confidence` and `historicalSuccessRate` inform instinct selection: `confidence` reflects pattern validity; `rate` reflects practical build impact.
