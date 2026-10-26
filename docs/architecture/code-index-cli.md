# Code index CLI: `evolve code` gives every phase agent a read-only code graph

> Research: [code-index-cli-2026-10](../research/code-index-cli-2026-10.md). Plan: [code-index-cli-2026-10](../plans/code-index-cli-2026-10.md). Decision: [ADR-0138](adr/0138-phase-agents-query-the-code-index-through-an-evolve-cli-not-mcp.md).
> Status: K1, K2 and K3 are built (2026-10-17). K6 (2026-10-26) moves the refresh from the K2 step into `sync-main`. K4a, K4b and K5 are planned, and a name with no code is marked "planned".
>
> This file is the spec of record for the `evolve code` verb. The research holds the evidence, the plan holds the scored designs and the order of work, and the ADR holds the decision.

On 2026-10-17 the operator asked that the loop agents use trail-code, "separated from the evo loop without interfere with evo loop's function. It should only boost the LLM performance." This spec shows how the design meets that request.

**Terms used throughout:**

| Term | Meaning |
|---|---|
| code index | A graph of the symbols and the calls of this repository. trail-code builds it with tree-sitter (C1, C7). |
| trail-code | The outside tool that builds and queries the index. Its binary is `dev/.tools/trail/node_modules/.bin/trail` (C6). |
| index worktree | A git worktree at `dev/code-index` (the default of `code_index.index_dir`). It holds the checkout that the index describes. The operator makes it. `evolve sync-main` refreshes it at each boundary (K6, built). |
| `Backend` | The Go interface `codeindex.Backend` (`go/internal/codeindex`) that the `evolve code` commands call. It has one method for each command. |
| adapter | The one package, `go/internal/adapters/codeindex/trail`, that runs the trail-code binary. It implements `Backend`. |
| boundary | The step between two waves where the loop merges and refreshes. `evolve wave next` and `evolve boundary run` run it. |
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

`evolve code` has four query commands. Each query command is read-only (D1). The code is `go/cmd/evolve/cmd_code.go`. Two more commands are not queries: `evolve code usage` reads the usage log (section 6), and `evolve code refresh` refreshes the index (section 4).

| Command | Question it answers | Backend call (trail-code) |
|---|---|---|
| `evolve code callers <symbol> [--direction in\|out]` | Who calls this symbol (`in`, the default)? Which symbols does it call (`out`)? | `trail callers` |
| `evolve code api <file>` | What are the exported signatures of this file, without the bodies? | `trail skeleton` |
| `evolve code find <query>` | Which symbols match a question in plain words? | `trail ask` |
| `evolve code map [dir]` | What is the structure of this directory? | `trail map` |

The output is plain text. Each hit is one line of three fields that a tab separates: `file:line`, the symbol and a short text. A hit that has no line, such as a directory of `map`, shows the `file` alone. A tab or a newline inside a field becomes a space. The agent opens the file at the line.

`find` joins its words, so `evolve code find catalog refresh` and `evolve code find "catalog refresh"` are the same query. With `--json`, a query command prints one `evolve.code.<command>/1` envelope on stdout ([cli-contract.md](cli-contract.md) §2). Its `data` holds `hits`, a list of objects with `file`, `line`, `symbol` and `text`, and the freshness fields of section 4. Every command takes `--project-root P` (default: `$EVOLVE_PROJECT_ROOT`, then the nearest directory at or above the working directory that holds `.git`).

| Exit code | Meaning | Output |
|---|---|---|
| 0 | The query found at least one hit. | the hit lines on stdout |
| 1 | The query ran and found nothing. | one line on stderr that names the query |
| 2 | The index or the backend is absent. | one line on stderr that says what is absent, with the path of the binary and of the index, and a pointer to section 4 |
| 3 | No backend in the list serves the command. | one line on stderr: `no backend serves <command>` |
| 4 | Any other failure: a refused policy, a root that is no hub, an unknown backend name or a failed backend. | one line on stderr |
| 10 | A usage error: a missing or extra argument, a wrong flag or a wrong direction. | the reason and the usage line on stderr |
| 21 | The index is stale, and `code_index.stale` is `refuse` (section 4). | one line on stderr that gives the count of commits and names `evolve code refresh` |

