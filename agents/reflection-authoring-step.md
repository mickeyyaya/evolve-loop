---
name: reflection-authoring-step
description: Uniform exit-protocol step every phase agent must execute before posting its completion ledger entry. Single source of truth so all 9 phase agents stay synchronized.
tools: []
---

# Reflection Authoring Step

> **v12.0.0 status:** `legacy/scripts/...` paths referenced below were removed in the v12 flag day. Reflection sidecar writes and aggregation are owned by the Go reflection package (`go/internal/reflection/`). Treat bash snippets as contracts; do not invoke them directly.
>
> Referenced verbatim by all 9 phase agent personas (Intent, Scout, Triage, Plan-Review, Build-Planner, TDD-Engineer, Tester, Builder, Auditor).
> Gated on `EVOLVE_REFLECTION_JOURNAL` (default `1` at v10.20.0; `0` opts out).
> Schema details: [agents/agent-templates.md](agent-templates.md) → Reflection Journal Schema.

## Purpose

Every phase agent surfaces a bounded reflection on its own execution so the Learn phase can aggregate per-phase friction and cross-cycle patterns. This is process retrospection, not code-quality review (distinct from `EVOLVE_BUILDER_SELF_REVIEW`).

## Workflow (3 steps, before posting your completion ledger entry)

1. **Read your phase-tracker metrics** from `.evolve/runs/cycle-<N>/.ephemeral/metrics/<phase>.json` (already produced by `legacy/scripts/observability/rollup-cycle-metrics.sh`). Extract `latency_ms`, `cost_usd`, `turns` — these populate your reflection's `phase_tracker_refs` block. Do NOT recompute these numbers.

2. **Write `$WORKSPACE/<phase>-reflection.yaml`** following the YAML schema in agent-templates.md → Reflection Journal Schema. Required keys: `schema_version: 1`, `cycle`, `phase`, `agent`, `phase_smooth`, `suggested_improvements[]` (≥1), `reflection_confidence` (0.0-1.0), `phase_tracker_refs{}`. Optional: `slowdowns[]`, `friction_received_from[]`, `blind_spots[]`. Keep the file ≤30 lines.

3. **Append the `## Reflection` markdown section** to your primary report (`<phase>-report.md` or equivalent — e.g., `intent.md`, `scout-report.md`, `triage-decision.md`, `plan-review.md`, `build-plan.md`, `test-report.md`, `tester-report.md`, `build-report.md`, `audit-report.md`). Bracket the section with `<!-- BEGIN reflection -->` / `<!-- END reflection -->` for idempotent replacement on re-run. Cap at ~350 tokens. Use the markdown template from agent-templates.md → Reflection Journal Schema → "Markdown section" verbatim.

## Anti-sycophancy directive

Include this discipline when authoring the reflection:

> A reflection is NOT a status report. "Phase went smoothly" is only acceptable when `phase_smooth: true` is asserted AND `phase_tracker_refs` shows no over-budget signal (cost ≤ baseline × 1.1, turns ≤ profile max). Otherwise you MUST cite at least one slowdown with artifact evidence. Affirmation without evidence is a `reflection-sycophancy` defect the Auditor flags MEDIUM (advisory, non-blocking).

Concretely:

- If `slowdowns: []` AND `phase_smooth: false` (or absent) → you are claiming everything went well WITHOUT asserting smoothness. The Auditor will flag this MEDIUM.
- If `reflection_confidence < 0.3` → the Auditor will flag this MEDIUM (your own confidence undermines the reflection).
- If `slowdowns[]` has entries, each MUST have `evidence` citing a file:line, log entry, or ledger ref. Vague evidence like "things were slow" is itself a defect.

## Phase-specific guidance (lightweight)

| Phase | Look for friction in… |
|-------|-----------------------|
| Intent | Ask-when-Needed classification ambiguity, missing context for challenged premises |
| Scout | Research-quota hits, kb-search misses, ambiguous task selection rubric |
| Triage | Inbox contention, top_n vs deferred boundary unclear |
| Plan-Review | Lens disagreement, missing rubric for verdict aggregation |
| Build-Planner | Plan vs actual diff (advisory rollout — large diff = upstream Scout AC drift) |
| TDD-Engineer | Untestable acceptance criteria, predicate-runner flakiness |
| Tester | Unverifiable AC count, predicate validation lint failures |
| Builder | Tool errors, profile restrictions, cost-guard threshold breaches, AC ambiguity from TDD |
| Auditor | Defect-detection blind spots, evidence-chain gaps, model-family separation enforcement friction |

These are starting points, not an exhaustive list. The `category` field uses the closed enum (`research-quota`, `tool-error`, `context-saturation`, `ambiguous-input`, `tool-batching`, `profile-restriction`, `other`).

## Skip when

If `EVOLVE_REFLECTION_JOURNAL=0` is set in the dispatcher environment, skip this step entirely (do not write the YAML or markdown section). This is the emergency opt-out per CLAUDE.md env-var ladder.

## Ledger entry adjustment

When you post your completion ledger entry, add one field:

```json
"reflection_emitted": <true|false>
```

`true` if you wrote `<phase>-reflection.yaml`; `false` only if `EVOLVE_REFLECTION_JOURNAL=0` was set.

## Cross-references

- Schema canonical: [agent-templates.md](agent-templates.md) → Reflection Journal Schema
- Reflector persona (consumes reflections): [evolve-reflector.md](evolve-reflector.md)
- Aggregator: [../legacy/scripts/observability/aggregate-reflections.sh](../legacy/scripts/observability/aggregate-reflections.sh)
- Design doc: [../docs/architecture/reflection-journal.md](../docs/architecture/reflection-journal.md)
- Learn-phase formalization: [../docs/architecture/learn-phase.md](../docs/architecture/learn-phase.md)
- Auditor enforcement (MEDIUM defect): [evolve-auditor.md](evolve-auditor.md) (task #7)
- Phase-gate enforcement (v10.21+): [../legacy/scripts/lifecycle/phase-gate.sh](../legacy/scripts/lifecycle/phase-gate.sh) (task #8)
