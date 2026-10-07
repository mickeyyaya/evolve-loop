# Reflection Journal — Per-Phase Improvement Signal

> Status: **advisory** (v10.20.0+). Promotion to **enforce** target: v10.21.0.
> Env-var gate: `EVOLVE_REFLECTION_JOURNAL` (default `1`; `0` opts out for emergencies).
> Schema: [agents/agent-templates.md](../../agents/agent-templates.md) → Reflection Journal Schema section.
> Learn-phase formalization: [learn-phase.md](learn-phase.md).

## Why this exists

Before v10.20.0, friction the phase agents felt — research-tool quota hits, profile restrictions, ambiguous upstream artifacts, context saturation — only surfaced *after* it caused an audit FAIL/WARN, when the retrospective agent dug it out post-mortem. PASS cycles dropped the signal entirely. There was no operator-facing surface that said "Builder hit the same tool-error trap 4 cycles in a row" until someone noticed by hand-comparing reports.

The Reflection Journal closes that gap with two complementary surfaces per cycle:

1. **Per-phase markdown** (`## Reflection` section appended to each phase's primary report) — human-readable summary of friction with cited evidence, intended for operator scan.
2. **Per-phase YAML sidecar** (`<phase>-reflection.yaml`) — flat, jq-greppable schema consumed by `aggregate-reflections.sh` for cross-cycle pattern detection.

The new every-cycle `evolve-reflector` agent produces a `learn/reflector-synthesis.md` that retrospective (FAIL/WARN) and memo (PASS) both consume — meaning every cycle ends with one operator-visible "what could be improved" rollup independent of verdict.

## Design tenets

| Tenet | Mechanism |
|-------|-----------|
| **Single source of truth for friction** | One schema (Reflection Journal Schema in agent-templates.md) used by all 9 phase agents — no scattered `Known Gap` / `Risk Assessment` / `Defects` synonyms. |
| **Mechanical aggregation** | YAML sidecar + grep-based `aggregate-reflections.sh` (no `yq` dep) → systemic patterns (`category 4/5 cycles`) surface without human pattern-matching. |
| **Anti-sycophancy** | `phase_smooth: true` only valid when backed by `phase_tracker_refs` numbers (cost ≤ baseline × 1.1, turns ≤ profile max). Empty `slowdowns` without `phase_smooth: true` → Auditor MEDIUM defect `reflection-sycophancy`. |
| **Confidence-weighted aggregation** | `reflection_confidence < 0.5` reflections excluded from aggregator tallies but still surfaced in operator-facing markdown with `[low-confidence]` tag. |
| **Reuse existing observability** | `phase_tracker_refs` block read from `.ephemeral/metrics/<phase>.json` produced by `rollup-cycle-metrics.sh` — no double-instrumentation. |
| **Single-writer invariant** | Each phase agent writes its own `<phase>-reflection.yaml` in its own exit protocol; never delegated to parallel fan-out. |
| **Bounded token cost** | 350-token cap per `## Reflection` markdown section; YAML companion (~30 lines) is the aggregation surface so markdown can stay narrative. |

## Schema

Defined canonically in [agents/agent-templates.md](../../agents/agent-templates.md) → Reflection Journal Schema. Summary:

- **Markdown:** `## Reflection` section with 4 required + 1 optional subsections, bracketed by `<!-- BEGIN reflection -->` / `<!-- END reflection -->` anchors for idempotent re-runs.
- **YAML sidecar:** `<phase>-reflection.yaml` flat-keys: `schema_version`, `cycle`, `phase`, `agent`, `phase_smooth`, `slowdowns[]`, `friction_received_from[]`, `suggested_improvements[]`, `blind_spots[]`, `reflection_confidence`, `phase_tracker_refs{}`.
- **Slowdown category enum (closed set):** `research-quota`, `tool-error`, `context-saturation`, `ambiguous-input`, `tool-batching`, `profile-restriction`, `other`.

## Lifecycle

```
Each phase agent (Intent, Scout, Triage, Plan-Review, Build-Planner,
                  TDD-Engineer, Tester, Builder, Auditor)
  │
  ├── runs its primary work → produces <phase>-report.md
  │
  ├── reads .ephemeral/metrics/<phase>.json (cost, turns, latency)
  │
  ├── writes ## Reflection section to <phase>-report.md   ◄── operator-facing
  │
  └── writes <phase>-reflection.yaml sidecar              ◄── machine-readable

Learn phase (every cycle, post-Ship):
  │
  ├── evolve-reflector reads all <phase>-reflection.yaml
  │     ├── runs aggregate-reflections.sh --window 5
  │     └── emits learn/reflector-synthesis.md (per-phase + cross-cycle)
  │
  ├── verdict == PASS → evolve-memo reads synthesis → adds Reflection
  │                     Highlights section to memo.md, populates carryoverTodos
  │
  └── verdict == FAIL/WARN → evolve-retrospective reads synthesis →
                              adds Reflection Synthesis section to
                              retrospective-report.md, candidates systemic
                              patterns for lesson YAMLs

Operator-facing rollup:
  legacy/scripts/observability/dashboard.sh reads aggregator --format=json
  → "Recent reflection hot-spots: top-3 categories" line
```

## Rollout ladder

Matches the `EVOLVE_BUILD_PLANNER` precedent in [control-flags.md](control-flags.md).

| Stage | Version | Default | Behavior |
|-------|---------|---------|----------|
| **advisory** | v10.20.0 | `EVOLVE_REFLECTION_JOURNAL=1` | All 9 phase agents emit reflections; reflector + retrospective + memo consume YAML. Missing YAML → WARN line in `learn/reflector-synthesis.md`, no FAIL. |
| **enforce** | v10.21.0 | unchanged default-on; opt-out via `=0` for emergencies | `phase-gate.sh check_reflection_artifact` blocks each `gate_<phase>_to_<next>` on presence of `<phase>-reflection.yaml`. Absent = exit 1, BLOCK. Promotion criterion: 3 consecutive cycles where ≥80% of phases emit valid reflections AND token overhead ≤ 800 tokens/cycle. |

## Anti-sycophancy rationale

Reflection systems that ask agents "how did it go?" reliably produce noise — "everything went smoothly" said by every agent every cycle, drowning the rare genuine signal. Three guardrails prevent that:

1. **Required positive-evidence framing:** the `## Reflection` instruction text mandates that "phase went smoothly" is only valid when `phase_smooth: true` is asserted AND `phase_tracker_refs` shows cost/turns within budget. Affirmation without evidence is a defect, not a default.

2. **Auditor enforcement (MEDIUM, advisory):** the Auditor agent's existing defect-detection pipeline gets a new check: empty `slowdowns[]` + `phase_smooth: false` (or absent) OR `reflection_confidence < 0.3` → emit `reflection-sycophancy` defect at severity MEDIUM. EGPS only blocks ship on `red_count == 0`, so MEDIUM surfaces in the audit report without weaponizing the gate against truly smooth phases.

3. **Aggregator confidence filter:** `reflection_confidence < 0.5` reflections silently excluded from `aggregate-reflections.sh` tallies. A confident "I'm not sure what slowed me down" remains visible in the per-cycle synthesis but won't pollute cross-cycle pattern detection.

These three combined are calibrated, not draconian: a phase that genuinely runs smoothly can attest to it (and remain unflagged), but the asymmetry favors surfacing friction over hiding it.

## What this is NOT

- **NOT a retrospective.** Reflections surface friction; retrospective (`evolve-retrospective`) does root-cause analysis on FAIL/WARN and produces lesson YAMLs.
- **NOT a memo replacement.** Memo (`evolve-memo`) continues to own PASS-cycle carryoverTodos. The reflector synthesis feeds memo with structured suggestions; memo decides which become deferred work.
- **NOT a code-quality review.** That's the source writers' required Self-review pass (`code-review-simplify`, recorded as `## Self-Review` in the phase report). Reflection journal is process-level — same cycle, every phase, regardless of code change.
- **NOT a replacement for phase-tracker.** Phase-tracker captures objective numbers (timing, cost, turns); reflection captures the agent's subjective friction. The two are complementary — reflection's `phase_tracker_refs` block cites phase-tracker as evidence.

## Cross-references

- Schema: [agents/agent-templates.md](../../agents/agent-templates.md) → Reflection Journal Schema
- Reflector persona: [agents/evolve-reflector.md](../../agents/evolve-reflector.md)
- Reflector profile: `.evolve/profiles/reflector.json`
- Aggregator: [legacy/scripts/observability/aggregate-reflections.sh](../../legacy/scripts/observability/aggregate-reflections.sh)
- Tests: [legacy/scripts/tests/reflection-schema-test.sh](../../legacy/scripts/tests/reflection-schema-test.sh), [legacy/scripts/tests/aggregate-reflections-test.sh](../../legacy/scripts/tests/aggregate-reflections-test.sh)
- Learn-phase formalization: [learn-phase.md](learn-phase.md)
- Phase-tracker integration: [phase-tracker.md](phase-tracker.md)
- Existing retrospective contract: [retrospective-pipeline.md](retrospective-pipeline.md)
- Env-var ladder precedent: [control-flags.md](control-flags.md) → `EVOLVE_BUILD_PLANNER`
- EGPS verdict model (severity calibration): [egps-v10.md](egps-v10.md)
