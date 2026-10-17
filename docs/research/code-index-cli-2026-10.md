# A code index for every phase agent: why the MCP trial got no use, and what a CLI layer can do (research, 2026-10)

> The plan that uses this research: [code-index-cli-2026-10](../plans/code-index-cli-2026-10.md). The design: [code-index-cli](../architecture/code-index-cli.md). The decision record: [ADR-0138](../architecture/adr/0138-phase-agents-query-the-code-index-through-an-evolve-cli-not-mcp.md).

Date: 2026-10-17. Method:

- a count of the calls to the trail-code MCP server in its call log, across the agy phases of cycles 1907–1918;
- a read of how the loop launches Claude Code and agy;
- timings of trail-code on this repository;
- a web survey of CLI against MCP for coding agents, of code-graph tools, and of `gopls`.

Each local claim names its command or file. "Synthesis" marks our own inference.

## Table of contents

- [1. The request](#1-the-request)
- [2. Local evidence](#2-local-evidence)
- [3. Current practice](#3-current-practice)
- [4. Adopt or reject](#4-adopt-or-reject)
- [5. Refinements to adopt](#5-refinements-to-adopt)

## 1. The request

On 2026-10-17 the operator asked to use trail-code across the loop: "I would like loop agent to use trail-code but I would like this feature to be separated from the evo loop without interfere with evo loop's function. It should only boost the LLM performance." Later the operator asked: "Did we leverage trail-code completely across all llm clis?" The operator chose to name the tools in the phase prompts, then asked: "Could we consider not to use mcp but build a cli layer for trail-code functions?"

## 2. Local evidence

**C1** trail-code builds a graph of this repository in 31 s: 51,347 symbols and 134,377 edges from 6,004 files. After a change of one file, a rebuild reads only that file and takes about 2 s. A query takes about 0.1 s (local runs of `trail build` and `trail callers`, 2026-10-17).
*Implication:* the index is fast enough to call from a phase agent at no real cost.

**C2** On 2026-10-17 the trail-code MCP server was registered for agy and for Claude Code at user scope. It loaded inside the sandbox of an agy phase: a live agy process had the server as its child.
*Implication:* the server worked. The agents had the tools.

**C3** The 20 agy phases of cycles 1907–1918 were 8 scout, 8 triage, 2 build, 1 bug-reproduction and 1 fault-localization. In them, the agents made **0** calls to the trail tools. The only call in the log is a console test (`/private/tmp/evolve-trail-calls.ndjson`). A hint in agy's global instructions (`~/.gemini/GEMINI.md`) did not change this.
*Implication:* a tool that an agent never chooses gives no gain. The agents chose their shell and grep.

**C4** The loop starts each Claude Code phase with `--strict-mcp-config --setting-sources project`. The flag makes Claude Code ignore every MCP server outside its config, so the user-scope trail server never reaches a Claude phase. The flag is a deliberate cut of 33–38% of boot tokens (`docs/architecture/token-floor-history.md`).
*Implication:* through MCP, the audit, tdd, reviewer and `deep` build phases cannot reach the index without a change to the loop's boot.

**C5** Every phase agent of every CLI has a shell tool. A phase agent already runs `evolve` verbs, `go test` and `grep` through it.
*Implication:* a command in the shell reaches every phase agent, with no change to how the loop starts a CLI.

**C6** trail-code has a CLI with the same functions as its MCP server: `trail callers`, `trail skeleton`, `trail ask`, `trail grep` and `trail map`. Each prints its answer as text. Each also prints a line that tells the agent to report "tokens saved" to the user (`dev/.tools/trail`, version 0.23.0).
*Implication:* a CLI layer can call trail-code directly. It must strip the line that tells the agent what to say.

**C7** trail-code parses with tree-sitter. It does not type-check, so it misses a call through an injected function, for example the `send` of `killGroup` (`trail callers killGroup --direction out` gave no callees).
*Implication:* the first backend gives a fast but approximate graph. A type-checked backend is a later option.

**C8** The repository keeps each outside library behind one adapter package ([third-party-dependencies](../conventions/third-party-dependencies.md)). The convention covers `go.mod` libraries and does not cover a binary today. `gopls` is not installed on this host.

## 3. Current practice

**P3.1** Several 2026 comparisons find that a CLI costs fewer tokens than MCP for coding agents. One test of 75 runs gave the CLI a 100% success rate and MCP 72%. MCP used 10 to 32 times more tokens for each task. A second test of the GitHub MCP server against the `gh` CLI gave 4 to 32 times more tokens for MCP.

The sources give one reason: each MCP server adds its tool schemas to the context. One server added about 55,000 tokens before any work ([DeployHQ](https://www.deployhq.com/blog/clis-or-mcp-for-coding-agents-practical-comparison), [Firecrawl](https://firecrawl.dev/blog/mcp-vs-cli), [noqta](https://noqta.tn/en/blog/mcp-vs-cli-ai-agents-tool-protocol-comparison-2026)). Most of these sources sell CLI or MCP tools. The tests are small, so the direction is the strong signal, not the numbers.

**P3.2** The same sources say that coding agents are built around a shell. They say that models learned shell use from much more data than MCP use. They also give the case for MCP: scoped permissions and discovery in an IDE ([DeployHQ](https://www.deployhq.com/blog/clis-or-mcp-for-coding-agents-practical-comparison), [rentierdigital](https://rentierdigital.xyz/blog/mcp-vs-cli-when-each-wins)).

**P3.3** Code-graph tools for agents report that agents often do not call their MCP tools. One project puts a hook on the agent's own grep: the hook runs the graph lookup and adds the result ([Codebase-Memory, arXiv 2603.27277](https://arxiv.org/html/2603.27277v1)).

Another sends its usage guidance in the MCP start-up response ([codegraph](https://github.com/colbymchenry/codegraph)). An independent benchmark found that a graph never beat grep and read on tokens. Its one clear gain was accuracy on structural questions. The gain came on a large repository with a small model ([code-graph-vs-grep](https://github.com/nefayran/code-graph-vs-grep)).
*Implication:* the 0 calls of C3 match what others see. The gain of a graph is on structural questions, such as "who calls this", more than on tokens.

**P3.4** Tool use depends on wording and on the prompt. An edited tool description gave over ten times more use in one study. Behaviour rules belong in the prompt, not only in the tool description ([AWS, MCP tool design](https://aws.amazon.com/blogs/machine-learning/mcp-tool-design-practical-approaches-and-tradeoffs/), [Anthropic, writing tools for agents](https://www.anthropic.com/engineering/writing-tools-for-agents)).
*Implication:* the phase prompt must name the command and say when to use it.

**P3.5** `gopls` has CLI commands for `call_hierarchy`, `references`, `implementation`, `workspace_symbol` and `definition`. The Go team calls the CLI experimental and meant for debugging. Each position is `file:line:col` ([gopls usage](https://go.googlesource.com/tools/+/master/gopls/internal/cmd/usage/usage.hlp), [gopls skill notes](https://tessl.io/registry/skills/github/samber/cc-skills-golang/golang-gopls)). The survey found no data on its speed for a large module.
*Implication:* `gopls` can give a type-checked backend later. Its start-up cost for each call is not measured.

## 4. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| A shell command for the index, not MCP | Adopt | C3–C5, P3.1, P3.2 |
| Name the command and its use in the phase prompts | Adopt | C3, P3.4 |
| trail-code as the first backend | Adopt | C1, C6 |
| A backend interface, so a type-checked backend can come later | Adopt | C7, P3.5 |
| A hook on grep that adds graph results | Reject for now | It changes every grep of every phase, which is more than the operator asked. |
| `gopls` as the first backend | Reject for now | C8: not installed, and P3.5: its CLI is experimental and its speed is not measured. |
| Keep the MCP server beside the CLI | Reject after the trial | C3: no use, and P3.1: it costs schema tokens in each Claude console turn. |

## 5. Refinements to adopt

- **R1** (C4, C5, P3.1): `evolve code` is a read-only CLI layer over the index, called from the shell of any phase agent.
- **R2** (C6): the layer strips the "tokens saved" line and any other instruction to the agent from the backend output.
- **R3** (C7, P3.5): the layer calls its backend through an interface. trail-code is the first backend; `gopls` or a native Go index can follow.
- **R4** (C3, P3.4): the scout, builder and debugger prompts name the `evolve code` commands and say when each one helps.
- **R5** (C3): each call writes one line to a usage log, so the gain is measured per phase.
- **R6** (C3, P3.1): the MCP registrations go once the CLI is live.
