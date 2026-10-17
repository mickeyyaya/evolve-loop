# Plan: every phase agent queries the code index through `evolve code`, a CLI layer over a swappable backend

- **Status:** proposed, 2026-10-17.
- **Research:** [code-index-cli-2026-10](../research/code-index-cli-2026-10.md). **Design:** [code-index-cli](../architecture/code-index-cli.md). **Decision:** [ADR-0138](../architecture/adr/0138-phase-agents-query-the-code-index-through-an-evolve-cli-not-mcp.md).

## Table of contents

- [1. Request](#1-request)
- [2. Facts](#2-facts)
- [3. Goals and non-goals](#3-goals-and-non-goals)
- [4. Candidate designs](#4-candidate-designs)
- [5. Decisions](#5-decisions)
- [6. Components](#6-components)
- [7. Order](#7-order)
- [8. Verification](#8-verification)
- [9. Risks](#9-risks)
- [10. Open questions](#10-open-questions)

## 1. Request

On 2026-10-17 the operator asked that the loop agents use trail-code, "separated from the evo loop without interfere with evo loop's function. It should only boost the LLM performance." The operator then chose to name the tools in the phase prompts ("Ok, A"), and asked: "Could we consider not to use mcp but build a cli layer for trail-code functions?"

## 2. Facts

| Fact | Research id |
|---|---|
| The trail-code index of this repository builds in 31 s, rebuilds in about 2 s and answers a query in about 0.1 s. | C1 |
| The MCP server loaded in an agy phase, but the agents made 0 calls in 20 phases. | C2, C3 |
| `--strict-mcp-config` keeps the MCP server away from every Claude Code phase, by design. | C4 |
| Every phase agent of every CLI has a shell tool. | C5 |
| trail-code has a CLI with the same functions as its MCP server. Its output tells the agent to report "tokens saved". | C6 |
| trail-code does not type-check, so it misses calls through injected functions. | C7 |
| Current practice: a CLI costs fewer tokens than MCP for coding agents, and agents use the shell more readily. | P3.1, P3.2 |
| Agents often skip code-graph MCP tools, and tool use depends on the prompt. | P3.3, P3.4 |

## 3. Goals and non-goals

**Goals**

- **G1:** every phase agent can query the code index, on every CLI, with no change to how the loop starts a CLI.
- **G2:** the agents use it: the scout, builder and debugger prompts name the commands and say when each one helps.
- **G3:** the gain is measured: calls per phase, and the build tokens and pass rate before and after.
- **G4:** the index never changes a cycle's tree, never writes in a sandbox, and never tells an agent what to say.
- **G5:** a type-checked backend can replace trail-code later with no change for the agents.

**Non-goals**

- A hook on grep. It changes every grep of every phase.
- A new code-graph engine. This plan wraps an existing one.

## 4. Candidate designs

### A. Keep MCP, and name its tools in the phase prompts

- **Advantages:** no new code. The tools exist, and the prompt names them.
- **Weaknesses:** it cannot reach a Claude Code phase (C4). The tool schemas cost tokens in each turn (P3.1). The agents ignored the tools in 20 phases (C3).

### B. `evolve code`, a CLI layer over trail-code, behind a backend interface

`evolve code callers|api|find|map` calls a `Backend` interface. The first implementation runs the trail-code CLI read-only against the index worktree. The layer strips each instruction to the agent from the output, and it logs each call. The phase prompts name the commands.

- **Advantages:**
  - It reaches every CLI through the shell (C5).
  - It adds no schema tokens (P3.1).
  - It is an `evolve` verb, as the repository requires for each control.
  - The backend can change behind the same commands (G5).
- **Weaknesses:** it runs a Node tool from Go, so it needs one adapter home (C8). D3 extends the rule of [third-party-dependencies](../conventions/third-party-dependencies.md) from `go.mod` libraries to outside binaries. The convention does not cover a binary today, so K1 adds a structure test for it, and the convention gets one row. Its graph is approximate (C7).

### C. `evolve code` over `gopls`

The same CLI layer, with `gopls` `call_hierarchy`, `references` and `workspace_symbol` as the backend.

- **Advantages:** type-checked answers. It is the Go team's own tool.
- **Weaknesses:** `gopls` is not installed here (C8). Its CLI is experimental, and its start-up cost for each call on 6,000 files is not measured (P3.5).

### D. `evolve code` over a native Go index

The same CLI layer. `evolve` builds a call graph with the Go analysis libraries at each wave boundary, and it answers from that graph.

- **Advantages:** type-checked, Go only, fast after the build, no outside process.
- **Weaknesses:** the most work. It adds a library to `go.mod`, with its adapter home (C8).

### Scored comparison

Scores are 1 (worst) to 5 (best).

| Criterion (weight) | A. MCP + prompts | B. CLI over trail | C. CLI over gopls | D. CLI over native index |
|---|:-:|:-:|:-:|:-:|
| Reaches every CLI, G1 (×3) | 2 | 5 | 5 | 5 |
| Agents use it, G2 (×3) | 1 | 4 | 3 | 3 |
| Accuracy of the answers (×2) | 3 | 3 | 5 | 5 |
| Effort and risk (×2) | 5 | 4 | 2 | 1 |
| Token cost for each turn (×1) | 2 | 5 | 5 | 5 |
| **Weighted total (out of 55)** | **27** | **46** | **43** | **41** |

C and D score 3 for G2, because the per-call latency of `gopls` and the build time of a native index are not measured.

**The winner is B.** It is the only design that reaches every CLI now, at a low cost, with a measured backend. The backend interface keeps C and D open. If the usage log shows that the agents call the commands, but the answers are wrong for injected functions, a type-checked backend replaces trail-code. The agents then see no change.

**Why not A.** It was the plan until the operator asked about a CLI. The 0 calls in 20 phases and the Claude Code block make A the weakest design.

## 5. Decisions

| Id | Decision | Research id |
|---|---|---|
| D1 | `evolve code` has four read-only commands: `callers`, `api`, `find` and `map`. | C6, R1 |
| D2 | `evolve code` calls a `Backend` interface. trail-code is its first implementation. Each backend declares the queries it serves, and an ordered set from the policy key `code_index.backends` picks the first backend that serves each query. A shared conformance suite runs on every backend. | C7, R3 |
| D3 | One adapter package runs the trail-code binary. No other package runs it. It extends the rule of [third-party-dependencies](../conventions/third-party-dependencies.md) from `go.mod` libraries to outside binaries. | C8 |
| D4 | The layer removes each line of backend output that tells the agent what to do or say. | C6, R2 |
| D5 | The layer reads the index worktree only. It never refreshes the index inside a phase. The boundary refreshes it. | G4 |
| D6 | Each call appends one record to a usage log under `/tmp`: time, command, phase and working directory. | C3, R5 |
| D7 | The scout, builder and debugger prompts name the commands and say when each one helps. | P3.4, R4 |
| D8 | The trail-code MCP registrations go after one wave with the CLI live. | C3, R6 |

## 6. Components

| Id | Component | Red test first |
|---|---|---|
| K1 | D1–D4: `evolve code` and the trail-code adapter, with a fake backend for tests | a fake backend returns a "tokens saved" line, and `evolve code callers` prints the answer without it |
| K2 | D5: the index location and the boundary refresh, as a step of `wave next`. The refresh writes only the separate index worktree, never the main tree. A failed refresh is a warning, never a halt. | `wave next --dry-run` lists the refresh step |
| K3 | D6: the usage log | a call writes one record with its command and phase |
| K4a | D7: the persona text in `agents/evolve-scout.md`, `agents/evolve-builder.md` and `agents/evolve-debugger.md` | a Go test reads each persona, finds the four commands, and checks that each instruction line has 20 words or fewer |
| K4b | D7: the gate for K4a | the prompt-regression eval passes, and the G3 waves before and after show no loss |
| K5 | D8: the removal of the MCP registrations, and a doc note | `claude mcp list` and `agy mcp list` show no `trail` |

## 7. Order

The order is K1, K3, K2, K4a, K4b, K5.

1. K1 comes first, because every other component uses the command.
2. K3 comes next, so the first prompt change has a measure.
3. K2 makes the index current at each boundary. The separate worktree keeps K2 inside the rule that only the boundary writes the main tree.
4. K4a changes the personas.
5. K4b gates the change with the prompt-regression eval and the before and after waves.
6. K5 removes MCP after one wave of CLI use.

## 8. Verification

| Goal | Measure | Target |
|---|---|---|
| G1 | the usage log, by CLI | calls from agy and Claude Code phases |
| G2 | calls per phase of scout, build and debug | more than 0, against 0 in 20 phases (C3) |
| G3 | build tokens and audit pass rate, 3 waves before and after K4a | tokens fall, and the pass rate does not fall |
| G4 | `git status` of a cycle worktree after a call | clean |
| G5 | a fake backend passes the same command tests as trail-code | green |

## 9. Risks

| Risk | Control |
|---|---|
| An agent trusts an approximate answer for an injected function (C7). | The prompt says that the index is for orientation, and that the agent reads the file before it changes code. |
| The index is at main, not at the cycle's own changes. | The prompt says so. A cycle starts from main, so the index matches its base. |
| A prompt change lowers the quality of a phase. | The prompt-regression eval runs on each persona change, and G3 measures the pass rate. |
| The refresh step fails and blocks the boundary. | The step is non-fatal: it warns and goes on. |
| Node is not on a CI runner. | `evolve code` exits with a clear message when the index or its backend is absent. No test depends on Node. |

## 10. Open questions

- **Q1 (resolved):** D7 names `agents/evolve-debugger.md`. K4a checks that the bug-reproduction and fault-localization personas are the same files.
- **Q2:** do the tdd and auditor phases also get the commands? We propose not in the first trial: they review the work of another phase, and the trial measures the builders first.
