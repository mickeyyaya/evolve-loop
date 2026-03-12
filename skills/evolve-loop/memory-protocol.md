# Evolve Loop — Shared Memory Protocol

All files live under `.claude/evolve/` in the **project directory** (not `~/.claude/`).

## Layer 1: JSONL Ledger (`.claude/evolve/ledger.jsonl`)

Structured, append-only, agent-to-agent communication. Each agent appends one entry per phase.

```jsonl
{"ts":"<ISO-8601>","cycle":<N>,"role":"<role>","type":"<type>","data":{...}}
```

Roles: `operator`, `pm`, `researcher`, `scanner`, `planner`, `architect`, `developer`, `reviewer`, `e2e-runner`, `security`, `eval`, `deployer`

## Layer 2: Markdown Workspace (`.claude/evolve/workspace/`)

Human-readable, overwritten each cycle. Each agent owns exactly one file:

| File | Owner | Contains |
|------|-------|----------|
| `loop-operator-log.md` | Operator | Pre-flight, checkpoint, post-cycle logs |
| `briefing.md` | PM | Internal project assessment (8 dimensions) |
| `research-report.md` | Researcher | External intelligence + ranked recommendations |
| `scan-report.md` | Scanner | Code quality, tech debt, hotspots, coverage |
| `backlog.md` | Planner | Prioritized tasks with acceptance criteria |
| `design.md` | Architect | Implementation spec, interfaces, tradeoffs, ADRs |
| `impl-notes.md` | Developer | What was built, TDD log, coverage stats |
| `review-report.md` | Reviewer | Code quality findings (READ-ONLY review) |
| `e2e-report.md` | E2E Runner | E2E results, acceptance verification |
| `security-report.md` | Security | Security scan, OWASP checks, dependency audit |
| `eval-report.md` | Eval Runner | Eval gate results (PASS/FAIL) |
| `deploy-log.md` | Deployer | PR link, merge status, CI result |

## Layer 3: Persistent State

### `.claude/evolve/state.json`

Cycle memory — avoids repeating searches, re-evaluating rejected tasks, or retrying failed approaches.

```json
{
  "lastUpdated": "2026-03-12T10:00:00Z",
  "costBudget": null,
  "research": {
    "queries": [
      {
        "query": "kids math game gamification trends 2026",
        "date": "2026-03-10",
        "keyFindings": ["spaced repetition trending", "AI tutors growing"],
        "ttlDays": 7
      }
    ]
  },
  "evaluatedTasks": [
    {
      "task": "Add multiplayer mode",
      "date": "2026-03-10",
      "decision": "rejected",
      "reason": "Too complex for current architecture",
      "revisitAfter": "2026-04-01"
    },
    {
      "task": "Add sound effects",
      "date": "2026-03-09",
      "decision": "completed",
      "cycle": 3
    }
  ],
  "failedApproaches": [
    {
      "feature": "WebSocket real-time sync",
      "date": "2026-03-10",
      "approach": "Socket.io with Redis pub/sub",
      "error": "Redis connection pooling issues in serverless",
      "alternative": "Consider SSE or polling instead"
    }
  ],
  "evalHistory": [],
  "instinctCount": 0,
  "operatorWarnings": [],
  "nothingToDoCount": 0
}
```

**Rules:**
- Read at the start of every cycle
- Research queries have a TTL (default 7 days) — skip if still fresh
- Rejected tasks have an optional `revisitAfter` date — skip until that date passes
- Failed approaches are logged with error context for alternative strategies
- Completed tasks are never re-proposed
- Write updates after Phase 1 (research), Phase 2 (decisions), Phase 4 (outcomes), Phase 5.5 (eval results), Phase 7 (wrap-up)

### `.claude/evolve/notes.md`

Cross-iteration context — bridges between cycles/sessions. Always append, never overwrite.

```markdown
## Cycle N — <date>
- **Task:** <what was built>
- **Coverage:** <X%>
- **Review:** <verdict>
- **E2E:** <verdict>
- **Security:** <verdict>
- **Eval:** <verdict>
- **Research:** <key findings>
- **Warnings deferred:** <tech debt items>
- **Instincts extracted:** <count>
- **Next cycle should consider:** <recommendations>
```

### `.claude/evolve/history/cycle-{N}/`

Archived workspace from each completed cycle. Copied from `workspace/` at the end of Phase 7.

## Layer 4: Eval State

### `.claude/evolve/evals/<task-name>.md`

Eval definitions created by the Planner in Phase 2. Each file defines code graders, regression evals, and acceptance checks for a task. Used by the Eval Runner in Phase 5.5.

### `.claude/evolve/evals/baseline.json` (optional)

Baseline eval results for regression comparison across cycles.

## Layer 5: Instincts

### `.claude/evolve/instincts/personal/`

Instinct files extracted during Phase 7 learning pass. YAML format with confidence scoring.

After 5+ cycles, instincts with confidence >= 0.8 promote to `~/.claude/homunculus/instincts/personal/` (global scope).

## Agent Data Flow

```
                    PHASE 0: Loop Operator → loop-operator-log.md (pre-flight)
                              |
                    PHASE 1: DISCOVER (3 parallel)
PM ─────────→ briefing.md ──────────┐
Researcher ──→ research-report.md ──┤
Scanner ─────→ scan-report.md ──────┘
                                     |
                    PHASE 2: PLAN    v
                    Planner → backlog.md + evals/<task>.md
                                     |
                    PHASE 3: DESIGN  v
                    Architect → design.md
                                     |
                    PHASE 4: BUILD   v
                    Developer → impl-notes.md
                                     |
                    PHASE 4.5: Loop Operator → checkpoint
                                     |
                    PHASE 5: VERIFY (3 parallel)
Reviewer ────→ review-report.md ─────────┐
E2E Runner ──→ e2e-report.md ────────────┤
Security ────→ security-report.md ───────┘
                                          |
                    PHASE 5.5: EVAL       v
                    Eval Runner → eval-report.md [HARD GATE]
                                          |
                    PHASE 6: SHIP         v (only if PASS)
                    Deployer → deploy-log.md
                                          |
                    PHASE 7: LOOP+LEARN   v
                    Archive + Instincts + Operator post-cycle
```
