# Configuration

## state.json

The primary configuration file is `.claude/evolve/state.json` in your project directory. It's auto-created on first run.

### Research Cooldown

Web research has a 12-hour cooldown. The Scout reuses cached results if queries haven't expired:

```json
{
  "research": {
    "queries": [
      {
        "query": "react server components patterns",
        "date": "2026-03-13T10:00:00Z",
        "ttlHours": 12
      }
    ]
  }
}
```

### Failed Approaches

When a task fails after 3 Builder attempts, the approach is logged:

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

The Scout reads this to avoid repeating failed approaches.

## Goal Modes

### Autonomous (no goal)

```
/evolve-loop
/evolve-loop 3
```

Scout performs broad discovery and picks highest-impact work.

### Directed (with goal)

```
/evolve-loop 1 add dark mode support
/evolve-loop add user authentication
```

Scout focuses discovery and task selection on the goal.

## Eval Definitions

Eval definitions are created by the Scout and stored in `.claude/evolve/evals/`. You can also pre-create them manually:

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
- All checks: pass@1 = 1.0
```

## Instinct Promotion

After 5+ cycles, instincts with confidence >= 0.8 promote from project-level to global:
- **Project:** `.claude/evolve/instincts/personal/`
- **Global:** `~/.claude/instincts/personal/`
