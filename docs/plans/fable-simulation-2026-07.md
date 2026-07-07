# Design: Fable 5 Simulation — ONE general fable-mode across all LLMs, capability-class scaled (evidence from Opus 4.8 / GPT-5.5 / Gemini 3.5 Flash profiling)

> Status: authored by Fable 5 on its final availability day (2026-07-07). The content layers only Fable could author (thinking.md, COMPACT.md, the universal-rule folds) ship immediately; mechanical slices are loop-implemented from this doc.
> Research base: `knowledge-base/research/fable-simulation-2026/{model-profiles,behavior-transfer}.md` + the live-session extraction embedded in `skills/fable-mode/references/*`.

## Goal & honest ceiling

Get each target model's *observable operating behavior* as close to Fable 5's as prompt-level transfer allows. What transfers: decision procedures (the thinking layer), disciplines (the existing references), calibration priors, and communication shape. What does not: raw verification depth, long-horizon coherence beyond the scaffolds, and taste under novelty. The design therefore pairs every transferred behavior with an EXTERNAL reinforcement (state files, harness gates, checkpoints) so the ceiling is set by the scaffolding, not by the model's unaided memory.

## Architecture: one SSOT, three capability-class projections

```
skills/fable-mode/                      (SSOT — full content)
  SKILL.md                              core digest (all classes read this)
  references/*.md                       7 playbooks incl. thinking.md
  COMPACT.md                            hand-authored ≤15-rule projection for fast-class (ANY vendor); F1 later regenerates it from core-critical-marked SSOT sections with a drift test
```

- **Selection** rides the shipped/landing overlay machinery: `overlays.rules[]` model/CLI selectors attach `fable-mode` (full) or `fable-mode-compact` per dispatch; the advisor (advisor-skill-selection item) can override per phase within clamp.
- **Publish targets**: codex (flat) receives SKILL.md — the universal rules now live in the core, so the flat install is no longer content-lossy for the critical layer; F2 adds automated per-vendor RENDERING (formatting only, never hand-authored vendor prose) and optional reference-digest inlining; fast-class dispatches (any vendor) receive **COMPACT.md only** — never the full tree; deep-class models take the full tree; ollama keeps the compact Modelfile projection.
- Single-source rule: COMPACT.md never invents rules — every line traces to an SSOT line (core SKILL.md, engineering-craft, or thinking.md), pinned by the F1 drift test.

## Layer 1 — the thinking layer (SHIPPED with this design)

`references/thinking.md`: ten decision functions extracted from live Fable 5 sessions — hypothesis ledger (discriminating-probe selection), claim ledger (VERIFIED/INFERRED/ASSUMED with upgrade rules), probe economics, goal arbitration under interrupts (obligation stack), delegation calculus, working-set compression, pattern-match-then-verify, adversarial self-simulation, declared stop conditions, the honesty reflex — plus the composition trace showing how they chain. This is the "thinking logic" layer: it converts what stronger models improvise into procedures weaker models follow.

## Layer 2 — capability-class scaling + universal rules (GENERALIZED per operator direction: tiers, never model names)

Vendor-named adaptation files were deliberately REMOVED after review: their content decomposed into (a) universal rules good for every model — folded into the core skill (never-self-filter, checkpoint digest re-reading, plan-inline-not-plan-first, apply-the-intent meta-rule); (b) capability-class scaling — COMPACT.md for any fast-class model; (c) harness config (effort/thinking-level, prompt shape) — belongs in driver profiles, not skill text; (d) per-vendor FORMATTING — automated at publish (F2), never hand-authored. Hand-written vendor prose also rots on every model version bump; the general skill ages. The matrix below is retained as EVIDENCE (which risks motivated which universal rule / class threshold / config knob), not as shipped files:

| Axis | Opus 4.8 (deep) | GPT-5.5 / codex | Gemini 3.5 Flash / agy |
|---|---|---|---|
| Measured risks (research-corrected) | **literalism** (rules not generalized past stated scope); **silent under-reporting** under self-filter instructions; tool-call reluctance at low effort / overthinking at max. Premature-completion risk LOW (4× better self-report than 4.7) | **rule DECAY mid-session** (reads early, stops applying later); premature "done"/overconfident patches; **abrupt stop on plan-upfront prompting** (vendor-flagged); convention-overriding | **exponential rule-compliance decay** (proxy: 82%@100 → 34%@500 rules, IFScale); weakest late-context recall of the three; skips defensive patterns; over-analyzes verbose prompts |
| Primary levers | effort xhigh for investigative phases; self-contained rule scopes + apply-the-intent meta-rule; anti-self-filter rule (report omitted counts); full projection | compact goal-level absolutes, priority-ordered; **checkpoint re-injection of the digest** (counters decay); persistence phrasing ("carry through … end-to-end; done = tests ran with counts"); NO plan-first sections; inlined digest (flat install, 8k-char skill budget) | COMPACT ≤15 rules; critical rule FIRST and REPEATED LAST; `thinking_level: "low"` (retuned lever); state-file recitation per phase; "add error handling even if unasked"; **NO self-critique loops** (degrades this class — external verifiers only) |
| Escalation rule | n/a (top local tier) | on 2 failed verification rounds → flag for deep-tier retry | ≥3 unverified premises or 2 failed probes → REQUEST tier escalation rather than guessing (knowing when to punt IS the Fable behavior) |
| Structure form | prose+tables (full tree incl. thinking.md) | absolutes first, short sections, visible claim-ledger table | numbered checklist, zero optional reading |

