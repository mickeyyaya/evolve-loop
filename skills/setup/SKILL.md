---
name: setup
description: Use when the user runs /setup, asks to configure evolve-loop, onboard, pick per-phase models, or learn how the pipeline works. Detects which LLM CLIs/subscriptions are available, explains the pipeline concisely, proposes a per-phase model assignment the user can adjust, writes .evolve/llm_config.json, and validates it against the integrity floor. Runs once on first launch (the loop nudges) and is re-runnable anytime.
argument-hint: "[--strict]"
---

# /setup

> Interactive onboarding. The deterministic parts (detection, validation, marker) run in the Go binary (`evolve setup detect|validate|complete`); the judgment parts (recommend models, explain the pipeline) run HERE, in your session — zero extra API cost. You PROPOSE; `evolve setup validate` is the kernel CLAMP (envelope + allowed_clis are hard, cross-family is advisory). See [docs/architecture/setup-onboarding.md](../../docs/architecture/setup-onboarding.md) and the deterministic core in [go/internal/setup/setup.go](../../go/internal/setup/setup.go).

## When to use

- The loop printed `[setup] First run …`, or the user typed `/setup`, or asked to configure models / learn the pipeline.
- Re-running is always safe — it re-detects and rewrites `.evolve/llm_config.json`.

## Binary

Call `evolve` if on PATH; otherwise `./go/bin/evolve` (or `$EVOLVE_GO_BIN`). All commands below are read-only except `complete` (stamps the marker) and your Write of `llm_config.json`.

## Procedure

1. **Detect.** Run `evolve setup detect --json` and parse it. The digest has `clis[]` (per family: `binary_present`, `auth_mode`, `subscription_type`, `capability_tier`, `verdict`) and `phases[]` (per role: `current_cli`, `current_model`/`current_tier`, `source`, `envelope` {min,default,max}, `cross_family_with`, `allowed_clis`).

2. **Present the detection** as a compact table — one row per CLI family with binary/auth/tier/verdict.
   - **Caveat (state, don't hide):** on macOS, `claude` may report `MISCONFIGURED`/`blocked` even when authed, because detection only checks `~/.claude/.credentials.json` and misses Keychain-stored OAuth. If the user is clearly running in a Claude session, treat claude as available and say so.

3. **Explain the pipeline** concisely (this is the teaching goal). Read the canonical sources first — do NOT invent: `README.md` "Pipeline Design", `docs/concepts/overview.md`, `docs/architecture/phase-architecture.md`, `docs/architecture/dynamic-phase-routing.md`. Cover, in ~6–10 lines: the cycle (Scout → Build → Audit → Ship → Learn), what each phase produces, and *why it is trustworthy* (deterministic EGPS verdicts, SHA-chained ledger, adversarial Builder≠Auditor). Personalize: reference the user's actual detected CLIs.

4. **Propose per-phase models** with a one-line rationale each (this answers "which LLM + model for each phase"). Honor these rules:
   - **Envelope:** the chosen tier must be within each phase's `envelope` [min..max]. (fast↔haiku, balanced↔sonnet, deep↔opus.)
   - **allowed_clis:** only assign a CLI the profile permits (or `["all"]`).
   - **Availability:** never route a phase to a CLI whose `verdict` is `blocked` (not authed / no binary), unless the user overrides.
   - **Cross-family (advisory):** when ≥2 families are authed, prefer different families for `builder` vs `auditor` (adversarial integrity). When only one is available, all-same-family is the legitimate fallback — note it, don't block.
   - **Tier heuristics** (`dynamic-phase-routing.md`): cheap/summarizing phases (triage, memo, evaluator) → fast; codegen/scan (scout, builder, tester) → balanced; adversarial/review/post-mortem (intent, plan-reviewer, tdd-engineer, auditor, retrospective) → deep.
   - Use **AskUserQuestion** to let the user accept the proposal or adjust specific phases.

5. **Write** `.evolve/llm_config.json` (schema_version 2). Each phase entry: `{ "provider": <anthropic|google|openai>, "cli": <claude|gemini|codex|agy>, "tier": <fast|balanced|deep>, "model": <haiku|sonnet|opus|exact-id> }`, plus a `_fallback`. Match the existing file's shape (see `examples/llm_config.example.json`).

6. **Validate (kernel clamp).** Run `evolve setup validate`. On exit `2`, read the printed `[error]` violations, fix the offending phases, and rewrite — loop until exit `0`. `[warn]` lines (e.g. cross-family on an all-one-family setup) are advisory; surface them but they do not block.

7. **Mark complete.** Run `evolve setup complete` to stamp the first-run marker (so the loop stops nudging). Confirm to the user that setup is done and they can re-run `/setup` anytime.

## Notes

- Detection + validation are deterministic and live in Go; this skill never re-implements them.
- `evolve setup complete` writes the marker via a lossless raw-merge — it never clobbers other `state.json` fields.
- All-Claude is a valid configuration; cross-family is preferred only when the user actually has multiple authed families.
