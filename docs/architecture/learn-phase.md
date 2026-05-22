# Learn Phase — Formalized Umbrella

> Introduced v10.20.0 alongside the [Reflection Journal](reflection-journal.md).
> Formalizes three previously-floating post-cycle agents under one named phase:
> Reflector (every cycle) + Retrospective (FAIL/WARN) + Memo (PASS).

## Why formalize

Before v10.20.0, three "learning" agents existed but none were formal phases:

| Agent | Trigger | Output |
|-------|---------|--------|
| `evolve-retrospective` | FAIL/WARN audit verdict | `retrospective-report.md` + lesson YAMLs |
| `evolve-memo` | PASS audit verdict (v8.57.0+) | `memo.md` + carryoverTodos |
| `evolve-reflector` | *(new in v10.20.0)* every cycle | `learn/reflector-synthesis.md` |

Treating these as ad-hoc post-cycle agents created two problems:

1. **No symmetry with the other phases.** Scout/TDD/Build/Audit/Ship were formal phases gated by `phase-gate.sh`. The learning surface was an after-thought — invoked by `run-cycle.sh` directly, with no shared lifecycle hook.

2. **Bifurcated by verdict.** Retrospective fired on FAIL/WARN; memo fired on PASS. Operators had to look in different files depending on outcome to find "what should we learn from this cycle." There was no "always-on" learning surface.

The Reflection Journal feature added the reflector — and that prompted formalizing all three into a single named **Learn phase** that runs after Ship on every cycle.

## Lifecycle

```
... → Audit → Ship → LEARN ─────────────────────────────────┐
                       │                                    │
                       ├── (always) evolve-reflector        │
                       │     reads <phase>-reflection.yaml  │
                       │     calls aggregate-reflections.sh │
                       │     writes learn/reflector-        │
                       │             synthesis.md           │
                       │                                    │
                       ├── verdict == FAIL/WARN             │
                       │     → evolve-retrospective         │
                       │       reads reflector-synthesis +  │
                       │             prior artifacts        │
                       │       writes retrospective-        │
                       │             report.md +            │
                       │             lesson YAMLs           │
                       │                                    │
                       └── verdict == PASS                  │
                             → evolve-memo                  │
                               reads reflector-synthesis +  │
                                     scout/triage           │
                               writes memo.md +             │
                                     carryoverTodos         │
                                                            │
                                                            ▼
                                                       cycle complete
```

The reflector always runs first; retrospective/memo then consume its synthesis to avoid re-aggregating.

## Single-writer invariant

Each sub-agent writes a distinct artifact. No two Learn-phase agents touch the same file:

| Agent | Writes (only) |
|-------|---------------|
| Reflector | `learn/reflector-synthesis.md` |
| Retrospective | `retrospective-report.md`, `handoff-retrospective.json`, `lessons-digest.md`, `.evolve/instincts/lessons/*.yaml`, `carryover-todos.json` (FAIL/WARN paths only) |
| Memo | `memo.md`, `carryover-todos.json` (PASS path only) |

The `carryover-todos.json` ownership is verdict-conditional: retrospective owns it on FAIL/WARN, memo owns it on PASS. They never both write in the same cycle (the verdict determines which runs).

## What the Learn phase does NOT do

- **Not a recovery phase.** If a cycle fails to ship (e.g., ship-gate denial), Learn does not auto-retry. Recovery is the operator's responsibility via `--resume` or manual intervention.
- **Not gated by phase-gate enforcement of write paths.** Each agent has its own profile-level allowlist (`.evolve/profiles/{reflector,retrospective,memo}.json`). `phase-gate.sh` does check for `reflector-synthesis.md` presence in the enforce stage of the Reflection Journal rollout (v10.21.0+), but the learn phase as a whole is not write-path-gated.
- **Not a replacement for `meta-cycle` analysis.** The longer-horizon meta-cycle (every 5 cycles per existing architecture) still owns instinct graduation and cross-cycle effectiveness review. Learn-phase aggregation is cycle-scoped + 5-cycle-window via the aggregator, not full meta-analysis.

## Operator surface

| Question | Where to look |
|----------|---------------|
| What did each phase find hard this cycle? | `.evolve/runs/cycle-N/learn/reflector-synthesis.md` → "This-Cycle Per-Phase Reflections" |
| What's the pipeline-level pattern over the last 5 cycles? | Same file → "Cross-Cycle Rollup" + "Top Pipeline-Level Patterns" |
| What lessons were extracted from this cycle (FAIL/WARN only)? | `.evolve/runs/cycle-N/retrospective-report.md` + `.evolve/instincts/lessons/*.yaml` |
| What's deferred to next cycle? | `.evolve/runs/cycle-N/carryover-todos.json` |
| Recurring hot-spots at a glance? | `bash scripts/observability/dashboard.sh` → "Recent reflection hot-spots" line |

## Cross-references

- [reflection-journal.md](reflection-journal.md) — feature design and rollout
- [retrospective-pipeline.md](retrospective-pipeline.md) — existing lesson persistence contract
- [agents/evolve-reflector.md](../../agents/evolve-reflector.md), [agents/evolve-retrospective.md](../../agents/evolve-retrospective.md), [agents/evolve-memo.md](../../agents/evolve-memo.md) — sub-agent personas
- [phase-tracker.md](phase-tracker.md) — the timing/cost numbers each reflection cites
