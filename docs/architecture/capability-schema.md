# Capability Manifest Schema (v8.51.0+)

> The schema operators and contributors must follow when authoring or modifying CLI adapter capability manifests.

## Why this exists

Pre-v8.51, each adapter's behavior was hardcoded — Gemini hard-failed if Claude was missing, Codex always exited 99. v8.51.0 introduced **declarative capability manifests** so:

- The pipeline reads what an adapter can structurally guarantee, instead of inferring from per-adapter shell logic.
- Adding a new CLI requires writing a manifest + adapter — the pipeline doesn't change.
- Operators see the resolved capability tier explicitly via `./bin/check-caps`, instead of debugging exit codes.
- Graceful degradation is a first-class concept: missing capabilities lower quality, never block the pipeline.

## Files

| File | Purpose |
|---|---|
| `scripts/cli_adapters/_capabilities-schema.json` | JSON Schema (Draft 2020-12) defining the manifest structure |
| `scripts/cli_adapters/_capability-check.sh` | Resolver: reads a manifest + runs probes, emits resolved tier per dimension |
| `scripts/cli_adapters/<cli>.capabilities.json` | One manifest per adapter (claude / gemini / codex; add a row for any new CLI) |
| `bin/check-caps` | Operator entry point — wraps `_capability-check.sh` |

## Schema overview

```jsonc
{
  "adapter": "claude" | "gemini" | "codex",     // matches the .sh file name
  "version": 1,                                  // manifest schema version
  "capabilities": {
    "subprocess_isolation":  <capability_value>, // see below
    "budget_cap":            <capability_value>,
    "sandbox":               <capability_value>,
    "profile_permissions":   <capability_value>,
    "challenge_token":       <capability_value>
  },
  "probes": [
    {
      "check": "<probe-name>",                   // e.g., claude_on_path
      "if_true_mode": "hybrid",                  // mode to apply when probe passes
      "if_false_mode": "degraded",               // mode to apply when probe fails
      "applies_to": ["subprocess_isolation"]     // capabilities this probe affects
    }
  ],
  "notes": "Free-form context for operators."
}
```

## Capability values

Each of the five capability fields can be either:

### Form 1: a fixed string

```json
"subprocess_isolation": "full"
```

Use when the adapter always provides this capability at the same tier (e.g., Claude Code always has `subprocess_isolation: full`).

### Form 2: an object with modes + default + warning

```json
"subprocess_isolation": {
  "modes": ["hybrid", "degraded"],
  "default": "degraded",
  "warning": "claude binary not on PATH; running in same-session mode"
}
```

Use when the adapter's tier depends on runtime probes. The resolver picks among `modes` based on probe results; if no probe matches, falls back to `default`. If the resolved mode is `degraded` or `none`, the warning is surfaced to the operator.

## Tier semantics

| Tier | Meaning |
|---|---|
| `full` | Adapter natively provides this capability with no compromise (e.g., `claude -p --max-budget-usd`). |
| `hybrid` | Adapter delegates to a more-capable runtime (e.g., Gemini → Claude binary) and inherits its caps. |
| `degraded` | Adapter runs in same-session mode; this capability is not available, but pipeline-level structural defenses still apply. |
| `none` | Adapter cannot provide the capability at all. Pipeline relies entirely on its own kernel hooks and forgery defenses. |

Resolved `quality_tier` per cycle is the **lowest mode across all five capabilities** — i.e., one degraded capability degrades the whole entry.

## Probe registry

Probes are runtime checks declared in the manifest's `probes` array. The resolver knows how to evaluate each named probe.

| Probe name | What it checks | Implementation |
|---|---|---|
| `claude_on_path` | Whether `claude` binary is invocable. Honors `EVOLVE_GEMINI_CLAUDE_PATH` / `EVOLVE_CODEX_CLAUDE_PATH` test seams when `EVOLVE_TESTING=1`. | `_capability-check.sh:probe_claude_on_path` |
| `sandbox_exec_available` | Darwin + `sandbox-exec` present. | `_capability-check.sh:probe_sandbox_exec_available` |
| `bwrap_available` | Linux + `bwrap` present. | `_capability-check.sh:probe_bwrap_available` |

Adding a new probe: extend `_capability-check.sh:run_probe()` with a new case branch and document it here.

## Resolved output

`./bin/check-caps <adapter> --json` emits:

```jsonc
{
  "adapter": "<name>",
  "version": 1,
  "resolved": {
    "subprocess_isolation": {"mode": "hybrid", "warning": ""},
    "budget_cap":           {"mode": "hybrid", "warning": "..."},
    "sandbox":              {"mode": "hybrid", "warning": ""},
    "profile_permissions":  {"mode": "hybrid", "warning": "..."},
    "challenge_token":      {"mode": "hybrid", "warning": ""}
  },
  "quality_tier": "hybrid",
  "warnings": ["<surfaced warnings for degraded/none caps>"],
  "probes": {"claude_on_path": true},
  "notes": "<from manifest>"
}
```

`subagent-run.sh` consumes this at adapter dispatch (resolves `quality_tier`, logs the per-capability warnings, passes `quality_tier` to the ledger writer as a 9th argument). Each `agent_subprocess` ledger entry post-v8.51 carries `quality_tier` as an annotation.

## Authoring a new adapter

To add a 4th CLI (e.g., `copilot`):

1. Write `scripts/cli_adapters/copilot.sh` mirroring `gemini.sh`'s pattern (HYBRID delegation when claude binary present, DEGRADED same-session otherwise).
2. Write `scripts/cli_adapters/copilot.capabilities.json` declaring its capabilities. Validate against the schema:
   ```bash
   jq empty scripts/cli_adapters/copilot.capabilities.json
   ```
3. Add `copilot` to the adapter enum in `_capabilities-schema.json:properties.adapter.enum`.
4. Add tests at `scripts/tests/copilot-adapter-test.sh` mirroring `codex-adapter-test.sh`.
5. Register the test in `scripts/utility/run-all-regression-tests.sh:SUITES`.
6. Document at `skills/evolve-loop/reference/copilot-runtime.md` and `copilot-tools.md`.
7. Run `./bin/preflight` to validate end-to-end.

The pipeline does NOT need to change. The capability framework absorbs the new adapter at dispatch time.

## Validation

The schema is enforced at two layers:

- **Static**: `jq empty <manifest>` confirms valid JSON. The `cli-capability-test.sh` regression suite walks all manifests and asserts they parse + cover all 5 required capabilities.
- **Runtime**: `_capability-check.sh` rejects unknown capability names and probe names with non-zero exit; `bin/check-caps` surfaces the failure to operators.

## Backward compatibility

The `quality_tier` field added to ledger entries in v8.51.0 is **backward-compatible**: pre-v8.51 readers tolerate missing fields via `// empty` jq filters. Existing analysis tools (`bin/cost`, `verify-ledger-chain.sh`, etc.) work unchanged. Operators upgrading from v8.50.x see no behavior change unless they explicitly query the new field.

## See also

- [docs/architecture/platform-compatibility.md](platform-compatibility.md) — capability matrix per CLI + install guidance
- [docs/incidents/gemini-forgery.md](../incidents/gemini-forgery.md) — why structural defenses are pipeline-level (so degraded mode is safe)
- [skills/evolve-loop/reference/claude-runtime.md](../../skills/evolve-loop/reference/claude-runtime.md), [gemini-runtime.md](../../skills/evolve-loop/reference/gemini-runtime.md), [codex-runtime.md](../../skills/evolve-loop/reference/codex-runtime.md) — per-CLI invocation patterns
