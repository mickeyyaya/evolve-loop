---
name: setup
description: Use when the user runs /evo:setup (or /evo:setup), asks to configure evolve-loop, onboard, pick per-phase models, or learn how the pipeline works. Auto-detects available LLM CLIs/subscriptions, explains the pipeline concisely, then presents THREE ready-made config presets (Recommended/Economy/Max-quality) the Go binary computes deterministically from the public profiles — the user makes ONE choice and the binary writes per-phase pins to .evolve/policy.json. Runs once on first launch (the loop nudges) and is re-runnable anytime.
argument-hint: ""
---

# /evo:setup

> Interactive onboarding. Everything deterministic — detection, the per-phase model **recommendation**, the policy write, and verification — runs in the Go binary (`evolve setup detect|recommend|apply`). The only judgment left HERE, in your session (zero extra API cost), is **teaching the pipeline** and **relaying the user's one preset choice**. You no longer hand-author pins; `evolve setup apply` writes them. Presets are defined in a public config file (`go/internal/setup/presets.json`, overridable per-repo via `.evolve/setup-presets.json`) — never hardcoded. See [docs/architecture/setup-onboarding.md](../../docs/architecture/setup-onboarding.md).
>
> Invoked as `/evo:setup` (the `evo` plugin namespace); `/evo:setup` is the same skill.

## When to use

- The loop printed `[setup] First run …`, or the user typed `/evo:setup` / `/evo:setup`, or asked to configure models / learn the pipeline.
- Re-running is always safe — it re-detects and re-applies the chosen preset (idempotent; lossless-merges into `.evolve/policy.json`).

## Binary

Call `evolve` if on PATH; otherwise `./go/bin/evolve` (or `$EVOLVE_GO_BIN`). Only `apply` (and `complete`) write; `detect` and `recommend` are read-only.

## Procedure

1. **Detect.** Run `evolve setup detect --json` and parse it. The digest has `clis[]` (per family: `binary_present`, `auth_mode`, `subscription_type`, `capability_tier`, `verdict`, and `tier_models`) and `phases[]` (per role: `current_cli`/`current_tier`, `source`, `default_cli`/`default_tier`, `envelope`, `allowed_clis`, `pin_violation`). A malformed `.evolve/policy.json` shows as a top-level `policy_error`.

2. **Present the detection** as a compact table — one row per CLI family with binary/auth/tier/verdict. (macOS Keychain OAuth is detected — a Keychain-authed `claude` shows `SUBSCRIPTION_OAUTH`, not blocked.)

3. **Explain the pipeline** concisely (this is the teaching goal). Read the canonical sources first — do NOT invent: `README.md` "Pipeline Design", `docs/concepts/overview.md`, `docs/architecture/phase-architecture.md`, `docs/architecture/dynamic-phase-routing.md`. Cover, in ~6–10 lines: the cycle (Scout → Build → Audit → Ship → Learn), what each phase produces, and *why it is trustworthy* (deterministic EGPS verdicts, SHA-chained ledger, adversarial Builder≠Auditor). Personalize: reference the user's actual detected CLIs.

4. **Recommend.** Run `evolve setup recommend --json`. It returns `available_families`, `cross_family_ok`, and `presets[]` — each preset a full per-phase `assignments[]` (`role`, `cli`, `tier`, `model`, `differs_from_default`, `warning`) plus a `description`; `default` names the recommended one. The binary already applied every rule (envelope clamp, `allowed_clis`, availability, cross-family split) — you do NOT re-derive any of this. A `degraded` preset has an unsatisfiable phase (e.g. no authed CLI); surface it but don't pick it.

5. **Present the THREE presets as ONE comparison** and let the user choose with a single **AskUserQuestion** (options: the preset names; pre-select `default`). Summarize each in a line or two from its `description` + a couple of notable assignments (e.g. "builder→codex, auditor→claude; cheaper phases on fast"). This is the whole "which model for which phase" decision — one pick, not twelve. (Advanced: a user can edit the preset definitions in `.evolve/setup-presets.json`; mention it only if asked.)

6. **Apply.** Run `evolve setup apply --preset <choice>`. The binary deterministically writes the chosen preset's per-phase pins into `.evolve/policy.json` (lossless merge — preserves `floor`/`cli_health`/foreign pins; emits a pin ONLY where it differs from the profile default; stores the abstract tier). It refuses (non-zero exit) a degraded preset or a malformed existing policy rather than write something illegal. Use `--dry-run` first if the user wants to preview the merged policy.

7. **Verify.** Re-run `evolve setup detect` and confirm each pinned phase shows `source: "policy-pin"` with an EMPTY `pin_violation` and no top-level `policy_error`. (The same clamp hard-fails an out-of-bounds pin at dispatch, so this is the pre-flight catch.)

8. **Mark complete.** Run `evolve setup complete` to stamp the first-run marker (so the loop stops nudging). Confirm setup is done and that re-running `/evo:setup` anytime is safe.

## Notes

- Detection, recommendation, the policy write, and verification are ALL deterministic and live in Go; this skill never re-implements them and never hand-authors `policy.json`.
- Presets are data, not code: the shipped default is `go/internal/setup/presets.json`; a repo may override it with `.evolve/setup-presets.json`. Each preset's `tier_bias` is a generic strategy (`default`/`down`/`up`/`min`/`max`).
- Pins live in `.evolve/policy.json` (the user-owned override layer); profiles own the per-phase defaults. `apply` only pins phases that differ from their default — a clean repo where the defaults are already optimal gets zero redundant pins.
- All-Claude is a valid configuration; cross-family (builder ≠ auditor family) is what `recommended` prefers when ≥2 families are authed.
- `evolve setup complete` and `apply` both write atomically (temp + rename); `complete`'s marker merge never clobbers other `state.json` fields.
