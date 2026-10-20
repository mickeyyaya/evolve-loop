## 2026-10-17 — a code index CLI for every phase agent (ADR-0138)

| Document | What it records |
|---|---|
| [research/code-index-cli-2026-10.md](research/code-index-cli-2026-10.md) | Research: why the trail-code MCP trial got 0 calls in 20 agy phases, and why Claude Code phases cannot reach it. It also records the index timings and surveys CLI against MCP, code-graph tools and `gopls`. Findings C1 to C8 and P3.1 to P3.5, refinements R1 to R6. |
| [plans/code-index-cli-2026-10.md](plans/code-index-cli-2026-10.md) | Plan: four designs with weighted scores. The winner is `evolve code` over a swappable backend, with trail-code first. Decisions D1 to D8 and components K1 to K5 with red tests. |
| [architecture/adr/0138-phase-agents-query-the-code-index-through-an-evolve-cli-not-mcp.md](architecture/adr/0138-phase-agents-query-the-code-index-through-an-evolve-cli-not-mcp.md) | Decision (Proposed): phase agents query the code index through four read-only `evolve code` commands, not MCP. A `Backend` interface keeps a type-checked backend open. |
| [architecture/code-index-cli.md](architecture/code-index-cli.md) | Spec: the commands and their exit codes, the `Backend` interface, the trail-code adapter, the index lifecycle, the sandbox, the usage log and the prompt text. It has a worked example and the guards. |
| [conventions/cli-only-no-mcp.md](conventions/cli-only-no-mcp.md) | Convention: no phase, lane or project config uses MCP. Each outside service is reached through a CLI. It names the three guards: the launch flag test, the structure test and the preflight warning. |
