# ADR 0009 — Phase Handoff Schemas (C2)

- Status: Accepted
- Date: 2026-05-15
- Cycle: 63
- Predecessor: [ADR 0006 — Layer-P Memo Handoff Template](0006-layer-p-memo-handoff-template.md)
- Supersedes: implicit per-phase markdown conventions documented prose-only in `agents/*.md`

## Context

Pre-cycle-38, the only enforced contract on a phase artifact was the challenge token (forgery guard). The audit-binding model (cycle 35–37) added SHA pinning of the artifact, but did **not** constrain *what* the artifact must contain. Each persona's expectations lived only in `agents/<role>.md` prose, so:

- Auditors silently accepted reports missing critical sections (e.g., no `## Verdict` heading) as long as the SHA matched.
- Builder reports without `## Quality Signals` shipped because no automated lint ran.
- Cross-phase changes broke handoffs without any deterministic signal — failures surfaced cycles later as reward-hacking forensics.

Cycle 38 introduced `schemas/handoff/{scout,build,audit}-report.schema.json` and `scripts/tests/validate-handoff-artifact.sh` as the first three phase contracts. Cycle 63 closes the gap by adding the remaining five.

## Decision

The complete set of phase handoff schemas is **8 files** under `schemas/handoff/`:

| Phase | Schema file | Status |
|---|---|---|
| Intent | `intent-report.schema.json` | cycle 63 |
| Scout | `scout-report.schema.json` | cycle 38 |
| Triage | `triage-decision.schema.json` | cycle 63 |
| TDD | `tdd-report.schema.json` | cycle 63 |
| Build | `build-report.schema.json` | cycle 38 |
| Audit | `audit-report.schema.json` | cycle 38 |
| Ship | `ship-report.schema.json` | cycle 63 |
| Retrospective | `retrospective-report.schema.json` | cycle 63 |

Each schema is a **bash-native / jq-readable JSON** file (no external JSON Schema validator). The format intentionally is **not** JSON Schema v2020-12 — that draft requires a 200KB+ validator that doesn't fit the project's bash-3.2 portability constraint. The format declares:

- `required_first_line` — pinned challenge-token guard
- `required_sections[]` — each item carries `name`, `patterns[]` (any-match satisfies), `fail_message`
- `conditional_sections[]` — same shape plus a `condition` key checked against `state.json`
- `required_content[]` — regex patterns that must appear somewhere in the artifact
- `min_words` — soft floor against empty handoffs

Validation runs via `bash scripts/tests/validate-handoff-artifact.sh --artifact <path> --type <phase>`. Exit 0 = PASS, 1 = FAIL with named violations, 2 = ERROR.

## Consequences

**Positive**:
- Every phase handoff is now lintable by deterministic bash — no LLM judgment in the gate.
- Schema additions are additive; existing cycles' artifacts validate without modification.
- The `output-contracts.md` reference table is single source of truth for persona authors.

**Negative**:
- Schema authors must update both the JSON file and the persona file when changing required sections.
- The custom format means external tooling (IDE JSON-Schema autocomplete) doesn't apply.

**Neutral**:
- The 5 new schemas in cycle 63 are *advisory* until `validate-handoff-artifact.sh` is extended to accept their types (tracked as inbox item for cycle 64). The schema files exist as the contract; enforcement wiring is a separate cycle to keep this ADR's blast radius small.

## References

- Source contract: `schemas/handoff/*.schema.json` (8 files)
- Reference table: [`.agents/skills/evolve-loop/reference/output-contracts.md`](../../.agents/skills/evolve-loop/reference/output-contracts.md)
- Validator: `scripts/tests/validate-handoff-artifact.sh`
- Predecessor ADR: [ADR 0006](0006-layer-p-memo-handoff-template.md)
