---
description: Run the Scout phase — fan out into codebase, research, and eval-design sub-scouts; merge into scout-report.md.
---

# /scout

Run a single Scout phase against the current cycle. Spawns three sub-scouts in parallel:
- `scout-codebase` — repo analysis (git diff, file reads, dependency graph)
- `scout-research` — web research for prior art (when allowed)
- `scout-evals` — eval definition design with mutation-test specs

Aggregator merges the three worker reports into `<workspace>/scout-report.md`.

## When to use

- Standalone discovery for a one-off task (without committing to a full `/loop`)
- Re-running Scout after a `/plan-review` REVISE verdict
- Investigating a goal before deciding whether to launch a cycle

## Execution

```bash
bash scripts/subagent-run.sh dispatch-parallel scout <cycle> <workspace>
```

Default fan-out: 3 workers. Override with `EVOLVE_FANOUT_CONCURRENCY` (cap) or `EVOLVE_TASK_MODE=research` (boost per-worker budget).

## Disable fan-out

Set `EVOLVE_FANOUT_ENABLED=0` (or unset `EVOLVE_FANOUT_SCOUT`) to fall back to single-scout monolithic invocation via `subagent-run.sh scout <cycle> <workspace>`.

## See also

- `skills/evolve-spec/SKILL.md` (workflow)
- `agents/evolve-scout.md` (persona)
- `.evolve/profiles/scout.json` (parallel_subtasks config)
- `docs/architecture/tri-layer.md`
