# Cycle 441 Dossier

**Goal:** STATUS (MR1-MR3 LANDED on main, PR #293): the model_routing axis (config.ModelRouting static|advisory|auto, orthogonal to Stage/Mode), router.PhasePlanEntry optional {cli,tier}, router.ClampPlanModelRouting (validate+clamp, UNWIRED), and phase_advisor.writeCard guardrail projection are ALL ON MAIN. Do NOT re-implement them. REMAINING: MR4 + MR5.

MR4 (the slice that makes auto FUNCTIONAL) MUST:
  (a) PROJECT + APPLY: build the phase->overlay map once, thread each running phase's {cli,tier} into PhaseRequest, and in phases/runner/runner.go apply it as a SOFT overlay (chain primary + tier, pin==nil so a benched CLI still falls back), validated by policy.ValidatePin. auto=apply, advisory=log-only, static=noop. Wire ClampPlanModelRouting into this path (model proposes, kernel disposes).
  (b) FIX THE DEFERRED go-reviewer MEDIUM: in ClampPlanModelRouting, normalize the CLI BASE NAME before catalog.Lookup — a raw suffixed name (e.g. "claude-tmux") passes policy.ValidatePin (which strips internally) but MISSES the catalog, wrongly clamping a valid {cli,tier}. Consolidate the TWO duplicated unexported helpers (policy.baseCLI policy.go:384 + bridge.baseCLIName catalog_overlay.go:116) into ONE exported base-name function (single-source) and use it in the clamp.
  (c) DEGRADE PATH: auto authoritative ONLY when a plan was produced; failed/absent advisor => profile-static per phase.
  (d) DEFAULT FLIP (user mandate): once auto is functional, set model_routing DEFAULT to `auto` in the checked-in .evolve/policy.json (config-driven) so ALL loops get advisor-authoritative routing by default; off/static are the escape hatch.
MR5: catalog C1 audit (route ollama list + classifier through the bridge if direct-exec; R3 refresh already exists — reuse).

SCOPE: the ADVISOR owns the STRUCTURE of each phase agent — its existence (mint/run/skip), its driver (CLI), its model level (tier: fast/balanced/deep), and its composition/persona. The mint mechanism (router.MintSpec) already shapes structure for NEW phases; extend that authority to EXISTING phases. {cli,tier} below are the first concrete levers. PREREQUISITE (ALREADY LANDED on main, PR #292): advisor-dispatch-resilience shipped — advisorLaunch now walks its cli_fallback chain (agy->claude) via llmroute.ChainFor+Dispatch, so the advisor no longer degrades every cycle and auto mode can actually route. Do NOT re-implement it; build the model_routing axis ON TOP of the now-reliable advisor.

Give the ADVISOR (dynamic-routing path) authority over BOTH the LLM CLI and the abstract model TIER (fast/balanced/deep) for each phase, under a NEW "auto" model-routing mode. Delivered as small cycle-sized SLICES, strict TDD (red->green->refactor), clean code, design patterns, `go test -race` green each slice, every new exported symbol named in a _test.go AST (apicover -enforce hard-fails main otherwise), no regression to any wedge/floor invariant.

WHY: today per-phase TIER is 100% static-from-profile (.evolve/profiles/*.json:model_tier_default via resolvellm.go); the advisor influences neither cli nor tier for EXISTING phases (it emits {cli,tier} only for MINTED phases via router.MintSpec, invariant "advisor emits a TIER, never a raw model"). The runner receives nothing from the advisor; the only per-phase authoritative overlay above the profile is the policy pin (policy.ValidatePin). Routing modes today = two orthogonal axes (config.Stage off/shadow/advisory/enforce = sequencing authority; config.Mode DynamicLLM/StaticPreset = brain); NEITHER governs model authority — "auto" is genuinely new.

HARD CONSTRAINTS: (1) ALL LLM-CLI control goes through the agent-bridge abstraction — no direct exec to reach a model. (2) Driver-agnostic: advisor emits an abstract TIER, never a vendor model; tier->concrete model is modelcatalog.Lookup(cli,tier) SSOT; a (cli,tier) with no catalog model must CLAMP, never dispatch. (3) Profile guardrails allowed_clis + model_tier_envelope{min,default,max} are kernel-enforced (policy.ValidatePin) — advisor proposes, kernel disposes. (4) Cross-family = PREFERENCE (WARN, never kernel-reject). (5) Config-driven, no feature-flag sprawl. (6) DEGRADE PATH (load-bearing — advisor currently fails exit=81 every cycle): auto is authoritative ONLY when a plan was produced; a failed/absent advisor falls back to profile-static PER PHASE; auto must never turn an advisor outage into a broken dispatch.

SLICES (order MR1->MR2->MR3->MR4; MR5 independent):
  MR1 SCHEMA+PROMPT (router/router.go, core/phase_advisor.go): add optional CLI/Tier to router.PhasePlanEntry (router.go:183) mirroring MintSpec; buildPlanPrompt/composePlanPrompt request per-phase tier+cli AND project each phase's allowed_clis + model_tier_envelope into the prompt so the advisor proposes in-bounds. Absent fields => byte-identical to today.
  MR2 CLAMP AT FLOOR (router/floor.go via ReplayPlanFromResponse phase_advisor.go:882): validate each entry's {cli,tier} against profile guardrails reusing policy.ValidatePin; emit router.Clamp on out-of-envelope / disallowed-CLI / modelcatalog.Lookup miss instead of honoring.
  MR3 AUTHORITY AXIS (config.go, policy.go): NEW orthogonal model_routing = static|advisory|auto, parsed from .evolve/policy.json + env override mirroring dynamic_routing (config.go:507/:552); threaded to core. Does NOT touch the dynamic_routing sequencing axis. static=profile+pin only (today); advisory=advisor proposes {cli,tier} logged to routing-decision, profile authoritative; auto=clamped {cli,tier} applied.
  MR4 PROJECTION+APPLY+DEGRADE (core/decision_branch.go, core/cyclerun_dispatch.go:94, phases/runner/runner.go:366-416): build phase->overlay map once at cycle start; thread the running phase's entry into PhaseRequest; runner applies it as a SOFT advisor overlay (sets chain primary + tier, leaves pin==nil so a benched advisor-chosen CLI still falls back via cli-health) validated by the same policy.ValidatePin. Implement the degrade path above.
  MR5 CATALOG C1 AUDIT (modelquery/ollama.go, modelquery/classifier.go): the catalog refresh (evolve models refresh) ALREADY queries each CLI live and caches tier->latest model — REUSE, do not rebuild. Audit that EVERY CLI-reaching call in the refresh path goes through the bridge: the /model picker capture already does (bridge.CaptureModelPicker); route `ollama list` and the classifier CLI run through the bridge abstraction if they are direct exec. Optional: expose catalog.ttl_hours policy knob for fresher-than-24h updates.

VERIFY: CLI x phase x tier matrix (extend TestAllProfilesSubstitutabilityAtParity) — all four CLIs x three tiers dispatch a concrete model, unsupported (cli,tier) clamps; mode semantics via routingeval golden corpus (static byte-identical, advisory logs-not-applies, auto applies+clamps); simulated advisor failure => profile-static per phase; catalog refresh path has zero direct CLI exec; go test -race + apicover -enforce clean on touched packages.

DEFAULT MANDATE (user directive): model_routing MUST DEFAULT to `auto` — advisor-authoritative dynamic routing is ON by default for ALL loops, not opt-in. Set the default explicitly in the checked-in .evolve/policy.json (config-driven, NOT a Go literal, per phase-settings-from-config); `off`/`static` remain the escape hatch. The safe per-phase degrade (failed advisor -> profile-static) makes auto-default safe now that advisor-dispatch-resilience landed. ACCEPTANCE: a fresh `evolve loop` with no routing override runs model_routing=auto (assert via a config-default test + `evolve` config echo).
**Final verdict:** FAIL
**Run ID:** 01KWFFNWZWDCR2JW1H2WMT33HM

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| cycle-recorded |  | FAIL |  | cycle completed; ledger walk deferred to future slice |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 441

