---
name: audit
description: Use after build has produced build-report.md. Validates the build via four parallel sub-auditors (eval-replay, lint, regression, build-quality) and produces ALL-PASS verdict. Adversarial mode default-on per CLAUDE.md.
---

# audit

> Sprint 1.2 fan-out + Sprint 3 composable skill (v8.16+). Sub-auditors run in parallel via `subagent-run.sh dispatch-parallel auditor`.

## When to invoke

- After `build` produces build-report.md
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

## Goal-integrity (metric-affecting cycles)

For any cycle that changes a scored metric — a flag-reduction cycle, a
registry/gate/marker/allowlist edit, or any cycle claiming a count reduction —
the auditor MUST apply the goal-integrity rubric in
[`skills/adversarial-testing/SKILL.md` §10.1](../adversarial-testing/SKILL.md#101-goal-integrity-rubric-adr-0064--metric-affecting-cycles)
as a mandatory BLOCK (co-equal with the deterministic gates, not a backstop). A
claimed reduction must cite the **reader that was deleted** and confirm no
surviving reader on any surface; "the row is gone" is not evidence. FAIL on
writer-fabrication, off-namespace/reflection rename, contract under-delivery, or
any `--class cycle` edit of a `guards.IsProtectedSurface` control-plane file.

<!-- GENERATED:phase-facts BEGIN — do not edit; run `evolve skills generate`. Sources: docs/architecture/phase-registry.json · go/internal/phasecontract · .evolve/profiles/auditor.json -->
## Phase facts

| Fact | Value |
|---|---|
| Phase | `audit` (evaluate archetype, mandatory) |
| Persona | `agents/evolve-auditor.md` |
| Profile | `.evolve/profiles/auditor.json` — CLI `claude-tmux`, tier `deep`, fan-out ×4 |
| Inputs | `build-report.md` · `tester-report.md` |
| Artifact | `audit-report.md` (cycle workspace) |

## Output contract

`audit-report.md` must declare:

- `## Verdict` (also accepted: `Verdict:`)

Verdict tokens: `PASS` | `FAIL` | `WARN` | `SKIPPED`.
<!-- GENERATED:phase-facts END -->

## Composition

Invoked by:
- `/evo:audit`
- `loop` macro after `/build`

Fan-out prompts live in `.evolve/profiles/auditor.json:parallel_subtasks` (count projected into Phase facts above).

## Reference

- `.evolve/profiles/auditor.json`
- `legacy/scripts/dispatch/aggregator.sh` (phase=audit)
- `legacy/scripts/lifecycle/phase-gate.sh:gate_audit_to_ship`
- `skills/loop/phase4-audit.md` (legacy detailed workflow)
