---
description: Run multi-lens plan review on scout-report.md — CEO, Eng, Design, Security lenses fan out and produce a verdict (PROCEED/REVISE/ABORT) before any code is written.
---

# /plan-review

Run the Sprint 2 multi-lens review against the current cycle's `scout-report.md`. Spawns four lens reviewers in parallel:
- `plan-ceo-reviewer` — scope ambition (gstack /autoplan inspired)
- `plan-eng-reviewer` — architectural feasibility
- `plan-design-reviewer` — API surface ergonomics
- `plan-security-reviewer` — trust-kernel impact (sandbox, ledger, phase-gate)

Aggregator computes verdict and writes `<workspace>/plan-review.md`.

## When to use

- Before TDD/Build, when scout-report.md proposes architectural changes
- After Scout completes, to catch misaligned tasks early
- When a previous cycle aborted due to wrong direction

## Execution

```bash
EVOLVE_PLAN_REVIEW=1 bash scripts/subagent-run.sh dispatch-parallel plan-reviewer <cycle> <workspace>
```

## Verdict semantics

| Verdict | Avg score | Any lens < 5 | Action |
|---|---|---|---|
| PROCEED | ≥ 7 | No | Advance to /tdd |
| REVISE | ≥ 5 | Yes | Re-run /scout (max 2 retries) |
| ABORT | < 5, or any explicit ABORT | — | End cycle |

## See also

- `skills/evolve-plan-review/SKILL.md`
- `agents/plan-reviewer.md`
- `.evolve/profiles/plan-reviewer.json`
- `scripts/phase-gate.sh:gate_plan_review_to_tdd`
