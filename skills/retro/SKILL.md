---
name: retro
description: Use after ship completes. Three sub-reflectors (instinct, gene, failure) run in parallel to extract lessons, update gene pool, and analyze any failures. Off the latency-critical path.
---

# retro

> Sprint 1.3 fan-out + Sprint 3 composable skill (v8.16+). Sub-reflectors merge via dedup-by-title.

## When to invoke

- After `ship` completes (PASS) or after audit FAIL/WARN (capture lessons either way)
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

## Report format

`retrospective-report.md` carries deduplicated `## Lesson:` blocks. Optionally one or more YAML files in `.evolve/instincts/lessons/`.

<!-- GENERATED:phase-facts BEGIN — do not edit; run `evolve skills generate`. Sources: docs/architecture/phase-registry.json · go/internal/phasecontract · .evolve/profiles/retrospective.json -->
## Phase facts

| Fact | Value |
|---|---|
| Phase | `retrospective` (control archetype, optional, gated by `EVOLVE_DISABLE_AUTO_RETROSPECTIVE`) |
| Persona | `agents/evolve-retrospective.md` |
| Profile | `.evolve/profiles/retrospective.json` — CLI `claude-tmux`, tier `deep`, fan-out ×3 |
| Inputs | `audit-report.md` · `build-report.md` |
| Artifact | `retrospective-report.md` (cycle workspace) |

## Output contract

`retrospective-report.md` — no required sections (classified by the phase runner).

<!-- GENERATED:phase-facts END -->

## Composition

Invoked by:
- `/evolve-loop:retro`
- `loop` macro after `/ship`

Fan-out prompts live in `.evolve/profiles/retrospective.json:parallel_subtasks` (count projected into Phase facts above).

## Reference

- `.evolve/profiles/retrospective.json`
- `legacy/scripts/dispatch/aggregator.sh` (phase=learn → lessons mode with dedup)
- `agents/evolve-retrospective.md`
- `skills/loop/phase6-learn.md` (legacy detailed workflow)
