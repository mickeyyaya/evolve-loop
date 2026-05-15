# Output Contracts — Phase Handoff Schemas

> Reference index for the 8 phase-artifact schemas under `schemas/handoff/`. Each phase persona MUST emit an artifact that satisfies its schema; failures are detected by `validate-handoff-artifact.sh`. See [ADR 0009](../../../../docs/adr/0009-phase-handoff-schemas.md) for the rationale.

## Phase × schema matrix

| # | Phase | Persona file | Artifact filename | Schema file | Required sections |
|---|---|---|---|---|---|
| 1 | Intent | `agents/evolve-intent.md` | `intent-report.md` | `schemas/handoff/intent-report.schema.json` | Goal · Constraints · Success Criteria |
| 2 | Scout | `agents/evolve-scout.md` | `scout-report.md` | `schemas/handoff/scout-report.schema.json` | Proposed Tasks · Acceptance Criteria · (Carryover Decisions when state has todos) |
| 3 | Triage | `agents/evolve-triage.md` | `triage-decision.md` | `schemas/handoff/triage-decision.schema.json` | Selected Tasks · Rejected Tasks · Cycle Size |
| 4 | TDD | `agents/evolve-tdd-engineer.md` | `tdd-report.md` | `schemas/handoff/tdd-report.schema.json` | Tests Written · AC Mapping · Red State |
| 5 | Build | `agents/evolve-builder.md` | `build-report.md` | `schemas/handoff/build-report.schema.json` | Changes · Self-Verification · Quality Signals |
| 6 | Audit | `agents/evolve-auditor.md` | `audit-report.md` | `schemas/handoff/audit-report.schema.json` | Artifacts Reviewed · Verdict (with PASS/WARN/FAIL value) |
| 7 | Ship | `scripts/lifecycle/ship.sh` (no persona — script) | `ship-report.md` | `schemas/handoff/ship-report.schema.json` | Commit · Tree SHA Binding · Ledger Entry |
| 8 | Retrospective | `agents/evolve-retrospective.md` | `retrospective-report.md` | `schemas/handoff/retrospective-report.schema.json` | What Happened · Root Cause · Lesson (with lesson YAML pointer) |

## Schema format (recap)

Schemas are bash-native / jq-readable JSON. **Not** JSON Schema v2020-12 — that's a deliberate choice (see ADR 0009).

| Schema key | Effect |
|---|---|
| `required_first_line.pattern` | Regex line-1 must match (challenge-token guard) |
| `required_sections[]` | Each `{name, patterns[], fail_message}`; any pattern match satisfies |
| `conditional_sections[]` | Same as required_sections plus `condition` (e.g., `has_carryover_todos`); requires `--state` flag |
| `required_content[]` | Each `{name, pattern, fail_message}`; pattern must match somewhere in artifact |
| `min_words` | Soft floor; FAIL if `wc -w < artifact < min_words` |

## Invocation

```bash
bash scripts/tests/validate-handoff-artifact.sh \
    --artifact .evolve/runs/cycle-N/build-report.md \
    --type build \
    [--state .evolve/state.json]   # required for conditional_sections
```

Exit codes: `0` = PASS · `1` = FAIL (named violations on stdout) · `2` = ERROR (usage / missing dep).

## Authoring rules

1. Add the **challenge token** as the first line of every artifact (`<!-- challenge-token: $TOKEN -->`). Bypassing this guard is a forgery signal — `ship-gate` rejects.
2. Use one of each section's `patterns[]` verbatim (e.g., `## Verdict` not `### Verdict`).
3. Anchored alternates (`<!-- ANCHOR:name -->`) are permitted and recommended when the human-readable heading must vary across persona dialects.
4. Conditional sections only fire when `--state` is passed and the condition holds — never include them unconditionally.

## Coverage gap (cycle 63)

The cycle-63 additions (Intent, Triage, TDD, Ship, Retrospective) ship the **schema files** but `validate-handoff-artifact.sh`'s `--type` argument currently only accepts `scout|build|audit`. Extending the validator's allowlist is a separate cycle (tracked: `inbox:c64-extend-handoff-validator`). Until then, the new schemas are advisory; personas should still author against them so the eventual enforcement is a no-op.
