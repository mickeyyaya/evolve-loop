# Cycle 442 Dossier

**Goal:** STATUS — MR1-MR4 ALL LANDED ON MAIN. Do NOT re-implement them. This cycle is MR5 ONLY (the last slice of the model-routing campaign).
  - MR1-MR3 (PR #293): model_routing axis (config.ModelRouting static|advisory|auto, orthogonal to Stage/Mode), router.PhasePlanEntry optional {cli,tier}, router.ClampPlanModelRouting, phase_advisor guardrail projection.
  - MR4 (cycle 440, commit a5df0a34, main CI green): projection+apply (soft overlay in phases/runner/runner.go, pin==nil so a benched CLI still falls back), ClampPlanModelRouting wired, base-name normalization consolidated, degrade path (failed/absent advisor => profile-static per phase), AND the DEFAULT FLIP — model_routing now defaults to `auto` via the checked-in docs/architecture/phase-registry.json ("model_routing":"auto"), verified by TestCheckedInPolicyDefaultsModelRoutingAuto. auto is ON by default for all loops; static/off are the escape hatch.
  If triage or scout proposes any MR1-MR4 work, that is a REDUNDANT re-pick (the cycle-441 failure mode) — DROP it. Pick ONLY the MR5 catalog C1 audit below.

GOAL: MR5 — CATALOG C1 AUDIT. Constraint C1 (a HARD, load-bearing invariant): ALL LLM-CLI control that REACHES A MODEL must go through the agent-bridge abstraction — no direct exec.Command(<cli>, ...) that sends a prompt to a model. The model-catalog live-refresh path (`evolve models refresh --source live`, auto-run at cycle start) queries each CLI for its current models; audit that EVERY model-reaching call in that path is bridge-routed. R3 (the refresh machinery itself) ALREADY EXISTS and is production-grade — REUSE it, do NOT rebuild. Two concrete gaps found by inspection:

  GAP 1 (HARD C1 VIOLATION — must fix): go/internal/modelquery/classifier.go:35-36 runs `run(ctx, name, args, "")` where classifierArgv returns e.g. ("codex",["exec","--skip-git-repo-check",prompt]) / ("agy",["-p",prompt]) / (cli,["-p",prompt]). This is a DIRECT exec that sends a classification PROMPT to a LIVE MODEL, bypassing the bridge (no sandbox, no liveness probe, no cli_fallback). Route this model-reaching classification dispatch through the SAME bridge abstraction the picker capture already uses (bridge.CaptureModelPicker / the llmroute.Dispatch seam) — driver-agnostic (any CLI). After the fix, there must be ZERO direct model-reaching exec in the refresh path.

  GAP 2 (DECISION — not an automatic fix): go/internal/modelquery/ollama.go:23 runs `run(ctx,"ollama",["list"],"")`. This is a direct exec but METADATA-ONLY — it enumerates locally-installed models; it reaches NO model, sends no prompt, consumes no tokens. C1 governs MODEL-DISPATCH control, which `ollama list` is not. DECIDE (plan-reviewer confirms): either (a) route it through the bridge for uniformity, OR (b) document it as an explicit, TESTED non-model metadata exception (a directory-listing does not fit a model-dispatch abstraction). RECOMMEND (b): add a clear comment at the call site stating it is metadata-only + not model-reaching, and a test asserting it dispatches no prompt/model. Do NOT force a model-dispatch abstraction onto a metadata enumeration if that is the only reason.

CONSTRAINTS: strict TDD (RED first — write the failing C1 test that proves the classifier prompt currently bypasses the bridge, then GREEN), clean code, design patterns (reuse existing bridge seams; no new parallel dispatch path), `go test -race` green, every NEW exported symbol named in a _test.go AST (apicover -enforce hard-fails main otherwise), no regression to any wedge/floor/routing invariant. Driver-agnostic: the bridge routing must work for every CLI classifierArgv can name.

ACCEPTANCE:
  - A test asserts the tier-classifier's classification prompt is dispatched THROUGH the bridge abstraction (not a raw exec.Command / injected direct runner).
  - The Gap-2 ollama-list decision is implemented and covered by a test (either bridge-routed, or documented-metadata-exception with a test asserting no model is reached).
  - No direct model-reaching exec remains in the refresh path (assert by test or a grep-style check over modelquery).
  - `go test -race ./internal/modelquery/... ./internal/bridge/...` green; `apicover -enforce` clean on every touched enforced package.

OPTIONAL (only if it fits cleanly in the same cycle): expose `catalog.ttl_hours` as a policy.json knob (config-driven, no Go literal) for fresher-than-24h catalog refresh; default preserves today's 24h TTL byte-for-byte when the key is absent.
**Final verdict:** PASS
**Run ID:** 01KWFNFG8EV7JJNGQGSRMECQP5

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| cycle-recorded |  | PASS |  | cycle completed; ledger walk deferred to future slice |
