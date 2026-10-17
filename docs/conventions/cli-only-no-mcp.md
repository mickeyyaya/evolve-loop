# Convention: outside services are reached through a CLI, never MCP

**Status:** active · **Introduced:** 2026-10-17 · **Scope:** every loop phase, every console lane, every manifest and profile, and each project config file in this repository

On 2026-10-17 the operator said: "Forbid any mcp use, we only use cli to commute with other services, if it doesn't exist, we can build cli for that service".

## The rule

No loop phase, console lane or project config uses an MCP server or an MCP tool. Each outside service is reached through a CLI. When a service has no CLI, build one:

- an `evolve` verb, or
- an adapter CLI behind one adapter package, as [third-party-dependencies](third-party-dependencies.md) requires for each outside binary.

Example: the code index. The first trial registered the trail-code MCP server for agy and for Claude Code. The `evolve code` CLI (#918) replaced it. A phase agent now runs `evolve code callers <symbol>` from its shell. Only the adapter `go/internal/adapters/codeindex/trail` runs the trail-code binary ([code-index-cli](../architecture/code-index-cli.md)).

## Why

| Evidence | Source |
|---|---|
| The agents made 0 calls to the trail MCP tools in 20 agy phases (cycles 1907–1918). | [research C2–C3](../research/code-index-cli-2026-10.md) |
| Each Claude Code phase starts with `--strict-mcp-config`, so a user-scope MCP server never reaches it. | [research C4](../research/code-index-cli-2026-10.md) |
| An MCP server puts its tool schemas into each turn. Outside tests measured 4 to 32 times more tokens for MCP than for a CLI. | [research P3.1](../research/code-index-cli-2026-10.md) |
| The decision for the code index: a CLI, not MCP. | [ADR-0138](../architecture/adr/0138-phase-agents-query-the-code-index-through-an-evolve-cli-not-mcp.md) |

*Implication:* an MCP server costs tokens in each turn, and the agents do not use it. A shell command reaches every agent of every CLI, and each call is an `evolve` verb that a test can pin.

| Option | Why it is not chosen |
|---|---|
| Allow MCP when a service has no CLI | The cost per turn stays, and each server is one more surface that the loop does not control. Build the CLI. |
| Allow MCP for the console only | The console sessions read the same project config as the phases. One rule for every agent is simpler to check. |

## What this convention does not do

- It does not change the claude.ai account connectors of the operator. They are outside the repository.
- It does not edit a user-scope file such as `~/.claude.json` or the agy MCP config. Preflight only warns about them.
- It does not stop a test from naming MCP in a fixture.

## Regression guard

- `TestEveryClaudeLaunchIsStrictMCPAndNamesNoMCPConfig` in `go/internal/bridge/no_mcp_launch_test.go`. The `claude-tmux` and `claude-p` manifests put `--strict-mcp-config` in `default_args`, so each Claude Code launch gets the flag. The test realizes each manifest with each tracked profile, and fails on a launch without the flag or with an `--mcp-config` flag.
- `TestNoTrackedFileRegistersAnMCPServer` in `go/test/structure/mcp_test.go`. It fails on a tracked `.mcp.json` file. It also fails on a non-test Go file that names `mcpServers`, `claude mcp` or `agy mcp`, or that runs a process with an `mcp` argument. `TestTheMCPConfigReaderOnlyReads` pins that the one file that names `mcpServers` never writes a file.
- The `mcp-config` check of the loop preflight, pinned by `TestRun_MCPConfig_AConfiguredServerWarnsAndNeverHalts` in `go/internal/looppreflight/mcpconfig_test.go`. The check reads the files in the `mcp_config_files` key of each bridge manifest:
  - `agy-tmux`: `~/.gemini/config/mcp_config.json` and `~/.gemini/antigravity/mcp_config.json`;
  - `claude-tmux`: the project `.mcp.json`.

  For each CLI with a server, preflight prints `WARN: agy has MCP servers configured (<names>); the repo rule is CLI only (docs/conventions/cli-only-no-mcp.md)`. It never halts.