The `map [dir]` command lists the directory clusters of the index. With a `dir`, it keeps the clusters at or below that directory. The other commands read the index, whatever the `dir`.

### The policy block

The wiring reads the block `code_index` of `.evolve/policy.json`. The decode is strict, like the other blocks. A relative path resolves against the hub root. The hub root is the parent of the bare store (`docs/operations/workspace-layout.md`). A path that is absolute needs no hub.

| Key | Default | Meaning |
|---|---|---|
| `backends` | `["trail"]` | the ordered list of backend names. It is a capability route: the first backend that serves a query answers it. There is no failover when that backend is absent. |
| `index_dir` | `dev/code-index` | the index worktree |
| `trail_bin` | `dev/.tools/trail/node_modules/.bin/trail` | the trail-code binary |
| `stale` | `warn` | what a query does when the index is not at `origin/main`: `warn` prints a warning and answers, `refuse` exits 21 (section 4). Another word is a policy error. |

The exit code 2 is the case of a host with no Node, no binary or no index worktree. The command stops with a clear message and does not guess (plan section 9). No query command writes to the repository, to the index or to the cycle worktree. Only `evolve code refresh` and `evolve sync-main` write the index worktree.

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

The index worktree is separate from every cycle worktree. The pipeline refreshes it (D5). A phase never does.

1. `evolve sync-main` runs `git checkout --detach origin/main` in the index worktree, after it merges `main`.
2. Then it runs an incremental `trail build`. The build reads only the changed files and takes about 2 s (C1).
3. A phase reads the index. A query takes about 0.1 s (C1). Each query also compares the index with `origin/main` and warns when the index is stale.

#### Findings of 2026-10-26 (K6)

**F4.1** On 2026-10-26 the index was 5 days and 114 commits behind `main`. The console refreshed it by hand at each boundary (`git -C dev/code-index fetch` and `checkout --detach origin/main`), and the step was missed (operator report, 2026-10-26).
*Implication:* a refresh that depends on a person is missed. The pipeline must own it.

**F4.2** The K2 step `code-refresh` ran only in `evolve wave next`, and it turned each failure into a WARN with exit 0 (`go/cmd/evolve/cmd_wave_codeindex.go:30-34` on `f6b8a7ab0`). `evolve boundary run` had no refresh step (`go/cmd/evolve/cmd_boundary.go:152-160` on `f6b8a7ab0`).
*Implication:* a boundary through `boundary run` never refreshed, and a failed refresh in `wave next` left only one stderr line.

**F4.3** No `evolve code` command read the commit of the index (`go/cmd/evolve/cmd_code.go:56-75` on `f6b8a7ab0`).
*Implication:* an agent got answers from old code with no sign that they were old.

**F4.4** In the 7 days to 2026-10-26, `evolve code usage` counted 2 calls, both from the console, and no call from a phase agent.
*Implication:* the phase agents do not see the index as a tool. An inbox item for the context-index builder covers this gap (K6, last row).

**F4.5** The index worktree is a worktree of the hub store: `git -C dev/code-index rev-parse --git-common-dir` printed `/Users/danleemh/ai/claude/evolve-loop/.repo.git` on 2026-10-26, the same store as the runtime plane.
*Implication:* the `origin/main` ref in the index worktree is the ref that `sync-main` fetches. A compare of `HEAD` with `origin/main` needs no network.

