# Setup Onboarding (`evolve setup` + `/evo:setup`)

> **Status:** Shipped 2026-05-27. Step-9b migration (2026-06-02) repointed the durable per-phase override to `.evolve/policy.json` `pins`. Auto-config update (2026-06-24): the per-phase model **recommendation is now deterministic Go** (`evolve setup recommend`/`apply`) driven by a public preset config — the skill is a thin presenter offering ONE preset choice instead of twelve per-phase prompts. Design: [setup-auto-config-presets.md](setup-auto-config-presets.md).
> **Audience:** Operators onboarding a new checkout; anyone changing per-phase model routing.
> **Source:** `go/internal/setup/{setup,recommend,apply,presets}.go`, `go/cmd/evolve/cmd_setup.go`, `skills/setup/SKILL.md`, `go/internal/setup/presets.json`. Design: [adr/0051-setup-onboarding.md](adr/0051-setup-onboarding.md), [setup-auto-config-presets.md](setup-auto-config-presets.md).

## TL;DR

Everything deterministic — detection, the per-phase model **recommendation**, the policy write, and verification — lives in `evolve setup` (Go). The `/evo:setup` skill only **explains the pipeline** and **relays the user's one preset choice** (your CLI session — no extra token cost). `evolve setup recommend` computes three presets (Recommended/Economy/Max-quality) from the **public profiles** + live detection; `evolve setup apply --preset X` writes the chosen preset's pins to `.evolve/policy.json` (lossless merge, only where they differ from the profile default). The **kernel clamp** (envelope + allowed_clis) is reported by `detect` as a `pin_violation` and hard-enforced at dispatch. Profiles own the per-phase defaults; presets are **data** (public config), never hardcoded. Setup runs once on first launch (the loop nudges) and is re-runnable anytime.

## Contents

