# Dynamic Model Routing (Multi-CLI Adaptive Allocation)

> **Target path:** `docs/architecture/dynamic-model-routing.md` (in evolve-loop repo)
> **Status:** Phase 0 (static baseline) staged 2026-05-17. Phases 1–5 pending batch boundary.
> **See also:** [model-routing.md](../reference/model-routing.md) (operator reference table), [multi-llm-review.md](multi-llm-review.md) (cross-CLI audit), [capability-schema.md](capability-schema.md) (agent capability declarations).

## Problem

The pipeline today picks per-phase models via `.evolve/llm_config.json` — a flat phase→model map. This has three structural defects:

1. **Flat allocation can't adapt to task difficulty.** A trivial bug fix pays the same as a security-critical refactor. The Intent persona already classifies `risk_level` and `awn_class` per cycle; those signals are thrown away today.
2. **Cross-family integrity is documentation-only.** `CLAUDE.md` mandates "Auditor defaults to Opus (different family from Builder's Sonnet) to break same-model-judge sycophancy," but no code enforces it. Both Builder and Auditor can collapse to the same family/model with no kernel-level block.
3. **Multi-CLI support is one-dimensional.** The current overlay can be "all Claude" or "all Gemini," but cannot mix (e.g. Claude Sonnet for Builder + Gemini-3.1-Pro for Auditor) — which gives cross-family integrity its strongest physical guarantee.

## Goal

Per-cycle adaptive routing that:

1. Picks the cheapest **capable** model for each phase based on intent signals.
2. Enforces a per-phase tier envelope (`min` ≤ `chosen` ≤ `max`) in the trust kernel.
3. Supports mixing CLIs across phases within a single cycle.
4. Is **deterministic by default** — a small bash/jq function reads structured signals, not an LLM.
5. Permits a **constrained LLM override** at the orchestrator level (UP only, within envelope, one phase per cycle, reason logged).

## Non-goals

- Replacing `llm_config.json` as a manual operator override hook.
- Routing decisions based on arbitrary natural-language reasoning (`gpt-pick-the-model`).
- Auto-purchasing new vendor API quota or switching providers mid-cycle.
- Cross-cycle model migration (e.g., switching mid-Builder from Sonnet to Opus). Tier is fixed at phase start.

## Architecture (5 layers)

```
1. PROFILE ENVELOPE          (.evolve/profiles/<phase>.json)
   model_tier_envelope: { min, default, max }
   allowed_clis: [...]
   cross_family_with: "<other-phase>"

2. DETERMINISTIC DECISION    (scripts/routing/decide-cycle-routing.sh)
   Reads: intent.md + state.json + retro lessons
   Writes: .evolve/runs/cycle-N/cycle-routing.json

3. ORCHESTRATOR OVERRIDE     (agents/evolve-orchestrator.md)
   May shift ONE phase UP by ONE tier, in-envelope, logged
   Cannot shift DOWN (conservative bias)

4. TRUST KERNEL ENFORCEMENT  (scripts/routing/envelope-check.sh)
   Validates: (cli, tier) ∈ profile.envelope
   Validates: builder.cli.family ≠ auditor.cli.family
   Out-of-envelope → rc=2

5. CLI TIER TRANSLATION      (scripts/routing/tier-map.json)
   deep / balanced / fast → concrete model per CLI
```

### Layer invariants

- **Layers 1, 2, 5 are pure data + pure functions.** Reproducible, reviewable, no side effects.
- **Layer 3 is the only LLM call.** Bounded by Layer 1 envelopes and Layer 4 kernel.
- **Layer 4 is below the persona layer.** Cannot be disabled from prompts.

## CLI-agnostic capability tiers

| Tier | Cognitive demand | Use cases |
|---|---|---|
| **deep** | Adversarial review, post-mortem, multi-lens verdicts, RED-phase test design, goal interpretation | Auditor, Retrospective, Plan-reviewer, TDD-engineer, Intent |
| **balanced** | Code generation, structured scan, predicate authorship, procedural sequencing | Builder, Scout, Tester, Orchestrator, Inspirer |
| **fast** | Summarization of pre-structured artifacts, rubric scoring, pattern-matching | Memo, Evaluator, Triage |