**F4.6** On 2026-10-26, `git -C dev/code-index status --porcelain` printed ` M .gitignore` and `?? .ignore`. The `.gitignore` edit adds `/graft/`, and `.ignore` puts `graft/` back into the search.
*Implication:* these two edits are the index tool's own and are expected. Any other change belongs to a person or to a defect.

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D5.1 | `sync-main` refreshes the index after it merges `main`. `boundary run` and `wave next` run `sync-main`, so each boundary refreshes the index. The `wave next` step `code-refresh` is removed. | F4.1, F4.2 | A manual console step: F4.1 shows that it is missed. Keep the `wave next` step too: each wave boundary then refreshes twice. A refresh in each query: a query then writes, which breaks G4, and a full build takes 31 s (C1). |
| D5.2 | After the merge, a failed refresh makes `sync-main` exit non-zero with one code for each cause. An absent index is not a failure. This decision amends decision 10 of ADR-0138. | F4.2 | A WARN with exit 0: F4.2 shows that a WARN hides a stale index for days. A failure for an absent index: a host without an index, such as CI or a new clone, then cannot sync. |
| D5.3 | Only `.gitignore` and `.ignore` can have changes in the index worktree. Any other change stops the refresh before the fetch, and nothing is reset. | F4.6 | `git reset --hard` or `git clean`: they destroy work that the pipeline did not write. A refusal of every change: F4.6 then stops each refresh. |
| D5.4 | Each query compares the index `HEAD` with the local `origin/main` ref and warns on stderr when they differ. The policy key `code_index.stale` set to `refuse` makes a stale query exit 21. The default is `warn`. | F4.3, F4.5 | A fetch for each query: it is slow and fails with no network. Refusal by default: an answer about old code is still correct, and the agent can use `grep`. |
| D5.5 | `evolve code refresh [--json]` refreshes on demand. Its agent class is `R` ([cli-contract.md](cli-contract.md) §4.1): inside a phase it refuses with exit 3. The request kind `code-refresh` of cli-contract §4.3 is proposed, not built. | F4.4, G4 | Class `A`: a phase agent then writes the shared index, which breaks G4. Class `D`: an agent then has no way to ask for a fresh index. |

What changes our mind: a dirty index can stop more than one boundary in a month. Then the refresh can carry the edits of the index tool across the checkout.

#### The refresh

The refresh is `codeIndexHost.refresh` in `go/cmd/evolve/cmd_code_refresh.go`. `sync-main` and `evolve code refresh` call it. Each `git` command runs with the index worktree as its working directory and names no other tree. Thus the refresh never writes the main tree, a cycle worktree or a lane worktree.

1. Read `code_index.index_dir` and resolve it against the hub root, as `evolve code` does. A relative path in a root that is not a hub worktree means "no index".
2. Clean `index_dir` and resolve its symbolic links. Then refuse it in these cases:
   - the project root, or a path inside it;
   - a path under `<project root>/.evolve/worktrees`;
   - the runtime plane root, or a path inside it.
3. Check that `index_dir` exists. If it does not exist, the index is absent.
4. Run `git rev-parse --show-toplevel`. The output must equal `index_dir`. If not, `index_dir` is a plain directory inside another repository.
5. Run `git status --porcelain --untracked-files=normal`. A path other than `.gitignore` and `.ignore` stops the refresh. The message names each path and says that nothing was reset.
6. Run `git fetch --quiet origin main`.
7. Run `git rev-parse HEAD origin/main`. If the two commits differ, run `git rev-list --count HEAD..origin/main`.
8. Run `git checkout --quiet --detach origin/main`. If a change of `main` conflicts with the edits of the index tool, the checkout fails. The message tells the operator to clean the index worktree.
9. Call `Backends.Refresh`. It runs `Refresh` on each backend that implements `codeindex.Refresher`, and skips the others. The trail adapter runs `trail build -- <index_dir>`. The refresh never runs `trail init`.
10. The whole refresh has a deadline of 5 minutes. At the deadline, `sysexec` sends SIGTERM to the child process and kills it after `WaitDelay` (5 s). The refresh then fails.

`sync-main` refreshes only when it synced the branch `main`. The sync of another branch does not move `origin/main`. When the refresh fails, the merge is already done, and the message says so. Fix the cause, run `evolve code refresh`, then run the boundary again without `--merge`.

