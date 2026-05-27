# Setup Onboarding (`evolve setup` + `/setup`)

> **Status:** Shipped 2026-05-27. First-launch onboarding: detect CLIs, propose + validate per-phase models, explain the pipeline.
> **Audience:** Operators onboarding a new checkout; anyone changing per-phase model routing.
> **Source:** `go/internal/setup/setup.go`, `go/cmd/evolve/cmd_setup.go`, `skills/setup/SKILL.md`. Design: [adr/0027-setup-onboarding.md](adr/0027-setup-onboarding.md).

## TL;DR

The deterministic core lives in `evolve setup` (Go); the interactive recommendation + pipeline explanation live in the `/setup` skill (your CLI session — no extra token cost). The skill **proposes** a per-phase model config; `evolve setup validate` is the **kernel clamp**. Setup runs once on first launch (the loop nudges) and is re-runnable anytime.

## Contents

- [Subcommands](#subcommands)
- [Detect digest](#detect-digest)
- [Validation rules](#validation-rules)
- [First-run marker + nudge](#first-run-marker--nudge)
- [The /setup skill flow](#the-setup-skill-flow)
- [Limitations](#limitations)

## Subcommands

| Command | Exit codes | Notes |
|---|---|---|
| `evolve setup detect [--json] [--evolve-dir DIR]` | 0 | Read-only digest (human table or JSON) |
| `evolve setup validate [--config P] [--strict] [--json] [--evolve-dir DIR]` | 0 OK · 2 error-violation · 1 IO/parse · 10 bad args | Clamps a config against the floor |
| `evolve setup complete [--evolve-dir DIR]` | 0 · 1 IO | Stamps `state.setupCompletedAt` + `setupVersion` (lossless) |

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
    { "role": "builder", "current_cli": "claude", "current_model": "sonnet",
      "current_tier": "", "source": "llm_config",
      "envelope": {"min":"balanced","default":"balanced","max":"deep"},
      "cross_family_with": "auditor", "allowed_clis": ["claude","agy"] }
  ],
  "setup_completed_at": "", "setup_version": 0
}
```

- **CLIs** are grouped by base family (claude/codex/gemini/agy); `bridge.Doctor`'s `-tmux`/`-p` driver rows collapse to one row per family.
- **auth_mode** (claude): `CUSTOM_PROXY` (base-url set) > `API_KEY` (api-key set) > `SUBSCRIPTION_OAUTH` (creds file) > `MISCONFIGURED`. Other CLIs: `SUBSCRIPTION` if configured, else `MISCONFIGURED`.
- **capability_tier**: `full` (budget + permission native), `delegated` (delegates to claude / kernel hooks), `n/a` (no binary). The precise 5-dimension quality tier is available via `./bin/check-caps`.
- **tier_models**: each abstract tier → that CLI's NATIVE model, sourced from the bridge manifest `tier_aliases` (single source of truth): `agy` → `gemini-3.5-flash` (all tiers; no model selector), `codex` → `gpt-5.4-mini`/`gpt-5.4`/`gpt-5.5`, `claude` → `haiku`/`sonnet`/`opus`. `/setup` writes these into `llm_config.model` so the config is self-documenting.

> **Resolution nuance (footgun).** `resolvellm` reads `cli`/`model`/`model_tier` and **ignores the `tier` field** — `model` is what flows to dispatch (the realizer maps it per-CLI via `tier_aliases`, so `model:"sonnet"` on codex still resolves to `gpt-5.4`). The `tier` field is consumed only by `validate` (envelope) + the digest. Keep `tier` and `model` consistent (the skill derives `model` from `tier_models[tier]`), or `model` silently wins over a disagreeing `tier`.
- **phases** cover the 12 configurable roles, each resolved by `resolvellm.Resolve` (precedence: `llm_config.phases` > `_fallback` > profile).

## Validation rules

`validate` reads `llm_config.json` and each phase's profile (`.evolve/profiles/<role>.json`):

| Check | Severity | Rule |
|---|---|---|
| Envelope | **error** (exit 2) | phase tier ∈ `[envelope.min .. envelope.max]`. Tiers normalize across both vocabularies: `fast↔haiku`, `balanced↔sonnet`, `deep↔opus`; exact model strings classify by substring. |
| allowed_clis | **error** (exit 2) | phase `cli` ∈ profile `allowed_clis` (unless `["all"]`) |
| Cross-family | **warn** (advisory) | `builder` family ≠ `auditor` family. WARN by default — all-Claude is a legitimate fallback. `--strict` promotes to error. Families: claude→anthropic, codex→openai, gemini/agy→google. |

## First-run marker + nudge

- `evolve setup complete` stamps `state.setupCompletedAt` (RFC3339) + `setupVersion` via a **lossless raw-merge** — it preserves all other `state.json` keys (e.g. `expected_ship_sha`) that `core.State`'s `WriteState` would drop.
- `evolve loop` prints one non-blocking stderr line when the marker is empty, then proceeds with defaults: `[setup] First run — run /setup …`.

## The /setup skill flow

`detect --json` → present CLIs → explain pipeline (grounded in README/overview/phase-architecture/[[dynamic-phase-routing]]) → propose per-phase models (envelope + allowed_clis + availability + cross-family-when-possible + tier heuristics) → AskUserQuestion to adjust → write `.evolve/llm_config.json` (schema v2) → `validate` (clamp loop on exit 2) → `complete`. Full procedure: `skills/setup/SKILL.md`.

## Limitations

- **macOS Keychain false-negative:** `bridge.doctorAuth` checks only `~/.claude/.credentials.json`, so claude OAuth stored in the Keychain reads as `MISCONFIGURED`. The skill treats claude as available when run from a Claude session. Fixing the Keychain probe is deferred.
- **No plan/quota detection:** subscription plan (Pro/Max/Free) and remaining quota are out of scope (no reliable local signal).
