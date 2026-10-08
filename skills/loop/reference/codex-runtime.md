# Codex CLI Runtime

> How `/evo:loop` reaches the dispatcher under OpenAI Codex CLI. The Go bridge has two native codex drivers: `codex` runs `codex exec`, and `codex-tmux` runs the interactive codex TUI in tmux. The checked-in routing table (`.evolve/policy.json` `cli_routing`) does not list codex, so a phase runs on codex only under `--bypass-policy`.

## Invocation chain

```
User: /evo:loop 5 polish improve dispatcher
  (typed into Codex CLI)

  ↓ Codex resolves the skill
  ↓ activates SKILL.md content; reads reference/platform-detect.md
  ↓ detects platform = codex, reads reference/codex-runtime.md (this file)

Skill activates → STRICT MODE: execute exactly one shell command:
  "${EVOLVE_GO_BIN:-<plugin_root>/go/bin/evolve}" loop 5 polish "improve dispatcher"

  ↓ (Codex calls its shell-execution tool)

evolve loop runs one cycle per iteration, in process:
  the Go orchestrator (go/internal/core) runs the phases in order

  ↓

For each phase, the orchestrator dispatches the phase agent through the
Go bridge (go/internal/bridge). The routing table and the agent profile
(.evolve/profiles/<agent>.json) choose the CLI, not the CLI that runs
the conversation; `evolve cli-routing explain <agent>` prints the chain.

  ↓

When the chosen CLI is codex, the codex driver runs
`codex exec --output-last-message <artifact>`
(go/internal/bridge/driver_codex.go), and the codex-tmux driver runs the
codex TUI in tmux (go/internal/bridge/driver_codextmux.go).
```

(The legacy bash CLI adapters under adapters/*.sh were removed in the
script→Go migration; only the *.capabilities.json manifests remain, read by
the Go capability package. The bridge dispatches every driver natively.)

## Removed design: HYBRID and DEGRADED modes

The v8.51 codex adapter delegated to the claude driver (HYBRID) or ran in the same session (DEGRADED). The Go codex drivers have neither mode, and they do not delegate to claude. `adapters/codex.capabilities.json` still declares the old modes. The Go capability package reads these modes only for the consensus tier filter (`capability.QualityTier`).

The `evolve guard` hooks are Claude Code PreToolUse hooks (`.claude/settings.json`). They fire only when Claude Code runs the tool, and Codex does not run them. See [the trust architecture](../../../docs/concepts/trust-architecture.md).

## Required environment

| Variable | Required | Purpose |
|---|---|---|
| `codex` binary on PATH | yes | The binary that both codex drivers run |
| `OPENAI_API_KEY` | no | If it is set, the `codex` driver refuses to start (exit 3) unless `BRIDGE_ALLOW_OPENAI_API_KEY=1`. Codex uses its own login (`codex login`). |

## Checks before you run cycles

```bash
# 1. Confirm Codex binary is present
evolve doctor probe codex
# Expected: [doctor] OK: codex found at <path>

# 2. List the bridge drivers and their tiers
evolve bridge probe
# Expected: entries for "codex" and "codex-tmux"

# 3. Smoke-test detection
evolve detect-cli
# Expected: prints "codex" if you're in a Codex session

# 4. Run the bridge driver tests (from go/)
go test -count=1 ./internal/bridge/
# Expected: ok
```

## Profile.cli and the routing table

Each phase profile (`.evolve/profiles/{scout,builder,auditor,intent,retrospective}.json`) declares its own `cli` field. The routing table (`.evolve/policy.json` `cli_routing`, ADR-0119) decides the dispatch target, not session-wide CLI detection. This keeps multi-LLM-per-phase: for example, Scout on agy and Auditor on Claude in a single cycle.

## See also

- [reference/codex-tools.md](codex-tools.md) — tool name translation map
- [reference/platform-detect.md](platform-detect.md) — how the skill identifies its platform
- [reference/claude-runtime.md](claude-runtime.md) — the reference runtime
- [docs/architecture/platform-compatibility.md](../../../docs/architecture/platform-compatibility.md) — CLI support and limits
- [docs/incidents/gemini-forgery.md](../../../docs/incidents/gemini-forgery.md) — historical incident motivating pipeline-level structural defenses

## Last verified

- **Date:** 2026-10-08, against v22.27.0: `go/internal/bridge/driver_codex.go`, `go/internal/bridge/driver_codextmux.go` and `go/internal/bridge/manifests/codex*.json`.
- **Re-verify cadence:** quarterly, or whenever OpenAI announces a Codex CLI feature release.
- **Quick re-verify command:** `codex --version && codex --help 2>&1 | grep -iE 'exec|login'`
