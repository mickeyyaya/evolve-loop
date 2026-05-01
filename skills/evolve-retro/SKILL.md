---
name: evolve-retro
description: Use after evolve-ship completes. Three sub-reflectors (instinct, gene, failure) run in parallel to extract lessons, update gene pool, and analyze any failures. Off the latency-critical path.
---

# evolve-retro

> Sprint 1.3 fan-out + Sprint 3 composable skill (v8.16+). Sub-reflectors merge via dedup-by-title.

## When to invoke

- After `evolve-ship` completes (PASS) or after audit FAIL/WARN (capture lessons either way)
- Cycle is in `ship` or `audit` phase, transitioning to `learn`

## When NOT to invoke

- Cycle aborted before producing any artifacts
- Pure-documentation cycles (no behavioral changes to reflect on)

## Workflow

| Step | Action | Exit criteria |
|---|---|---|
| 1 | Read build-report.md, audit-report.md, state.json | Inputs loaded |
| 2 | Dispatch 3 sub-reflectors in parallel | 3 worker artifacts |
| 3 | Aggregator dedups `## Lesson:` blocks across workers | `<workspace>/retrospective-report.md` produced |
| 4 | Optionally write to `.evolve/instincts/lessons/<slug>.yaml` | Lessons persisted |

## The three sub-reflectors

| Sub-reflector | Focus | Output style |
|---|---|---|
| `retro-instinct` | Reusable patterns across cycles | `## Lesson: <pattern title>` |
| `retro-gene` | Gene-pool updates based on cycle outcome | `## Lesson: <gene insight>` |
| `retro-failure` | Failed approaches with reproduction steps | `## Lesson: <failure pattern>` |

## Output contract

`<workspace>/retrospective-report.md` with deduplicated `## Lesson:` blocks. Optionally one or more YAML files in `.evolve/instincts/lessons/`.

## Composition

Invoked by:
- `/retro` slash command
- `evolve-loop` macro after `/ship`

Fan-out controlled by `.evolve/profiles/retrospective.json:parallel_subtasks` (3 entries).

## Reference

- `.evolve/profiles/retrospective.json`
- `scripts/aggregator.sh` (phase=learn → lessons mode with dedup)
- `agents/evolve-retrospective.md`
- `skills/evolve-loop/phase6-learn.md` (legacy detailed workflow)
