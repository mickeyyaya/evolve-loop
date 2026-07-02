# Cycle 447 Dossier

**Goal:** GOAL: MODEL-SWITCH THROUGH THE ABSTRACT LAYER FOR EVERY LLM CLI (user directive 2026-07-02: "make sure we can translate the model command through the abstract layer to different LLM CLIs to switch to the target model"). Build with strict TDD (red->green->refactor), clean code, design patterns (reuse the existing Realizer channel Strategy — NO new parallel dispatch path).

AUDIT ALREADY DONE (verified file:line — do NOT redo the audit, BUILD the fix):
The abstraction EXISTS: the Realizer (ADR-0022, go/internal/bridge/realizer.go) translates intent.ModelTier per CLI via declarative manifest channels:
  - `flag`: claude-tmux `--model` + codex-tmux `-m` — both PROVEN in production (cycle-444 launched every claude phase with `--model fable`).
  - `repl`: Template '/model {alias}' — realizeScalar (realizer.go:212-215) emits REPLInput, seeded post-boot at driver_tmux_repl.go:340 — IMPLEMENTED BUT UNUSED by any manifest today.
  - ollama-tmux: channel:noop BY DESIGN — model is the POSITIONAL arg composed by ollamaComposeLaunchCmd (driver_ollamatmux.go:104, pinned by launch-cmd tests). WORKS; leave functional. Optionally formalize as a declarative 'positional' channel ONLY if it genuinely simplifies — no churn.
  - Tier->concrete resolution: manifest model_tier_map overlaid by the LIVE catalog (go/internal/bridge/catalog_overlay.go applyCatalogTierMap; only source=="live" overrides).
  - The 'auto' sentinel guard (realizer.go:204-206, cycle-262) MUST stay: unresolved tier => omit the model param entirely, never pass 'auto' to any CLI.

THE GAP — agy-tmux CANNOT switch models through the abstraction (the ONE broken cell in the matrix):
  1. go/internal/bridge/manifests/agy-tmux.json params.model_tier = {channel: "noop"} => intent.ModelTier is silently DROPPED for agy. The advisor/profile can demand tier=deep and agy launches its default model regardless.
  2. agy-tmux.json model_tier_map is FLAT (fast=balanced=deep=gemini-3.5-flash) while agy's LIVE catalog entry (source=live, .evolve/model-catalog.json) has real tiers: fast='Gemini 3.5 Flash (Low)', balanced='Claude Sonnet 4.6 (Thinking)', deep='Claude Opus 4.6 (Thinking)'. The catalog overlay would supply these the moment the channel is non-noop.

REQUIRED WORK:
A. PROBE agy's model-switch affordances through the bridge: does the agy CLI accept a launch flag (`agy -m <id>` / `--model <id>`)? does its REPL accept a direct '/model <name>' command, or is the /model picker arrow-key-interactive only? (go/internal/modelquery/picker.go + bridge.CaptureModelPicker ALREADY drive that picker to LIST models — the same machinery can SELECT.) Record the probe findings in the build report.
B. WIRE the best channel for agy (preference order, least-code-first):
   (1) launch flag exists => params.model_tier {channel:"flag", flag:"<verified>", from:"model_tier_map"};
   (2) else direct REPL command => {channel:"repl", template:"<verified '/model {alias}' form>"} reusing the existing repl channel + REPL seed path (this also gives the repl channel its FIRST production consumer — add the missing seed-path test);
   (3) else (picker-only) extend the picker driver to SELECT a model by resolved name — through the bridge abstraction in every case.
C. FIX agy's manifest model_tier_map to real distinct per-tier model ids that the CLI ACCEPTS AS SELECTABLE TOKENS. The catalog display names (e.g. 'Claude Opus 4.6 (Thinking)') may not be the selectable token — verify the accepted form; if display!=token, add the normalization at the modelquery/picker seam so the catalog stays SSOT (single-source-with-projection; never duplicate the mapping).
D. MATRIX PIN (the "make sure" part): a parity test asserting EVERY tmux driver manifest translates intent.ModelTier through SOME effective channel — flag emits flag+value, repl emits REPLInput, ollama's positional path stays pinned by its existing launch-cmd tests — and NO driver whose catalog offers >1 model has channel:noop. Plus Realize() unit tests for the chosen agy channel (tier resolves via overlay; 'auto' sentinel still omits; empty/unknown tier emits nothing), and an integration-style test that a dispatched agy launch carries the resolved deep-tier model.
E. DOCS: add the per-CLI channel table (flag/repl/positional) to docs/architecture/model-discovery-and-catalog.md so the translation matrix is documented.

CONSTRAINTS: all model-reaching control through the agent-bridge (C1 — no direct exec to a model); driver-agnostic (abstract tier vocabulary fast/balanced/deep everywhere; concrete ids ONLY in manifest/catalog); `go test -race` green on touched packages; apicover -enforce clean (every NEW exported symbol named in a _test.go AST — the recurring gate); NO regression to claude/codex/ollama translation (their existing tests keep passing byte-identical); the 'auto' sentinel guard preserved; config/data changes in manifests (declarative), Go changes minimal and behind existing seams.

OUT OF SCOPE: mid-session model switching of an already-running pane — the pipeline launches a fresh session per phase; LAUNCH-TIME translation is the contract. (If the repl channel is chosen it happens to run post-boot in the same session — that satisfies the contract; do not build pane-hot-swap beyond it.)

ACCEPTANCE: the CLI x tier matrix has zero silent-noop cells for multi-model CLIs; agy dispatched at tier=deep demonstrably runs its catalog deep model; all existing model-translation tests green; new tests red-first then green; docs updated.
**Final verdict:** FAIL
**Run ID:** 01KWG8AFD8B18MC3Q20708Q1BR

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| cycle-recorded |  | FAIL |  | cycle completed; ledger walk deferred to future slice |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 447

