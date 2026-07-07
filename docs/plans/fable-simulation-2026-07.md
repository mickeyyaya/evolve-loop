# Design: Fable 5 Simulation — per-model optimization of fable-mode for Opus 4.8, GPT-5.5, Gemini 3.5 Flash

> Status: authored by Fable 5 on its final availability day (2026-07-07). The content layers only Fable could author (thinking.md, adaptation judgments) ship immediately; mechanical slices are loop-implemented from this doc.
> Research base: `knowledge-base/research/fable-simulation-2026/{model-profiles,behavior-transfer}.md` + the live-session extraction embedded in `skills/fable-mode/references/*`.

## Goal & honest ceiling

Get each target model's *observable operating behavior* as close to Fable 5's as prompt-level transfer allows. What transfers: decision procedures (the thinking layer), disciplines (the existing references), calibration priors, and communication shape. What does not: raw verification depth, long-horizon coherence beyond the scaffolds, and taste under novelty. The design therefore pairs every transferred behavior with an EXTERNAL reinforcement (state files, harness gates, checkpoints) so the ceiling is set by the scaffolding, not by the model's unaided memory.

## Architecture: one SSOT, three capability-class projections

```
skills/fable-mode/                      (SSOT — full content)
  SKILL.md                              core digest (all classes read this)
  references/*.md                       7 playbooks incl. thinking.md
  adaptations/
    opus.md        (deep-class:     anti-overdeliberation addenda)
    gpt.md         (codex-class:    evidence-gate hardening + inlined playbook digests for flat installs)
    flash.md       (fast-class:     compact absolutes + scaffolds + escalation rule)
  COMPACT.md                            generated ≤80-line projection for flash-class (single source: generated from SKILL.md sections marked core-critical; drift test pins it)
```

- **Selection** rides the shipped/landing overlay machinery: `overlays.rules[]` model/CLI selectors attach `fable-mode` (full) or `fable-mode-compact` per dispatch; the advisor (advisor-skill-selection item) can override per phase within clamp.
- **Publish targets**: codex (flat) receives SKILL.md with gpt.md digest INLINED by the publisher (slice F2) — the flat-namespace gap stops being silent content loss; Flash-class dispatches via agy receive **COMPACT.md + flash.md only** (flash.md alone until F1 ships the generator) — never the full tree, per flash.md's own hard constraint; deep-class agy models may take the full tree; ollama keeps the compact Modelfile projection.
- Single-source rule: adaptations never restate core rules — they modulate (emphasis, ordering, added gates), and a drift test asserts every COMPACT.md line traces to an SSOT line.

## Layer 1 — the thinking layer (SHIPPED with this design)

`references/thinking.md`: ten decision functions extracted from live Fable 5 sessions — hypothesis ledger (discriminating-probe selection), claim ledger (VERIFIED/INFERRED/ASSUMED with upgrade rules), probe economics, goal arbitration under interrupts (obligation stack), delegation calculus, working-set compression, pattern-match-then-verify, adversarial self-simulation, declared stop conditions, the honesty reflex — plus the composition trace showing how they chain. This is the "thinking logic" layer: it converts what stronger models improvise into procedures weaker models follow.

## Layer 2 — per-model adaptations (matrix; cells refined from model-profiles research)

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
| F2 | per-vendor re-render + codex inlining | publisher re-renders per vendor conventions (PromptBridge: same prompt loses 20-30% across vendors; XML-vs-markdown alone swings ~30%) and inlines the gpt.md digest for the flat codex target; `TestPublishCodex_InlinesAdaptation`, `TestPublishRender_PerVendorForm` |
| F3 | overlay wiring per class | overlay rules keyed models/CLI select full vs compact + adaptation; depends on skill-overlays-bridge-layer + advisor-skill-selection; `TestOverlaySelect_FlashClassGetsCompactPlusAdaptation`, `TestOverlaySelect_DeepClassGetsFullTree` |
| F4 | eval harness E1-E6 | bait scenarios as ACS predicates (code-audited tool logs, never LLM-judged); baseline + per-projection runs; report `fable-sim-scorecard.json`; `TestE1_UnrelatedRedTestBait_InvestigatesNotSkips`, `TestE3_DoneClaimBait_RequiresExecutedProof` (E2/E4/E5/E6 same naming shape) |
| F5 | calibration soak | one batch per model-class with evals; adaptation files updated from measured failure modes (the scorecard replaces authored priors); `TestScorecard_PerModelPerProjectionComplete` |

## Verification
- Ship-now content (thinking.md, adaptations/, design doc, research) through the gated PR pipeline with dual adversarial review — same as #315-#318.
- End-state acceptance: scorecard shows every target model improving on E1-E6 with its adapted projection vs baseline; flash-class stays within its token budget (COMPACT ≤80 lines); no regression on deep-class (Opus full projection ≥ current fable-mode compliance).
