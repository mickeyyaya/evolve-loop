# Evolve Loop — Shared Memory Protocol

All files live under `.claude/evolve/` in the **project directory** (not `~/.claude/`).

## Layer 1: JSONL Ledger (`.claude/evolve/ledger.jsonl`)

Structured, append-only log. Each agent appends one entry per invocation.

```jsonl
{"ts":"<ISO-8601>","cycle":<N>,"role":"<role>","type":"<type>","data":{...}}
```

Roles: `scout`, `builder`, `auditor`, `operator`, `eval`

## Layer 2: Markdown Workspace (`.claude/evolve/workspace/`)

Overwritten each cycle. Each agent owns exactly one file:

| File | Owner | Contains |
|------|-------|----------|
| `scout-report.md` | Scout | Discovery findings + selected task list |
| `build-report.md` | Builder | Implementation notes per task |
| `audit-report.md` | Auditor | Single-pass review + eval results |
| `operator-log.md` | Operator | Post-cycle health assessment |

**Orchestrator-written files** (not agent-owned):
| File | Written by | Contains |
|------|-----------|----------|
| `eval-report.md` | Orchestrator (eval-runner) | Eval gate results if run separately |

## Layer 3: Persistent State

### `.claude/evolve/state.json`

Cycle memory — avoids repeating searches, re-evaluating rejected tasks, or retrying failed approaches.

```json
{
  "lastUpdated": "2026-03-13T10:00:00Z",
  "lastCycleNumber": 0,
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
  "maxCyclesPerSession": 10,
  "warnAfterCycles": 5,
  "tokenBudget": {
    "perTask": 80000,
    "perCycle": 200000
  },
  "synthesizedTools": [
    {"name": "validate-yaml", "path": ".claude/evolve/tools/validate-yaml.sh", "purpose": "Validate YAML syntax", "cycle": 7, "useCount": 0}
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
    "learn": 0.0
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
  "instinctSummary": []
}
```

**Rules:**
- Read at the start of every cycle
- Research queries have a 12hr TTL — skip if still fresh
- Rejected tasks have optional `revisitAfter` — skip until date passes
- Deferred tasks may include a `counterfactual` annotation: `{predictedComplexity, estimatedReward, alternateApproach, deferralReason}`. This lightweight what-if record enables retrospective accuracy checks — did the actual outcome match the prediction when the task was eventually completed?
- Failed approaches logged with structured reasoning: `error` (what happened), `reasoning` (why it failed), `filesAffected` (blast radius), `cycle` (when), `alternative` (what to try instead)
- Completed tasks are never re-proposed
- `lastCycleNumber` (default 0): the last completed cycle number — used to compute the start of the next invocation (additive cycling)
- `maxCyclesPerSession` (default 10): hard cap — orchestrator halts if cumulative cycle number would exceed this value
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
- `synthesizedTools`: tools generated by Builder capability gap detection. Entries include `name`, `path`, `purpose`, `cycle`, `useCount`
- `ledgerSummary`: aggregated stats from ledger.jsonl so agents never need to read the full ledger. Updated in Phase 4. Contains `totalEntries`, `cycleRange`, `scoutRuns`, `builderRuns`, `totalTasksShipped`, `totalTasksFailed`, `avgTasksPerCycle`
- `instinctSummary`: compact array of all active instincts with `id`, `pattern`, `confidence`, `type`, and optional `graduated` flag. Updated in Phase 5 after instinct extraction. Agents read this instead of all instinct YAML files
- `fitnessScore`: composite score (0.0-1.0) computed as weighted average of processRewards: `0.25*discover + 0.30*build + 0.20*audit + 0.15*ship + 0.10*learn`. Updated in Phase 4. A single "did the project get better?" signal inspired by autoresearch's monotonic fitness gate
- `fitnessHistory`: rolling array of last 3 fitnessScores for trend detection. Schema: `[{"cycle": N, "score": 0.85}, ...]`
- `fitnessRegression`: boolean flag set to `true` when fitnessScore decreases for 2 consecutive cycles. Operator reads this as a HALT-worthy signal
- `evalHistory` is trimmed to the last 5 entries in state.json — older data is captured by `ledgerSummary`

### `.claude/evolve/notes.md`

Cross-cycle context with a rolling window structure:

```markdown
# Evolve Loop Cross-Cycle Notes

## Summary (cycles 1 through N-5)
<rewritten fixed-size paragraph, ~500 bytes: total tasks, key milestones, active deferred items>

## Recent Cycles
<full detail for last 5 cycles only>
```

Every 5 cycles (aligned with meta-cycle), entries older than 5 cycles are compressed into the Summary section. Full history is preserved in `history/cycle-N/` archives.

### `.claude/evolve/workspace/project-digest.md`

Project structure digest (~2-3KB) generated on cycle 1 and regenerated every 10 cycles. Contains: directory tree with file sizes, tech stack, conventions, and recent git log. Scout reads this on cycle 2+ instead of full codebase scan.

### `.claude/evolve/history/cycle-{N}/`

Archived workspace from each completed cycle.

## Layer 4: Eval State

### `.claude/evolve/evals/<task-slug>.md`

Eval definitions created by the Scout. Each file defines code graders, regression evals, and acceptance checks. Used by the Auditor and eval-runner.

## Layer 5: Instincts

### `.claude/evolve/instincts/personal/`

Instinct files extracted during Phase 5 learning pass. YAML format with confidence scoring.

Instincts start at confidence 0.5 and increase when confirmed across multiple cycles.

## Layer 6: Experiment Journal

### `.claude/evolve/workspace/experiments.jsonl`

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
Phase 1: DISCOVER
Scout ──→ scout-report.md + evals/<task>.md
              |
Phase 2: BUILD (per task)    v
Builder ──→ build-report.md
              |
Phase 3: AUDIT               v
Auditor ──→ audit-report.md [GATE: MEDIUM+ blocks]
              |
Phase 4: SHIP                v (only if PASS)
Orchestrator ── git commit + push
              |
Phase 5: LEARN               v
Orchestrator ── instincts + archive
Operator ──→ operator-log.md (post-cycle)
```
