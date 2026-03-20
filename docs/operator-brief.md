# Operator Brief (`next-cycle-brief.json`)

The operator brief is the primary cross-cycle communication channel from the Operator agent to the Scout. Written at the end of each cycle during Phase 5 LEARN, it steers the next cycle's task selection and strategy without requiring the Scout to re-analyze the full pipeline state.

## Schema

```json
{
  "cycle": 22,
  "weakestDimension": "featureCoverage",
  "recommendedStrategy": "innovate",
  "taskTypeBoosts": ["feature", "stability"],
  "avoidAreas": ["phases.md (just refactored)", "Phase 2-3 extraction (deferred)"]
}
```

### Field Reference

| Field | Type | Description |
|-------|------|-------------|
| `cycle` | number | The cycle number when this brief was written |
| `weakestDimension` | string | The benchmark dimension with the lowest composite score, or the dimension showing the most regression. Scout should prioritize tasks that improve this dimension. |
| `recommendedStrategy` | string | One of `balanced`, `innovate`, `harden`, `repair`, `ultrathink`. Overrides the session-level strategy for task selection weighting. |
| `taskTypeBoosts` | string[] | Task types (e.g., `feature`, `stability`) that should receive a +1 priority boost in the Scout's selection ranking. Complements the bandit mechanism. |
| `avoidAreas` | string[] | Files or subsystems the Scout should not target this cycle. Typically recently-refactored areas or deferred work with explicit revisit dates. |

## Storage Locations

The brief is written to two locations for different consumers:

| Location | Purpose | Lifetime |
|----------|---------|----------|
| `$WORKSPACE_PATH/next-cycle-brief.json` | Run-local — consumed by the next cycle within the same invocation | Deleted with run directory after 48 hours |
| `.evolve/latest-brief.json` | Shared — last-writer-wins, consumed by parallel runs or new invocations | Overwritten each cycle by whichever run finishes last |

## Read Priority

The orchestrator checks for the brief before launching Scout (Phase 1):

1. Check `$WORKSPACE_PATH/next-cycle-brief.json` (own run's previous cycle)
2. Fall back to `.evolve/latest-brief.json` (shared, from any run)
3. If neither exists, Scout runs without operator guidance

## Consumers

- **Scout** — reads `taskTypeBoosts` to adjust bandit selection weights, `avoidAreas` to filter candidates, `weakestDimension` to prioritize benchmark-improving tasks, `recommendedStrategy` to override session strategy
- **Orchestrator** — reads `recommendedStrategy` to potentially adjust model routing or audit strictness

## Relationship to Other State

The brief is a lightweight, cycle-scoped recommendation. It does not replace:
- `state.json` — authoritative persistent state (benchmark scores, task history, instincts)
- `notes.md` — human-readable cycle history
- `operator-log.md` — detailed Operator analysis and session narrative

See [architecture.md](architecture.md) § Operator Next-Cycle Brief for the architectural context.
