---
name: plan-review
description: Use when scout-report.md exists and TDD/Build hasn't started yet. Runs four lenses (CEO, Eng, Design, Security) in parallel on the task list and produces a verdict (PROCEED, REVISE, ABORT) before code is written. Catches misaligned plans before they cost cycles.
---

# plan-review

> Sprint 2 multi-lens review (v8.16+). Inspired by `garrytan/gstack/autoplan`. Default-off; enable via `workflow.phase_enables.plan-review=on` in policy.json.

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
| 2 | Dispatch 4 lenses in parallel with `evolve subagent dispatch-parallel plan-reviewer <cycle> <workspace>` | 4 worker artifacts |
| 3 | Aggregator computes verdict (PROCEED/REVISE/ABORT) | `<workspace>/plan-review-report.md` present, first line is `Verdict: <X>` |

## Verdict semantics

| Verdict | Trigger | Aggregator result |
|---|---|---|
| `PROCEED` | Avg score ≥ 7 AND no lens < 5 | exit 0 |
| `REVISE` | Not ABORT, AND (any lens < 5 OR avg < 7) | exit 0 |
| `ABORT` | Any lens explicit ABORT, OR avg < 5 | exit 1 |

Removed design: no Go gate acts on the verdict. The former orchestrator actions (advance to TDD, re-run Scout, end the cycle) do not run now.

## Report format

First line `Verdict: <X>`, second `Average Score: <N.N>`, then per-lens reports.

<!-- GENERATED:phase-facts BEGIN — do not edit; run `evolve skills generate`. Sources: docs/architecture/phase-registry.json · go/internal/phasecontract · .evolve/profiles/plan-reviewer.json -->
## Phase facts

| Fact | Value |
|---|---|
| Phase | `plan-review` (plan archetype, optional, gated by `workflow.phase_enables.plan-review=on`) |
| Persona | `agents/plan-reviewer.md` |
| Profile | `.evolve/profiles/plan-reviewer.json` — CLI `codex-tmux`, tier `deep`, fan-out ×4 |
| Inputs | `scout-report.md` · `triage-report.md` · `triage-decision.json` |
| Artifact | `plan-review-report.md` (cycle workspace) |

## Output contract

`plan-review-report.md` — no required sections (classified by the phase runner).

<!-- GENERATED:phase-facts END -->

## Composition

Invoked by:
- `/evo:plan-review`
- `loop` macro (between Scout and TDD when `workflow.phase_enables.plan-review=on`)

The `plan-reviewer` persona uses `parallel_subtasks` (see `.evolve/profiles/plan-reviewer.json`) — the lens sub-personas (count projected into Phase facts above) run concurrently and merge with `evolve aggregator plan-review` (`go/internal/aggregator`).

## Reference

- `.evolve/profiles/plan-reviewer.json` for lens prompt templates
- `go/internal/aggregator` (plan-review merge mode)
- `docs/architecture/tri-layer.md` (anti-patterns)