- [Subcommands](#subcommands)
- [Detect digest](#detect-digest)
- [Pin verification rules](#pin-verification-rules)
- [First-run marker + nudge](#first-run-marker--nudge)
- [The /setup skill flow](#the-setup-skill-flow)
- [Limitations](#limitations)

## Subcommands

| Command | Exit codes | Notes |
|---|---|---|
| `evolve setup detect [--json] [--evolve-dir DIR]` | 0 | Read-only digest (human table or JSON); overlays `.evolve/policy.json` pins onto the per-phase routing and reports any `pin_violation` |
| `evolve setup recommend [--json]` | 0 · 1 runtime | Read-only. Computes the configured presets from detection + the public profiles + the preset config; emits a `RecommendReport` (`available_families`, `cross_family_ok`, `presets[]`, `default`) |
| `evolve setup apply --preset NAME [--dry-run]` | 0 · 10 bad args · 1 refusal | Writes the chosen preset's per-phase pins into `.evolve/policy.json` (lossless merge). Refuses a degraded preset or a malformed existing policy. `--dry-run` prints the merged policy without writing. `--preset` is required |
| `evolve setup complete [--evolve-dir DIR]` | 0 · 1 IO | Stamps `state.setupCompletedAt` + `setupVersion` (lossless) |

> The standalone `evolve setup validate` subcommand was removed in Step 9b (it
> validated the now-deleted `llm_config.json`). The same envelope + allowed_clis
> clamp is now applied by `policy.ValidatePin` and surfaced inline by `detect`'s
> `pin_violation` field; an out-of-bounds pin also hard-fails the phase at dispatch.

Project root resolves `--project-root` > `EVOLVE_PROJECT_ROOT` > cwd; `--evolve-dir` overrides `.evolve/`; plugin root from `EVOLVE_PLUGIN_ROOT` (or project); adapters from `<plugin>/adapters`.

> **Root-parity note.** `evolve loop` resolves its root from `--project-root` (default cwd) and does **not** read `EVOLVE_PROJECT_ROOT`. So `setup complete`'s marker only silences the loop's first-run nudge when both resolve to the same `.evolve/`. In normal use they agree (the dispatcher runs both from the project, or passes `--project-root` to both). When invoking explicitly, pass the **same** `--project-root` to `evolve setup complete` and `evolve loop`.

## Detect digest

`detect --json` emits `DetectReport`:

```json
{
  "scanned_at": "<RFC3339>",
  "clis": [
    { "cli": "claude", "binary_present": true, "auth_configured": true,
      "auth_mode": "SUBSCRIPTION_OAUTH", "subscription_type": "",
      "capability_tier": "full", "verdict": "ready", "env_warnings": [],
      "tier_models": { "fast": "haiku", "balanced": "sonnet", "deep": "opus" } }
  ],
  "phases": [
    { "role": "builder", "current_cli": "claude", "current_tier": "sonnet",
      "source": "policy-pin", "pin_violation": "",
      "default_cli": "agy-tmux", "default_tier": "sonnet",
      "envelope": {"min":"balanced","default":"balanced","max":"deep"},
      "cross_family_with": "auditor", "allowed_clis": ["claude","agy"] }
  ],
  "setup_completed_at": "", "setup_version": 0, "policy_error": ""
}
```

- **CLIs** are grouped by base family (claude/codex/gemini/agy); `bridge.Doctor`'s `-tmux`/`-p` driver rows collapse to one row per family.
- **auth_mode** (claude): `CUSTOM_PROXY` (base-url set) > `API_KEY` (api-key set) > `SUBSCRIPTION_OAUTH` (creds file) > `MISCONFIGURED`. Other CLIs: `SUBSCRIPTION` if configured, else `MISCONFIGURED`.
- **capability_tier**: `full` (budget + permission native), `delegated` (delegates to claude / kernel hooks), `n/a` (no binary). The precise 5-dimension quality tier is available via `./bin/check-caps`.
- **tier_models**: each abstract tier → that CLI's NATIVE model, sourced from the bridge manifest `tier_aliases` (single source of truth): `agy` → `gemini-3.5-flash` (all tiers; no model selector), `codex` → `gpt-5.4-mini`/`gpt-5.4`/`gpt-5.5`, `claude` → `haiku`/`sonnet`/`opus`. `/setup` writes the chosen native model into `.evolve/policy.json` `pins[<phase>].model`.

> **Resolution nuance.** Without a pin, `resolvellm` resolves a phase from its profile (`cli` + `model_tier_default`, default `balanced`) — Step 9 removed the `llm_config.json` layer, so the profile is the only default source. A `.evolve/policy.json` `pin` then overrides: `pin.cli` sets the CLI and `pin.model` sets the exact model the realizer dispatches. The pin's tier (for the envelope check) is classified from `pin.model` via `policy.TierRank` (Claude models classify by substring; gemini/gpt models are rank 0 → envelope check skipped, `allowed_clis` still applies).
- **phases** cover the 12 configurable roles, each resolved from its profile by `resolvellm.Resolve`, then the matching `.evolve/policy.json` pin overlaid (`source` becomes `policy-pin`). A malformed `policy.json` sets the top-level `policy_error` and disables overlay (the floor still applies at dispatch).
- **default_cli / default_tier** carry the PROFILE default (profile `cli` + `model_tier_default`) independently of any pin overlay, so `Recommend` can compute "differs from default" without re-loading the profile. Unpinned phases have these equal to `current_*`; pinned phases keep the profile default here while `current_*` shows the pin.

## Presets + recommendation

`evolve setup recommend` is a pure, deterministic function over the `detect` digest plus a **public preset config**. It emits one `Preset` per configured entry, each a full `assignments[]` (`role`, `cli`, `tier`, `model`, `differs_from_default`, `tier_clamped`, `cli_fallback`, `warning`).

- **Baseline = the public profiles.** The `recommended` preset's tier is each phase's `model_tier_default`; its CLI is the profile `cli`, swapped only when that family is unavailable or when builder/auditor must split across families (cross-family). There is **no** hardcoded role→tier table — per-phase intent lives in the profiles.
- **Preset definitions are data.** They live in the shipped `go/internal/setup/presets.json` (overridable per-repo via `.evolve/setup-presets.json`). Each preset declares a generic `tier_bias` strategy the Go interpreter applies — `default` (profile default), `down` (one rank cheaper), `up` (one rank richer), `min`/`max` (envelope floor/ceiling). The shipped default ships `recommended`=default, `economy`=down, `max-quality`=max.
- **Clamping + fallback.** Every tier is clamped into the phase's `[envelope.min..envelope.max]`; every CLI must be in `allowed_clis` (or `["all"]`) AND authed (`verdict != blocked`). An unavailable preferred CLI falls back to an available allowed family; when no allowed family is available the phase carries a `warning` and the preset is `degraded`.
- **`apply` is the gate.** It writes a pin ONLY where `differs_from_default`, lossless-merges into `policy.json` (preserving `floor`/`cli_health`/foreign pins), stores the **abstract tier** (never the native model id — a native id ranks 0 in `TierRank` and would skip the envelope check), validates every emitted pin with `policy.ValidatePin`, and **refuses** to persist a degraded preset or a malformed existing policy.

## Pin verification rules

`detect` overlays each `.evolve/policy.json` pin and runs `policy.ValidatePin` against the phase's profile (`.evolve/profiles/<role>.json`), reporting the first breach in `pin_violation`. The same check hard-fails the phase at dispatch.

| Check | Severity | Rule |
|---|---|---|
| Envelope | **pin_violation** + dispatch hard-fail | pinned model's tier ∈ `[envelope.min .. envelope.max]`. Tiers normalize across both vocabularies: `fast↔haiku`, `balanced↔sonnet`, `deep↔opus`; exact model strings classify by substring. Non-Claude models (gemini/gpt) are tier-rank 0 → envelope check skipped (allowed_clis still applies). |
| allowed_clis | **pin_violation** + dispatch hard-fail | pinned `cli` family ∈ profile `allowed_clis` (unless `["all"]`) |
| Cross-family | advisory (skill-applied) | `builder` family ≠ `auditor` family is PREFERRED for adversarial integrity but enforced by no command — the `/setup` skill surfaces it from each phase's `cross_family_with`. All-Claude is a legitimate fallback. Families: claude→anthropic, codex→openai, gemini/agy→google. |

## First-run marker + nudge

- `evolve setup complete` stamps `state.setupCompletedAt` (RFC3339) + `setupVersion` via a **lossless raw-merge** — it preserves all other `state.json` keys (e.g. `expected_ship_sha`) that `core.State`'s `WriteState` would drop.
- `evolve loop` prints one non-blocking stderr line when the marker is empty, then proceeds with defaults: `[setup] First run — run /setup …`.

## The /evo:setup skill flow

`detect --json` → present CLIs → explain pipeline (grounded in README/overview/phase-architecture/[[dynamic-phase-routing]]) → `recommend --json` → present the THREE presets as ONE comparison → a single **AskUserQuestion** for the user's choice → `apply --preset <choice>` (Go writes the pins) → re-run `detect` to confirm `source:"policy-pin"` with empty `pin_violation` → `complete`. The skill no longer proposes per-phase models or hand-authors `policy.json` — `recommend` + `apply` do. Full procedure: `skills/setup/SKILL.md`.

## Limitations

- **No plan/quota detection:** subscription plan tier (Pro/Max/Free) and remaining quota are out of scope — no reliable local signal. The recommender keys off what's robustly detectable: auth posture (`auth_mode`), availability (`verdict`), and each CLI's tier→model map.
- **Live per-model availability deferred:** the recommender uses the manifest tier→model maps, not a live `models list` query per CLI. A live query would shell to each CLI (tmux) and risk hanging the wizard, so it's a best-effort follow-up; `verdict`-based availability already prevents recommending a blocked CLI.
- **Profile-less phases:** a phase with no `<role>.json` profile has no default to recommend against; `recommend` shows a best-effort assignment but `apply` **skips** it (never pins what it can't validate). A complete repo ships all 12 profiles.
