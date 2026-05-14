# ADR-0001: `.evolve/llm_config.json` as LLM Router

**Status**: Accepted  
**Date**: 2026-05-14  
**Cycle**: 52 (Slice A, Foundation)  
**Plan**: `/Users/danleemh/.claude/plans/100-focus-on-only-smooth-kay.md` § ADR-1

---

## Context

Every one of the 13 phase profiles in `.evolve/profiles/*.json` declares `"cli": "claude"`. The dispatch seam at `scripts/dispatch/subagent-run.sh` reads `profile.cli` and routes to `scripts/cli_adapters/${cli}.sh`, but there is no way for an operator to say "Scout uses gemini-3-pro-preview, Builder uses gpt-5.5, Auditor uses claude-opus-4-7" without editing production profile files — which encode permission policy, not LLM choice.

These two concerns are conflated: **permission policy** (which tools are allowed, sandbox settings, budget caps) and **LLM selection** (which CLI and model to use for a phase).

---

## Decision

Create `.evolve/llm_config.json` as the authoritative LLM-selection file for evolve-loop. Keep `.evolve/profiles/<role>.json` for permission and sandbox policy. When `subagent-run.sh` resolves a phase, it consults `llm_config.json` FIRST for CLI+model selection; if absent, it falls back to the profile's `cli` and `model_tier_default`.

The lookup is implemented by `scripts/dispatch/resolve-llm.sh` — a pure function sourced by `subagent-run.sh`.

### Schema v1

```json
{
  "schema_version": 1,
  "phases": {
    "scout":     { "provider": "google",    "cli": "gemini", "model": "gemini-3-pro-preview" },
    "builder":   { "provider": "openai",    "cli": "codex",  "model": "gpt-5.5" },
    "auditor":   { "provider": "anthropic", "cli": "claude", "model": "claude-opus-4-7" }
  },
  "_fallback":   { "provider": "anthropic", "cli": "claude", "model_tier": "sonnet" },
  "_global_overrides": {
    "max_budget_usd_per_phase": null,
    "force_strict_isolation": false
  }
}
```

### Resolution precedence

| Priority | Source | Condition | `source` field emitted |
|---|---|---|---|
| 1 | `llm_config.phases.<role>` | Phase entry present with `cli` key | `llm_config` |
| 2 | `llm_config._fallback` | Fallback entry present with `cli` key | `llm_config_fallback` |
| 3 | Profile `cli` + `model_tier_default` | Config absent or role not declared | `profile` |

### Backward compatibility

`llm_config.json` is **operator-owned and optional**. When absent, behavior is identical to today: `subagent-run.sh` reads `profile.cli` directly. The resolver always succeeds (exits 0) when `llm_config.json` is absent; it returns `source="profile"`. Existing installs require zero config changes.

A reference template ships at `.evolve/llm_config.example.json`.

---

## Rationale

**Separation of concerns.** Permission policy (allowed tools, sandbox, budget caps) is stable per-role and changes rarely. LLM selection changes per-operator and per-experiment. One file edit (`llm_config.json`) re-routes Scout from claude to gemini without touching the profile's permission policy.

**Explicit over implicit.** Previously the gemini/codex adapters silently delegated to `claude.sh` when features were missing. Operators had no visibility into which CLI was actually running. The `source` field in resolver output makes LLM selection observable.

**Additive.** No existing profile, script, or persona changes in this cycle. The resolver is a new helper; `subagent-run.sh` integration is a cycle-53 change.

---

## Affected Files

| File | Change | Cycle |
|---|---|---|
| `scripts/dispatch/resolve-llm.sh` | New — LLM resolver pure function | 52 (this ADR) |
| `.evolve/llm_config.example.json` | New — reference template | 52 (this ADR) |
| `scripts/dispatch/subagent-run.sh:501-503` | Insert `resolve-llm.sh` call before adapter selection | **53** (deferred) |

---

## Non-Goals

- **HTTP API integrations**: this ADR covers CLI-based adapters only.
- **Removing Claude-specific profile concepts**: `allowed_tools`, `disallowed_tools`, `sandbox` remain Claude-adapter-specific; other adapters read them via `adapter_overrides.<cli>` (ADR-6).
- **Adding providers beyond claude/gemini/codex**: the abstraction layer is extensible; new providers are a future operator change.

---

## Verification (EGPS predicates)

| Predicate | Path | What it verifies |
|---|---|---|
| 001 | `acs/cycle-52/001-llm-config-load-and-override-cli.sh` | Phase entry in `llm_config.json` overrides profile `cli` |
| 002 | `acs/cycle-52/002-llm-config-missing-phase-fallback.sh` | Missing phase falls through to profile (not config fallback) |
| 003 | `acs/cycle-52/003-llm-config-absent-zero-config-works.sh` | Absent `llm_config.json` → backward-compat profile path |

All 3 predicates pass at cycle 52 ship.
