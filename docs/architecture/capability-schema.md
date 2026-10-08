# Capability Manifest Schema (v8.51.0+)

> The schema operators and contributors must follow when authoring or modifying CLI adapter capability manifests.

## Why this exists

Pre-v8.51, each adapter's behavior was hardcoded — Gemini hard-failed if Claude was missing, Codex always exited 99. v8.51.0 introduced **declarative capability manifests** so:

- The pipeline reads what an adapter can structurally guarantee, instead of inferring from per-adapter shell logic.
- Adding a new CLI requires writing a manifest + adapter — the pipeline doesn't change.
- Operators see the resolved capability tier explicitly, instead of debugging exit codes. Removed design: `./bin/check-caps` calls the deleted `_capability-check.sh` and fails; `evolve bridge probe` prints a tier for each bridge driver.
- Graceful degradation is a first-class concept: missing capabilities lower quality, never block the pipeline.

## Files

| File | Purpose |
|---|---|
| `adapters/_capabilities-schema.json` | JSON Schema (Draft 2020-12) defining the manifest structure |
| `go/internal/capability` | Resolver: `QualityTier` reads a manifest, runs the probes and returns the lowest tier; `Inspect` reads the `supports` block |
| `adapters/<cli>.capabilities.json` | One manifest per adapter (claude / claude-tmux / gemini / codex / antigravity / agy; add a row for any new CLI) |
| `bin/check-caps` | Removed design: it wraps the deleted `_capability-check.sh` and fails |

## Schema overview

```jsonc
{
  "adapter": "claude" | "gemini" | "codex" | "antigravity", // usually the file name; agy.capabilities.json declares "antigravity"
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
| `claude_on_path` | The `claude` binary is on PATH. Tests inject a `capability.Probe` function. | `capability.DefaultProbe` |
| `agy_on_path` | The `agy` binary is on PATH. | `capability.DefaultProbe` |
| `sandbox_exec_available` | Darwin + `sandbox-exec` present. | `capability.DefaultProbe` |
| `bwrap_available` | Linux + `bwrap` present. | `capability.DefaultProbe` |

Adding a new probe: extend `DefaultProbe` in `go/internal/capability/qualitytier.go` with a new case and document it here. An unknown probe name reports false.

## Resolved output

Removed design: `./bin/check-caps <adapter> --json` emitted this JSON, and no command prints it now:

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

`subagent-run.sh` consumed this JSON, and the script is removed. Now `evolve subagent run` writes a `quality_tier` into each `agent_subprocess` ledger entry. It takes the tier from the manifest's `supports` block (`subagentrun.QualityTier`), not from this output. `evolve consensus-dispatch` uses `capability.QualityTier` to filter voters by tier.

## Authoring a new adapter

To add a new CLI (for example, `copilot`):

1. Write a Go bridge driver (`go/internal/bridge/driver_copilot.go`) and its bridge manifest (`go/internal/bridge/manifests/copilot.json`).
2. Write `adapters/copilot.capabilities.json` declaring its capabilities. Validate against the schema:
   ```bash
   jq empty adapters/copilot.capabilities.json
   ```
3. Add `copilot` to the adapter enum in `_capabilities-schema.json:properties.adapter.enum`.
4. Add Go tests for the driver in `go/internal/bridge/`. `go test ./...` runs them.
5. Document at `skills/loop/reference/copilot-runtime.md` and `copilot-tools.md`.
6. Run `evolve bridge probe` to confirm that the bridge lists the new driver.

Removed design: the bash adapter steps (`copilot.sh` and the bash test suite) are gone, and a new CLI now needs a Go driver. `./bin/preflight` still exists, but it calls the deleted `full-dry-run.sh` and fails.

## Validation

The schema is enforced at two layers:

- **Static**: `jq empty <manifest>` confirms valid JSON. `TestQualityTier_GoldenParityWithBashManifests` (`go/internal/capability`) reads the in-tree manifests and checks their resolved tiers.
- **Runtime**: Removed design: the Go resolver does not reject unknown names, so an unknown probe reports false and an unknown mode ranks as `none`.

## Backward compatibility

The `quality_tier` field added to ledger entries in v8.51.0 is **backward-compatible**: pre-v8.51 readers tolerate missing fields through `// empty` jq filters. Existing analysis tools (for example, `evolve ledger verify`) work unchanged. Operators upgrading from v8.50.x see no behavior change unless they explicitly query the new field.

## See also

- [docs/architecture/platform-compatibility.md](platform-compatibility.md) — capability matrix per CLI + install guidance
- [docs/incidents/gemini-forgery.md](../incidents/gemini-forgery.md) — why structural defenses are pipeline-level (so degraded mode is safe)
- [skills/loop/reference/claude-runtime.md](../../skills/loop/reference/claude-runtime.md), [gemini-runtime.md](../../skills/loop/reference/gemini-runtime.md), [codex-runtime.md](../../skills/loop/reference/codex-runtime.md) — per-CLI invocation patterns
