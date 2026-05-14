# ADR-6: Profile `adapter_overrides` — Per-Adapter Tool and Flag Policy

**Status:** Accepted  
**Date:** 2026-05-15  
**Cycle:** 54  
**Implemented in:** `scripts/dispatch/subagent-run.sh` (cmd_validate_profile, cmd_run)

---

## Context

Phase profiles (`.evolve/profiles/<role>.json`) contain Claude-specific policy: `allowed_tools`, `disallowed_tools`, `effort_level`, `permission_mode`, `extra_flags`, `sandbox`. When a phase resolves to a non-Claude CLI (via `llm_config.json` ADR-1), the Claude policy fields are either ignored or translated by the target adapter — but there was no mechanism for operators to declare *what* the non-Claude adapter should use instead.

Without this, operators who route Scout to Gemini have no way to tell `subagent-run.sh`: "when running on gemini, pass these gemini-specific tools instead of Claude's `allowed_tools`."

---

## Decision

Add an **optional, additive** `adapter_overrides` block to the profile schema:

```json
{
  "name": "scout",
  "cli": "claude",
  "allowed_tools": ["Read", "Grep", "Glob"],
  "adapter_overrides": {
    "gemini": {
      "tools": ["read_file", "grep", "list_directory", "web_search"],
      "extra_flags": ["--no-confirm"]
    },
    "codex": {
      "system_prompt_addendum": "Use codex-style tool naming throughout."
    }
  }
}
```

`subagent-run.sh` reads `profile.adapter_overrides.<resolved_cli>` after resolving the CLI and exports:
- `ADAPTER_TOOLS_OVERRIDE` — JSON array string of tools for the target adapter
- `ADAPTER_EXTRA_FLAGS_OVERRIDE` — JSON array string of extra flags

Both are passed to the adapter in its invocation environment (both in `cmd_run` and `cmd_validate_profile`).

---

## Schema

The `adapter_overrides` key is optional at the profile level. Each sub-key is the CLI name (`claude`, `gemini`, `codex`). All fields within the sub-key are optional:

| Field | Type | Consumed by |
|---|---|---|
| `tools` | `string[]` | Exported as `ADAPTER_TOOLS_OVERRIDE` (JSON string) |
| `extra_flags` | `string[]` | Exported as `ADAPTER_EXTRA_FLAGS_OVERRIDE` (JSON string) |
| `system_prompt_addendum` | `string` | Future use — adapter-specific system prompt suffix |

---

## Implementation

Insertion point: after `CAP_BUDGET_NATIVE` export in both `cmd_validate_profile` and `cmd_run`:

```bash
# ADR-6: adapter_overrides — read per-adapter tool/flag overrides from profile.
local ao_tools="" ao_flags=""
if command -v jq >/dev/null 2>&1; then
    local _ao_block
    _ao_block=$(jq -r ".adapter_overrides.\"${cli}\" // empty" "$profile" 2>/dev/null) || _ao_block=""
    if [ -n "$_ao_block" ]; then
        ao_tools=$(printf '%s' "$_ao_block" | jq -r '.tools // empty | if type == "array" then tojson else "" end' 2>/dev/null) || ao_tools=""
        ao_flags=$(printf '%s' "$_ao_block" | jq -r '.extra_flags // empty | if type == "array" then tojson else "" end' 2>/dev/null) || ao_flags=""
    fi
fi
```

The resolved values are passed as env prefix to `bash "$adapter"`:
```bash
ADAPTER_TOOLS_OVERRIDE="${ao_tools:-}" \
ADAPTER_EXTRA_FLAGS_OVERRIDE="${ao_flags:-}" \
bash "$adapter"
```

---

## Consequences

- **Positive**: Operators can specify per-CLI tool policy without modifying Claude's permission profile.
- **Positive**: Change is additive and non-breaking — existing profiles without `adapter_overrides` are unaffected (the block defaults to empty).
- **Positive**: Predicate 009 provides ongoing regression coverage.
- **Neutral**: Adapter implementations (gemini.sh, codex.sh) are responsible for consuming `ADAPTER_TOOLS_OVERRIDE` when passing tools to their respective CLIs. This ADR only covers the dispatch-side export.
- **Risk**: If an operator sets `adapter_overrides.claude` for a Claude-routed phase, `ADAPTER_TOOLS_OVERRIDE` is populated but `claude.sh` currently ignores it (it uses `allowed_tools` from the profile directly). This is intentional — claude.sh already has full permission policy in the profile; `adapter_overrides.claude` is reserved for future fine-grained Claude-side overrides.

---

## Related

- ADR-1: `.evolve/llm_config.json` LLM router (determines which CLI the adapter_overrides key is looked up for)
- ADR-2: Capability matrix (declares what capabilities each CLI supports)
- ADR-3: True native CLI invocation
- Predicate: `acs/cycle-54/009-adapter-overrides-block-honored.sh`
