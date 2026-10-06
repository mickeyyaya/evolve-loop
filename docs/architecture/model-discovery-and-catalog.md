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

`Catalog{FetchedAt, CLIs: map[cli]CLIEntry}`; `CLIEntry{TierModels map[tier]model, Available []string, Source "live"|"detect", TierFallbacks map[tier][]model, CandidatesHash, FallbackReason}` (`go/internal/modelcatalog/catalog.go`). Canonical tiers: `fast | balanced | deep | top` (`refresh.go:10`; `top` is the frontier tier, `high` is an input alias of `deep`). Cache file: `.evolve/model-catalog.json`, written atomically (temp+rename, `store.go`) with the outgoing catalog retained as `model-catalog.prev.json` for rollback; shadow-stage output lands in `model-catalog.shadow.json` (`shadow.go`) which dispatch never reads. All writes go through the `Commit` seam (`commit.go`), which carries operator-authored `tier_fallbacks` forward so a refresh can never destroy them.

### Discovery (per-CLI listers)

- **ollama**: parse `ollama list` stdout table (`go/internal/modelquery/ollama.go`).
- **agy**: parse `agy models` (`AgyLister`, `agy.go`), taking the display-name column (`Gemini 3.8 Flash (Low)`), because agy's picker shows the family and the effort as two separate controls and a pane capture yields names agy does not accept. One listing feeds two catalog entries, `agy` and `agy-claude`: `DefaultRouter` gives both keys one `onceLister`, so a refresh runs `agy models` once (see [Provider-aware entries](#provider-aware-entries-agy-claude-2026-10-06)).
- **codex / claude**: `RecipeLister` (`recipe.go`) drives the `/model` picker via `ModelCapturer.CaptureModelPicker` (tmux pane capture, ADR-0031), then per-CLI parsers (`picker.go`): codex numbered rows → first token; claude rows → family (`opus|sonnet|haiku`). `DefaultRouter` (`exec.go`) is the one registry of which CLI is listed how.

### Tier classification (LLM, validated)

`CLIClassifier` invokes one *ready* CLI headlessly with a one-shot prompt whose tier block and JSON template are **generated from `modelcatalog.CanonicalTiers`** (`classifier.go:buildClassifyPrompt` + `tierBriefs`) — a canonical tier can never be silently omitted (the original hardcoded three-tier prompt deleted `tier_models.top` on every refresh). Validation drops hallucinated models (answer must be in the offered list) and non-canonical tiers — the LLM judges, deterministic code verifies. Any tier the reply still omits is filled by `CompleteTiers` (`complete.go`) from a nearest-neighbour ladder over the canonical order (more-capable side first on ties) — it only reuses ids the validator already accepted, never invents one.

