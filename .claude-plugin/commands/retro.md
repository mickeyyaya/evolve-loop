---
description: Three sub-reflectors (instinct, gene, failure) run in parallel to extract lessons and update gene pool.
---

# /retro

Run the Retrospective phase against the just-shipped cycle. Spawns three sub-reflectors in parallel:
- `retro-instinct` — extracts reusable patterns across cycles
- `retro-gene` — proposes gene-pool updates
- `retro-failure` — analyzes failedApproaches

Aggregator dedups `## Lesson:` blocks across workers (by title).

## When to use

- After `/ship` completes successfully
- After audit FAIL/WARN (capture lessons even on failed cycles)

## Execution

```bash
bash scripts/dispatch/subagent-run.sh dispatch-parallel retrospective <cycle> <workspace>
```

## Output

- `<workspace>/retrospective-report.md` (deduplicated lesson blocks)
- Optionally `.evolve/instincts/lessons/<slug>.yaml` (when a lesson is durable enough to promote)

## See also

- `skills/evolve-retro/SKILL.md`
- `agents/evolve-retrospective.md`
- `.evolve/profiles/retrospective.json`