| Result | `evolve code refresh` exit | `evolve sync-main` exit | Output |
|---|---|---|---|
| The index moved, or it was at `origin/main` | 0 | 0 | one line on stdout with the index path, the new commit and the count of commits after the old commit |
| The index is absent, or the root is not a hub worktree | 2 | 0 | one line on stderr (`refresh`) or on stdout (`sync-main`) that says what is absent |
| Inside a phase | 3 | not applicable: `sync-main` is class `D` | one line on stderr |
| A refused `index_dir`, a policy error, a failed `git` command or rebuild, or the deadline | 4 | 1 | one line on stderr |
| A change in the index worktree that is not the index tool's own | 22 | 22 | one line on stderr that names each path |
| `git fetch origin main` failed | 23 | 23 | one line on stderr with the first line of the `git` error |
| A usage error | 10 | 1 | the reason and the usage line on stderr |

With `--json`, `evolve code refresh` prints one `evolve.code.refresh/1` envelope on stdout ([cli-contract.md](cli-contract.md) §2). Its `data` holds `index_dir`, `from`, `to`, `behind` and `rebuilt`, the list of the backends that rebuilt.

#### The freshness of a query

Each query command runs two local reads in the index worktree before it asks the backend: `git rev-parse HEAD origin/main`, and `git rev-list --count HEAD..origin/main` when the commits differ. The code is `checkCodeFreshness` in `go/cmd/evolve/cmd_code_freshness.go`.

| State | stderr | Exit | `--json` fields |
|---|---|---|---|
| `HEAD` equals `origin/main` | nothing | the exit of the query | `stale: false`, `behind: 0`, `index_head`, `main_head` |
| `HEAD` differs, `code_index.stale` is `warn` | `evolve code: WARN: the code index is N commit(s) behind origin/main (index <sha>, origin/main <sha>); an answer can miss newer code; evolve code refresh moves the index` | the exit of the query | `stale: true`, `behind: N`, and the warning in `warnings` |
| `HEAD` differs, `code_index.stale` is `refuse` | one refusal line | 21, and no query runs | `error.kind: "stale"` |
| The index is absent | nothing; the backend reports the absence | 2 | none |
| A read failed | `evolve code: WARN: the freshness of the code index is unknown: <reason>` | the exit of the query | no `stale` and no `behind` |

The count is 0 only when the index is ahead of `origin/main`, so that `origin/main` is an ancestor of the index `HEAD`. The index is then stale, but no commit of `main` is missing. The WARN then says `the code index is not at origin/main but misses none of its commits`.

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

The feature touches the loop in two places: the persona paragraph (K4a) and the refresh in `sync-main` (K6). It adds no feature flag and changes no phase order. If the index is absent, `evolve code` exits 2 and the agent uses grep, and `sync-main` goes on. If the refresh fails, `sync-main` exits with the code of the cause, after the merge (section 4). One revert of K4a and K6 removes the feature. The prompt-regression eval and the G3 measure show that the loop does not lose quality.

## 8. The flow and a worked example

```mermaid
flowchart LR
    A[phase agent] -->|shell| B[evolve code]
    B -->|Backend interface| C[trail-code adapter]
    C -->|DO_NOT_TRACK=1, refresh off| D[trail binary]
    D --> E[(index at dev/code-index)]
    B -.->|one record per call| F[code-calls.ndjson: cycle workspace or user cache/]
    G[boundary: sync-main or code refresh] -->|checkout --detach origin/main, trail build| E
```

The solid arrows are the query path. The agent calls the command. The command calls the `Backend`. The adapter runs the binary against the index. The dotted arrow is the log. The refresh is the only writer of the index.

**Worked example.** A scout reads `runStagedCatalogRefresh` in the source and asks who calls it.

1. The agent runs `evolve code callers runStagedCatalogRefresh`.
2. The command calls `Backend.Callers` with the direction `in`.
3. The adapter runs `trail callers runStagedCatalogRefresh` and drops any line that tells the agent what to say.
4. The command prints one line for each caller. The real caller is `makeCatalogRefresher`, in `go/cmd/evolve/cmd_models_live.go:35-49`. That function returns a closure that calls `runStagedCatalogRefresh` at line 37 and passes it a `liveRefresh` function.
5. The command exits 0 and appends one record to the usage log.

The query uses the index of the 6,004 files, 51,347 symbols and 134,377 edges (C1). It takes about 0.1 s, against a `grep` that returns every text match and needs a read of each file. The output text is the three-field line of section 1. Tests in `go/cmd/evolve/cmd_code_test.go` pin it.

