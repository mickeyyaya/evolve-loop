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
    {"slug": "add-feature-y", "decision": "rejected", "reason": "too complex", "revisitAfter": "2026-04-01"}
  ],
  "failedApproaches": [
    {"feature": "real-time sync", "approach": "WebSocket", "error": "serverless incompatible", "alternative": "SSE polling"}
  ],
  "evalHistory": [
    {"cycle": 1, "verdict": "PASS", "checks": 9, "passed": 9, "failed": 0}
  ],
  "instinctCount": 4,
  "operatorWarnings": [],
  "nothingToDoCount": 0,
  "maxCyclesPerSession": 10,
  "warnAfterCycles": 5
}
```

**Rules:**
- Read at the start of every cycle
- Research queries have a 12hr TTL — skip if still fresh
- Rejected tasks have optional `revisitAfter` — skip until date passes
- Failed approaches logged with error context for alternative strategies
- Completed tasks are never re-proposed
- `maxCyclesPerSession` (default 10): hard cap — orchestrator halts if cycle count would exceed this value
- `warnAfterCycles` (default 5): soft threshold — orchestrator warns user when cycle count reaches this value

### `.claude/evolve/notes.md`

Cross-cycle context. Always append, never overwrite.

### `.claude/evolve/history/cycle-{N}/`

Archived workspace from each completed cycle.

## Layer 4: Eval State

### `.claude/evolve/evals/<task-slug>.md`

Eval definitions created by the Scout. Each file defines code graders, regression evals, and acceptance checks. Used by the Auditor and eval-runner.

## Layer 5: Instincts

### `.claude/evolve/instincts/personal/`

Instinct files extracted during Phase 5 learning pass. YAML format with confidence scoring.

Instincts start at confidence 0.5 and increase when confirmed across multiple cycles.

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
