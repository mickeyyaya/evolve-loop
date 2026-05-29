# Retired regression predicates — superseded targets (obsolete, not deleted)

> These predicates differ from the `_retired-v12/` set: they reference **live** files (agent `.md`s, profile JSON), but assert a numeric target that was **deliberately superseded by a later, documented decision** — or was a one-shot refactor milestone never meant to be a standing invariant. They are retired (moved here, **not deleted**) because making them GREEN would require *regressing* a config value that was intentionally raised, which is the opposite of correctness.
>
> Each retirement below is backed by git-evidence. The ACS suite globs `acs/regression-suite/cycle-*/`; moving a predicate here excludes it from the live suite while preserving it for audit.

## Retired predicates (with evidence)

| Predicate | Asserts | Current | Why retired (evidence) |
|---|---|---|---|
| `cycle-75/001-orchestrator-phase-loop-reduction.sh` | `agents/evolve-orchestrator.md` ≤ 291 lines | 329 | **One-shot refactor milestone.** AC1 was the cycle-75 *deliverable* ("baseline 341 → remove ≥50 → ≤291"), satisfied at the time. The file legitimately re-grew to 329 as the orchestrator gained v12 Go-port content + status banners (`69626c6`). A line-count deliverable is not a permanent invariant; re-growth is not a regression. |
| `cycle-78/001-retrospective-stage9-cold-move.sh` | AC7: `scout.json` `max_turns` == 30 | 42 | **Deliberately superseded.** `08bdfc2` — *"chore(cycle-102): raise 4 agent profile max_turns ceilings; add turn-overrun and ship-refused incident docs"* — intentionally raised the scout ceiling 30→42 (the old ceiling starved scout). (The retrospective.md line-count AC in this predicate actually still passes: 208 ≤ 253; only the stale scout-ceiling AC7 fails.) |
| `cycle-96/001-builder-stop-criterion-turn18.sh` | `builder.json` `max_turns` == 25 | 36 | **Deliberately superseded** by the same `08bdfc2` (cycle-102) ceiling raise (25→36). Lowering it back to 25 would re-starve the builder — empirically confirmed when a low turn ceiling caused the cycle-145 builder to fail. The config is correct; the assertion is stale. |

Reproduce the evidence:

```bash
git show 08bdfc2 --stat | head        # the cycle-102 ceiling-raise commit
jq '.max_turns' .evolve/profiles/scout.json    # -> 42 (was 30)
jq '.max_turns' .evolve/profiles/builder.json  # -> 36 (was 25)
```

## Distinction from `_retired-v12/`

| | `_retired-v12/` | `_retired-obsolete/` (here) |
|---|---|---|
| Failure cause | invokes a deleted v12 bash script (unsatisfiable) | asserts a deliberately-superseded numeric target |
| Referenced file | a `scripts/*.sh` that no longer exists | a file that still exists |
| Correct action | port behaviour to a Go test if uncovered | leave config as-is; the target moved on purpose |

## Reinstatement

Reinstate only if the superseded decision is itself reverted (e.g. ceilings lowered back). In that case, update the asserted target to the new intended value before moving the predicate back under `acs/regression-suite/cycle-<N>/` — do not reinstate a predicate that pins a value the project has deliberately moved away from.
