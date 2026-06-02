# Setup Onboarding (`evolve setup` + `/setup`)

> **Status:** Shipped 2026-05-27. Step-9b migration (2026-06-02) repointed the durable per-phase override from the removed `llm_config.json` to `.evolve/policy.json` `pins`. First-launch onboarding: detect CLIs, propose per-phase CLI/model pins, verify them against the floor, explain the pipeline.
> **Audience:** Operators onboarding a new checkout; anyone changing per-phase model routing.
> **Source:** `go/internal/setup/setup.go`, `go/cmd/evolve/cmd_setup.go`, `skills/setup/SKILL.md`. Design: [adr/0027-setup-onboarding.md](adr/0027-setup-onboarding.md).

## TL;DR

The deterministic core lives in `evolve setup` (Go); the interactive recommendation + pipeline explanation live in the `/setup` skill (your CLI session — no extra token cost). The skill **proposes** per-phase CLI/model pins written to `.evolve/policy.json`; the **kernel clamp** (envelope + allowed_clis) is reported by `evolve setup detect` as a `pin_violation` and hard-enforced at dispatch. Profiles own the per-phase defaults; pins are the OPT-IN override layer (Step 9 removed the old `llm_config.json`). Setup runs once on first launch (the loop nudges) and is re-runnable anytime.

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

## The /setup skill flow

`detect --json` → present CLIs → explain pipeline (grounded in README/overview/phase-architecture/[[dynamic-phase-routing]]) → propose per-phase models (envelope + allowed_clis + availability + cross-family-when-possible + tier heuristics) → AskUserQuestion to adjust → write `.evolve/policy.json` `pins` (only where overriding a profile default) → re-run `detect` and loop until every pinned phase shows `source:"policy-pin"` with empty `pin_violation` (and no `policy_error`) → `complete`. Full procedure: `skills/setup/SKILL.md`.

## Limitations

- **macOS Keychain false-negative:** `bridge.doctorAuth` checks only `~/.claude/.credentials.json`, so claude OAuth stored in the Keychain reads as `MISCONFIGURED`. The skill treats claude as available when run from a Claude session. Fixing the Keychain probe is deferred.
- **No plan/quota detection:** subscription plan (Pro/Max/Free) and remaining quota are out of scope (no reliable local signal).
