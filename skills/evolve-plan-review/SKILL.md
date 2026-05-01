---
name: evolve-plan-review
description: Use when scout-report.md exists and TDD/Build hasn't started yet. Runs four lenses (CEO, Eng, Design, Security) in parallel on the task list and produces a verdict (PROCEED, REVISE, ABORT) before code is written. Catches misaligned plans before they cost cycles.
---

# evolve-plan-review

> Sprint 2 multi-lens review (v8.16+). Inspired by `garrytan/gstack/autoplan`. Default-off via `EVOLVE_PLAN_REVIEW=0`; enable per cycle.

## When to invoke

- After Scout writes `<workspace>/scout-report.md`, before TDD/Build
- Whenever the cycle goal mentions architecture, sandbox, or kernel changes
- When the previous cycle aborted due to misaligned scope

## When NOT to invoke

- Cycle is a documented retry of a previously-approved plan
- Single-line fix or trivial bug
- Eval-only cycles (no code change)

## Workflow

| Step | Action | Exit criteria |
|---|---|---|
| 1 | Verify `<workspace>/scout-report.md` exists + fresh | scout-report.md valid |
| 2 | Dispatch 4 lenses in parallel via `subagent-run.sh dispatch-parallel plan-reviewer` | 4 worker artifacts |
| 3 | Aggregator computes verdict (PROCEED/REVISE/ABORT) | `<workspace>/plan-review.md` present, first line is `Verdict: <X>` |
| 4 | Phase gate `gate_plan_review_to_tdd` enforces verdict | Gate passes only on PROCEED |

## Verdict semantics

| Verdict | Trigger | Orchestrator action |
|---|---|---|
| `PROCEED` | Avg score ≥ 7 AND no lens < 5 | Advance to TDD |
| `REVISE` | Avg ≥ 5 AND any lens < 5 | Re-run Scout (max 2 retries) |
| `ABORT` | Any lens explicit ABORT, OR avg < 5 | End cycle |

## Output contract

`<workspace>/plan-review.md` with first line `Verdict: <X>`, second `Average Score: <N.N>`, then per-lens reports.

## Composition

Invoked by:
- `/plan-review` slash command
- `evolve-loop` macro (between Scout and TDD when `EVOLVE_PLAN_REVIEW=1`)

The `plan-reviewer` persona uses `parallel_subtasks` (see `.evolve/profiles/plan-reviewer.json`) — four lens sub-personas run concurrently and merge via `aggregator.sh phase=plan-review`.

## Reference

- `.evolve/profiles/plan-reviewer.json` for lens prompt templates
- `scripts/aggregator.sh` plan-review merge mode
- `scripts/phase-gate.sh:gate_plan_review_to_tdd`
- `docs/architecture/tri-layer.md` (anti-patterns)
