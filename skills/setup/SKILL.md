---
name: setup
description: Use when the user runs /evo:setup (or /evo:setup), asks to configure evolve-loop, onboard, pick per-phase models, or learn how the pipeline works. Auto-detects available LLM CLIs/subscriptions, explains the pipeline concisely, then presents THREE ready-made config presets (Recommended/Economy/Max-quality) the Go binary computes deterministically from the public profiles — the user makes ONE choice and the binary writes per-phase pins to .evolve/policy.json. On a project that declares a `cli_routing` table (evolve-loop's own does) presets do not apply: the table owns every route, and the skill shows it with `evolve cli-routing show` instead. Runs once on first launch (the loop nudges) and is re-runnable anytime.
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

Call `evolve` if on PATH; otherwise `./go/bin/evolve` (or `$EVOLVE_GO_BIN`). Only `apply` (and `complete`) write; `detect`, `recommend` and `latest` are read-only (`latest` live-probes every ready CLI's bridge in parallel).

## Procedure

1. **Detect.** Run `evolve setup detect --json` and parse it. The digest has `clis[]` (per family: `binary_present`, `auth_mode`, `subscription_type`, `capability_tier`, `verdict`, and `tier_models`) and `phases[]` (per role: `current_cli`/`current_tier`, `source`, `default_cli`/`default_tier`, `envelope`, `allowed_clis`, `pin_violation`). A malformed `.evolve/policy.json` shows as a top-level `policy_error`. **If the digest has `"routing_table_declared": true`, the project's `cli_routing` table owns every route: skip steps 4, 5, 6 and 7 (`recommend` and `apply` refuse with exit 1), explain the table with `evolve cli-routing show` (and `evolve cli-routing explain <agent>` for one phase), and say that a change is `evolve cli-routing set agents.<role> <clis> [--model <tier>]`. Still run step 4b: model currency is the catalog's, not the routing's, so when it finds a `map_stale` family ask the "Adopt the latest live models?" question on its own (one AskUserQuestion, the same options and the same `evolve models refresh` handling as in step 5). Then go to step 8.**

2. **Present the detection** as a compact table — one row per CLI family with binary/auth/tier/verdict. (macOS Keychain OAuth is detected — a Keychain-authed `claude` shows `SUBSCRIPTION_OAUTH`, not blocked.)

3. **Explain the pipeline** concisely (this is the teaching goal). Read the canonical sources first — do NOT invent: `README.md` "Pipeline Design", `docs/concepts/overview.md`, `docs/architecture/phase-architecture.md`, `docs/architecture/dynamic-phase-routing.md`. Cover, in ~6–10 lines: the cycle (Scout → Build → Audit → Ship → Learn), what each phase produces, and *why it is trustworthy* (deterministic EGPS verdicts, SHA-chained ledger, adversarial Builder≠Auditor). Personalize: reference the user's actual detected CLIs.

4. **Recommend.** Run `evolve setup recommend --json`. It returns `available_families`, `cross_family_ok`, and `presets[]` — each preset a full per-phase `assignments[]` (`role`, `cli`, `tier`, `model`, `differs_from_default`, `warning`) plus a `description`; `default` names the recommended one. The binary already applied every rule (envelope clamp, `allowed_clis`, availability, cross-family split) — you do NOT re-derive any of this. A `degraded` preset has an unsatisfiable phase (e.g. no authed CLI); surface it but don't pick it.

4b. **Live latest-model probe.** Run `evolve setup latest --json` (read-only; queries EVERY ready CLI's bridge IN PARALLEL — cost ≈ the slowest single capture, well under 2 minutes). Each row carries `current_deep_model` (what dispatch resolves today, catalog-first), `latest_model` (freshest in that model's own lineage among the LIVE candidates), `map_stale` with the offending `stale_tiers` (staleness covers the WHOLE tier map, not just deep), and `current_seen_live`. Present the rows beside the detection table. A row with `error` is a failed probe — report it, never guess. `current_seen_live:false` means the mapped model never appeared in the live capture: present it as "needs manual verification", NEVER as confirmed-fresh. Do NOT re-derive freshness; the binary owns the lineage/alias rules.

5. **Present the THREE presets as ONE comparison** and let the user choose with a single **AskUserQuestion** (options: the preset names; pre-select `default`). Summarize each in a line or two from its `description` + a couple of notable assignments (e.g. "builder→codex, auditor→claude; cheaper phases on fast"). This is the whole "which model for which phase" decision — one pick, not twelve. **If step 4b found any `map_stale` family, add a SECOND question in the same AskUserQuestion call**: "Adopt the latest live models before applying?" quoting each stale family's `stale_tiers` entries verbatim (`codex balanced: gpt-5.4-mini → gpt-5.5-mini`), options **Adopt latest (Recommended)** / **Keep current maps**. On Adopt, run `evolve models refresh` BEFORE `apply` — refresh is the one sanctioned write path (family-filtered by `policy.json` `catalog.allowed_families`, carries operator `tier_fallbacks` forward); never hand-edit a tier map. Tell the user plainly: refresh re-probes ALL ready families, not only the stale one (unchanged offerings reuse their stored tier maps via the candidates fingerprint). Read its per-family `source:` lines: `source: detect fallback, reason="…"` means that family's live probe failed and it kept its manifest map, so report the reason and do not call that family adopted. Exit 1 means no family was classified live and nothing was written; say so, and do not claim the latest models were adopted. (Advanced: a user can edit the preset definitions in `.evolve/setup-presets.json`; mention it only if asked.)

6. **Apply.** Run `evolve setup apply --preset <choice>`. The binary deterministically writes the chosen preset's per-phase pins into `.evolve/policy.json` (lossless merge — preserves `floor`/`cli_health`/foreign pins; emits a pin ONLY where it differs from the profile default; stores the abstract tier). It refuses (non-zero exit) a degraded preset, a malformed existing policy, or a project that declares a `cli_routing` table (pins beside a table would refuse every launch, and a preset cannot say what the table says) rather than write something illegal. Use `--dry-run` first if the user wants to preview the merged policy.

7. **Verify.** Re-run `evolve setup detect` and confirm each pinned phase shows `source: "policy-pin"` with an EMPTY `pin_violation` and no top-level `policy_error`. (The same clamp hard-fails an out-of-bounds pin at dispatch, so this is the pre-flight catch.)

8. **Mark complete.** Run `evolve setup complete` to stamp the first-run marker (so the loop stops nudging). Confirm setup is done and that re-running `/evo:setup` anytime is safe.

## Notes

- Detection, recommendation, the policy write, and verification are ALL deterministic and live in Go; this skill never re-implements them and never hand-authors `policy.json`.
- Presets are data, not code: the shipped default is `go/internal/setup/presets.json`; a repo may override it with `.evolve/setup-presets.json`. Each preset's `tier_bias` is a generic strategy (`default`/`down`/`up`/`min`/`max`).
- Pins live in `.evolve/policy.json` (the user-owned override layer); profiles own the per-phase defaults. `apply` only pins phases that differ from their default — a clean repo where the defaults are already optimal gets zero redundant pins.
- A declared `cli_routing` table replaces pins altogether: its `clis`, `default`, `work`, `tiers` and `agents` keys are the routing, and `evolve cli-routing init|set|unset|migrate` is its only writer (compile before write, refused inside a phase or under a live cycle lease). Presets are profile-relative and assign one CLI per phase, which a table of chains, tier ceilings and the Claude floor cannot take as rules, so setup refuses them there.
- All-Claude is a valid configuration; cross-family (builder ≠ auditor family) is what `recommended` prefers when ≥2 families are authed.
- `evolve setup complete` and `apply` both write atomically (temp + rename); `complete`'s marker merge never clobbers other `state.json` fields.