**The classifier is a chain (2026-10-05).** `ChainClassifier` (`chain.go`) holds every ready CLI in the order `pickClassifierCLI` gives (`cmd/evolve/cmd_models_live.go`: a ready `overrideCLI` first, then the ready members of the classifier's family order from the CLI routing table, described below; with no `cli_routing` block that order is codex > claude > agy, the CLIs `bridgePromptDispatcher` has a headless driver for; ollama has none, so it never classifies) and runs a `CLIClassifier` on each in turn. Any failure (a launch error, a reply with no JSON object, a reply that maps no tier to an offered id) is logged as `[modelquery] WARN <family>: classifier cli=<cli> failed: <reason>` and the next CLI is tried; the first success wins, a cancelled context stops the chain before the next launch, and an exhausted chain returns `every classifier CLI failed (<clis>): <each reason>`. Before this, one CLI was picked and nothing else was tried: on 2026-10-05 setup detect called codex `ready` from a stale auth file (the operator has no codex subscription), the codex launch failed (`bridge: launch exit=1`), and agy, claude and ollama all fell back to their detect maps, so agy stayed on the 3.7 Flash map while `agy models` already offered 3.8 Flash. A second defect hid behind the first. `bridgePromptDispatcher` launches the classifier under the bridge's artifact completion contract, which reads the reply from `ArtifactPath`, and only codex's headless driver writes that file itself (`codex exec --output-last-message`). `claude -p` and `agy -p` printed their answer, nothing told them to write the file, and the bridge returned an empty reply, so the classifier could only ever succeed on codex (the first live smoke of the chain: claude-p and agy exited 0 and both failed with `no JSON object in reply:` and nothing after it). The dispatcher's request (`bridgePromptDispatcher.request`) now appends `Write that JSON object, and nothing else, to the file <ArtifactPath>, then reply with the same JSON.`, the pattern the route judge's prompt already follows, and removes the file before each launch, since every link and family shares it and a link that writes nothing must not read the previous link's reply. codex still overwrites the artifact with its last message, which the prompt asks to be the same JSON; that codex path is unverified live here, since this operator's codex has no subscription. Resolving the classifier CLI through the CLI routing table (`internal/cliroute`, ADR-0119) instead of the `classifierCLIPreference` literal was filed as an inbox follow-up.

**The classifier order comes from the routing table (2026-10-06, L1b of the CLI routing table, [ADR-0119](adr/0119-one-routing-table-one-resolver.md)).** That follow-up is done. `liveRefresh` compiles the table (`classifierPreference` → `loadCLIRouter`) and `classifierFamilies` asks the router for `Resolve{Agent: "model-classifier", Launch: LaunchClassifier}`, keeping each candidate's family once, in chain order. `pickClassifierCLI` then takes the ready CLIs in that order. With no `cli_routing` block, the legacy projection returns codex > claude > agy, the order the deleted `classifierCLIPreference` literal held. With a declared table, the classifier routes like any other agent, so a table whose `clis` are `[agy, claude]` never launches codex to classify. A table that refuses to compile fails the live refresh loudly instead of falling back to the old order. Pinned by `TestClassifierOrder_TheLegacyProjectionKeepsTodaysOrder`, `TestClassifierOrder_ComesFromTheRoutingTable` and `TestClassifierPreference_ARefusedTableFailsTheRefresh` (`cmd/evolve/cmd_models_classifier_routing_test.go`).

### Provenance — the trust rule

`source: "live"` (queried from the CLI) is **dispatch-authoritative**; `source: "detect"` (derived from the static manifest) is informational only. `DispatchModel(cli, tier)` returns `ok=false` for anything non-live (`catalog.go:64-74`), so a detect-only or empty catalog leaves dispatch **byte-identical** to the pre-catalog manifest. Live-refresh failures degrade per-CLI to the detect fallback (marked `detect`, hence non-authoritative) rather than poisoning the cache. The fallback records why: `fallback_reason` is the live failure (`list models: …`, `CLI offered no models`, `no models in allowed families …`, `classify models: …`), written into the catalog entry and shown by `evolve models refresh` and `evolve models list`.

### Freshness & refresh — staged write path

`DefaultTTL = 24h`; `IsStale` treats never-fetched as stale and future timestamps (clock skew) as fresh. The cycle-start refresh is wired via `WithCatalogRefresher` (best-effort, WARN-not-abort) and staged by `policy.json` `catalog.refresh_stage` (`runStagedCatalogRefresh`, `cmd/evolve/cmd_models_live.go`):

| Stage | TTL-gates on | Writes | Dispatch effect |
|---|---|---|---|
| `off` | — | nothing | none (today's frozen-catalog posture) |
| `shadow` | `model-catalog.shadow.json` | shadow file only, plus per-tier `would-change cli.tier: old -> new` diff lines against the live catalog | **none** — the overlay reads only `model-catalog.json`, byte-identical to `off` |
| `enforce` | `model-catalog.json` | live catalog via the `Commit` seam | live entries overlay dispatch |

Absent `refresh_stage` derives from `catalog.auto_refresh` (true ⇒ `enforce`, false ⇒ `off`) so existing deployments keep their exact behavior; an unknown value fails safe to `off` (`policy.resolveRefreshStage` — a typo disables the write, never arms one). Shadow gates its TTL on the file *it* writes: gating on the live file would either never run or drive the expensive live probe every cycle. Manual: `evolve models refresh [--source live|detect] [--json]` (an explicit operator action that writes the live catalog), `evolve models list` (prints staleness). Each CLI row prints `source: live`, `source: detect`, or `source: detect fallback, reason="…"`, and `--json` carries `fallback_reason` on each fallback entry. A live refresh's header counts both (`source: live /model; 2 live, 2 detect fallback`). When **no** ready CLI is classified live, the live refresh writes nothing, prints the per-CLI reasons, and exits 1 (`no ready CLI was classified live`), so a refresh that learned nothing cannot replace the catalog on disk and cannot read as success. `--source detect` is unchanged.

The exit 1 belongs to the CLI verb only. Nothing in the loop runs `evolve models refresh` or reads its exit code: the loop's cycle-start refresh calls `liveRefresh` in-process through `makeCatalogRefresher` → `runStagedCatalogRefresh`, whose error the orchestrator only WARNs and stamps as a `catalog_refresh` `failed` ledger row. `liveRefresh` keeps returning a catalog with a nil error when every CLI fell back, so the cycle-start path keeps its stage behaviour (shadow writes the shadow file, enforce commits) and a degraded probe can never stop a cycle; its fallback rows now carry `fallback_reason`.

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

**Date-stamped snapshots (`lineage-datestamp-normalization`, resolved):** date-stamped snapshot ids (`gpt-4o-2024-08-06` style) now share a lineage key with same-line siblings — `LineageKey` additionally strips a calendar-year-anchored date run (`dateRun`: `YYYY[-MM[-DD]]`, anchored on a `19`/`20`-prefixed 4-digit year so it cannot collide with capability-bearing digits like `:8b`/`:70b`/`32b`), and `NewestInLineage` compares the numeric version first (read with the date removed, so a date is never a version) and uses the calendar date only to order equal versions that are BOTH dated. Because the date strip widens buckets (`gpt-5` and `gpt-4-2024-04-09` both key `gpt`), `PromoteLatest` starts from the selection (`incumbentFirst`), so only a strictly newer member replaces it: a date never outranks a version, and a same-version dated/undated pair (`gpt-4o` / `gpt-4o-2024-08-06`) is a tie that keeps the classifier's pick. Compact `YYYYMMDD` dates (`claude-3-5-sonnet-20241022`) are not normalized and keep distinct keys (fail-safe). `decisionVersion` was bumped (`v1` → `v2`) for the semantics change. Probe diagnostics (escalation reports, launch errors, the `llm-calls.ndjson` token ledger) written under the scratch workspace are salvaged to `.evolve/models-probe/` before teardown (`salvageProbeDiagnostics`) — the durable trail survives every refresh. The `decisionVersion` bump discipline is enforced by a source-hash ratchet (`decisionversion_pin_test.go`): any decision-surface edit fails the pin until the editor answers whether semantics changed.

- **Reuse gate** (`fingerprint.go` + `query.go:liveTiers`): `Fingerprint` hashes the decision inputs (algorithm `decisionVersion`, CLI, sorted candidates, policy, tier vocabulary; length-prefixed NUL-separated framing) into `CLIEntry.CandidatesHash`. An unchanged offering reuses the prior tier map with **zero classifier LLM calls** — but only when all three conditions hold: hash matches and is non-empty, prior `Source == "live"` (a detect entry is never laundered into an authoritative one), and the prior covers every canonical tier (a pre-fix `top`-less entry reclassifies once instead of staying sticky forever). `decisionVersion` is bumped by hand on any prompt/promotion change so a fix is never silently reused away.

### Provider-aware entries: `agy-claude` (2026-10-06)

**Request.** The operator directive of 2026-10-06 reads "agy could also use claude opus 5.5 and sonnet 5.5, prioritize using agy owned claude models first then using claude code models after". `agy models` (agy 1.2.17) lists `Claude Opus 5.5` and `Claude Sonnet 5.5` at `(Low)`, `(Medium)` and `(High)`, beside the Gemini models and `GPT-OSS 120B (Medium)`. The bridge reaches them through the routable target `agy-claude-tmux` ([internal-bridge.md, Provider-aware targets](packages/internal-bridge.md#provider-aware-targets-manifest_basego-model_familygo-agy-claude-tmux)), whose catalog key is `agy-claude` (`policy.BaseCLI("agy-claude-tmux")`, the key `applyCatalogTierMap` overlays).

**How the entry is built.** It is one more CLI of the same refresh, not a second probe:

1. **Detect.** `setup.Detect` reports an `agy-claude` row from the doctor's `agy-claude-tmux` row, with the same binary, auth and verdict as agy. Its detect fallback is `agy-claude-tmux`'s tier map (`llmroute.DefaultDriverForFamily("agy-claude")`), and its capability manifest is agy's (`antigravity`), because the family's driver runs the agy binary.
2. **List.** `Router.List("agy-claude")` reads the listing `agy` already read (`onceLister`): one `agy models` run serves both keys.
3. **Filter.** `catalog.allowed_families` filters each key to its model family: `agy` to `gemini` and `agy-claude` to `claude`, in the checked-in `.evolve/policy.json`. `TestTheCheckedInCatalogFiltersEachTargetToItsModelFamily` (cmd/evolve) pins every target's filter to `bridge.ModelFamily` of that target, so one listing can never put a Gemini model in the Claude entry or the reverse.
4. **Classify, promote, complete.** The unchanged pipeline runs per key. `PromoteLatest` moves a classified Claude pick to the newest version in its own lineage, so `Claude Opus 4.6 (High)` becomes `Claude Opus 5.5 (High)` when both are listed; `(High)` and `(Low)` are different lineages and are never substituted for each other.

Pinned by `TestRefresh_OneAgyListingYieldsAGeminiEntryAndAClaudeEntry`, which runs one fixture listing with Gemini, Claude and GPT-OSS rows. It checks that `agy models` runs once, that each classifier sees only its family, and that the result is two live entries with their tier maps. `TestDefaultRouter_AgyAndAgyClaudeShareOneLister`, `TestOnceLister_AFailedListingFailsEveryEntryThatSharesIt` and the setup tests `TestTierModelsFor_AgyClaudeReadsTheAgyClaudeTargetsTierMap` and `TestDetectCLIs_ReportsAgyClaudeBesideAgy` pin the rest.

**The baseline and the live pick.** The manifest's offline tier map is fast `Claude Sonnet 5.5 (Low)`, balanced `Claude Sonnet 5.5 (High)`, deep and top `Claude Opus 5.5 (High)`. agy offers no Haiku-class Claude, so fast is Sonnet at its lowest effort. A live refresh classifies with an LLM, and its pick can choose another effort. The first live run (2026-10-06, a scratch copy of the plane's catalog and policy, classified by claude-p after codex failed) wrote:

| Tier | Live pick |
|---|---|
| fast | `Claude Sonnet 5.5 (Low)` |
| balanced | `Claude Sonnet 5.5 (Medium)` |
| deep | `Claude Opus 5.5 (Medium)` |
| top | `Claude Opus 5.5 (High)` |

The `available` list held the six Claude rows only. The plane's catalog stays at `refresh_stage: shadow`, so dispatch reads the manifest's map until an operator refresh commits one. An operator who wants deep at `(High)` when the classifier picks otherwise can say so: a policy pin, or after L1c an `agents.<name>.model`.

**Nothing dispatches it yet.** The entry is data. Routing a phase to `agy-claude` is the CLI routing table's follow-up L1c ([cli-routing-table-2026-10.md](../plans/cli-routing-table-2026-10.md#provider-aware-targets-2026-10-06)).

### Dispatch integration

`LoadManifest` finishes by overlaying live catalog entries onto the embedded manifest's `ModelTierMap` (`go/internal/bridge/catalog_overlay.go`), memoized by file mtime. Policy pins (`.evolve/policy.json`) name an exact model and never trigger a catalog lookup ([policy-config.md](policy-config.md)). Fallback chain on missing/corrupt/stale catalog: unchanged manifest → static tier map; corrupt cache logs `[models] WARN unreadable catalog` and returns empty (fail-open, never blocks dispatch).

### Model-tier translation channels (per-CLI, cycle 447)

The **Realizer** (`go/internal/bridge/realizer.go`, ADR-0022) is the **single translation seam** from the abstract tier vocabulary — `fast | balanced | deep | top` — to whatever each CLI actually accepts. Each `*-tmux` manifest declares its channel in `params.model_tier`; the table below is a cross-checked projection of those manifests (the manifests stay the SSOT — `TestModelTierMatrixParity` and the cycle-447 doc predicates fail this table against them):

| CLI | Channel | Mechanism | Verified |
|---|---|---|---|
| claude | flag | `--model <model>` launch flag | manifest + realizer tests |
| codex | flag | `-m <model>` launch flag | manifest + realizer tests |
| agy | flag | `--model "<display name>"` launch flag (agy 1.0.15; tokens are `agy models` display names with spaces/parens, shell-quoted by `launchCmdLine`) | probed live 2026-07-02 |
| agy-claude | flag | the same `--model "<display name>"` flag on the agy binary (`agy-claude-tmux`, a merge patch over `agy-tmux`); a model-less launch realizes `params.model_tier.default` (fast) so it never boots agy's Gemini default | `evolve doctor live agy-claude-tmux --model …`, 2026-10-06 |
| ollama | positional | model is the positional argument of `ollama run <model>` (`driver_ollamatmux.go`), composed by the driver, not a flag | launch-cmd test pins |

Rules the seam enforces, matrix-wide:

- **Unresolved-token omission**: `auto` (the loop's resolve-me sentinel), any canonical tier name, and the `high` input alias are vocabulary, never concrete models — when resolution leaves one of them intact, the Realizer omits the model parameter entirely and the CLI boots on its own default (`isUnresolvedModelToken`, widening the cycle-262 `auto` guard: `claude --model top` was reachable and fatal before claude's manifest declared a real `top`). One guard at the single emit point covers every flag/repl CLI.
- **Resolution order**: policy pin (exact model, bypasses the catalog) → live catalog overlay (`source=="live"` entries only) → the manifest's `model_tier_map` offline defaults. Unknown non-tier values pass through verbatim as raw model identifiers.
- **No silent drops**: a multi-model CLI may not declare a do-nothing channel — the parity pin (`go/internal/bridge/model_tier_parity_test.go`) rejects that shape; agy carried exactly that defect from 2026-05-31 (agy 1.0.3 had no model flag at all — incident cycle-154) until the 2026-07-02 re-probe found agy 1.0.15 grew `--model`.

### Currency: the installed CLI bounds the catalog (2026-10-05)

A CLI's `/model` picker lists only the models its installed version knows, so a refresh can be no more current than the CLI. Sonnet 5.5 was missed until `claude update` (2026-09-30), and on 2026-10-05 every agy dispatch still ran Gemini 3.7 Flash while `agy models` listed 3.8. The operator's answer is the [model currency plan](../plans/model-currency-2026-10.md): every dispatch runs the latest model of its line, checked and adopted at each boundary. Its components are C1 (classifier fallback and loud degradation), C2 (lineage-keyed reuse), C3 (`refresh_stage: enforce`, adopted per family), C4 (the boundary CLI update), C5 (launch verification) and C6 (classifier routing).

C4 has landed. `evolve cli update` ([internal-cliupdate.md](packages/internal-cliupdate.md)) runs each subscribed family's manifest updater (`update_argv`) at loop boot and at every later wave boundary, smoke-tests a changed version with `evolve doctor live`, and halts the next wave when that smoke fails. It runs before the catalog refresh, so the refresh lists the new version's models. CLIs stay frozen within a wave, and `cli-version-drift` treats a change the updater recorded in `.evolve/cli-updates.json` as expected. agy's listers (`agy models`, `agy --help`) run with the agy manifest's `default_env` through `modelquery.UseProcessEnv(bridge.ProcessEnv)`, so a refresh never self-updates agy mid-wave.

## Deferred

- Feed picker capability-descriptions to the classifier for sharper tiering (noted in PR #31).

## Verification

- TDD coverage: `modelcatalog/catalog_test.go` (staleness, DispatchModel gate), `store_test.go` (atomic write), `modelquery/picker_test.go` (per-CLI parsers tested against real captured frames).
- Latest-selection layer: `modelquery/lineage_test.go` (capability classes never collide), `latest_test.go` (within-lineage promotion, alias preference), `fingerprint_test.go` (framing unambiguous, order-insensitive), `refresh_reuse_test.go` (zero classifier calls on unchanged offering + the three reuse refusals), `complete_test.go` (prompt generated from `CanonicalTiers`, nearest-neighbour fill), `bridge/model_freshness_test.go` (claude declares alias, everyone else zero-value), `cmd/evolve/cmd_models_stage_test.go` (off/shadow/enforce write behavior, shadow-TTL gating, diff lines).
- Classifier chain and loud refresh (2026-10-05): `modelquery/chain_test.go` (a launch failure, a no-JSON reply and an unmapped reply each fall through to the next CLI; the exhausted chain names every failure; a cancelled context launches nothing; `TestRefresh_ChainClassifierRecoversLiveTiersWhenTheFirstCLIFails`), `modelquery/fallbackreason_test.go` (each live failure is recorded as the entry's `fallback_reason`), `modelcatalog/fallbackreason_test.go` (the reason survives the store), `cmd/evolve/cmd_models_live_test.go` (`TestPickClassifierCLI`, `TestTierClassifier_FirstPreferredCLIFailsTheNextClassifiesLive`, `TestBridgePromptDispatcher_NamesTheArtifactTheBridgeReadsTheReplyFrom`, `TestBridgePromptDispatcher_ALinkThatWritesNothingNeverReadsTheLastLinksReply`), `cmd/evolve/cmd_models_refresh_test.go` (no live CLI exits 1 and keeps the prior catalog, human and `--json`; a partial fallback commits and names each fallback).
- Provider-aware entries (2026-10-06): `modelquery/agy_claude_catalog_test.go`, `setup/agy_claude_detect_test.go`, `cmd/evolve/catalog_model_family_policy_test.go`.
- Safety properties under test: empty/detect-only catalog ⇒ dispatch byte-identical to pre-catalog; shadow stage ⇒ live catalog file byte-identical (dispatch unaffected); `evolve models refresh --source detect` is idempotent and its manifest-backed entries never map a tier to a bare tier name.
- Shadow soak bar (before any `enforce` conversation): ≥10 cycles with `refresh_stage: "shadow"` spanning a TTL boundary; would-change diff empty or explainable every run; live catalog mtime unchanged; shadow shows `claude.deep == "opus"` and agy's `deep` on `Pro`, not `Flash`.