## 9. Backends and the benchmark

> [ADR-0140](adr/0140-the-type-checker-answers-callers-and-each-query-goes-to-its-best-measured-backend.md) · plan: [code-index-backends-2026-10](../plans/code-index-backends-2026-10.md) · research: [code-index-backends-2026-10](../research/code-index-backends-2026-10.md).
> Status: G1 built (2026-10-17): the `gotypes` backend, in [internal-adapters-codeindex-gotypes](packages/internal-adapters-codeindex-gotypes.md). G2 to G5 are planned. "Decision N" in this section is decision N of ADR-0140, which is also D N of the plan.

### Issue

On 2026-10-17 the operator asked if graphify looks up code better than trail-code, through the CLI that we built. We measured both against the Go type checker, on 40 symbols and 10 files (research F2.15). trail-code gives `callers` a precision of 0.11 and a recall of 0.32. It matches a name without its case or its type (F2.9), and misses calls through a package name (F2.10). graphify gives `callers` 0.93 and 0.75, but only after a second call with the right file for 18 of 40 names. It needs a unique name (F2.7), and takes about 3.5 s for each call (F2.6).

### Gap

Section 2 gives a capability route, but only one backend fills it, and nothing measures which backend is best for which query. A tree-sitter graph cannot fix the name problem: it has no types. On the same day the operator forbade MCP: "Forbid any mcp use, we only use cli to commute with other services". So the gopls, codegraph and Serena servers are not options (research section 4).

### Solution

Three parts: a type-checked backend, a benchmark against the type checker, and a route that the benchmark fills.

**The backends and the four queries.** Each backend is a CLI or an `evolve` verb (decision 9).

| Query | `gotypes` (built, G1) | `trail` (built, K1) | graphify CLI (bench-only run, no adapter) |
|---|---|---|---|
| `callers --direction in` | the static call sites of the symbol, from the index | `trail callers` | the `<--` edges of relation `calls` in `graphify explain <name> --all` |
| `callers --direction out` | the calls in the body of the symbol | `trail callers --direction out` | the `-->` edges of relation `calls` in the same output |
| `api <file>` | the top-level declarations of the file, with constants and variables | `trail skeleton` | the `-->` edges of relation `contains` in `graphify explain <file> --all` |
| `find <query>` | not served (`ErrUnsupported`) | `trail ask` | `graphify query <words>`, the `NODE` lines |
| `map [dir]` | not served (`ErrUnsupported`) | `trail map` | not scored |

**The `gotypes` backend.** It lives in `go/internal/adapters/codeindex/gotypes`, because its build runs `go list`. It uses the standard library only (`go/parser`, `go/types`). At the boundary, it type-checks the module with its test variants and keeps the 85,694 static call edges to module functions (F2.20).

- **The index file.** One gob file, `dev/code-index-gotypes/index.gob`, beside the index worktree and outside every git tree, so no ignore rule is needed. `gotypes.IndexPath` gives this path from `code_index.index_dir`. The file holds the definitions, the module call edges, the declarations and the interface links. A definition answers to its bare name, to `Type.Method` and to its package-qualified name. On this repository it is 5.4 MB (G1, 2026-10-17), and the prototype file of F2.20 was 19 MB.
- **The refresh order.** The refresh of section 4 runs `git checkout --detach origin/main` and the incremental `trail build` in the index worktree. Then the `gotypes` step type-checks the same checkout and writes the index, in about 5 s (F2.13). A failed step keeps the last good index and prints a warning.
- **The cost of a query.** The query loads the index and prints the three-field lines of section 1. The prototype took 0.044–0.047 s at p50, and `evolve` starts in 0.019 s (F2.20). The G1 backend, measured as one run of `evolve code callers`, took 0.031–0.037 s at p50 for 5 symbols on 2026-10-17. Its `Build` took 3.7–8.2 s, with a peak memory of 434–461 MB.