Tier map (latest CLI models, May 2026):

| Tier | Claude | Gemini | Codex | Grok |
|---|---|---|---|---|
| **deep** | `opus` → `claude-opus-4-7` | `gemini-3.1-pro-preview` + `thinkingLevel=high` | `gpt-5.5` + `reasoning=high` | `grok-4-heavy` |
| **balanced** | `sonnet` → `claude-sonnet-4-6` | `gemini-3-pro-preview` + `thinkingLevel=medium` | `gpt-5.3-codex` | `grok-4-3` |
| **fast** | `haiku` → `claude-haiku-4-5-20251001` | `gemini-3-flash-lite-preview` | `gpt-5.4-mini` | `grok-4-20-non-reasoning` |

## Per-phase envelope

| Phase | min | default | max | cross_family_with | allowed_clis |
|---|---|---|---|---|---|
| memo | fast | fast | balanced | — | all |
| evaluator | fast | fast | balanced | — | all |
| triage | fast | balanced | balanced | — | all |
| orchestrator | balanced | balanced | balanced | — | all |
| scout | balanced | balanced | deep | — | all |
| builder | balanced | balanced | deep | **auditor** | claude only† |
| tester | balanced | balanced | balanced | — | claude only† |
| inspirer | balanced | balanced | deep | — | all |
| intent | deep | deep | deep | — | all |
| plan-reviewer | balanced | deep | deep | — | all |
| tdd-engineer | balanced | deep | deep | — | claude only† |
| auditor | deep | deep | deep | **builder** | all |
| retrospective | balanced | deep | deep | — | all |

† Phase-4-pending: Gemini/Codex/Grok adapters need sandbox + permission scoping + budget cap parity with Claude before Builder/Tester/TDD `allowed_clis` extends beyond `claude`.

## Decision-function signal table

| Signal source | Field | Rule |
|---|---|---|
| `intent.md` | `risk_level: critical` | Bump Builder + Auditor + TDD to envelope.max |
| `intent.md` | `risk_level: low` AND `awn_class: CLEAR` | Drop Triage + Scout toward envelope.min |
| `intent.md` | `awn_class ∈ {IMKI, IMR, IBTC}` | Bump Plan-reviewer to `deep` |
| `intent.md` | `challenged_premises.length >= 3` | Pre-bump Retrospective to `deep` |
| `intent.md` | `interfaces.length >= 4` | Bump Builder to `deep` |
| `state.json` | `failedApproaches[]` matches current intent fingerprint | Bump Builder + TDD to `deep` |
| `state.json` | `fitnessRegression == true` | Bump Auditor to envelope.max |
| `state.json` | `mastery.consecutiveSuccesses >= 5` | Allow downshift to `fast` where envelope permits |
| `state.json` | `carryoverTodos[HIGH].length >= 2` | Bump Builder to `deep` |
| (retro lesson) | `routing-upgrade` tag matching intent fingerprint | Apply lesson's recommended tier |

Bias: **conservative on uncertainty**. Multiple signals → take max. Orchestrator override is UP-only.

## Cross-family invariant enforcement

1. Profile declares: `builder.cross_family_with: "auditor"` and vice versa.
2. Decision function honors: when picking Auditor's CLI, excludes Builder's family from `allowed_clis`.
3. Envelope check enforces: `role-gate.sh` calls `envelope-check.sh` before spawning Auditor; refuses if `family(auditor.cli) == family(builder.cli)`.
4. Degraded fallback: when only one family installed, Auditor uses different MODEL within same family with logged WARN.

## Cost projection

