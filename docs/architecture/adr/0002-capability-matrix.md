# ADR-0002: Capability Matrix per CLI Adapter

**Status:** Accepted  
**Date:** 2026-05-14  
**Cycle:** 53 (Slice A, Cycle 2 — capability matrix + subagent-run.sh integration)  
**Supersedes:** n/a  
**Related:** ADR-0001 (LLM router)

---

## Context

`scripts/dispatch/subagent-run.sh` constructs the flag set passed to each CLI adapter
(budget cap, sandbox, permission scoping). Before v10.X, it assumed every adapter
supported the same set of flags. This caused silent failures when routing to `gemini` or
`codex`: subagent-run.sh would pass `--max-budget-usd` to an adapter that would silently
ignore it, and operators had no visibility into which pipeline guarantees were absent.

The v8.51.0 capability check (`_capability-check.sh`) resolved a `quality_tier` string
(`full`/`hybrid`/`degraded`/`none`) but didn't expose individual capability booleans in a
form that code could gate on. The existing `capabilities.*.modes` / `default` schema was
correct but verbose — extracting a boolean required knowing the full mode interpretation.

---

## Decision

**Option A (extend in-place):** Keep the existing v8.51 `capabilities.{cap}.modes/default`
structure and add a flat `supports.*` boolean block at the top level of each
`<cli>.capabilities.json`. The `supports` booleans are derived from the existing mode data
by convention (see mapping rule below) and are the primary API for ADR-2 consumers.

### Schema addition (all three adapters)

```json
{
  "adapter": "<cli>",
  "version": 2,
  "supports": {
    "budget_cap_native":    <bool>,
    "permission_scoping":   <bool>,
    "sandbox_native":       <bool>,
    "non_interactive_prompt": <bool>,
    "structured_logs":      <bool>,
    "model_flag":           <bool>
  },
  "capabilities": { ... }
}
```

**Mapping rule** (non-normative):
- `supports.budget_cap_native`   = `capabilities.budget_cap.default != "none" AND != "hybrid"`
- `supports.permission_scoping`  = `capabilities.profile_permissions.default != "none" AND != "hybrid"`
- `supports.sandbox_native`      = `capabilities.sandbox.default != "none" AND != "hybrid"`

### Per-adapter values (v10.X cycle-53 baseline)

| Field | claude | gemini | codex |
|---|---|---|---|
| `budget_cap_native` | `true` | `false` | `false` |
| `permission_scoping` | `true` | `false` | `false` |
| `sandbox_native` | `true` | `false` | `false` |
| `non_interactive_prompt` | `true` | `true` | `true` |
| `structured_logs` | `true` | `true` | `true` |
| `model_flag` | `true` | `true` | `true` |

### Behavioral contract in subagent-run.sh (v10.X)

When `subagent-run.sh` resolves a phase to a CLI (via ADR-1 llm_config router or profile
fallback), it reads `$ADAPTERS_DIR/${cli}.capabilities.json` and checks `supports.*`:

1. **`supports.budget_cap_native == false`** → emit to stderr:
   ```
   [adapter-cap] WARN cli=<cli> missing=budget_cap_native substitute=wall_clock_timeout
   ```
   and set `CAP_BUDGET_NATIVE=false` in the adapter's env. The adapter omits
   `--max-budget-usd` when `CAP_BUDGET_NATIVE=false`.

2. **`supports.permission_scoping == false`** → emit to stderr:
   ```
   [adapter-cap] WARN cli=<cli> missing=permission_scoping substitute=kernel_role_gate_only
   ```

3. Both WARNs are written to `EVOLVE_DISPATCH_PLAN_LOG` (when set) in a
   `capability_warns[]` array for machine-readable consumption by predicates and tooling.

4. `WARN` lines are parseable: `[adapter-cap] WARN cli=<name> missing=<cap> substitute=<sub>`.
   Regex: `\[adapter-cap\] WARN cli=[a-z]+ missing=[a-z_]+ substitute=[a-z_]+`.

### Why not Option B (new schema)?

Option B (replacing the v8.51 `capabilities.*.modes` structure with the flat `supports.*`
booleans) would require updating all consumers of `_capability-check.sh` and existing code
that reads `capabilities.budget_cap.modes`. The risk of silent regression is high. Option A
is purely additive: version bumped from 1→2 to signal the extension, existing readers
continue to work unchanged.

---

## Consequences

### Positive
- Operators get explicit, machine-readable WARN lines when a capability is absent — no more
  silent degradation.
- `CAP_BUDGET_NATIVE` env var lets adapters conditionally omit unsupported flags without
  hardcoding CLI names.
- `EVOLVE_DISPATCH_PLAN_LOG` enables hermetic predicate testing without live CLI invocations.
- Backward-compatible: existing v8.51 consumers of `capabilities.*` are unchanged.
- Trust kernel (role-gate, ship-gate, phase-gate, ledger SHA chain) is unaffected — it
  operates on bash commands, not LLM output.

### Negative / Risks
- `supports.*` values are currently static (not probed at runtime). If a CLI gains
  `budget_cap_native` support in a future release, the manifest must be updated manually.
  Mitigation: `probes[]` array in the schema (v8.51 pattern) can automate detection in a
  future cycle.
- Two schema versions (1 and 2) coexist. `_capability-check.sh` must tolerate `version: 1`
  files (absent `supports` block → all capabilities assumed `true` for backward compat).

---

## Files Affected

| File | Change |
|---|---|
| `scripts/cli_adapters/claude.capabilities.json` | Added `supports.*` block, `version` 1→2 |
| `scripts/cli_adapters/gemini.capabilities.json` | Added `supports.*` block, `version` 1→2 |
| `scripts/cli_adapters/codex.capabilities.json` | Added `supports.*` block, `version` 1→2 |
| `scripts/dispatch/subagent-run.sh` lines ~889–940 | Added capability WARN emission, `EVOLVE_DISPATCH_PLAN_LOG`, `cli_resolution_json` |
| `scripts/dispatch/subagent-run.sh` `cmd_validate_profile` | Same WARNs in validate path |
| `scripts/dispatch/subagent-run.sh` `write_ledger_entry` | 10th arg `cli_resolution` |
| `scripts/cli_adapters/gemini.sh` | Added `VALIDATE_ONLY=1` early exit |
| `scripts/cli_adapters/codex.sh` | Added `VALIDATE_ONLY=1` early exit |

---

## Test Coverage

EGPS predicates (cycle 53):
- `acs/cycle-53/004-capability-matrix-honored-no-budget-cap.sh` — WARN emitted, `CAP_BUDGET_NATIVE=false` reaches adapter
- `acs/cycle-53/007-degraded-mode-emits-structured-warn.sh` — dispatch plan JSON, warn format, count match
- `acs/cycle-53/008-model-routed-via-llm-config.sh` — model from llm_config reaches adapter as `RESOLVED_MODEL`

Regression suite (cycle 52):
- `acs/regression-suite/cycle-52/001-003-*.sh` — backward compat / zero-config preserved