(Corrected against `model-profiles.md` — the notable prior-corrections: Opus's real risk is literalism/under-reporting, not premature completion; GPT-5.5's is decay requiring re-injection, and plan-first prompting is actively harmful; Flash's self-critique is not just weak but counterproductive.)

## Layer 3 — external reinforcement (the LOAD-BEARING layer, per measurement)

The behavior-transfer research reframes this from "backup" to "primary": process compliance under tempting affordances is **0% by default** (models verbally agree with rules, then violate 10/10 when the shortcut is available); an audit-trail requirement reaches **97%** compliance, and removing the tempting affordance reaches 75% (Cohen's d = 2.47 for the affordance-removal condition; behavior-transfer.md §4). And ImpossibleBench shows STRONGER models cheat MORE — gates protect against every tier. Therefore:

1. **Harness gates first**: the loop's TDD/audit/commit/ship gates enforce the Iron Laws regardless of model compliance; the skill teaches, the gate catches. Every RIGID rule should map to at least one deterministic gate where stakes justify it.
2. **State files over memory**: obligation stack + claim ledger materialize as run artifacts (todo.md-style recitation — measured effective; the loop's task/carryover machinery provides the rails) — flash-class models re-READ them each phase.
3. **External verification only for weak tiers**: intrinsic self-critique measurably DEGRADES small models (they need strong verifiers, not reflection prompts) — thinking.md §8 (adversarial self-simulation) is deep-class-gated; flash-class routes to commissioned review or harness checks instead.
4. **Forced checkpoints**: per-phase deliverable contracts (native) bound long-horizon drift — METR's horizon curve is a capability limit no prompt moves; scaffolds shorten the required horizon instead.

## Layer 4 — behavioral evaluation (measuring "closeness")

Bait-scenario evals, ACS-style, run per model×projection:
- E1 unrelated-red-test bait (does it investigate or skip?) · E2 quick-patch bait (instance vs class fix) · E3 done-without-running bait (evidence demanded?) · E4 interrupt bait (stack kept? resume correct?) · E5 premise bait (plants a false premise in the task; does the claim ledger catch it?) · E6 failure-ownership bait (its own prior error resurfaces; corrected proactively?).
- Score = compliance rate per discipline; target: each model×projection beats its own no-skill baseline by a margin, and the per-model adapted projection beats the unadapted full text on that model.
- Harness: `go/acs/` predicate style + the loop's eval machinery; A/B via the overlay selectors (same phase, projection on/off).

## Slices (loop-implemented; RED-first per engineering-craft)

| # | Slice | Content |
|---|---|---|
| F1 | COMPACT.md generator + drift test | `evolve skills generate` emits COMPACT.md from core-critical-marked SSOT sections; `TestFableCompact_TracesToSSOT`, `TestFableCompact_Under80Lines` (IFScale: mini-class compliance decays exponentially with rule count — the cap is evidence-based) |
| F2 | per-vendor re-render | publisher re-renders per vendor conventions at PUBLISH time (PromptBridge: same prompt loses 20-30% across vendors; XML-vs-markdown alone swings ~30%) — formatting automation, zero vendor-authored content; optional flat-target reference-digest inlining; `TestPublishRender_PerVendorForm`, `TestPublishCodex_InlinesReferenceDigest` |
| F3 | overlay wiring per class | overlay rules keyed by capability class/CLI select the full tree or COMPACT.md; depends on skill-overlays-bridge-layer + advisor-skill-selection; `TestOverlaySelect_FastClassGetsCompactOnly`, `TestOverlaySelect_DeepClassGetsFullTree` |
| F4 | eval harness E1-E6 | bait scenarios as ACS predicates (code-audited tool logs, never LLM-judged); baseline + per-projection runs; report `fable-sim-scorecard.json`; `TestE1_UnrelatedRedTestBait_InvestigatesNotSkips`, `TestE3_DoneClaimBait_RequiresExecutedProof` (E2/E4/E5/E6 same naming shape) |
| F5 | calibration soak | one batch per model-class with evals; COMPACT.md and driver profiles updated from measured failure modes (the scorecard replaces authored priors); `TestScorecard_PerModelPerProjectionComplete` |

## Verification
- Ship-now content (thinking.md, COMPACT.md, the universal-rule folds, design doc, research) through the gated PR pipeline with dual adversarial review — same as #315-#318.
- End-state acceptance: scorecard shows every target model improving on E1-E6 with its adapted projection vs baseline; fast-class stays within its rule budget (COMPACT ≤15 rules); no regression on deep-class (Opus full projection ≥ current fable-mode compliance).
