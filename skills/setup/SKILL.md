---
name: setup
description: Use when the user runs /setup, asks to configure evolve-loop, onboard, pick per-phase models, or learn how the pipeline works. Detects which LLM CLIs/subscriptions are available, explains the pipeline concisely, proposes a per-phase CLI/model assignment the user can adjust, writes per-phase pins to .evolve/policy.json, and verifies them against the integrity floor. Runs once on first launch (the loop nudges) and is re-runnable anytime.
argument-hint: ""
---

# /setup

> Interactive onboarding. The deterministic parts (detection, pin verification, marker) run in the Go binary (`evolve setup detect|complete`); the judgment parts (recommend models, explain the pipeline) run HERE, in your session — zero extra API cost. You PROPOSE per-phase pins; the kernel CLAMP (envelope + allowed_clis) is enforced two ways: `evolve setup detect` reports any pin that breaches the floor as `pin_violation`, and dispatch hard-fails an out-of-bounds pin at cycle time. Step 9 removed the old `llm_config.json` layer — the durable per-phase override is now `.evolve/policy.json` `pins`. See [docs/architecture/setup-onboarding.md](../../docs/architecture/setup-onboarding.md) and the deterministic core in [go/internal/setup/setup.go](../../go/internal/setup/setup.go).

## When to use

- The loop printed `[setup] First run …`, or the user typed `/setup`, or asked to configure models / learn the pipeline.
- Re-running is always safe — it re-detects and rewrites `.evolve/policy.json` pins.

## Binary

Call `evolve` if on PATH; otherwise `./go/bin/evolve` (or `$EVOLVE_GO_BIN`). All commands below are read-only except `complete` (stamps the marker) and your Write of `.evolve/policy.json`.

## Procedure

1. **Detect.** Run `evolve setup detect --json` and parse it. The digest has `clis[]` (per family: `binary_present`, `auth_mode`, `subscription_type`, `capability_tier`, `verdict`, and **`tier_models`** {fast,balanced,deep} → that CLI's NATIVE model) and `phases[]` (per role: `current_cli`, `current_tier`, `source` — `profile` or `policy-pin`, `envelope` {min,default,max}, `cross_family_with`, `allowed_clis`, and `pin_violation` when a pin breaches the floor). A malformed `.evolve/policy.json` shows up as a top-level `policy_error`.

2. **Present the detection** as a compact table — one row per CLI family with binary/auth/tier/verdict.
   - **Caveat (state, don't hide):** on macOS, `claude` may report `MISCONFIGURED`/`blocked` even when authed, because detection only checks `~/.claude/.credentials.json` and misses Keychain-stored OAuth. If the user is clearly running in a Claude session, treat claude as available and say so.

3. **Explain the pipeline** concisely (this is the teaching goal). Read the canonical sources first — do NOT invent: `README.md` "Pipeline Design", `docs/concepts/overview.md`, `docs/architecture/phase-architecture.md`, `docs/architecture/dynamic-phase-routing.md`. Cover, in ~6–10 lines: the cycle (Scout → Build → Audit → Ship → Learn), what each phase produces, and *why it is trustworthy* (deterministic EGPS verdicts, SHA-chained ledger, adversarial Builder≠Auditor). Personalize: reference the user's actual detected CLIs.

4. **Propose per-phase models** with a one-line rationale each (this answers "which LLM + model for each phase"). **Always name the CLI's NATIVE model** from that CLI's `tier_models` — never a Claude alias for a non-Claude CLI. E.g. agy/balanced → `gemini-3.5-flash`; codex/deep → `gpt-5.5`, codex/balanced → `gpt-5.4`, codex/fast → `gpt-5.4-mini`; claude/balanced → `sonnet`. Honor these rules:
   - **Envelope:** the chosen tier must be within each phase's `envelope` [min..max]. (fast↔haiku, balanced↔sonnet, deep↔opus.)
   - **allowed_clis:** only assign a CLI the profile permits (or `["all"]`).
   - **Availability:** never route a phase to a CLI whose `verdict` is `blocked` (not authed / no binary), unless the user overrides.
   - **Cross-family (advisory):** when ≥2 families are authed, prefer different families for `builder` vs `auditor` (adversarial integrity). When only one is available, all-same-family is the legitimate fallback — note it, don't block.
   - **Tier heuristics** (`dynamic-phase-routing.md`): cheap/summarizing phases (triage, memo, evaluator) → fast; codegen/scan (scout, builder, tester) → balanced; adversarial/review/post-mortem (intent, plan-reviewer, tdd-engineer, auditor, retrospective) → deep.
   - Use **AskUserQuestion** to let the user accept the proposal or adjust specific phases.

5. **Write** `.evolve/policy.json` — the user-owned override layer. Set `pins`, a map of phase → `{ "cli": <claude|gemini|codex|agy>, "model": <the CLI's native model from tier_models> }`. Only write a pin where you are OVERRIDING the profile default (an unpinned phase keeps its profile's `cli` + `model_tier_default` — don't pin every phase just to restate the defaults). `model` is optional: pinning `cli` alone routes the phase to that CLI at the profile's tier; add `model` to also fix the exact model. Always use the CLI's NATIVE model from `detect.clis[].tier_models[tier]` (e.g. agy/balanced → `gemini-3.5-flash`; codex/deep → `gpt-5.5`; claude/deep → `claude-opus-4-7`). Merge into any existing `policy.json` — never drop the user's `mandatory_phases`. Example:
   ```json
   { "pins": { "auditor": { "cli": "codex", "model": "gpt-5.5" },
               "builder": { "cli": "claude" } } }
   ```

6. **Verify (kernel clamp).** Re-run `evolve setup detect` and check the `phases[]`: each phase you pinned should show `source: "policy-pin"` with your CLI/tier and an EMPTY `pin_violation`. If any `pin_violation` is present (cli ∉ `allowed_clis`, or model tier outside the `envelope`) or a `policy_error` appears, fix the offending pin and rewrite — loop until clean. Cross-family (builder vs auditor sharing a family) is advisory: surface it, don't block.

7. **Mark complete.** Run `evolve setup complete` to stamp the first-run marker (so the loop stops nudging). Confirm to the user that setup is done and they can re-run `/setup` anytime.

## Notes

- Detection + pin verification are deterministic and live in Go; this skill never re-implements them.
- Pins live in `.evolve/policy.json` (the user-owned override layer); profiles own the per-phase defaults. Pinning is OPT-IN per phase — leave a phase unpinned to keep its profile default.
- The kernel clamp is enforced at dispatch too: an out-of-bounds pin hard-fails the phase at cycle time, so `pin_violation` in `detect` is your pre-flight catch.
- `evolve setup complete` writes the marker via a lossless raw-merge — it never clobbers other `state.json` fields.
- All-Claude is a valid configuration; cross-family is preferred only when the user actually has multiple authed families.
