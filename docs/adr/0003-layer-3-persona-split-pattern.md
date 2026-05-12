# ADR 0003 — Layer-3 Persona Split Pattern

| Field | Value |
|---|---|
| Status | Accepted |
| Date | 2026-05-12 |
| Cycle | 24 |
| Affects | Agent persona files, token economics, subagent context load |

## Context

During the token-reduction campaign (cycles 15–24), analysis showed that persona files were loaded in full on every subagent invocation, regardless of whether the agent needed deep-reference content. A Scout summarizing a small change loaded the same 334-line persona as a Scout investigating a systemic incident. This inflated per-cycle token costs with no value for the common path.

Two measurements confirmed the problem:
- `agents/evolve-scout.md`: 334 lines loaded unconditionally.
- `skills/evolve-loop/SKILL.md` (`phases.md`): 28,911 bytes loaded unconditionally.

The existing `docs/architecture/tri-layer.md` Skill/Persona/Command structure addressed file organisation but not intra-file density. A complementary pattern was needed for per-role context load reduction.

## Decision

Split each `agents/evolve-<role>.md` into two tiers:

1. **Common-path layer** (`agents/evolve-<role>.md`) — lean persona containing only content needed on every invocation. Loaded unconditionally.
2. **On-demand Layer-3** (`agents/evolve-<role>-reference.md`) — deep-reference content (rationale, edge cases, incident history, operator block templates). Loaded only when a specific decision branch requires it.

Naming convention: `<file>.md` = persona; `<file>-reference.md` = Layer-3 on-demand.

Each persona includes a **Reference Index** table mapping "when" conditions to specific sections of the reference file, so the agent reads the minimum necessary.

Commits `d73eabf` (Scout split) and `0e4bff1` (phases.md split) implemented the first two instances:
- Scout split: 334 → 167 lines (−50%).
- phases.md split: 28,911 → 13,987 bytes (−51.6%).

Test suite (`swarm-architecture-test.sh`): 41/41 PASS after Scout split.

## Consequences

**Positive:**
- Common-path token load halved for affected personas, benefiting prompt-cache hit rates.
- Deep reference remains available on-demand; no functionality lost.
- Pattern is repeatable: any oversized persona can be split using the same naming convention.
- Auditor and Orchestrator can reference the Layer-3 files when needed without loading them every cycle.

**Negative:**
- Two files per role to maintain instead of one; contributors must know to update both.
- The "Reference Index" table in each persona adds a small fixed overhead (~5–10 lines) that partially offsets the savings on very short personas.
- Semantic split requires judgment about what is "common-path" vs "on-demand"; wrong splits increase the common-path load instead of reducing it.

## Alternatives considered

| Alternative | Why rejected |
|---|---|
| LLMLingua-2 automated compression | Runtime latency; additional dependency; static files already benefit from prompt-cache without compression |
| Truncate at a fixed line count | Loses information unpredictably; no semantic boundary |
| Role-context-builder.sh per-role injection | Orthogonal to this pattern; implemented separately in v8.56 to control which artifacts each role receives |
| Single large reference file for all roles | Cross-role content mixed; harder to maintain; no per-role on-demand scoping |

## Cross-reference

- `docs/architecture/token-reduction-roadmap.md` — P-NEW-3 (Scout split), P-NEW-7 (phases.md split).
- `docs/architecture/tri-layer.md` — the Skill/Persona/Command tri-layer; a related but distinct pattern (structural organization vs context load optimization).