**Names.** A package-qualified name, such as `router.Digest` or `loopwave.Launcher.Run`, gives an exact answer (decision 4). The prototype scored 1.00 and 1.00 with qualified names, and 0.44 and 1.00 with bare names. A bare name merges the homonyms of every package. So for a bare name with several definitions, each hit's `Text` starts with the qualified name of its definition. The agent can then drop the hits of the other definitions.

G1 starts the `Text` of every hit with the qualified name of its definition, so one rule covers both forms.

**Kinds.** Each `gotypes` hit names its kind in its `Text`: `static`, `interface` or `func-value` (decision 5). A call through an interface counts for the interface method, not for the concrete method (F2.14). A concrete method with no static caller gets one line for each interface that it implements, from `go/types.Implements`. For example: "implements `Bridge.Launch`: 5 calls through it" for `fakeBridge.Launch` (F2.22). So the answer is not empty with no reason.

G1 prints that line as `audit.fakeBridge.Launch implements core.Bridge.Launch: 6 call(s) through it`, on 2026-10-17. G1 counts 6 calls, and F2.22 counted 5. The sixth site, `go/internal/core/apicover_ports_test.go:58`, dates from 2026-06-17 (`git log -S`). The truth missed it: the site is in the test variant `core [core.test]`, so `types.Implements` saw a second `Bridge` type. The right number is 6 ([internal-adapters-codeindex-gotypes](packages/internal-adapters-codeindex-gotypes.md), section "Measurement").

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
| decision 1: `gotypes` serves `callers` and `api` only | `TestGotypesBackend_PassesTheConformanceSuite` runs `codeindextest.Run` on a fixture module. `find` and `map` give `ErrUnsupported` | built (G1) |
| decision 4: a qualified name gives only its own callers | `TestGotypesBackend_AQualifiedNameGivesOnlyTheCallersOfItsOwnDefinition` and `TestGotypesBackend_ABareNameWithTwoDefinitionsNamesTheDefinitionOfEachHit`: the fixture has two functions of the same bare name in two packages | built (G1) |
| decision 5: each hit names its kind, and a concrete method gets its "implements" line | `TestGotypesBackend_EachHitNamesItsKind` and `TestGotypesBackend_AConcreteMethodReachedOnlyThroughAnInterfaceGetsItsImplementsLine`: the fixture has interface calls and one function-value call | built (G1) |
| decision 7: the bench scores exactly and checks its truth | a fake backend with two false positives and one false negative gives the exact precision and recall. A truth that misses a real call fails the grep check | planned (G3) |
| decision 8: a route change carries a bench table | review of the pull request; no automatic guard | planned (G4) |
| decision 10: the boundary refreshes only route members | `wave next --dry-run` lists no graphify step | planned (G2) |

### Limits of the backends

- `gotypes` is Go only. trail-code still answers for the other languages.
- `gotypes` has the files of the host platform only. `CGO_ENABLED=0` leaves out the cgo files, and the files of another `GOOS` or `GOARCH` are not in the index.
- `gotypes` counts calls only. A method value or a function reference that is not a call is not counted. A call in a package-level initializer has the caller `<package>.<init>`.
- A broken package gives a warning and a partial index, not a failed build.
- The static answer does not give the concrete target of an interface call. A CHA or VTA mode is an open question (plan Q4).
- The independent check covered 10 functions and no interface methods (F2.19).
- The bench measures `callers`, `api` and `find` against the type checker. It cannot score `map`, and it cannot prove that an agent gets a better result. The K3 usage log and the A/B of the plan, section 8, measure that.

## Guards

