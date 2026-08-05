# Model Discovery & Live Tier→Model Catalog

> Retro-documented 2026-06-05 from the shipped implementation (PR #31, Steps 10a/10b; 10c wiring landed in follow-ups). Closes the documentation gap found by the PR↔ADR audit ([docs/research/pr-adr-documentation-audit-2026-06-05.md](../research/pr-adr-documentation-audit-2026-06-05.md)). Companion docs: [step9-llm-config-removal.md](step9-llm-config-removal.md) (why the catalog owns `tier → model`), [policy-config.md](policy-config.md) (pins bypass the catalog).

## Request / requirement

Routing decisions need the **models a CLI can actually serve right now**, not a static guess. Before this feature, `tier → model` lived in the embedded bridge manifest: hand-maintained, instantly stale when a CLI gained/lost models (codex quota windows, new claude families, locally-pulled ollama tags). Step 9 removed `llm_config.json` on the premise that *"profile/policy decide CLI + tier; the catalog resolves tier to a model"* — so the catalog had to exist and be trustworthy enough to dispatch from.

## Approaches considered

1. **Keep the static manifest, update by hand** — rejected: the exact drift problem being fixed; every model launch requires a code change.
2. **Per-provider HTTP APIs** (`/v1/models` etc.) — rejected: not uniform across codex/agy/claude/ollama; needs per-provider auth handling; bypasses what the *CLI* is actually configured to serve (subscription tier, local config).
3. **Ask each CLI itself** (chosen) — `ollama list` for the non-interactive case; drive the interactive `/model` picker through the tmux recipe engine (ADR-0031) for codex/agy/claude and parse the rendered pane. The CLI's own picker is ground truth for "what can this CLI dispatch right now".
4. **Hardcoded tier classification** (regex on model names) — rejected for the judgment step: tier-ness ("fast" vs "deep") is qualitative and changes per release; per AGENTS.md Rule 5 this is LLM work. A one-shot LLM classification with strict validation was chosen instead.

## Chosen solution

### Schema & storage

`Catalog{FetchedAt, CLIs: map[cli]CLIEntry}`; `CLIEntry{TierModels map[tier]model, Available []string, Source "live"|"detect", TierFallbacks map[tier][]model, CandidatesHash}` (`go/internal/modelcatalog/catalog.go`). Canonical tiers: `fast | balanced | deep | top` (`refresh.go:10`; `top` is the frontier tier, `high` is an input alias of `deep`). Cache file: `.evolve/model-catalog.json`, written atomically (temp+rename, `store.go`) with the outgoing catalog retained as `model-catalog.prev.json` for rollback; shadow-stage output lands in `model-catalog.shadow.json` (`shadow.go`) which dispatch never reads. All writes go through the `Commit` seam (`commit.go`), which carries operator-authored `tier_fallbacks` forward so a refresh can never destroy them.

### Discovery (per-CLI listers)

- **ollama**: parse `ollama list` stdout table (`go/internal/modelquery/ollama.go`).
- **codex / claude / agy**: `RecipeLister` (`recipe.go`) drives the `/model` picker via `ModelCapturer.CaptureModelPicker` (tmux pane capture, ADR-0031), then per-CLI parsers (`picker.go`): codex numbered rows → first token; claude rows → family (`opus|sonnet|haiku`); agy flat list bounded by the "Switch Model" header, markers stripped.

### Tier classification (LLM, validated)

`CLIClassifier` invokes one *ready* CLI headlessly (preference codex > claude > agy, `pickClassifierCLI`) with a one-shot prompt whose tier block and JSON template are **generated from `modelcatalog.CanonicalTiers`** (`classifier.go:buildClassifyPrompt` + `tierBriefs`) — a canonical tier can never be silently omitted (the original hardcoded three-tier prompt deleted `tier_models.top` on every refresh). Validation drops hallucinated models (answer must be in the offered list) and non-canonical tiers — the LLM judges, deterministic code verifies. Any tier the reply still omits is filled by `CompleteTiers` (`complete.go`) from a nearest-neighbour ladder over the canonical order (more-capable side first on ties) — it only reuses ids the validator already accepted, never invents one.

### Provenance — the trust rule

`source: "live"` (queried from the CLI) is **dispatch-authoritative**; `source: "detect"` (derived from the static manifest) is informational only. `DispatchModel(cli, tier)` returns `ok=false` for anything non-live (`catalog.go:64-74`), so a detect-only or empty catalog leaves dispatch **byte-identical** to the pre-catalog manifest. Live-refresh failures degrade per-CLI to the detect fallback (marked `detect`, hence non-authoritative) rather than poisoning the cache.

### Freshness & refresh — staged write path

`DefaultTTL = 24h`; `IsStale` treats never-fetched as stale and future timestamps (clock skew) as fresh. The cycle-start refresh is wired via `WithCatalogRefresher` (best-effort, WARN-not-abort) and staged by `policy.json` `catalog.refresh_stage` (`runStagedCatalogRefresh`, `cmd/evolve/cmd_models_live.go`):

| Stage | TTL-gates on | Writes | Dispatch effect |
|---|---|---|---|
| `off` | — | nothing | none (today's frozen-catalog posture) |
| `shadow` | `model-catalog.shadow.json` | shadow file only, plus per-tier `would-change cli.tier: old -> new` diff lines against the live catalog | **none** — the overlay reads only `model-catalog.json`, byte-identical to `off` |
| `enforce` | `model-catalog.json` | live catalog via the `Commit` seam | live entries overlay dispatch |

Absent `refresh_stage` derives from `catalog.auto_refresh` (true ⇒ `enforce`, false ⇒ `off`) so existing deployments keep their exact behavior; an unknown value fails safe to `off` (`policy.resolveRefreshStage` — a typo disables the write, never arms one). Shadow gates its TTL on the file *it* writes: gating on the live file would either never run or drive the expensive live probe every cycle. Manual: `evolve models refresh [--source live|detect] [--json]` (always writes the live catalog — an explicit operator action), `evolve models list` (prints staleness).

The live probe (tmux `/model` capture + one-shot classifier) runs in a **throwaway scratch workspace** (`liveRefresh`), never the repo: the router profile's sandbox declares `read_only_repo`, so an artifact path under the project root is either denied or litters an untracked file in main.

### Latest-model selection & stability (2026-08-05)

**Issue.** The operator asked the loop to track the latest model per tier per CLI automatically. The centerpiece comparator `NewestInLineage` had zero production call sites — and wiring it naively was a capability downgrade: `parseVersion("Gemini 3.5 Flash (Medium)")=[3,5]` beats `("Gemini 3.1 Pro (High)")=[3,1]`, so "newest wins" over a whole candidate list replaces Pro with Flash. Separately, tier assignment was a live LLM call with no stability check (documented flap: an identical agy list reclassified Sonnet-4.6 → GPT-OSS-120B between refreshes), and for claude, caching any concrete id would *freeze* the version its alias tracks.

**Gap.** "Latest" is only well-defined *within a lineage* (same model line, different versions), and nothing grouped ids by lineage. "Freshest" is CLI-relative (claude resolves `opus` to the newest release at launch; enumerating CLIs need the newest concrete id), and nothing declared that fact per CLI. And an unchanged offering had no way to keep its previous classification.

**Solution** (`go/internal/modelquery`, pipeline order `List → family-filter → [reuse-gate] → Classify → PromoteLatest → CompleteTiers`):

- **`LineageKey` / `GroupByLineage`** (`lineage.go`): version-free identity — id lowercased, the SAME version token `NewestInLineage` compares removed, separator runs collapsed. Same key ⇔ mutually substitutable; different keys are different capability classes and are NEVER substituted (`gemini-pro-(high)` ≠ `gemini-flash-(medium)`; `gpt` ≠ `gpt-mini`).
- **`FreshnessPolicy.Freshest` / `PromoteLatest`** (`latest.go`): promotion upgrades each classified tier model to the freshest member of *its own* bucket — the classifier keeps 100% of the qualitative decision, Go keeps 100% of the numeric one. `NewestInLineage` is composed, not modified.
- **Per-CLI freshness is manifest DATA, not Go conditionals** (`bridge.ModelFreshness`, `claude-tmux.json` `model_freshness`): claude declares `prefer: "alias"` (verified 2026-07-27: `--model opus` → `canonicalModel claude-opus-5`); every other manifest omits the block and gets the zero value (newest concrete version). Mapped to `FreshnessPolicy` by the composition root (`freshnessFromManifests`) — `modelquery` never imports `bridge`.

| CLI | Freshness rule | Why |
|---|---|---|
| claude | alias (`opus`/`sonnet`/`haiku`) | CLI resolves the alias to the newest release at LAUNCH; a concrete id would freeze it |
| codex / agy / ollama | newest concrete version within lineage | enumerating CLIs; the picker list is ground truth |

**Known limitation (deliberate, pinned):** date-stamped snapshot ids (`gpt-4o-2024-08-06` style) keep distinct lineage keys — only the first dotted numeric run is stripped — so they never cross-promote. Fail-safe by construction (an uncertain identity must never substitute; stripping all digit runs would collide `:8b` with `:70b`); the classifier's pick is simply kept. No live CLI reports dated ids today; follow-up queued as `lineage-datestamp-normalization` (`TestLineageKey_DatedSnapshotsStayDistinct_KnownLimitation` pins the behavior). Probe diagnostics (escalation reports, launch errors, the `llm-calls.ndjson` token ledger) written under the scratch workspace are salvaged to `.evolve/models-probe/` before teardown (`salvageProbeDiagnostics`) — the durable trail survives every refresh. The `decisionVersion` bump discipline is enforced by a source-hash ratchet (`decisionversion_pin_test.go`): any decision-surface edit fails the pin until the editor answers whether semantics changed.

- **Reuse gate** (`fingerprint.go` + `query.go:liveTiers`): `Fingerprint` hashes the decision inputs (algorithm `decisionVersion`, CLI, sorted candidates, policy, tier vocabulary; length-prefixed NUL-separated framing) into `CLIEntry.CandidatesHash`. An unchanged offering reuses the prior tier map with **zero classifier LLM calls** — but only when all three conditions hold: hash matches and is non-empty, prior `Source == "live"` (a detect entry is never laundered into an authoritative one), and the prior covers every canonical tier (a pre-fix `top`-less entry reclassifies once instead of staying sticky forever). `decisionVersion` is bumped by hand on any prompt/promotion change so a fix is never silently reused away.

### Dispatch integration

`LoadManifest` finishes by overlaying live catalog entries onto the embedded manifest's `ModelTierMap` (`go/internal/bridge/catalog_overlay.go`), memoized by file mtime. Policy pins (`.evolve/policy.json`) name an exact model and never trigger a catalog lookup ([policy-config.md](policy-config.md)). Fallback chain on missing/corrupt/stale catalog: unchanged manifest → static tier map; corrupt cache logs `[models] WARN unreadable catalog` and returns empty (fail-open, never blocks dispatch).

### Model-tier translation channels (per-CLI, cycle 447)

The **Realizer** (`go/internal/bridge/realizer.go`, ADR-0022) is the **single translation seam** from the abstract tier vocabulary — `fast | balanced | deep | top` — to whatever each CLI actually accepts. Each `*-tmux` manifest declares its channel in `params.model_tier`; the table below is a cross-checked projection of those manifests (the manifests stay the SSOT — `TestModelTierMatrixParity` and the cycle-447 doc predicates fail this table against them):

| CLI | Channel | Mechanism | Verified |
|---|---|---|---|
| claude | flag | `--model <model>` launch flag | manifest + realizer tests |
| codex | flag | `-m <model>` launch flag | manifest + realizer tests |
| agy | flag | `--model "<display name>"` launch flag (agy 1.0.15; tokens are `agy models` display names with spaces/parens, shell-quoted by `launchCmdLine`) | probed live 2026-07-02 |
| ollama | positional | model is the positional argument of `ollama run <model>` (`driver_ollamatmux.go`), composed by the driver, not a flag | launch-cmd test pins |

Rules the seam enforces, matrix-wide:

- **Unresolved-token omission**: `auto` (the loop's resolve-me sentinel), any canonical tier name, and the `high` input alias are vocabulary, never concrete models — when resolution leaves one of them intact, the Realizer omits the model parameter entirely and the CLI boots on its own default (`isUnresolvedModelToken`, widening the cycle-262 `auto` guard: `claude --model top` was reachable and fatal before claude's manifest declared a real `top`). One guard at the single emit point covers every flag/repl CLI.
- **Resolution order**: policy pin (exact model, bypasses the catalog) → live catalog overlay (`source=="live"` entries only) → the manifest's `model_tier_map` offline defaults. Unknown non-tier values pass through verbatim as raw model identifiers.
- **No silent drops**: a multi-model CLI may not declare a do-nothing channel — the parity pin (`go/internal/bridge/model_tier_parity_test.go`) rejects that shape; agy carried exactly that defect from 2026-05-31 (agy 1.0.3 had no model flag at all — incident cycle-154) until the 2026-07-02 re-probe found agy 1.0.15 grew `--model`.

## Deferred

- Feed picker capability-descriptions to the classifier for sharper tiering (noted in PR #31).

## Verification

- TDD coverage: `modelcatalog/catalog_test.go` (staleness, DispatchModel gate), `store_test.go` (atomic write), `modelquery/picker_test.go` (per-CLI parsers tested against real captured frames).
- Latest-selection layer: `modelquery/lineage_test.go` (capability classes never collide), `latest_test.go` (within-lineage promotion, alias preference), `fingerprint_test.go` (framing unambiguous, order-insensitive), `refresh_reuse_test.go` (zero classifier calls on unchanged offering + the three reuse refusals), `complete_test.go` (prompt generated from `CanonicalTiers`, nearest-neighbour fill), `bridge/model_freshness_test.go` (claude declares alias, everyone else zero-value), `cmd/evolve/cmd_models_stage_test.go` (off/shadow/enforce write behavior, shadow-TTL gating, diff lines).
- Safety properties under test: empty/detect-only catalog ⇒ dispatch byte-identical to pre-catalog; shadow stage ⇒ live catalog file byte-identical (dispatch unaffected); `evolve models refresh --source detect` is idempotent and its manifest-backed entries never map a tier to a bare tier name.
- Shadow soak bar (before any `enforce` conversation): ≥10 cycles with `refresh_stage: "shadow"` spanning a TTL boundary; would-change diff empty or explainable every run; live catalog mtime unchanged; shadow shows `claude.deep == "opus"` and agy's `deep` on `Pro`, not `Flash`.
