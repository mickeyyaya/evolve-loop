# Code index CLI: `evolve code` gives every phase agent a read-only code graph

> Research: [code-index-cli-2026-10](../research/code-index-cli-2026-10.md). Plan: [code-index-cli-2026-10](../plans/code-index-cli-2026-10.md). Decision: [ADR-0138](adr/0138-phase-agents-query-the-code-index-through-an-evolve-cli-not-mcp.md).
> Status: K1, K2 and K3 are built (2026-10-17): the `evolve code` verb, the `codeindex` layer, the trail adapter, the policy block `code_index`, the boundary refresh and the usage log. The components K4a, K4b and K5 are planned. A name in this file that has no code is marked "planned".
>
> This file is the spec of record for the `evolve code` verb. The research holds the evidence, the plan holds the scored designs and the order of work, and the ADR holds the decision.

On 2026-10-17 the operator asked that the loop agents use trail-code, "separated from the evo loop without interfere with evo loop's function. It should only boost the LLM performance." This spec shows how the design meets that request.

**Terms used throughout:**

| Term | Meaning |
|---|---|
| code index | A graph of the symbols and the calls of this repository. trail-code builds it with tree-sitter (C1, C7). |
| trail-code | The outside tool that builds and queries the index. Its binary is `dev/.tools/trail/node_modules/.bin/trail` (C6). |
| index worktree | A git worktree at `dev/code-index` (the default of `code_index.index_dir`). It holds the checkout that the index describes. The operator makes it. The boundary refreshes it (K2, built). |
| `Backend` | The Go interface `codeindex.Backend` (`go/internal/codeindex`) that the `evolve code` commands call. It has one method for each command. |
| adapter | The one package, `go/internal/adapters/codeindex/trail`, that runs the trail-code binary. It implements `Backend`. |
| boundary | The step between two waves where the loop merges and refreshes. `evolve wave next` runs it (`go/cmd/evolve/cmd_wave_next.go:102`). |
| phase agent | The LLM CLI process of one phase: scout, builder, debugger and others. Each has a shell tool (C5). |
| usage log | The file `code-calls.ndjson` (built, K3). A call inside a phase writes the cycle workspace copy. A call outside a phase writes the per-user cache copy. |

## Goals

| Id | Goal | Measure | Target |
|---|---|---|---|
| G1 | Every phase agent of every CLI can query the index, with no change to how the loop starts a CLI. | the usage log, by CLI | calls from agy and Claude Code phases |
| G2 | The agents use the index. | calls per phase of scout, build and debug | more than 0, against 0 in 20 phases (C3) |
| G3 | The gain is measured. | build tokens and audit pass rate, 3 waves before and after the prompt change | tokens fall, pass rate does not fall |
| G4 | The index never changes a cycle's tree, never writes in a sandbox, and never tells an agent what to say. | `git status` of a cycle worktree after a call | clean |
| G5 | A type-checked backend can replace trail-code with no change for the agents. | a fake backend passes the same command tests as trail-code | green |

## Table of contents

