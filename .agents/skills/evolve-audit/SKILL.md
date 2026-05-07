---
name: evolve-audit
description: Use after evolve-build has produced build-report.md. Validates the build via four parallel sub-auditors (eval-replay, lint, regression, build-quality) and produces ALL-PASS verdict. Adversarial mode default-on per CLAUDE.md.
---

# evolve-audit

> Sprint 1.2 fan-out + Sprint 3 composable skill (v8.16+). Sub-auditors run in parallel via `subagent-run.sh dispatch-parallel auditor`.

## When to invoke

- After `evolve-build` produces build-report.md
- Cycle is in `build` phase per cycle-state

## When NOT to invoke

- Build status is FAIL (no point auditing broken code; orchestrator must re-build first)
- Eval-only cycles (only run `audit-eval-replay` sub-auditor)

## Workflow

| Step | Action | Exit criteria |
|---|---|---|
| 1 | Verify `<workspace>/build-report.md` exists, fresh, status ≠ FAIL | Build verified |
| 2 | Dispatch 4 sub-auditors in parallel | 4 worker artifacts |
| 3 | Aggregator applies ALL-PASS rule | `<workspace>/audit-report.md` first line is `Verdict: <X>` |
| 4 | Phase gate `gate_audit_to_ship` enforces PASS | Gate passes only on PASS |

## Verdict semantics

| Verdict | Trigger | Phase-gate behavior |
|---|---|---|
| `PASS` | Every sub-auditor reports PASS | Allow ship |
| `FAIL` | Any sub-auditor reports FAIL | Block ship; orchestrator → retrospective |
| `WARN` | Any sub-auditor reports WARN (no FAIL) | Block ship; review case-by-case |

## Adversarial mode (CLAUDE.md rule 8)

Default ON: each sub-auditor's prompt prepends "ADVERSARIAL AUDIT MODE — require positive evidence for PASS". Disable only via `ADVERSARIAL_AUDIT=0` for deliberately permissive sweeps. Auditor model defaults to Opus while Builder defaults to Sonnet — different family breaks same-model-judge sycophancy.

## Output contract

`<workspace>/audit-report.md` with first line `Verdict: <X>`, then four lens reports concatenated.

## Composition

Invoked by:
- `/audit` slash command
- `evolve-loop` macro after `/build`

Fan-out controlled by `.evolve/profiles/auditor.json:parallel_subtasks` (4 entries).

## Reference

- `.evolve/profiles/auditor.json`
- `scripts/aggregator.sh` (phase=audit)
- `scripts/phase-gate.sh:gate_audit_to_ship`
- `skills/evolve-loop/phase4-audit.md` (legacy detailed workflow)
