# Model Discovery & Live Tier→Model Catalog

> Retro-documented 2026-06-05 from the shipped implementation (PR #31, Steps 10a/10b; 10c wiring landed in follow-ups). Closes the documentation gap found by the PR↔ADR audit ([knowledge-base/research/pr-adr-documentation-audit-2026-06-05.md](../../knowledge-base/research/pr-adr-documentation-audit-2026-06-05.md)). Companion docs: [step9-llm-config-removal.md](step9-llm-config-removal.md) (why the catalog owns `tier → model`), [policy-config.md](policy-config.md) (pins bypass the catalog).

## Request / requirement

Routing decisions need the **models a CLI can actually serve right now**, not a static guess. Before this feature, `tier → model` lived in the embedded bridge manifest: hand-maintained, instantly stale when a CLI gained/lost models (codex quota windows, new claude families, locally-pulled ollama tags). Step 9 removed `llm_config.json` on the premise that *"profile/policy decide CLI + tier; the catalog resolves tier to a model"* — so the catalog had to exist and be trustworthy enough to dispatch from.

## Approaches considered

1. **Keep the static manifest, update by hand** — rejected: the exact drift problem being fixed; every model launch requires a code change.
2. **Per-provider HTTP APIs** (`/v1/models` etc.) — rejected: not uniform across codex/agy/claude/ollama; needs per-provider auth handling; bypasses what the *CLI* is actually configured to serve (subscription tier, local config).
3. **Ask each CLI itself** (chosen) — `ollama list` for the non-interactive case; drive the interactive `/model` picker through the tmux recipe engine (ADR-0031) for codex/agy/claude and parse the rendered pane. The CLI's own picker is ground truth for "what can this CLI dispatch right now".
4. **Hardcoded tier classification** (regex on model names) — rejected for the judgment step: tier-ness ("fast" vs "deep") is qualitative and changes per release; per AGENTS.md Rule 5 this is LLM work. A one-shot LLM classification with strict validation was chosen instead.

## Chosen solution

### Schema & storage

`Catalog{FetchedAt, CLIs: map[cli]CLIEntry}`; `CLIEntry{TierModels map[tier]model, Available []string, Source "live"|"detect"}` (`go/internal/modelcatalog/catalog.go:30-58`). Canonical tiers: `fast | balanced | deep` (`refresh.go:8`). Cache file: `.evolve/model-catalog.json`, written atomically (temp+rename, `store.go`); dir resolvable via `EVOLVE_MODEL_CATALOG_DIR`.

### Discovery (per-CLI listers)

- **ollama**: parse `ollama list` stdout table (`go/internal/modelquery/ollama.go`).
- **codex / claude / agy**: `RecipeLister` (`recipe.go`) drives the `/model` picker via `ModelCapturer.CaptureModelPicker` (tmux pane capture, ADR-0031), then per-CLI parsers (`picker.go`): codex numbered rows → first token; claude rows → family (`opus|sonnet|haiku`); agy flat list bounded by the "Switch Model" header, markers stripped.

### Tier classification (LLM, validated)

`CLIClassifier` invokes one *ready* CLI headlessly (preference `EVOLVE_MODELCATALOG_CLASSIFIER_CLI` > codex > claude > agy) with a one-shot prompt returning `{"fast":…,"balanced":…,"deep":…}` (`classifier.go`). Validation drops hallucinated models (answer must be in the offered list) and non-canonical tiers — the LLM judges, deterministic code verifies.

### Provenance — the trust rule

`source: "live"` (queried from the CLI) is **dispatch-authoritative**; `source: "detect"` (derived from the static manifest) is informational only. `DispatchModel(cli, tier)` returns `ok=false` for anything non-live (`catalog.go:64-74`), so a detect-only or empty catalog leaves dispatch **byte-identical** to the pre-catalog manifest. Live-refresh failures degrade per-CLI to the detect fallback (marked `detect`, hence non-authoritative) rather than poisoning the cache.

### Freshness & refresh

`DefaultTTL = 24h`; `IsStale` treats never-fetched as stale and future timestamps (clock skew) as fresh. Cycle-start auto-refresh: `shouldRefreshCatalog` (on unless `EVOLVE_MODELCATALOG_AUTOREFRESH=0`) wired into cycle orchestration via `WithCatalogRefresher` — best-effort, WARN-not-abort. Manual: `evolve models refresh [--source live|detect] [--json]`, `evolve models list` (prints staleness).

### Dispatch integration

`LoadManifest` finishes by overlaying live catalog entries onto the embedded manifest's `ModelTierMap` (`go/internal/bridge/catalog_overlay.go`), memoized by file mtime. Policy pins (`.evolve/policy.json`) name an exact model and never trigger a catalog lookup ([policy-config.md](policy-config.md)). Fallback chain on missing/corrupt/stale catalog: unchanged manifest → static tier map; corrupt cache logs `[models] WARN unreadable catalog` and returns empty (fail-open, never blocks dispatch).

## Deferred

- Feed picker capability-descriptions to the classifier for sharper tiering (noted in PR #31).

## Verification

- TDD coverage: `modelcatalog/catalog_test.go` (staleness, DispatchModel gate), `store_test.go` (atomic write), `modelquery/picker_test.go` (per-CLI parsers tested against real captured frames).
- Safety property under test: empty/detect-only catalog ⇒ dispatch byte-identical to pre-catalog.
