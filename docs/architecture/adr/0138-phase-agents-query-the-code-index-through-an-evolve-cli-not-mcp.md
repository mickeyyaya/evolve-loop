# ADR-0138: Phase agents query the code index through an `evolve` CLI, not MCP

- **Status:** Proposed (2026-10-17).
- **Plan:** [code-index-cli-2026-10](../../plans/code-index-cli-2026-10.md). **Research:** [code-index-cli-2026-10](../../research/code-index-cli-2026-10.md). **Design:** [code-index-cli](../code-index-cli.md).
- **Relates to:**
  - D3 extends the rule of [third-party-dependencies](../../conventions/third-party-dependencies.md) from `go.mod` libraries to outside binaries. The convention does not cover a binary today, so K1 adds a structure test for it, and the convention gets one row.
- **Evidence:**
  - C3: the agents made 0 calls to the MCP tools in 20 agy phases.
  - C4: `--strict-mcp-config` keeps every MCP server away from the Claude Code phases, by design.
  - C5: every phase agent of every CLI has a shell tool.
  - P3.1: a CLI costs fewer tokens than MCP for coding agents.

## Context

The operator wants the loop agents to use a code index, with no change to how the loop works. The first trial used MCP. The server loaded, but no agent called it, and it cannot reach a Claude Code phase. Current practice finds that agents use the shell more readily than MCP tools, and that a CLI adds no schema tokens to each turn.

A shell command reaches every agent of every CLI. It is also an `evolve` verb, as the repository requires for each control. The index itself is approximate today, so the command calls its backend through an interface. A type-checked backend can then replace it with no change for the agents.

## Decision

1. `evolve code` gives four read-only commands: `callers`, `api`, `find` and `map`.
2. `evolve code` calls a `Backend` interface, and trail-code is its first backend.
3. Each backend declares the queries it serves. An ordered set from the policy picks a backend for each query.
4. Every backend passes one shared conformance suite.
5. One adapter package runs the trail-code binary.
6. The layer removes each instruction to the agent from the backend output.
7. The layer reads the index worktree only, and the boundary refreshes it.
8. Each call appends one record to a usage log.
9. The scout, builder and debugger prompts name the commands and say when each one helps. The persona edit is a loop change. It is justified because without it the agents made 0 calls (C3, P3.4).
10. A failed index refresh at the boundary is a warning, never a halt. **Amended on 2026-10-26** by D5.2 of [code-index-cli.md](../code-index-cli.md) §4: an absent index is still no halt. Each other refresh failure makes `sync-main` fail after the merge, because a WARN hid a stale index for 5 days.
11. The MCP registrations go after one wave with the CLI live. **Done on 2026-10-17:** the console removed them. The [cli-only-no-mcp](../../conventions/cli-only-no-mcp.md) convention now forbids MCP in the repository.

## Consequences

**Good:**
- Every phase agent can reach the index, Claude Code phases included, with no change to the boot.
- No schema tokens go into each turn.
- The usage log measures the gain for each phase.
- A type-checked backend can replace trail-code behind the same commands.

**Bad:**
- `evolve` runs a Node tool, so the host needs Node and the index worktree.
- The first backend misses calls through injected functions. The prompt says to read the file before a change.
- The prompt change is a change to loop behaviour. The prompt-regression eval checks it.

**What changes this decision:**
- If the usage log shows no calls after the prompt change, the index gives no gain, and it goes.
- If the agents use it but the answers are wrong, a type-checked backend replaces trail-code.

## Rejected

| Option | What it offers | Why it is not chosen |
|---|---|---|
| Keep MCP and name its tools in the prompts | No new code. | It cannot reach Claude Code phases, it costs schema tokens, and the agents ignored it in 20 phases. |
| `gopls` as the first backend | Type-checked answers from the Go team's tool. | It is not installed here, its CLI is experimental, and its start-up cost is not measured. It stays a later backend. |
| A native Go index | Type-checked, Go only, no outside process. | It is the most work, and it adds a library to `go.mod`. It stays a later backend. |
| A hook on grep that adds graph results | Use without a choice by the agent. | It changes every grep of every phase, which is more than the operator asked. |