- [1. The command surface](#1-the-command-surface)
- [2. The Backend interface](#2-the-backend-interface)
- [3. The trail-code adapter](#3-the-trail-code-adapter)
- [4. The index lifecycle](#4-the-index-lifecycle)
- [5. The sandbox](#5-the-sandbox)
- [6. The usage log](#6-the-usage-log)
- [7. The prompt text](#7-the-prompt-text)
- [8. The flow and a worked example](#8-the-flow-and-a-worked-example)
- [9. Backends and the benchmark](#9-backends-and-the-benchmark)
- [Separation from the loop](#separation-from-the-loop)
- [Guards](#guards)
- [Limits](#limits)

## 1. The command surface

### Issue

An agent that wants to know "who calls this function" today runs `grep` and reads many files. The MCP trial gave the agents a graph, and they made 0 calls in 20 phases (C3). The MCP server also cannot reach a Claude Code phase, because `--strict-mcp-config` hides it by design (C4).

### Gap

An MCP tool lives in the tool list of one CLI. A shell command is the one surface that all CLIs share (C5). Current practice agrees: a CLI costs fewer tokens than MCP, and models use the shell more readily (P3.1, P3.2). The agents need a command with a short name and a plain output.

### Solution

`evolve code` has four commands. Each command is read-only (D1). The code is `go/cmd/evolve/cmd_code.go`.

| Command | Question it answers | Backend call (trail-code) |
|---|---|---|
| `evolve code callers <symbol> [--direction in\|out]` | Who calls this symbol (`in`, the default)? Which symbols does it call (`out`)? | `trail callers` |
| `evolve code api <file>` | What are the exported signatures of this file, without the bodies? | `trail skeleton` |
| `evolve code find <query>` | Which symbols match a question in plain words? | `trail ask` |
| `evolve code map [dir]` | What is the structure of this directory? | `trail map` |

The output is plain text. Each hit is one line of three fields that a tab separates: `file:line`, the symbol and a short text. A hit that has no line, such as a directory of `map`, shows the `file` alone. A tab or a newline inside a field becomes a space. The agent opens the file at the line.

`find` joins its words, so `evolve code find catalog refresh` and `evolve code find "catalog refresh"` are the same query. Every command takes `--project-root P` (default: `$EVOLVE_PROJECT_ROOT`, then the nearest directory at or above the working directory that holds `.git`).

| Exit code | Meaning | Output |
|---|---|---|
| 0 | The query found at least one hit. | the hit lines on stdout |
| 1 | The query ran and found nothing. | one line on stderr that names the query |
| 2 | The index or the backend is absent. | one line on stderr that says what is absent, with the path of the binary and of the index, and a pointer to section 4 |
| 3 | No backend in the list serves the command. | one line on stderr: `no backend serves <command>` |
| 4 | Any other failure: a refused policy, a root that is no hub, an unknown backend name or a failed backend. | one line on stderr |
| 10 | A usage error: a missing or extra argument, a wrong flag or a wrong direction. | the reason and the usage line on stderr |

The `map [dir]` command lists the directory clusters of the index. With a `dir`, it keeps the clusters at or below that directory. The other commands read the index, whatever the `dir`.

### The policy block

The wiring reads the block `code_index` of `.evolve/policy.json`. The decode is strict, like the other blocks. A relative path resolves against the hub root. The hub root is the parent of the bare store (`docs/operations/workspace-layout.md`). A path that is absolute needs no hub.

| Key | Default | Meaning |
|---|---|---|
| `backends` | `["trail"]` | the ordered list of backend names. It is a capability route: the first backend that serves a query answers it. There is no failover when that backend is absent. |
| `index_dir` | `dev/code-index` | the index worktree |
| `trail_bin` | `dev/.tools/trail/node_modules/.bin/trail` | the trail-code binary |

The exit code 2 is the case of a host with no Node, no binary or no index worktree. The command stops with a clear message and does not guess (plan section 9). No command writes to the repository, to the index or to the cycle worktree.

## 2. The Backend interface

### Issue

trail-code parses with tree-sitter and does not type-check. It misses a call through an injected function: `trail callers killGroup --direction out` gave no callees (C7). If the command code calls trail-code directly, a better engine needs a rewrite of the command.

### Gap

`gopls` has a call hierarchy (P3.5), and the Go analysis libraries can build a type-checked graph (plan design D). Neither can replace trail-code unless the commands depend on a stable seam. The plan scores C and D below B today, but keeps them open (plan section 4).

### Solution

The commands call the `codeindex.Backend` interface (D2, `go/internal/codeindex`). This is the Strategy pattern: the commands do not know which engine answers. Notes: [internal-codeindex](packages/internal-codeindex.md).

| Method | Serves | Returns |
|---|---|---|
| `Callers(ctx, symbol, direction)` | `evolve code callers` | a list of `Hit` (`File`, `Line`, `Symbol`, `Text`) |
| `API(ctx, file)` | `evolve code api` | a list of `Hit`, one for each definition, with its signature as `Text` |
| `Find(ctx, query)` | `evolve code find` | a list of `Hit` |
| `Map(ctx, dir)` | `evolve code map` | a list of `Hit`, one for each directory, with `Line` 0 |

Besides the four methods, a `Backend` has `Name()` and `Capabilities()`. Each method returns `([]Hit, error)`. The typed error `ErrNotFound` means exit 1, `ErrAbsent` means exit 2 and `ErrUnsupported` means exit 3. The command code maps the three errors to the exit codes of section 1.

| Backend | State | Reason |
|---|---|---|
| trail-code adapter | first, built (K1): `go/internal/adapters/codeindex/trail` | C1, C6: fast, and it has a CLI |
| fake backend | built (K1), for tests: `codeindextest.Fake` | G5: the same command tests run on any backend |
| `gopls` | later | P3.5: type-checked, but its CLI is experimental and its start-up cost is not measured (C8) |
| native Go index | later | the most work, and it adds a library to `go.mod` |

### Adapt the layer to any search algorithm

On 2026-10-17 the operator asked: "build the cli layer as abstract layer which can be adapted to different indexing fast code search algorithm for llm". So a new algorithm must plug in as data and one new package, with no change to the commands. Four rules make that true:

1. **One result type.** Every backend returns the same `Hit`: `file`, `line`, `symbol` and a short `text`. A command prints hits in one format, whatever the backend.
2. **Declared capabilities.** Each backend says which of the four queries it serves (`Capabilities()`). A call graph serves `callers`. A text or embedding search serves `find` only. A command that no backend serves exits 3 with the message "no backend serves `<command>`".
3. **A backend set, chosen by config.** `main` builds a `Backends` set from the policy key `code_index.backends`, an ordered list such as `["trail"]`. `codeindex.New` builds it from a map of factory functions and fails on an unknown name. For each query, the first backend in the list that serves it answers. This is the Chain of Responsibility pattern over the Strategy interface. There is no `init()` registry: the set is built and injected, as ADR-0137 asks for the bridge.

   The list is a capability route and not a failover chain. When the chosen backend is absent, the command exits 2 and does not ask the next backend. An absent backend is an operator fix. A silent switch to another backend hides the fault and changes the answer.
4. **One conformance suite.** The shared suite `codeindextest.Run` runs the same cases on every backend. A new backend is done when it passes the suite. It runs on the fake and on the trail adapter.

   The cases are a known caller, a known file API, a query with no answer (`ErrNotFound`) and an absent index (`ErrAbsent`). A query that the backend does not serve must give `ErrUnsupported`.

| Example backend | Algorithm | Serves | Cost to add |
|---|---|---|---|
| `trail` | tree-sitter graph, pre-built | `callers`, `api`, `find`, `map` | K1 |
| `gopls` | type-checked call hierarchy | `callers`, `api`, `find` | one adapter package |
| `native` | Go analysis call graph, built at the boundary | `callers`, `api` | one package and one `go.mod` library |
| `grep` | text search over the index worktree | `find` | one small package, and a safe last entry in the list |

**Why a chain and not one backend.** No single algorithm is the best at every query. A type-checked graph is exact for `callers`, and a text or embedding search is better for `find` when the agent knows only some words. With a chain, the operator puts the best backend for each query first, and a cheap one last.

The decision to change the backend belongs to the usage log. If the agents call the commands but the answers fail for injected functions, a type-checked backend replaces trail-code (ADR-0138, "What changes this decision").

## 3. The trail-code adapter

### Issue

trail-code is a Node program. The repository gives each outside library one adapter package ([third-party-dependencies](../conventions/third-party-dependencies.md), C8). This design applies the same rule to the trail-code binary. D3 extends the rule of [third-party-dependencies](../conventions/third-party-dependencies.md) from `go.mod` libraries to outside binaries. The convention does not cover a binary today, so K1 adds a structure test for it, and the convention gets one row. The trail-code CLI also prints a line that tells the agent to report "tokens saved" to the user (C6).

### Gap

A phase agent must not receive an instruction from a tool. A line such as "tell the user ..." is a prompt that the operator did not write. The agent can act on it. A plain pass-through of the output fails G4.

### Solution

One package, `go/internal/adapters/codeindex/trail` (D3), runs the binary. Notes: [internal-adapters-codeindex-trail](packages/internal-adapters-codeindex-trail.md). No other package runs it. The adapter does four things:

1. It runs `dev/.tools/trail/node_modules/.bin/trail` (the path today) with `DO_NOT_TRACK=1` and with the refresh off. The refresh is off through the environment variable `GRAFT_NO_REFRESH=1`, not through a flag.
2. It points the binary at the index worktree `dev/code-index`, never at the cycle worktree. `evolve code` resolves the path `code_index.index_dir` against the hub root (section 1). The adapter passes that path to trail as the explicit `[dir]` argument. The reason: the default of trail is the nearest ancestor with a `graft/` index.
3. It maps each `Backend` method to one trail-code verb, as the table in section 1 shows.
4. It removes each output line that tells the agent what to do or say (D4). The "[trail] tokens saved ... tell the user" header is the known case (C6).

The strip is a filter in the adapter (`strip.go`). A test feeds the captured binary output, with its "tokens saved" header, and expects hits without it. Only parsed fields leave the adapter, so a line that the parser does not know cannot reach an agent. The filter is a second control. If trail-code adds a new instruction line in a later version, the filter must learn it. The adapter keeps the match rules in one list so that a new rule is one row.

## 4. The index lifecycle

### Issue

The index must match the code that the agent works on. A rebuild of the full index takes 31 s (C1). A phase that rebuilds the index on each call wastes time. A phase that rebuilds it also writes files, which breaks G4.

### Gap

A cycle writes to its own worktree. If the index follows the cycle worktree, each change needs a rebuild, and the index tool writes inside the sandbox of the phase.

### Solution

The index worktree is separate from every cycle worktree. The boundary refreshes it (D5). A phase never does.

1. The boundary runs `git checkout --detach origin/main` in the index worktree.
2. The boundary runs an incremental `trail build`. It reads only the changed files and takes about 2 s (C1).
3. A phase reads the index. A query takes about 0.1 s (C1).

The refresh is a step of `evolve wave next` (built, K2), beside the other boundary steps. `wave next --dry-run` lists it as "refresh the code index".

K2 is built as follows:

1. The step is the verb `code-refresh`. It runs after the merges, the build and the `doctor live` checks, and before `loop-stop --release`. A lane therefore starts on a fresh index.
2. The step reads `code_index.index_dir` and resolves it against the hub root, as `evolve code` does.
3. The step cleans `index_dir` and resolves its symbolic links. It then refuses `index_dir` in four cases. The first case is the project root. The second is a path inside the project root. The third is a path under `<project root>/.evolve/worktrees`. The fourth is the runtime plane root of the hub, or a path inside it.
4. A refusal is a WARN, and no `git` or trail command runs.
5. The step runs `git rev-parse --show-toplevel` with `index_dir` as the working directory. The output must equal the cleaned `index_dir`. If not, `index_dir` is a plain directory inside another repository. The step then prints a WARN. It runs no `git fetch` and no `git checkout`.
6. The step runs `git fetch --quiet origin` and then `git checkout --quiet --detach origin/main`. Both commands run with the index worktree as the working directory. No command names the project root, so the step does not write the main tree.
7. If local changes in the index worktree conflict, the checkout fails. The step then prints a WARN. It does not run `reset` and does not run trail. The operator cleans the index worktree.
8. The whole step has a deadline of 5 minutes. When the deadline ends, `sysexec` sends SIGTERM to the child process. After `WaitDelay` (5 s), it kills the child process. The step then prints a WARN and exits 0.
9. The step calls `Backends.Refresh`. It runs `Refresh` on each backend that implements the small interface `codeindex.Refresher`. A backend that cannot refresh stays valid and is skipped. The trail adapter runs `trail build -- <index_dir>`. This is the incremental rebuild.
10. A failure prints `evolve wave: WARN: code index not refreshed: <reason>` to stderr. The step exits 0 and the boundary goes on.
11. A WARN has one of these causes: a policy error, a path outside a hub, or a refused `index_dir`.
12. A WARN can also come from a failed `git` command, a deadline, or a failed rebuild.

Why the index at `main` is right for a cycle: a cycle starts from `main`, so the index matches the base of the cycle. The index does not show the edits that the builder made in this cycle. The prompt tells the agent this (section 7). The agent reads a file before it changes it, and the file on disk is always current.

## 5. The sandbox

### Issue

Each phase runs under a sandbox that allows a small set of writes. A tool that writes outside this set fails or, worse, writes to the shared tree.

### Gap

The index tool must read a worktree that is not the phase worktree. The usage log must be written by a process that the sandbox confines.

### Solution

The command only reads. The index worktree is read through the file system. The one write is the usage log (section 6 gives the path rule).

Inside a phase, the log is in the cycle workspace (`.evolve/runs/cycle-N/code-calls.ndjson`), which the phase can write under both sandboxes. The log is not in `/tmp`, because the Linux phase sandbox mounts a private `tmpfs` on `/tmp` (`go/internal/adapters/sandbox/sandbox.go:245`, `--tmpfs /tmp`). A record in `/tmp` is lost when the phase ends. The command needs no new grant.

If the log write fails, the command prints the answer and goes on. A log failure must not block a query. The command does print one line on stderr when the write fails, so the failure is not silent. Built (K3): `TestCodeVerbLogged_AWriteFailureKeepsTheExitCodeAndTheOutput`.

The index worktree at `dev/code-index` is a path that the sandbox must allow for read. The read grant covers `dev/code-index` when `HomeDir` is set (`go/internal/adapters/sandbox/sandbox.go:175-176`; on Linux `go/internal/adapters/sandbox/sandbox.go:249-251`). No test runs `evolve code map` under the generated profile yet. K1 did not build it, because it needs the macOS sandbox tool. It stays open.

## 6. The usage log

### Issue

The 20 phases of C3 had 0 calls, and we know it only because a log existed for the MCP server (`/private/tmp/evolve-trail-calls.ndjson`). Without a log, we cannot say if the new prompt works.

### Gap

G2 and G3 need numbers by phase. The backend cannot give them, because the backend does not know which phase called.

### Solution

Each call appends one record (D6) to the usage log (built, K3). One record is one JSON object on one line. The command writes the whole line with one `write` call on a file that it opens with `O_APPEND`. A reader never sees a part of a line. Two calls at the same time never mix their lines.

| Field | Value |
|---|---|
| `ts` | the time of the call, in UTC |
| `command` | `callers`, `api`, `find` or `map` |
| `arg_len` | the length of the argument, in bytes. The log never holds the argument, because it can hold code. |
| `exit` | the exit code of the call |
| `hits` | the number of result lines |
| `duration_ms` | the run time of the call, in milliseconds |
| `phase` | the phase name, from the agent part of `EVOLVE_DISPATCH_ID`, else empty |
| `cycle` | the cycle number, from `EVOLVE_DISPATCH_ID`, else 0 |
| `cli` | the CLI family, from `EVOLVE_CLI` if present, else empty |
| `cwd` | the working directory of the call |

The bridge sets `EVOLVE_DISPATCH_ID` (form `run/cycle/agent/pPIDnNONCE`) in the environment of each phase agent, so every shell command of the agent carries it. No `EVOLVE_PHASE` variable exists. A call outside a phase has an empty `phase`. The command logs a call that has a known verb, also when it fails.

The command writes no record for an unknown verb.

The path of the log follows the identity of the process:

| Place of the call | Log file | Rotation |
|---|---|---|
| Inside a phase (`EVOLVE_DISPATCH_ID` is set) | `<project root>/.evolve/runs/cycle-N/code-calls.ndjson`, where N is the cycle in the tag | none. The existing gc removes the file with the run directory of the cycle. |
| Outside a phase | `<user cache dir>/evolve/code-calls.ndjson` (`os.UserCacheDir`) | When the file is larger than 5 MB (`MaxUserBytes`), the next write renames it to `code-calls.ndjson.1` and starts a new file. One old file is kept. |

The command creates the per-user directory with mode 0700 and the file with mode 0600. No other user can read the log. The command adds no environment variable: the path uses only `EVOLVE_DISPATCH_ID` and the project root.

A failed write prints one line on stderr (`evolve code: usage log: ...`) and changes neither the exit code nor the output.

`evolve code usage [--since D] [--json]` reads the per-user file, its `.1` file and every `<project root>/.evolve/runs/cycle-*/code-calls.ndjson`, and counts the calls by phase and command. D is a Go duration such as `24h`. The table has the columns PHASE, COMMAND, CALLS, HITS and FAILURES (a failure is a call with a nonzero exit). An empty phase shows as `(none)`. `--json` prints a list of `{phase, command, calls, hits, failures}`. A missing log gives an empty result.

A line that does not parse is skipped, and one stderr line gives the count. The code is `go/internal/codeindex/usagelog` and `go/cmd/evolve/cmd_code_usage.go`.

How the plan reads the log:

- **G2:** count the records by `phase`. A scout, builder or debugger phase with more than 0 records meets G2.
- **G3:** join the records to the per-phase token and pass-rate figures of 3 waves before and 3 waves after the prompt change.
- **G1:** group the records by `cwd` or by `phase` to see that agy and Claude Code phases both call.

## 7. The prompt text

### Issue

Tool use depends on the wording of the prompt. An edited tool description gave over ten times more use in one study (P3.4). A hint in the global instructions of agy did not change the 0 calls (C3).

### Gap

The agent does not know that `evolve code` exists, or when it helps. The scout and the builder already read their persona on each start (`agents/evolve-scout.md`, `agents/evolve-builder.md`, `agents/evolve-debugger.md`).

### Solution

Add these two paragraphs to the scout, builder and debugger personas (D7, K4a). Together they have eight sentences, in ASD-STE100.

> Use `evolve code` before you read many files to find code. When you must find who calls a symbol, run `evolve code callers <symbol>`. When you must know what a file offers, run `evolve code api <file>`. When you only know the words for a symbol, run `evolve code find <query>`. When you must learn a new directory, run `evolve code map <dir>`.

> The index shows `main`, not your edits. It can miss calls through injected functions. Read a file before you change it.

K4a tests each instruction line for 20 words or fewer. K4b runs the prompt-regression eval on each persona change. The tdd and auditor phases do not get the text in the first trial (plan Q2).

## Separation from the loop

The feature touches the loop in two places: the persona paragraph (K4a) and the refresh step of `wave next` (K2). It adds no flag and changes no phase order. If the index is absent, `evolve code` exits 2 and the agent uses grep. If the refresh fails, `wave next` prints a warning and goes on. One revert of K4a and K2 removes the feature. The prompt-regression eval and the G3 measure show that the loop does not lose quality.

## 8. The flow and a worked example

```mermaid
flowchart LR
    A[phase agent] -->|shell| B[evolve code]
    B -->|Backend interface| C[trail-code adapter]
    C -->|DO_NOT_TRACK=1, refresh off| D[trail binary]
    D --> E[(index at dev/code-index)]
    B -.->|one record per call| F[code-calls.ndjson: cycle workspace or user cache/]
    G[boundary: wave next] -->|checkout --detach origin/main, trail build| E
```

The solid arrows are the query path. The agent calls the command. The command calls the `Backend`. The adapter runs the binary against the index. The dotted arrow is the log. The boundary is the only writer of the index.

**Worked example.** A scout reads `runStagedCatalogRefresh` in the source and asks who calls it.

1. The agent runs `evolve code callers runStagedCatalogRefresh`.
2. The command calls `Backend.Callers` with the direction `in`.
3. The adapter runs `trail callers runStagedCatalogRefresh` and drops any line that tells the agent what to say.
4. The command prints one line for each caller. The real caller is `makeCatalogRefresher`, in `go/cmd/evolve/cmd_models_live.go:35-49`. That function returns a closure that calls `runStagedCatalogRefresh` at line 37 and passes it a `liveRefresh` function.
5. The command exits 0 and appends one record to the usage log.

The query uses the index of the 6,004 files, 51,347 symbols and 134,377 edges (C1). It takes about 0.1 s, against a `grep` that returns every text match and needs a read of each file. The output text is the three-field line of section 1. Tests in `go/cmd/evolve/cmd_code_test.go` pin it.

## 9. Backends and the benchmark

> [ADR-0140](adr/0140-the-type-checker-answers-callers-and-each-query-goes-to-its-best-measured-backend.md) · plan: [code-index-backends-2026-10](../plans/code-index-backends-2026-10.md) · research: [code-index-backends-2026-10](../research/code-index-backends-2026-10.md).
> Status: planned (2026-10-17). No name in this section has code yet, except `Backends`, `Capabilities` and `codeindextest.Run` of section 2. "Decision N" in this section is decision N of ADR-0140, which is also D N of the plan.

### Issue

On 2026-10-17 the operator asked if graphify looks up code better than trail-code, through the CLI that we built. We measured both against the Go type checker, on 40 symbols and 10 files (research F2.15). trail-code gives `callers` a precision of 0.11 and a recall of 0.32. It matches a name without its case or its type (F2.9), and misses calls through a package name (F2.10). graphify gives `callers` 0.93 and 0.75, but only after a second call with the right file for 18 of 40 names. It needs a unique name (F2.7), and takes about 3.5 s for each call (F2.6).

### Gap

Section 2 gives a capability route, but only one backend fills it, and nothing measures which backend is best for which query. A tree-sitter graph cannot fix the name problem: it has no types. On the same day the operator forbade MCP: "Forbid any mcp use, we only use cli to commute with other services". So the gopls, codegraph and Serena servers are not options (research section 4).

### Solution

Three parts: a type-checked backend, a benchmark against the type checker, and a route that the benchmark fills.

**The backends and the four queries.** Each backend is a CLI or an `evolve` verb (decision 9).

| Query | `gotypes` (planned, G1) | `trail` (built, K1) | graphify CLI (bench-only run, no adapter) |
|---|---|---|---|
| `callers --direction in` | the static call sites of the symbol, from the index | `trail callers` | the `<--` edges of relation `calls` in `graphify explain <name> --all` |
| `callers --direction out` | the calls in the body of the symbol | `trail callers --direction out` | the `-->` edges of relation `calls` in the same output |
| `api <file>` | the top-level declarations of the file, with constants and variables | `trail skeleton` | the `-->` edges of relation `contains` in `graphify explain <file> --all` |
| `find <query>` | not served (`ErrUnsupported`) | `trail ask` | `graphify query <words>`, the `NODE` lines |
| `map [dir]` | not served (`ErrUnsupported`) | `trail map` | not scored |

**The `gotypes` backend.** It lives in `go/internal/adapters/codeindex/gotypes`, because its build runs `go list`. It uses the standard library only (`go/parser`, `go/types`). At the boundary, it type-checks the module with its test variants and keeps the 85,694 static call edges to module functions (F2.20).

- **The index file.** One gob file, `dev/code-index-gotypes/index.gob`, beside the index worktree and outside every git tree, so no ignore rule is needed. It holds the edges keyed by the bare name, by `Type.Method` and by the package-qualified name. On this repository it is 19 MB.
- **The refresh order.** The K2 refresh runs `git checkout --detach origin/main` and the incremental `trail build` in the index worktree. Then the `gotypes` step type-checks the same checkout and writes the index, in about 5 s (F2.13). A failed step keeps the last good index and prints a warning.
- **The cost of a query.** The query loads the index and prints the three-field lines of section 1. The prototype took 0.044–0.047 s at p50, and `evolve` starts in 0.019 s. So a call costs about 0.07 s, an estimate from two measured parts (F2.20).

**Names.** A package-qualified name, such as `router.Digest` or `loopwave.Launcher.Run`, gives an exact answer (decision 4). The prototype scored 1.00 and 1.00 with qualified names, and 0.44 and 1.00 with bare names. A bare name merges the homonyms of every package. So for a bare name with several definitions, each hit's `Text` starts with the qualified name of its definition. The agent can then drop the hits of the other definitions.

**Kinds.** Each `gotypes` hit names its kind in its `Text`: `static`, `interface` or `func-value` (decision 5). A call through an interface counts for the interface method, not for the concrete method (F2.14). A concrete method with no static caller gets one line for each interface that it implements, from `go/types.Implements`. For example: "implements `Bridge.Launch`: 5 calls through it" for `fakeBridge.Launch` (F2.22). So the answer is not empty with no reason.

**graphify.** graphify is not in the route and has no adapter (decision 11). The bench runs its CLI from outside, with a clean environment, `GRAPHIFY_QUERY_LOG_DISABLE=1` and `GRAPHIFY_NO_AUTO_REFRESH=1` (F2.2). graphify refreshes only inside a bench run, never at the boundary (decision 10), because its refresh takes about 66–130 s (F2.17). If a bench run shows graphify best for a query type, a new plan adds an adapter with the rules of research R4.

**The capability route.** The default of `code_index.backends` becomes `["gotypes", "trail"]` (G4). `Backends.For` picks the first backend that serves the query (`go/internal/codeindex/backends.go:27-34`), so the route works with no code change:

```mermaid
flowchart LR
    Q[evolve code query] --> R{first backend that serves it}
    R -->|callers, api| G[gotypes: type-checked index]
    R -->|find, map| T[trail: tree-sitter graph]
    B[evolve code bench] -.->|table in the pull request| P[code_index.backends]
    P --> R
```

The boundary refreshes only the indexes that the route names (decision 10). A change to the route carries a bench table (decision 8). What latency an agent accepts for a better answer is open (plan Q5), so the bench reports latency beside accuracy and does not combine them.

**The benchmark and its truth.** `evolve code bench` (planned, G3) repeats the measurement of research section 1 for any set of backends.

1. It type-checks the index worktree with its test variants, as the `gotypes` build does. That is the truth.
2. It checks the truth with grep on 10 symbols, as research F2.19 did. A truth site that grep cannot find fails the run.
3. It samples symbols by a hash of their position, so no person picks them. It always adds the symbols of `codeindextest.StandardProbes()`.
4. It runs each query through `Backends.Search` for each backend, 3 times.
5. It prints the precision, recall, p50, p95, median bytes and failures of each backend and query, with the load average.

A caller is the pair of its file and its enclosing function, so two call sites in one function count once. `map` has no type-checked truth and is "not scored". The truth is the static call graph: a call through reflection or in a package-level initializer is not in it (F2.14).

**Worked example.** `evolve code callers Digest` today returns the callers of the lower-case `digest` in `go/internal/acssuite/evidence.go` (F2.9). The truth has 62 call sites of `router.Digest` (`go/internal/router/digest.go:17`). With the route `["gotypes", "trail"]`, `Backends.For(QueryCallers)` picks `gotypes`.

- `evolve code callers router.Digest` returns those 62 sites, each with the kind `static`.
- `evolve code callers Digest` also returns the calls of the methods `Digest` of other types, and each hit names its definition.
- A `find catalog refresh` still goes to trail-code.

### Guards for the backends

| Rule | Guard test | State |
|---|---|---|
| decision 1: `gotypes` serves `callers` and `api` only | `codeindextest.Run` on a fixture module. `find` and `map` give `ErrUnsupported` | planned (G1) |
| decision 4: a qualified name gives only its own callers | the fixture has two functions of the same bare name in two packages | planned (G1) |
| decision 5: each hit names its kind, and a concrete method gets its "implements" line | the fixture has one interface call and one function-value call | planned (G1) |
| decision 7: the bench scores exactly and checks its truth | a fake backend with two false positives and one false negative gives the exact precision and recall. A truth that misses a real call fails the grep check | planned (G3) |
| decision 8: a route change carries a bench table | review of the pull request; no automatic guard | planned (G4) |
| decision 10: the boundary refreshes only route members | `wave next --dry-run` lists no graphify step | planned (G2) |

### Limits of the backends

- `gotypes` is Go only. trail-code still answers for the other languages.
- The static answer does not give the concrete target of an interface call. A CHA or VTA mode is an open question (plan Q4).
- The independent check covered 10 functions and no interface methods (F2.19).
- The bench measures `callers`, `api` and `find` against the type checker. It cannot score `map`, and it cannot prove that an agent gets a better result. The K3 usage log and the A/B of the plan, section 8, measure that.

## Guards

| Rule | Guard test | State |
|---|---|---|
| D3: one package runs the trail-code binary | `TestOnlyTheTrailAdapterRunsTheTrailBinary` in `go/test/structure/binary_test.go` fails if another package both runs a process and names the trail binary path. `TestTheCodeIndexAbstractionRunsNoProcessAndNamesNoBackend` keeps `internal/codeindex` free of both. The convention has one row | built (K1) |
| D4: the output has no instruction line | `TestTrailBackend_NoOutputLineThatInstructsTheAgentReachesAnyHit` and `TestCodeVerb_ThePhaseAgentDoesNotSeeAnInstructionLineOfTheBackendOutput` feed a "tokens saved" header and expect the answer without it | built (K1) |
| D5: the command is read-only, so a cycle worktree stays clean | `TestCodeVerb_EveryVerbLeavesTheCheckedOutTreeClean` runs the four verbs in a temporary git repository and expects an empty `git status` | built (K1) |
| D5: the refresh writes only the index worktree and never halts the boundary | `TestWaveCodeRefresh_NeverNamesTheProjectRootTreeInAnyCommand` records every command and its working directory and expects no path under the project root. `TestWaveCodeRefresh_AFailureIsAWarningAndTheStepStillSucceeds` and `TestWaveCodeRefresh_AFailedRefreshDoesNotStopTheBoundarySteps` expect a WARN and a next step that runs. `TestWaveNext_DryRunListsTheRefreshStepBeforeTheLaunch` expects the step in the dry run. `TestTrailBackend_RefreshRunsTheIncrementalBuildWithTheIndexDirAfterTheSeparator` pins the argv | built (K2) |
| D5a: the refresh refuses an unsafe `index_dir` | `TestWaveCodeRefresh_AnIndexDirInsideTheProjectIsAWarningAndRunsNothing`, `TestWaveCodeRefresh_AnIndexDirThroughASymlinkIntoTheRootIsRefused` and `TestWaveCodeRefresh_AnIndexDirInTheRuntimePlaneIsAWarningAndRunsNothing` expect a WARN and zero runner calls | built (K2) |
| D5b: the refresh touches only a real, own worktree | `TestWaveCodeRefresh_AnIndexDirThatIsNotItsOwnWorktreeIsAWarningWithNoFetchOrCheckout` expects only the `rev-parse` call. `TestWaveCodeRefresh_ADirtyIndexWorktreeWarnsAndNeverReachesTheRebuild` expects no trail call after a failed checkout | built (K2) |
| D5c: a hung command ends at the deadline | `TestWaveCodeRefresh_AHungGitCallEndsAtTheDeadlineWithAWarning` expects a return before the test limit and a deadline WARN | built (K2) |
| D6: one record for each call | `TestCodeVerbLogged_ACallWritesOneRecordWithItsCommandAndPhase` runs one command and reads one new record with the command and the phase. `TestAppend_ConcurrentCallsGiveWholeLines` pins whole lines. `TestCodeUsage_AggregatesCallsByPhaseAndCommand` pins the summary | built (K3) |
| G5: any backend passes the same tests | `codeindextest.Run` runs on the fake and on the trail adapter (`TestTrailBackend_PassesTheConformanceSuite`); the command tests run on the fake. A backend that implements `Refresher` also gets two checks. A refresh of a present index succeeds. A refresh of an absent index is `ErrAbsent`. The strip and the real process run through the trail adapter | built (K1) |

## Limits

- The first backend does not type-check. It misses calls through injected functions (C7). The prompt says so.
- The index shows `main`. It does not show the edits of the current cycle (section 4).
- The design needs Node and the index worktree on the host. A host without them gets exit 2 (section 1), and no test depends on Node.
- The speed of `gopls` for each call on 6,000 files is not measured (P3.5). The design does not depend on it.
- The name of the environment variable for the phase is open (section 6).
- The gain in tokens is not proven. Independent tests find that a graph helps most on structural questions, not on tokens (P3.3). G3 measures it for this repository.
- A path from the cycle worktree maps to the index worktree at main, so a file that the cycle edited shows its old API.
- A relative `code_index` path needs a worktree of the hub store. A checkout outside the hub gets exit 4, and an absolute path in the policy avoids the need. K1 did not test a phase worktree of the runtime plane. It stays open.
- The spec does not remove the MCP registrations. K5 does that after one wave with the CLI live (D8).
