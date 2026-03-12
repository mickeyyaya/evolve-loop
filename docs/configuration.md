# Configuration

## state.json

The primary configuration file is `.claude/evolve/state.json` in your project directory. It's auto-created on first run.

### Cost Budget

Set a maximum cost per cycle:

```json
{
  "costBudget": 5.00
}
```

The Loop Operator will:
- Flag when projected cost exceeds 120% of average cycle cost
- HALT if remaining budget doesn't allow another cycle

Set to `null` (default) for unlimited.

### Research TTL

Control how often the Researcher re-searches topics:

```json
{
  "research": {
    "queries": [
      {
        "query": "react server components patterns",
        "date": "2026-03-10",
        "ttlDays": 7
      }
    ]
  }
}
```

Default TTL is 7 days. Queries within TTL are skipped.

### Failed Approaches

When a task fails after 3 Developer attempts, the approach is logged:

```json
{
  "failedApproaches": [
    {
      "feature": "WebSocket sync",
      "approach": "Socket.io with Redis",
      "error": "Connection pooling in serverless",
      "alternative": "Consider SSE or polling"
    }
  ]
}
```

The Planner reads this to avoid repeating failed approaches.

## Goal Modes

### Autonomous (no goal)

```
/evolve-loop
/evolve-loop 3
```

All agents perform broad discovery. The PM evaluates all dimensions, the Researcher searches general trends, and the Planner picks highest-impact work.

### Directed (with goal)

```
/evolve-loop 1 add dark mode support
/evolve-loop add user authentication
```

All agents focus on the goal. The PM assesses what's needed for the goal, the Researcher finds relevant patterns, and the Planner selects tasks that advance the goal.

## Eval Definitions

Eval definitions are created by the Planner in Phase 2 and stored in `.claude/evolve/evals/`. You can also pre-create them manually:

```markdown
# Eval: add-auth

## Code Graders
- `npm test -- --grep "auth"`
- `npx tsc --noEmit`

## Regression Evals
- `npm test`

## Acceptance Checks
- `grep -r "export.*authMiddleware" src/`
- `npm run build`

## Thresholds
- Code graders: pass@1 = 1.0
- Regression: pass@1 = 1.0
- Acceptance: pass@1 = 1.0
```

## Instinct Promotion

After 5+ cycles, instincts with confidence >= 0.8 promote from project-level to global:
- **Project:** `.claude/evolve/instincts/personal/`
- **Global:** `~/.claude/homunculus/instincts/personal/`

Global instincts are available to all projects.
