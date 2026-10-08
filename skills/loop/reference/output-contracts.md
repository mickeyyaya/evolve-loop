# Output Contracts — Phase Handoff Schemas

> Reference index for the 8 phase-artifact schemas under `schemas/handoff/`. Each phase persona MUST emit an artifact that satisfies its schema. The phase runner checks each report against its contract in `go/internal/phasecontract`, and `evolve phase verify <phase>` runs the same check. See [ADR 0009](../../../docs/adr/0009-phase-handoff-schemas.md) for the rationale.

## Phase × schema matrix

| # | Phase | Persona file | Artifact filename | Schema file | Required sections |
|---|---|---|---|---|---|
| 1 | Intent | `agents/evolve-intent.md` | `intent-report.md` | `schemas/handoff/intent-report.schema.json` | Goal · Constraints · Success Criteria |
| 2 | Scout | `agents/evolve-scout.md` | `scout-report.md` | `schemas/handoff/scout-report.schema.json` | Proposed Tasks · Acceptance Criteria · (Carryover Decisions when state has todos) |
| 3 | Triage | `agents/evolve-triage.md` | `triage-report.md` | `schemas/handoff/triage-decision.schema.json` | Selected Tasks · Rejected Tasks · Cycle Size |
| 4 | TDD | `agents/evolve-tdd-engineer.md` | `tdd-report.md` | `schemas/handoff/tdd-report.schema.json` | Tests Written · AC Mapping · Red State |
| 5 | Build | `agents/evolve-builder.md` | `build-report.md` | `schemas/handoff/build-report.schema.json` | Changes · Self-Verification · Quality Signals |
| 6 | Audit | `agents/evolve-auditor.md` | `audit-report.md` | `schemas/handoff/audit-report.schema.json` | Artifacts Reviewed · Verdict (with PASS/WARN/FAIL value) |
| 7 | Ship | `evolve ship` (no persona — native Go, `go/internal/phases/ship`) | `ship-report.md` | `schemas/handoff/ship-report.schema.json` | Commit · Tree SHA Binding · Ledger Entry |
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
evolve phase verify build --workspace .evolve/runs/cycle-N [--json]
```

The phase is a built-in contract (for example `scout`, `triage`, `tdd`, `build`, `audit`, `ship`, `retro`, `intent`) or a catalog phase with a contract. Exit codes: `0` = well-formed · `1` = confirmed violation · `2` = infra ambiguity (callers fail open) · `10` = usage error.

## Authoring rules

1. Add the **challenge token** as the first line of every artifact (`<!-- challenge-token: $TOKEN -->`). Bypassing this guard is a forgery signal — `ship-gate` rejects.
2. Use one of each section's `patterns[]` verbatim (e.g., `## Verdict` not `### Verdict`).
3. Anchored alternates (`<!-- ANCHOR:name -->`) are permitted and recommended when the human-readable heading must vary across persona dialects.
4. Conditional sections only fire when `--state` is passed and the condition holds — never include them unconditionally.

## Coverage gap (cycle 63)

Removed design: the bash `validate-handoff-artifact.sh` validator is gone, and no Go code reads the `schemas/handoff/` files to check a report. The enforced contract is `go/internal/phasecontract`, and `evolve phase verify <phase>` runs the same check. Personas must still author against the schemas.
