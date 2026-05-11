# Token-Floor History

This document tracks the per-cycle token-floor measurements across the
v9.0.0 four-tier rebuild (Campaigns A–D). Numbers are bytes (and approximate
tokens at 4 bytes/token, the upper-bound English heuristic used throughout
this codebase).

The measurement methodology and the harness:

- `scripts/observability/measure-context-tokens.sh <cycle> [--json]`
  (added in Campaign A Cycle A3)
- `bash scripts/lifecycle/role-context-builder.sh <role> <cycle> <workspace>`
  invoked per phase to produce the actual context shipped to the LLM

## Baseline (pre-v9.0.0, cycle-10 fixture)

Per-phase context bytes loaded into the user prompt by role-context-builder:

| Phase | Bytes | Approx tokens |
|---|---|---|
| scout | 13,615 | 3,404 |
| triage | 27,733 | 6,933 |
| tdd | 28,094 | 7,024 |
| builder | 27,743 | 6,936 |
| auditor | 35,250 | 8,813 |
| retrospective | 44,060 | 11,015 |
| **Sum** | **176,495** | **~44,124 tokens** |

The orchestrator's own prompt (orchestrator-prompt.md = persona + cycle
context block) was an additional ~21,106 bytes (~5,277 tokens), loaded once
per cycle.

## Post-v9.0.0 (with EVOLVE_CONTEXT_DIGEST=1, cycle-10 fixture)

| Phase | Bytes | Δ vs baseline |
|---|---|---|
| scout | 2,012 | **−85%** |
| triage | 16,677 | −40% |
| tdd | 16,683 | −40% |
| builder | 15,810 | −43% |
| auditor | 35,250 | 0% (synthesizer; full files preserved) |
| retrospective | 44,060 | 0% (synthesizer; full files preserved) |
| **Sum** | **130,492** | **−26% (−46,003 bytes / ~−11,500 tokens per cycle)** |

The auditor and retrospective stay at 0% reduction in Campaign B because
they legitimately need the full intent.md for deep acceptance-criteria
checks and full prior-artifact synthesis. Campaign C's anchor extraction
addresses the auditor case (with anchored future artifacts).

## Post-v9.0.0 (with EVOLVE_CONTEXT_DIGEST=1 + EVOLVE_ANCHOR_EXTRACT=1)

For artifacts that have `<!-- ANCHOR:name -->` markers (cycles run by
v9.0.0+ post-release templates), the auditor and triage phases drop
further:

- Auditor: ~35 KB → projected ~10 KB (~−70%) once cycles use anchored
  scout-report and build-report templates. Pre-v9.0.0 cycles (no anchor
  markers) fall back gracefully to full-file emission via
  `emit_artifact_with_anchors`'s once-per-file fallback.
- Triage: drops from ~16 KB to ~5 KB on anchored scout-reports
  (proposed_tasks-only extraction).

## Persona size changes (Campaign D)

Personas are loaded as the orchestrator's user prompt. The Layer 1 / Layer 3
split keeps common-path content in the registered file and moves rare-
trigger content (deep failure procedures, conditional E2E workflows,
streak-table rationales) to per-persona reference files loaded on demand
via `Read`.

| File | Pre-v9.0.0 | Post-v9.0.0 | Layer 3 file (on-demand) |
|---|---|---|---|
| `agents/evolve-orchestrator.md` | 19,030 | 17,842 (−6.2%) | `evolve-orchestrator-reference.md` (5,121) |
| `agents/evolve-builder.md` | 15,801 | 15,304 (−3.1%) | `evolve-builder-reference.md` (3,348) |
| `agents/evolve-auditor.md` | 16,891 | 16,361 (−3.1%) | `evolve-auditor-reference.md` (2,284) |
| `agents/evolve-scout.md` | 15,962 | 15,962 (0%) | (no extraction — monolithic responsibilities) |
| `agents/evolve-retrospective.md` | 12,988 | 12,988 (0%) | (only fires on FAIL/WARN; persona load is already conditional) |

## How to reproduce

```bash
# Per-phase baseline (legacy)
bash scripts/lifecycle/role-context-builder.sh scout 10 .evolve/runs/cycle-10 | wc -c
# Digest mode (Campaign B+)
EVOLVE_CONTEXT_DIGEST=1 bash scripts/lifecycle/role-context-builder.sh scout 10 .evolve/runs/cycle-10 | wc -c
# Digest + anchor mode (Campaign B + Campaign C)
EVOLVE_CONTEXT_DIGEST=1 EVOLVE_ANCHOR_EXTRACT=1 bash scripts/lifecycle/role-context-builder.sh scout 10 .evolve/runs/cycle-10 | wc -c

# Full per-cycle measurement
bash scripts/observability/measure-context-tokens.sh 10
bash scripts/observability/measure-context-tokens.sh 10 --json
```

## Promotion ladder for the new flags

All Campaign A–D opt-in flags follow the v8.55/v8.59 ladder:

| Flag | v9.0.0 default | Verify | Default-on target | Enforce target |
|---|---|---|---|---|
| `EVOLVE_CACHE_PREFIX_V2` | 0 | v9.0.x verify cycle | v9.1 | v9.2+ |
| `EVOLVE_CONTEXT_DIGEST` | 0 | v9.0.x verify cycle | v9.1 | v9.2+ |
| `EVOLVE_ANCHOR_EXTRACT` | 0 | v9.0.x verify cycle | v9.1 | v9.2+ |

Operators set them via env when running `/evolve-loop`:

```bash
EVOLVE_CACHE_PREFIX_V2=1 EVOLVE_CONTEXT_DIGEST=1 EVOLVE_ANCHOR_EXTRACT=1 /evolve-loop --cycles 3 balanced "<goal>"
```

## Research foundations

The 4-tier architecture explicitly maps to 2026 production-state patterns:

- **Tier 1 (cache)**: Anthropic's "static-content first, dynamic-content
  last" guidance for prompt caching (claude.com/blog/lessons-from-building-
  claude-code-prompt-caching-is-everything).
- **Tier 2 (digest)**: Factory's anchored-summary pattern (4.04 accuracy on
  technical-detail preservation across compression cycles); the "Context
  Dump Fallacy" warning from XTrace and Google Developers blog.
- **Tier 3 (anchored artifacts)**: Hierarchical summarization at sub-task
  completion (Zylos research, AWS Bedrock).
- **Tier 4 (progressive disclosure)**: OpenHands AgentSkills three-layer
  pattern; ACON failure-driven optimization (NeurIPS-track,
  openreview.net/pdf?id=7JbSwX6bNL).

Detailed citations live in `memory/reference_token_optimization_research.md`.

## Runtime-side dataset (cycle 11, post-v9.0.2)

This doc captures STATIC context-floor measurements (input bytes loaded into
each phase's prompt). The complementary *runtime* dataset — per-phase cost,
turn count, cache-create vs cache-read split, and the optimization roadmap
that follows — lives at [`token-economics-2026.md`](token-economics-2026.md).

Headline cycle-11 number: **$6.70 total**, of which cache-creation paid 5×
(once per phase) is ~$2.00 of fixed overhead. Scout (49 turns) and Builder
(58 turns) are the next biggest reduction targets, following the v9.0.2
intent fix pattern.