| Rule | Guard test | State |
|---|---|---|
| D3: one package runs the trail-code binary | `TestOnlyTheTrailAdapterRunsTheTrailBinary` in `go/test/structure/binary_test.go` fails if another package both runs a process and names the trail binary path. `TestTheCodeIndexAbstractionRunsNoProcessAndNamesNoBackend` keeps `internal/codeindex` free of both. The convention has one row | built (K1) |
| D4: the output has no instruction line | `TestTrailBackend_NoOutputLineThatInstructsTheAgentReachesAnyHit` and `TestCodeVerb_ThePhaseAgentDoesNotSeeAnInstructionLineOfTheBackendOutput` feed a "tokens saved" header and expect the answer without it | built (K1) |
| D5: the command is read-only, so a cycle worktree stays clean | `TestCodeVerb_EveryVerbLeavesTheCheckedOutTreeClean` runs the four verbs in a temporary git repository and expects an empty `git status` | built (K1) |
| D5: the refresh writes only the index worktree | `TestCodeRefresh_NeverNamesTheProjectRootTreeInAnyCommand` records every command and its working directory and expects no path under the project root. `TestCodeRefresh_MovesTheIndexWorktreeToOriginMainDetachedThenRebuilds` pins the order of the commands. `TestTrailBackend_RefreshRunsTheIncrementalBuildWithTheIndexDirAfterTheSeparator` pins the argv of the rebuild | built (K6) |
| D5: the refresh refuses an unsafe `index_dir` | `TestCodeRefresh_AnIndexDirInsideTheProjectIsRefusedAndRunsNothing`, `TestCodeRefresh_AnIndexDirThroughASymlinkIntoTheRootIsRefused` and `TestCodeRefresh_AnIndexDirInTheRuntimePlaneIsRefused` expect exit 4 and zero runner calls. `TestCodeRefresh_AnIndexDirThatIsNotItsOwnWorktreeExitsFourWithNoFetch` expects only the `rev-parse` call | built (K6) |
| D5.1: `sync-main` refreshes the index, and `wave next` has no second step | `TestSyncMain_MovesTheCodeIndexWorktreeToTheNewOriginMain` runs `sync-main` on real repositories and expects a detached index at the new `origin/main`. `TestSyncMain_OnABranchOtherThanMainLeavesTheIndexAlone` expects no move. `TestWaveNext_HasNoSeparateRefreshStepBecauseSyncMainOwnsIt` expects no `code-refresh` step | built (K6) |
| D5.2: each failure has its own exit code | `TestCodeRefresh_AFetchFailureExitsTwentyThreeBeforeTheCheckout`, `TestCodeRefresh_AnotherFailureExitsFourAndStopsAtTheFailedStep`, `TestCodeRefresh_AnAbsentIndexExitsTwoAndRunsNothing`, `TestCodeRefresh_AHungGitCallEndsAtTheDeadline` and `TestSyncMainCodeIndex_EachOutcomeHasItsMessageAndExitCode` | built (K6) |
| D5.3: a foreign change stops the refresh and nothing is reset | `TestCodeRefresh_AForeignChangeInTheIndexExitsTwentyTwoAndResetsNothing` expects no fetch and no checkout. `TestSyncMain_AForeignEditInTheIndexFailsLoudlyAfterTheMergeAndKeepsTheEdit` expects the edit on disk. `TestCodeRefresh_TheIndexToolsOwnEditsDoNotBlockTheMove` expects the full refresh | built (K6) |
| D5.4: a stale query warns, or refuses under the policy | `TestCodeVerb_AStaleIndexWarnsOnStderrWithTheCountAndStillAnswers`, `TestCodeVerb_AStaleIndexUnderTheRefusePolicyExitsTwentyOneBeforeTheQuery`, `TestCodeVerb_JSONCarriesTheHitsAndTheFreshness` and `TestCodeIndexHostFreshness_ComparesTheIndexHeadWithTheLocalOriginMainRef` | built (K6) |
| D5.5: `code refresh` refuses inside a phase | `TestCodeRefresh_InsideAPhaseItRefusesWithExitThreeAndRunsNothing` | built (K6) |
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
- The refresh takes no lock. Two refreshes at the same time in the index worktree can collide. `git` then refuses the second checkout, but two `trail build` runs are not kept apart. A query during a refresh can read a part-built index. Only the boundary and the operator start a refresh.
- The trail adapter finds a missing binary only at the rebuild, after the checkout. The refresh then fails loudly, but the checkout stays at the new commit with the old build. A query then reports the index as fresh. Install the binary, then run `evolve code refresh`.
- The spec does not remove the MCP registrations. K5 does that after one wave with the CLI live (D8).