| Allocation | Easy | Medium | Hard | Avg @ 30/50/20 | Cross-family |
|---|---|---|---|---|---|
| Sonnet-centric (pre-Phase-0) | $1.36 | $1.36 | $1.36 | $1.36 | ❌ |
| All-Opus (current recovery) | $4.83 | $4.83 | $4.83 | $4.83 | ❌ |
| Phase 0 static 3-tier | $2.50 | $2.50 | $2.50 | $2.50 | partial |
| Phase 3 adaptive (this design) | $1.80 | $2.50 | $3.80 | $2.55 | ✓ (Phase 4) |

The win is in **distribution**, not average. Easy cycles 28% cheaper, hard cycles 50% more expensive — but easy cycles aren't where quality matters, and hard cycles are where the marginal `deep` tier prevents failure cascades.

## Rollout phases

| Phase | Scope | Outcome |
|---|---|---|
| **0** | Replace `llm_config.json` with 3-tier baseline. Staged at `.proposed.json`; atomic rename at batch boundary. | ~40% cost drop vs all-Opus. Tier differentiation restored. |
| **1** | Add `model_tier_envelope` + `allowed_clis` + `cross_family_with` to all 13 profiles. Advisory mode (WARN). | Schema in place. No behavior change. |
| **2** | Build `decide-cycle-routing.sh` + `tier-map.json` + `envelope-check.sh`. Wire into `run-cycle.sh` after Intent. Advisory (write but don't consume). | Validation phase. |
| **3** | Cutover. `subagent-run.sh` consumes `cycle-routing.json`. `role-gate.sh` enforces envelope + cross-family. Orchestrator override layer. | Adaptive routing live. |
| **4** | Gemini/Codex/Grok adapter parity (sandbox, permission scoping, budget cap). Lift `claude only†`. | True cross-family integrity. |
| **5** | Retro lessons emit `routing-upgrade` tags mutating envelopes. | Self-tuning routing. |

## Invariants preserved

| Invariant | How |
|---|---|
| Bash 3.2 portability | No `declare -A`, `mapfile`, GNU-only flags. Atomic writes via `mv` of `.tmp.$$`. |
| Trust kernel boundary | Envelope enforcement is separate gate, NOT a kernel bypass. |
| Audit-binding | `cycle-routing.json` SHA-bound in `ledger.jsonl`. |
| Single source of truth | `tier-map.json` is the only file containing concrete CLI model IDs. |
| Backward compatibility | Profiles without `model_tier_envelope` fall back to `{ min: balanced, default: model_tier_default, max: deep }`. |
| Determinism + reproducibility | `intent.md` + `state.json` + `tier-map.json` + function version → unique decision. |
| Sequential write discipline | Decision function is single-writer. Orchestrator override is single-writer. |

## Risks

| Risk | Mitigation |
|---|---|
| Decision function false-negative on difficulty | Conservative bias: any signal pushes UP; multiple → take max. Orchestrator UP-only. Retro reviews FAILs with min-tier routing. |
| Cross-family enforcement breaks with single-CLI install | Documented degraded mode: different MODEL within same family + WARN. Phase 4 unblocks structurally. |
| `tier-map.json` IDs go stale | Versioned via `schema_version`. CI smoke test probes models quarterly. |
| 3P CLIs lack effort/thinkingLevel knobs | Function emits `tier` + `effort` separately; adapters consume what they support. |
| Orchestrator override becomes sycophancy vector | UP-only, max 1 per cycle, reason required, retro reviews every override. |
| Phase 0 changes mid-batch | Staged at `.proposed.json`. Atomic rename ONLY at batch boundary. |
| Routing logic adds latency | Bash+jq ~50ms; orchestrator override ~$0.05 marginal. Net <10s/cycle. |
| Operator-pinned llm_config fights decision function | Operator wins: order is `llm_config > cycle-routing.json > profile.default`. |

## References

- [model-routing.md](../reference/model-routing.md), [multi-llm-review.md](multi-llm-review.md), [capability-schema.md](capability-schema.md), [intent-phase.md](intent-phase.md), [phase-architecture.md](phase-architecture.md), [control-flags.md](control-flags.md)
- Plan file: `~/.claude/plans/expressive-bouncing-owl.md`
