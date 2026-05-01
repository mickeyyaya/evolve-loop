---
description: Validate the build via four parallel sub-auditors (eval-replay, lint, regression, build-quality). ALL-PASS rule.
---

# /audit

Run the Auditor phase against the current cycle. Spawns four sub-auditors in parallel:
- `audit-eval-replay` — re-runs evals from `.evolve/evals/cycle-*/`
- `audit-lint` — static analysis on changed files
- `audit-regression` — runs `scripts/run-all-regression-tests.sh`
- `audit-build-quality` — dep hygiene, secret scan, large-file check

ALL-PASS rule: aggregate verdict is `PASS` only if every sub-auditor reports PASS. Any FAIL → aggregate FAIL.

## When to use

- After `/build` produces build-report.md
- Cycle is in `build` phase per `cycle-state.sh get phase`

## Execution

```bash
bash scripts/subagent-run.sh dispatch-parallel auditor <cycle> <workspace>
```

## Adversarial mode (default ON)

CLAUDE.md rule 8: each sub-auditor's prompt prepends "ADVERSARIAL AUDIT MODE — require positive evidence for PASS". Auditor model defaults to Opus while Builder defaults to Sonnet — different family breaks same-model-judge sycophancy.

Disable only with `ADVERSARIAL_AUDIT=0` for deliberately permissive sweeps.

## See also

- `skills/evolve-audit/SKILL.md`
- `agents/evolve-auditor.md`
- `.evolve/profiles/auditor.json`
- `scripts/aggregator.sh` (phase=audit)
