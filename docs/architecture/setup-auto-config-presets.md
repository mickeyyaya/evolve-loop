# Setup Auto-Config Presets (design)

> **Status:** Shipped 2026-06-24. **Audience:** anyone changing the onboarding/recommendation flow.
> **Source:** `go/internal/setup/{recommend,apply,presets}.go`, `go/internal/setup/presets.json`, `go/cmd/evolve/cmd_setup.go`, `skills/setup/SKILL.md`. Companion: [setup-onboarding.md](setup-onboarding.md).

## Request

> "Review the setup flow so the user can clearly understand how the skill works, and offer a flow that auto-detects the subscription and offers the option (with auto-config of which LLM/model is used for which phase). Build the SIMPLE choices for the user — auto-config and auto-detect should already be done. Make it easy to config. Strict TDD. The setup should only configure the public/exposed config files — nothing at the code level."

## Problem

A setup flow already existed (`/setup` skill + `evolve setup detect|complete`), but:

1. **The per-phase model proposal was manual LLM work** in the skill (12 per-phase `AskUserQuestion` decisions) — not "auto-config", not testable, and prone to drift.
2. **Detection looked shallow.** The skill carried a caveat that Claude's macOS Keychain OAuth might misdetect. (On inspection this was *stale* — `bridge.NewEngine` already wires `defaultKeychainProbe`; `bridge/doctor_test.go` already locks it. The fix was deleting the caveat, not code.)
3. **Twelve decisions** is not "simple choices".

## Approaches considered

**Subscription detection depth** — (A) *capability-based*: detect auth posture (subscription-OAuth vs API-key vs none) + each CLI's tier→model map, and rely on `verdict` for availability; no fragile plan-name scraping. (B) additionally scrape Pro/Max/Free from each CLI's `/status` pane — richer labels but brittle. **Chosen: A.** Plan tier is not reliably exposed locally; capability + availability is what the recommender actually needs.

**Config UX** — (A) *pick one preset*: the binary computes 3 ready bundles, the user picks one; (B) two dials (CLI arrangement × cost); (C) keep per-phase with smart defaults. **Chosen: A.** One decision instead of twelve, with an optional per-phase tweak after.

**Where the logic lives** — (A) *Go writes it* (`recommend` + `apply` subcommands); (B) the skill writes the JSON. **Chosen: A.** Deterministic work belongs in code, not a prompt (Karpathy rule 5); it makes strict TDD possible.

**Where the preset DEFINITIONS live** — (A) *public config file* (Go is a generic interpreter); (B) derive-from-profiles with a thin Go transform. **Chosen: A**, per the directive "configure only public/exposed config files, nothing at the code level." Presets are data in `presets.json` (shipped default) overridable by `.evolve/setup-presets.json`; the Go knows only generic tier-bias strategies.

## Chosen design

```
detect (Go, existing)  ─┐
profiles/*.json (config)─┼─►  Recommend(DetectReport, PresetConfig)  ──►  RecommendReport (3 presets)
presets.json (config)  ─┘            pure, deterministic                         │
                                                                                 ▼
                                              Apply(rep, cfg, preset, policyJSON, profLoader)
                                              lossless merge + ValidatePin gate ──► .evolve/policy.json
```

- **`Recommend` is pure** over `(DetectReport, PresetConfig)` — no clock/env/disk/rand, no map-iteration order leaks (families/CLIs sorted before any "first" pick). The baseline tier/CLI come from each phase's profile (`default_tier`/`default_cli`, threaded into `PhaseStatus` by `Detect`). The only preset behavior in code is a generic `tier_bias` interpreter (`default`/`down`/`up`/`min`/`max`); the preset *set*, descriptions, and bias selection are config.
- **Clamping + cross-family.** Every tier is clamped into `[envelope.min..envelope.max]`; every CLI must be in `allowed_clis` (or `all`) AND authed. Builder/auditor split across families when ≥2 are authed, preferring each profile's default family; single-family is a legitimate fallback (not a warning). No allowed family available → `warning` + `degraded`.
- **`Apply` is the write + gate.** Lossless raw-map merge (preserves `floor`/`cli_health`/foreign pins); a pin emitted ONLY where `differs_from_default`; the **abstract tier** stored in `pin.model` (a native id ranks 0 in `policy.TierRank` and would silently skip the envelope floor — the one load-bearing correctness trap); every emitted pin re-validated with `policy.ValidatePin`; degraded presets and malformed existing policy are refused (no bytes written).
- **The skill is a thin presenter.** It teaches the pipeline and relays one preset choice; it never re-derives routing or hand-authors `policy.json`.

## "Config not code"

The setup's only write target is the public `.evolve/policy.json`. Everything it reads is public config: `.evolve/profiles/*.json` (per-phase defaults) and `presets.json` / `.evolve/setup-presets.json` (preset definitions). No per-phase setting or preset behavior is a Go literal — the Go is a generic mechanism (Strategy interpreter + safety gate), matching the codebase's `phase_settings_from_config_not_code` rule.

## Test coverage (strict TDD, all RED-first)

- `recommend_test.go` (18): preset shape; baseline=profile default; envelope clamp; no-envelope passthrough; economy down + min==max stays; max-quality up + default==max stays; zero-families degraded; one-family single-family; two-family cross-family; forced-same-family; preferred-unavailable fallback; allowed-restricted-to-unavailable warns; model from tier_models; determinism (byte-identical JSON); differs-from-default.
- `apply_test.go` (10): unknown preset; only-differing pinned; lossless merge; foreign pins survive; re-apply clears upgrade; malformed refuses; emitted pins pass ValidatePin; degraded refuses; **tier-not-native-model**; cross-family legal.
- `presets_test.go` (6): embedded default; override wins; malformed/invalid-default/empty refused; shipped-default valid.
- `cmd_setup_test.go`: recommend JSON/human; apply missing-preset (10); apply unknown-preset (1); apply host-robust write (foreign keys survive; pins on rc 0; no-clobber on rc 1).
- `setup_test.go`: `Detect` populates `default_cli`/`default_tier` from the profile, independent of pin overlay.

## Deferred

- **Live per-model availability** (`modelquery.Refresh`) — would shell to each CLI (tmux) and risk hanging the wizard; manifest tier maps + `verdict` are used instead.
- **Plan-tier (Pro/Max/Free) detection** — no reliable local signal.
