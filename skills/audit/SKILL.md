---
name: audit
description: Use after build has produced build-report.md. Validates the build with one adversarial auditor agent and produces the audit verdict. Adversarial mode default-on per CLAUDE.md.
---

# audit

> Sprint 3 composable skill (v8.16+). The Sprint 1.2 sub-auditor fan-out is removed. `.evolve/profiles/auditor.json` has no `parallel_subtasks`, so one auditor agent writes the report.

## When to invoke

- After `build` produces build-report.md
- Cycle is in `build` phase per cycle-state

## When NOT to invoke

- Build status is FAIL (no point auditing broken code; orchestrator must re-build first)

## Workflow

| Step | Action | Exit criteria |
|---|---|---|
| 1 | Verify `<workspace>/build-report.md` exists, fresh, status ≠ FAIL | Build verified |
| 2 | Dispatch the auditor agent through the bridge | `<workspace>/audit-report.md` written |
| 3 | The auditor declares the verdict | `audit-report.md` has a `## Verdict` (or `Verdict:`) line |
| 4 | `evolve ship` verifies the audit binding (`go/internal/phases/ship/audit.go`) | Ship refuses a FAIL verdict |

## Verdict semantics

| Verdict | Trigger | Ship behavior |
|---|---|---|
| `PASS` | The auditor reports PASS | Allow ship |
| `FAIL` | The auditor reports FAIL | Block ship; orchestrator → retrospective |
| `WARN` | The auditor reports WARN | Ship continues and logs the WARN. `workflow.strict_audit` in `.evolve/policy.json` blocks it |

## Adversarial mode (CLAUDE.md rule 8)

Default ON: the runner adds the "ADVERSARIAL AUDIT MODE (default-on)" framing to the end of the auditor's prompt (`go/internal/subagent/subagentrun/prepare.go`). The framing requires positive evidence for PASS. Disable it only with `ADVERSARIAL_AUDIT=0` for deliberately permissive sweeps. Auditor model defaults to Opus while Builder defaults to Sonnet — different family breaks same-model-judge sycophancy.

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
| Profile | `.evolve/profiles/auditor.json` — CLI `claude-tmux`, tier `deep`, single-writer |
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
- `loop` macro after `/evo:build`

## Reference

- `.evolve/profiles/auditor.json`
- `go/internal/phases/audit/` (the audit phase and its gates)
- `go/internal/phases/ship/audit.go` (the audit binding that ship verifies)
