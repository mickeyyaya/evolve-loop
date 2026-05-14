# ADR-3: True Native CLI Invocation — NATIVE/HYBRID/DEGRADED Three-Mode Pattern

**Status:** Accepted  
**Date:** 2026-05-15  
**Cycle:** 54  
**Implemented in:** `scripts/cli_adapters/gemini.sh`, `scripts/cli_adapters/codex.sh`

---

## Context

Prior to this ADR, `gemini.sh` and `codex.sh` operated in two modes only:
- **HYBRID**: delegate everything to `claude.sh` when `claude` binary is on PATH.
- **DEGRADED**: same-session execution when no `claude` binary is present.

The capability matrices (`gemini.capabilities.json`, `codex.capabilities.json`) already declared `non_interactive_prompt: true`, indicating these CLIs *can* be invoked natively for prompt-driven non-interactive execution. But the adapters never acted on this capability — they always delegated to Claude or ran in-process.

This gap was documented in predicate 005 (gemini) and 006 (codex) as RED tests before this ADR's implementation.

---

## Decision

Each adapter now supports a three-mode execution hierarchy:

| Mode | When | Behavior |
|---|---|---|
| **NATIVE** | CLI binary on PATH AND `supports.non_interactive_prompt: true` in capabilities | Invoke the adapter's own binary directly (`exec $BIN < $PROMPT_FILE`). Takes priority over HYBRID. |
| **HYBRID** | `claude` binary on PATH | Delegate to `claude.sh` for full subprocess isolation, profile permissions, budget cap. |
| **DEGRADED** | Neither binary available | Same-session execution. Pipeline kernel hooks provide structural safety. |

**Priority order**: NATIVE > HYBRID > DEGRADED.

Rationale for NATIVE taking priority over HYBRID: if an operator has `gemini` installed, they expect Gemini to handle Scout — not to silently delegate to Claude. The HYBRID fallback is for operators who have Claude but not the target CLI.

---

## Test Seams

Both adapters support a `EVOLVE_*_BINARY` test seam (gated by `EVOLVE_TESTING=1`) that overrides the binary detected from PATH:

```bash
# gemini.sh
EVOLVE_TESTING=1 EVOLVE_GEMINI_BINARY=/path/to/mock bash scripts/cli_adapters/gemini.sh

# codex.sh
EVOLVE_TESTING=1 EVOLVE_CODEX_BINARY=/path/to/mock bash scripts/cli_adapters/codex.sh
```

Setting the env var to empty (`EVOLVE_GEMINI_BINARY=""`) explicitly simulates "no binary found" — the test seam returns 1 from `detect_gemini_native()`.

---

## Capability Gate

NATIVE mode is gated on `supports.non_interactive_prompt: true` in the adapter's capabilities manifest. If an operator sets this to `false` (e.g., because the target CLI's non-interactive mode is not yet confirmed), NATIVE mode is skipped and the adapter falls through to HYBRID/DEGRADED.

Current default: `true` for both gemini and codex (optimistic; allows the code path when binaries are available). Production use depends on the actual CLI version supporting prompt-from-stdin invocation.

---

## Consequences

- **Positive**: Operators with `gemini` or `codex` on PATH now get true native execution. The abstraction layer (ADR-1) is honored end-to-end.
- **Positive**: Predicates 005 and 006 are GREEN, providing ongoing regression coverage of the NATIVE path.
- **Neutral**: Claude-only deployments are unaffected — HYBRID mode still activates when only `claude` is on PATH.
- **Risk**: If a native binary's non-interactive mode differs from Claude's in ways the adapter doesn't handle (e.g., different stdin format, different stdout structure), the phase may fail at artifact verification. Mitigation: `run-acs-suite.sh` regression predicates catch regressions early.

---

## Related

- ADR-1: `.evolve/llm_config.json` LLM router
- ADR-2: Capability matrix per adapter
- ADR-6: Profile `adapter_overrides` block
- Predicates: `acs/cycle-54/005-gemini-native-invocation.sh`, `acs/cycle-54/006-codex-native-invocation.sh`
