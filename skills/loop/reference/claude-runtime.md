# Claude Code Runtime

> How `/evo:loop` reaches the dispatcher under Claude Code. This is the reference runtime — every gate and adapter assumes Claude semantics first.

## Invocation chain

```
User: /evo:loop 5 polish improve dispatcher

  ↓ Claude Code resolves the slash command
  ↓ via .claude-plugin/plugin.json → skills/loop/SKILL.md

Skill activates → STRICT MODE: execute exactly one Bash command:
  "${EVOLVE_GO_BIN:-<plugin_root>/go/bin/evolve}" loop 5 polish "improve dispatcher"

  ↓

evolve loop runs one cycle per iteration, in process:
  the Go orchestrator (go/internal/core) runs the phases in order

  ↓

For each phase, the orchestrator dispatches the phase agent through the
Go bridge (go/internal/bridge). The routing table (.evolve/policy.json
cli_routing) and the agent profile (.evolve/profiles/<agent>.json) choose
the CLI; `evolve cli-routing explain <agent>` prints the chain.

  ↓

The claude-tmux driver runs Claude Code in a tmux pane, and the claude-p
driver runs `claude -p`. Both run under the OS sandbox, sandbox-exec (macOS)
or bwrap (Linux), when it applies — see internal/bridge/sandbox_wrap.go
(sandbox.ShouldWrap, gated by EVOLVE_SANDBOX) and driver_tmux_boot.go.
The claude-p driver also applies --allowedTools / --disallowedTools / etc.
(The legacy bash CLI adapters under adapters/*.sh were removed in the
script→Go migration; the bridge now dispatches natively.)

  ↓

`evolve subagent run <agent> <cycle> <workspace>` dispatches one phase
agent by hand through the same bridge.
```

## Required environment

| Variable | Required | Purpose |
|---|---|---|
| `claude` binary on PATH | yes | The runtime engine. Verify with `command -v claude`. |
| `ANTHROPIC_API_KEY` | no — leave it unset | Both claude drivers refuse to start when it is set (exit 3). They use the Claude Code subscription login. |
| `CLAUDE_CODE_INTERACTIVE` | set automatically by Claude Code | Used by `evolve detect-cli` to identify the platform. |

Optional but recommended:

| Variable | Purpose |
|---|---|
| `EVOLVE_SANDBOX=on` | `EVOLVE_SANDBOX=on` warns when the sandbox-exec or bwrap wrap cannot apply. A profile that requires the sandbox fails closed. `off` is the host opt-out. The values are `auto` (the default), `on` and `off`. |

## Trust boundary

Four PreToolUse kernel hooks fire on Claude Code:

| Hook | Tool | Job |
|---|---|---|
| `evolve guard ship` | Bash | A git commit, git push or gh release command must be the native `evolve ship` command |
| `evolve guard role` | Edit, Write | Edit/Write must match the active phase's path allowlist |
| `evolve guard phase` | Agent, Task | While a cycle is active, the in-process dispatch tool is denied, so phase agents go through the bridge |
| `evolve guard docdelete` | Bash | `rm` or `mv` on `docs/**` or `knowledge-base/**` must be the canonical archival `mv` |

These hooks are configured in `.claude/settings.json`. The Go state machine (`go/internal/core`) enforces the phase order. Together they are the structural enforcement of the trust boundary. The orchestrator cannot edit source directly, cannot push without `evolve ship`, and cannot skip phases.

## When to NOT use the strict dispatcher

The dispatcher is mandatory for `/evo:loop` invocations. The only documented exception is `dispatch.policy: "off"` in `.evolve/policy.json`, which skips per-cycle ledger verification — used solely for debugging the dispatcher itself. Setting it for real cycles disables the only structural enforcement of pipeline completeness; do not.

## Failure modes

| Dispatcher exit | Meaning | Follow-up |
|---|---|---|
| `0` | All cycles ran AND ledger verified end-to-end | Report summary; done |
| `1` | The batch stopped early (for example, the cycle-number circuit breaker tripped) | Surface the cycle's stderr; do NOT retry inline |
| `2` | A cycle bypassed Scout/Builder/Auditor (CRITICAL) | Quote ledger counts; recommend `git log` of `.evolve/runs/cycle-N/`; STOP |
| `3` | The batch completed, but it absorbed a recoverable failure or a verdict FAIL | Report the failed cycles |
| `4` | System-failure halt (ADR-0072) | HALT and report the escalation; do NOT retry |
| `5` | Quota pause | Resume later with `evolve loop --resume` |
| `10` | Bad CLI arguments | Re-prompt with valid args |
| `130` | Interrupt (SIGINT or SIGTERM) | Report the stop |

## See also

- SKILL.md STRICT MODE section
- [reference/claude-tools.md](claude-tools.md) — tool names this runtime expects
- [docs/guides/publishing-releases.md](../../../docs/guides/publishing-releases.md) — the publish vs push vs ship distinction
