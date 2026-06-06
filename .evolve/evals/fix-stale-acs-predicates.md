---
score_cap:
  - criterion: "cycle89 EVOLVE_RESEARCH_CACHE_ENABLED predicate targets runtime-reference.md and passes"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestC89_ClaudeMdResearchEnvVars ./acs/cycle89/"
  - criterion: "cycle100 EVOLVE_OBSERVER_ENFORCE predicate targets runtime-reference.md and passes"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestC100_001_ObserverEnforceDefaultOn ./acs/cycle100/"
  - criterion: "TestTierModelsFor is hermetic against any live model catalog (per-test EVOLVE_MODEL_CATALOG_DIR isolation)"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestTierModelsFor ./internal/setup/"
---

# Eval: Fix 3 stale test predicates broken by the CLAUDE.md split + live catalog

> Pins the cycle-218 repair of three environment-rot test failures. The
> d8ac721 CLAUDE.md split moved the `EVOLVE_*` env-var table to
> `docs/operations/runtime-reference.md`, stranding two ACS Go predicates
> (cycle89, cycle100) that grepped CLAUDE.md for `EVOLVE_RESEARCH_CACHE_ENABLED`
> / `EVOLVE_OBSERVER_ENFORCE`. Separately, `TestTierModelsFor` in
> `go/internal/setup` read whatever live `model-catalog.json` the
> `EVOLVE_MODEL_CATALOG_DIR` / `EVOLVE_PROJECT_ROOT` environment pointed at
> (via `go/internal/bridge/catalog_overlay.go`), so the loop environment's
> live agy entries ("Gemini 3.5 Flash (Low)" etc.) overrode the manifest map
> and broke the test in-loop while it passed in a clean shell. Source
> incident: cycle 218 scout (2026-06-05) — 3 FAIL packages blocked the green
> baseline. The deeper lesson this eval pins: doc-grepping predicates must
> point at the doc that actually owns the content, and any test touching
> `bridge.LoadManifest` must isolate the catalog overlay seam per-test.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| doc-relocation rot | cycle89 env-var predicate green against runtime-reference.md | 5/10 | `go test -run TestC89_ClaudeMdResearchEnvVars ./acs/cycle89/` |
| doc-relocation rot | cycle100 observer predicate green against runtime-reference.md | 5/10 | `go test -run TestC100_001_ObserverEnforceDefaultOn ./acs/cycle100/` |
| env-leaky test | TestTierModelsFor hermetic vs live catalog overlay | 6/10 | `go test -run TestTierModelsFor ./internal/setup/` |
