# Code-index backends: trail-code, graphify and a type-checked Go index, measured against the type checker (research, 2026-10)

> The plan that uses this research: [code-index-backends-2026-10](../plans/code-index-backends-2026-10.md). The design: [code-index-cli](../architecture/code-index-cli.md), section "Backends and the benchmark". The decision record: [ADR-0140](../architecture/adr/0140-the-type-checker-answers-callers-and-each-query-goes-to-its-best-measured-backend.md).
>
> This dossier extends [code-index-cli-2026-10](code-index-cli-2026-10.md) (findings C1 to C8, P3.1 to P3.5). It does not repeat them.

Date: 2026-10-17. Method:

- a type-checked truth for 40 symbols and 10 files, from a stdlib-only Go program (`go/parser`, `go/types`);
- each query through trail-code (`evolve code`), through the graphify CLI, and against the truth;
- precision, recall, latency, output size, build time, disk size and refresh time for each backend;
- an independent check of the truth: a grep of the defining package and its importers, read by hand for 10 symbols;
- a scratch CLI prototype of the type-checked index, and a `grep -rn` baseline;
- a read of the graphify 0.9.92 source for its writes and its output text;
- a web survey of graphify, of code-graph tools for agents, and of type-checked options for Go.

Each local claim names its command or file. "Synthesis" marks our own inference. "Not verified" marks an outside claim that we did not check. The scratch files of the measurement are not committed. Their place is in section 1.

## Table of contents

- [1. The request and the method](#1-the-request-and-the-method)
- [2. Local evidence](#2-local-evidence)
- [3. Current practice](#3-current-practice)
- [4. Adopt or reject](#4-adopt-or-reject)
- [5. Refinements to adopt](#5-refinements-to-adopt)

**Terms used throughout:**

| Term | Meaning |
|---|---|
| truth | The answer of the Go type checker for a query. A stdlib-only program computes it (section 1). |
| precision | The part of the returned answers that the truth also has: TP / (TP + FP). |
| recall | The part of the truth that the backend returns: TP / (TP + FN). |
| caller key | One caller, as the pair of its file and the name of its enclosing function. Several call sites in one function make one key. |
| EXTRACTED, INFERRED | The two confidence tags of a graphify edge. EXTRACTED is read from the source. INFERRED is a guess of the graphify resolver. |
| capability route | The rule of `code_index.backends`: for each query, the first backend in the list that serves it answers ([code-index-cli](../architecture/code-index-cli.md), section 2). |
| bench | The measurement of this dossier, and the planned `evolve code bench` verb that repeats it. |

## 1. The request and the method

On 2026-10-17 the operator asked: "Beside trail-code, have we tried graphify github open source for llm to look up the code more efficient? Could we leverage the cli we build for code look up and integrate with graphify to see which one perform better?" Then: "Again, Document all research, finding, decision, and design, architecture in docs following the rule defined under docs/".

On the same day the operator gave a new rule: "Forbid any mcp use, we only use cli to commute with other services, if it doesn't exist, we can build cli for that service". So this measurement uses graphify through its CLI only. It does not start, register or measure an MCP server.

**The corpus.** The trail-code index at `dev/code-index` is a worktree at commit `639504fd2` (`git -C dev/code-index log -1`). The measurement copies the 12,726 tracked files of that worktree to a scratch directory. graphify and the truth run on the copy. trail-code answers from its own index of the same commit. So the three see the same code, and the index worktree stays unchanged.

**The truth.** A Go program of about 600 lines uses the standard library only. It runs `go list -json -deps -test ./...` with `CGO_ENABLED=0`, parses each file with `go/parser` and type-checks each package with `go/types`. The test variants are checked too, so a call from a test counts. A call counts when its callee name resolves to the target function or method (`types.Info.Uses`, with `Origin()` for generics). The truth reads function bodies only, so a call inside a package-level initializer is not counted.

The symbols:

- 12 are chosen by hand: the three probe symbols of the conformance suite, the `killGroup` case of C7, and 7 interface methods;
- 18 production functions and 10 test-only helpers are sampled by a hash of their position, so that no person chose them;
- the 10 API files are the files of the first symbols that are not test files.

The counts that the scores use:

- The 40 symbols have 275 caller sites. They make 237 caller keys, because several call sites in one function make one key.
- The 10 files have 167 top-level declarations. They make 163 API keys, because a key is the bare name. Two methods `Review` in `go/internal/core/reviewer.go`, and two each of `Decide` and `Recover` in `go/internal/router/strategy.go`, share a key.

**The queries.** Each query runs 3 times. Latency is the wall time of one process run, and a retry is one more run.

| Query | trail-code | graphify CLI | Truth |
|---|---|---|---|
| callers (in) | `evolve code callers <Type.Method or name>` | `graphify explain <name> --all`, the `<--` edges of relation `calls` | the static call sites |
| callees (out) | `evolve code callers --direction out <name>` | the `-->` edges of relation `calls` in the same output | the callees of the function body |
| find (definition) | `evolve code find <name>`, the first hit | `graphify explain <name>`, the `Source:` line; also `graphify query <name>` | the position of the name |
| api | `evolve code api <file>` | `graphify explain <file> --all`, the `-->` edges of relation `contains` | the top-level declarations of the file |
| map | `evolve code map` | (none) | (none: no type-checked truth for a directory summary) |

graphify has no "who calls" verb. `explain` lists every edge of one node, so the `<--` edges with relation `calls` are its callers. If `explain` exits 3 (an ambiguous name), the bench retries with the candidate id whose file is the definition file. This retry gives graphify the answer that an agent can get only if it already knows the file. `path` answers "how does A reach B", and no `evolve code` command asks that, so the bench does not score it.

**The scratch files** are in the session scratch directory under `cib/`. They are not committed, as the operator asked:

- the programs `truth/main.go`, `bench/main.go`, `proto/main.go`, `indep/check.py` and `recompute.py`;
- the data `truth.json`, `results-*.json` and `table-final.json`, and the raw output in `raw/`.

## 2. Local evidence

### graphify on this host

**F2.1** The PyPI package is `graphifyy` 0.9.92, uploaded 2026-10-16. The wheel `graphifyy-0.9.92-py3-none-any.whl` has the sha256 `9722846e627a369150107dd792145c3e3a53e3d3da46f7a7bff9f96b75da857c`. The download matched the PyPI digest (`shasum -a 256`).

`uv tool install` 0.13.0 has no `--tool-dir` flag, so the wheel went into a venv at `dev/.tools/graphify`. The venv holds 30 packages (`uv pip freeze`). The PyPI metadata names 30 required dependencies, `tree-sitter-go` among them, and lists no MCP library outside the extras.

*Implication:* a graphify backend adds a Python runtime with 30 packages from PyPI to the host. Each one is a supply-chain input.

**F2.2** graphify writes outside its output directory in two cases. The paths `graphify/*.py` in F2.2 and F2.3 are in the installed package, under `dev/.tools/graphify/lib/python3.12/site-packages/`.

- Each run refreshes the skill files of assistants that `graphify install` wrote earlier. `GRAPHIFY_NO_AUTO_REFRESH=1` stops it (`graphify/__main__.py:291-295`, `__main__.py:429-430`).
- The query log is off by default in 0.9.92, and `GRAPHIFY_QUERY_LOG_DISABLE=1` keeps it off whatever the other variables say (`graphify/querylog.py:15-32`). When enabled, it writes each question to `~/.cache/graphify-queries.log`.

The install verbs write to the assistants' files, for example `~/.claude/CLAUDE.md` (`graphify/install.py:967`). This measurement set both variables on each call, through a wrapper that also clears the environment. After the build, `~/.graphify` did not exist and `~/.cache` had no graphify file.

*Implication:* an adapter must set both variables, must clear the environment, and must never call an install verb.

**F2.3** graphify prints text that tells the agent what to do. The search hook text says "MANDATORY: graphify-out/graph.json exists. You MUST run `graphify query`..." (`graphify/cli.py:18-37`). The `query` output adds advice lines that start with `[i]` and `[!]`, for example "Narrow with context_filter=['call'] or use get_node". The ambiguous answer ends with "Retry with path::symbol...".

*Implication:* rule D4 of ADR-0138 applies. The adapter keeps only parsed fields, as the trail adapter does (`go/internal/adapters/codeindex/trail/strip.go:9`).

**F2.4** The code-only build of the copy took 80.9 s of wall time, with a peak memory of 1.86 GB (`/usr/bin/time -l graphify extract <copy> --code-only --out <dir>`). It read 8,130 code files and skipped 4,474 non-code files. It wrote 54,006 nodes, 936,934 edges and 1,387 communities. The output takes 488 MB on disk, and `graph.json` alone is 312 MB. No API key was set, so no LLM pass ran.

*Implication:* the graphify index takes 2.0 times the disk of the trail index (239 MB for `dev/code-index/graft`, `du -sh`). Its build takes 2.6 times the 31 s of C1.

**F2.5** Most graphify edges are not calls. Of the 936,934 edges, 768,104 are `imports_from` (82%), 47,529 `references`, 41,514 EXTRACTED `calls`, 41,403 `contains` and 33,521 INFERRED `calls` (a count over `graph.json`). So 45% of the call edges are guesses. The type checker finds 85,694 static calls to module functions (F2.13).

*Implication:* graphify's call graph is about half read and half guessed. The INFERRED share decides its precision.

**F2.6** Each graphify query is a new Python process that loads the 312 MB `graph.json`. A query takes about 3.5 s at p50 and 3.9 s at p95 (F2.15). The host load average was 12 to 22 during the runs, so the times are noisy. The CLI has no resident server. The MCP server is the only resident form, and the operator's rule forbids it.

*Implication:* graphify is about 10 times slower for each call than trail-code through `evolve code`, and about 10 times slower than `grep -rn` (F2.21).

**F2.7** graphify asks for a unique name. `explain DispatchPrompt` exits 3 and lists five candidates in five files. `explain PromptDispatcher.DispatchPrompt` exits 4: the CLI does not take the `Type.Method` form. The retry needs `<path>::<name>` or the node id. In the bench, 18 of 40 symbols were ambiguous on the first call.

*Implication:* an agent that does not know the file gets a list, not an answer. A backend for graphify needs the file, or a second call.

**F2.8** An INFERRED edge can point at the wrong function. For `killGroup`, graphify gives the callee `.send()` at `go/internal/phases/ship/landing/push.go:80`, a method of another package. The real callee is the parameter `send` (`go/internal/phases/swarmrunner/kill_unix.go:20`). graphify also gives the edge `cmd_cycle.go --imports_from--> kill_unix.go`: the import is of the package, not of the file.

*Implication:* graphify finds the injected call that trail-code misses (C7), but it can name the wrong target.

### trail-code through `evolve code`

**F2.9** trail-code matches a name without its case and without its type. `evolve code callers Digest` returns the callers of the lower-case `digest` in `go/internal/acssuite/evidence.go`. `evolve code callers Launcher.Run` returns 216 lines, which name any method `run` or `Run`: 36 in `go/internal/commitgate`, 36 in `go/internal/bridge` and 13 in `docs/private/research` (`raw/trail/callers/Launcher.Run.txt`).

*Implication:* the trail answer for a common name is mostly noise. That is the main cause of its precision of 0.11 for callers.

**F2.10** trail-code misses calls through a package name. `NewFromFS` has 37 callers in the truth, most of them `profiles.NewFromFS(...)`. `evolve code callers NewFromFS` exits 1 with "no result".

*Implication:* the trail graph does not join a call `pkg.F` to `F` in all cases, so its recall falls for exported functions.

**F2.11** `evolve code callers killGroup --direction out` exits 1. The callees are `fmt.Errorf` and the parameter `send` (C7, still true at `639504fd2`). `evolve code api` on `kill_unix.go` returns `signalSender`, `groupKiller` and `killGroup`, but not the constant `initProcessGroup`.

*Implication:* the trail `api` leaves out constants and variables. Its recall for `api` is 0.77, with a precision of 0.99.

**F2.12** trail-code is fast. Through `evolve code`, `callers` takes about 0.3 s at p50 and 0.45 s at p95. `find` takes about 0.85 s at p50 and 1.0 s at p95 (120 runs each). This includes the start of `evolve` and of Node. The host load average was 12 to 22 during the runs, so the times are noisy.

*Implication:* the CLI layer adds no cost that matters. The trail weakness is accuracy, not speed.

### A type-checked index

**F2.13** The type checker is fast on this module. The truth program checks 974 packages, test variants included, in 3.1 s, with 0 type errors in module packages. It skips the bodies of standard-library functions.

A second mode writes every static call edge in 4.8 s, with a peak memory of 1.69 GB. There are 235,357 edges, and 85,694 of them go to module functions. The NDJSON file is 57 MB, and 3.2 MB after gzip. F2.20 measures a compact index of the module edges as a CLI.

*Implication:* a type-checked index costs about 5 s to build at the boundary. It needs only the standard library, so it adds no library to `go.mod`. ADR-0138 rejected a native index because it "adds a library to `go.mod`". This measure removes that reason.

**F2.14** The truth has limits.

- A call through an interface counts for the interface method, not for the concrete method. So `fakeBridge.Launch` has 0 static callers and 5 calls through `Bridge.Launch` (F2.22).
- A call through a function value counts as a callee named after the variable (`send`), with no definition.
- Calls inside a package-level variable initializer are not counted.
- Reflection is not seen.


*Implication:* the truth is the static call graph, not the runtime graph. A `gotypes` backend uses the same checker as the truth, so a score against the truth alone is circular. F2.19 checks the truth against an independent method, and F2.20 scores the backend with the names that an agent types.

### The measurement

**F2.15** The full table, recomputed from the JSON files by `recompute.py` on 2026-10-17. The sources are `results-trail.json`, `results-graphify.json`, `results-graphify-noretry.json`, `results-gotypes-proto.json` and `results-grep.json`.

The method of aggregation:

- Precision and recall are micro-averaged: the sum of TP, FP and FN over the 40 symbols or the 10 files.
- p50 and p95 are nearest-rank percentiles over every process run of the row (3 for each query, and 3 more for each graphify retry).
- The bytes are the stdout and stderr of the last run of each query, because the output is the same on each run. The column gives the median over the queries (the mean of the two middle values for an even count).
- "≈ tokens" is an estimate: the median bytes divided by 4.

| Backend and query | Precision | Recall | p50 (s) | p95 (s) | Median bytes (≈ tokens, estimate) | Failures (of 40) |
|---|---|---|---|---|---|---|
| trail `callers` | 0.11 | 0.32 | 0.29 | 0.45 | 304.5 (≈ 76) | 7 wrong, 4 absent |
| trail `callers --direction out` | 0.04 | 0.21 | 0.29 | 0.44 | 57 (≈ 14) | 5 wrong, 13 absent |
| trail `api` (10 files) | 0.99 | 0.77 | 0.26 | 0.28 | 1,265.5 (≈ 316) | 0 |
| trail `api`, exported names only | 0.52 | 0.72 | (same runs) | | | |
| trail `find`, definition | top-1: 19/40 | found: 28/40 | 0.85 | 1.04 | 944 (≈ 236) | 12 not found |
| graphify `explain` callers, with the retry on exit 3 | 0.93 | 0.75 | 3.50 | 3.89 | 655 (≈ 164) | 18 ambiguous, 5 absent |
| graphify `explain` callers, first run only, no retry | 0.97 | 0.24 | (same runs) | | | 18 ambiguous |
| graphify `explain` callees | 0.88 | 0.34 | (same runs) | | | 15 absent |
| graphify `explain <file>` as `api` | 1.00 | 0.59 | 3.64 | 4.57 | 16,587.5 (≈ 4,147) | 0 |
| graphify `explain`, definition | top-1: 22/40 | found: 40/40 | (same runs) | | | 18 as a list of candidates |
| graphify `query`, definition | top-1: 25/40 | found: 27/40 | 3.82 | 4.67 | 6,501 (≈ 1,625) | 13 not found |
| `gotypes` prototype `callers`, name as typed (F2.20) | 0.44 | 1.00 | 0.047 | 0.079 | 267 (≈ 67) | 0 |
| `gotypes` prototype `callers`, package-qualified name (F2.20) | 1.00 | 1.00 | 0.044 | 0.095 | 245.5 (≈ 61) | 0 |
| `grep -rn -w --include=*.go <name>`, no scoring (F2.21) | | | 0.35 | 0.45 | 1,225 (≈ 306) | |

The index costs, from F2.4, F2.13, F2.17, F2.20 and C1:

| Backend | Build | Disk | Refresh after one edit | Peak memory of the build |
|---|---|---|---|---|
| trail-code | 31 s (C1, not measured again) | 239 MB | about 2 s (C1, not measured again) | not measured |
| graphify 0.9.92 | about 81 s | 488 MB (`graph.json` 312 MB) | about 66 s; 109–130 s with no change | 1.86 GB |
| type checker, all edges | about 5 s | 57 MB NDJSON (3.2 MB gzip) | about 5 s (a full build) | 1.69 GB |
| `gotypes` prototype, module edges | about 5 s, then 0.43 s for the index | 19 MB gob (2.3 MB gzip) | about 5 s (a full build) | 1.69 GB |

Notes on the method:

- "Found" for `find` means that the definition is in the output, at the right file and within one line.
- The graphify retry gives graphify the definition file of the symbol, which an agent often does not have (section 1).
- The graphify INFERRED caller edges were 91 true and 0 false. Its 13 false callers came from EXTRACTED edges after a retry.
- The host load average was 12 to 43 during the runs (`uptime`), because other sessions ran. All latencies are on the same host and day, but they are noisy. Compare them by order of size, not by the second digit.

**F2.16** The callers result by category of symbol (precision / recall):

| Category (symbols) | trail-code | graphify, with the retry |
|---|---|---|
| seed probes of `codeindextest` (3) | 1.00 / 1.00 | 1.00 / 1.00 |
| `killGroup` and `groupKiller` (2) | 1.00 / 1.00 | 1.00 / 1.00 |
| interface methods (7) | 0.03 / 0.18 | 1.00 / 0.06 |
| sampled production functions (18) | 0.38 / 0.23 | 0.91 / 0.99 |
| sampled test-only helpers (10) | 0.08 / 0.85 | 1.00 / 1.00 |


*Implication:* trail-code is exact on unique names (the seed probes and the `killGroup` case) and fails on interface methods and common names. graphify is accurate on functions with a unique node, but it has almost no caller for a call through an interface.

**F2.17** The refresh after a change of one file. graphify `extract` is incremental: it keeps a manifest and an AST cache under `graphify-out/` (`graphify --help`, `--force` "skip the incremental manifest gate"). 

After an edit of one Go file, a rerun of `extract` took 66.4 s. Two reruns with no change took 129.9 s and 109.0 s. Each run re-extracted 2,067 files, so the incremental gate saves little. For trail-code, the rule of this task forbids `trail build`, so its refresh is not measured again: C1 gives about 2 s. For the type-checked index, the full build of F2.13 is the refresh: about 5 s, with no incremental mode.

*Implication:* graphify cannot refresh in seconds at the boundary. trail-code and the type checker can.

**F2.18** The failure modes, summed over the callers queries:

| Backend | Wrong (answers, none true) | Ambiguous (exit 3) | Absent (no answer) |
|---|---|---|---|
| trail-code | 7 | 0 (it merges the names, F2.9) | 4 |
| graphify | 0 | 18 | 5 |
| `gotypes` prototype, name as typed | 0 | 0: it merges the homonyms of a bare name (F2.20) | 0 |

### Checks added after the review

**F2.19** An independent check of the truth. For 10 symbols, `indep/check.py` greps for `\bName\s*\(` in the files of the defining package and of every package that imports it (`go list -json -deps -test`). It then compares the hits with the truth sites, and a person reads each difference.

- Both methods found the same 130 call sites. The truth missed 0 sites that the grep found.
- The grep found 41 more lines, and none is a call of the target. 18 are inside string literals, such as `t.Errorf("killGroup(%d) must be refused", pgid)`. 23 are calls of homonyms: 21 of `prompts.NewFromFS` and 2 of the methods `l.Digest()` and `lesson.Digest()`.

`gopls` is not installed (C8), so its CLI was not a second check. The 10 symbols are the 3 seed probes, `killGroup`, `groupKiller` and 5 sampled functions, so the check does not cover interface methods.

*Implication:* on the checked symbols, the truth agrees with grep and reading on 130 of 130 sites. A precision of 1.0 against the truth is then a measured claim for functions, not a circular one.

**F2.20** A CLI prototype of the `gotypes` index (`proto/main.go`). It keeps the 85,694 module edges, keyed by the bare name, by `Type.Method` and by the package-qualified name. The index is one gob file of 19 MB, written in 0.43 s from the edge file. A query loads it and prints the three-field lines of `evolve code`. The scores over the 40 symbols are:

| Name form | Precision | Recall | p50 | p95 | Median bytes |
|---|---|---|---|---|---|
| as an agent types it (`Digest`, `Launcher.Run`) | 0.44 | 1.00 | 0.047 s | 0.079 s | 267 |
| package-qualified (`router.Digest`, `loopwave.Launcher.Run`) | 1.00 | 1.00 | 0.044 s | 0.095 s | 245.5 |

`evolve code` itself starts in 0.019 s at p50 (30 runs of the usage error, `results-evolve-startup.json`). So a `gotypes` call through `evolve code` costs about 0.07 s at p50. That figure is an estimate, the sum of two measured parts.

*Implication:* the type checker is exact only for a qualified name. A bare name merges the homonyms of every package, so the backend must name the definition of each hit.

**F2.21** The grep baseline. `grep -rn -w --include=*.go <name>` over the copy takes 0.35 s at p50 and 0.45 s at p95 (120 runs). Its median output is 1,225 bytes, and one name gave 856,215 bytes (`results-grep.json`). This is the cost that an agent pays today before it reads files.

*Implication:* trail-code costs about what grep costs, and the `gotypes` prototype costs about a fifth of it. graphify costs about 10 times as much. No measure yet gives the latency that an agent can accept for a better answer.

**F2.22** A concrete method that only an interface calls. The truth program records the interface of each such call with `go/types.Implements`, at no measurable cost. `fakeBridge.Launch` has 0 static callers and 5 calls through `Bridge.Launch`. `reportingErrRunner.Name` has 0 of both: it only satisfies an interface, and no production file of `go/internal/core` calls `Name()` on a phase value (a grep).

*Implication:* an empty `callers` answer for a concrete method is wrong when the method is reached through an interface. The answer can name the interface and its call count instead.

## 3. Current practice

### graphify

**P3.1** graphify turns a folder of code, documents and papers into a knowledge graph. It parses code locally with tree-sitter, with no LLM. A semantic pass with an LLM reads documents, papers and images. Each edge is tagged EXTRACTED or INFERRED, and Leiden clustering groups the nodes into communities. The output is `graph.json`, an HTML view and a report.

The CLI verbs `query`, `path` and `explain` exit 0 with an answer. They exit 3 for an ambiguous name and 4 when no node matches ([graphify README](https://github.com/Graphify-Labs/graphify), [graphify ARCHITECTURE](https://github.com/Graphify-Labs/graphify/blob/main/ARCHITECTURE.md)).

The exit codes are confirmed locally by `graphify --help` and by F2.7. The repository had 127,444 stars on 2026-10-17 (`gh api repos/Graphify-Labs/graphify`). Its license is Apache-2.0, with an MIT file beside it.

**P3.2** The README says that graphify parses about 40 languages and does not name Go. The local evidence confirms Go: the package needs `tree-sitter-go` (F2.1), and the graph has 6,051 Go file nodes, with methods and `embeds` edges (F2.5). The ARCHITECTURE file was not read in full. Not verified: how the resolver picks an INFERRED target in Go.

**P3.3** graphify's own benchmarks are about memory and a code question set. ERPNext is a Python repository of about 1M lines. There, one graphify tool lifts the key-fact coverage of a fixed agent from 70.8% (grep and read) to 82.0%. The cost is about 140K tokens for each query, and n=6. A manual check found all 12 INFERRED cross-file edges of a FastAPI demo correct. The file says that test helpers with the same name "produce noisier INFERRED `calls` edges" ([graphify BENCHMARKS.md](https://github.com/Graphify-Labs/graphify/blob/main/BENCHMARKS.md), read through `gh api` on 2026-10-17).

The benchmarks do not measure precision or recall of callers or definitions. Graphify Labs sells a hosted product, Graphify Cloud, and the CLI help advertises its trial.

*Implication:* our F2.15 is the first measure of graphify's call graph against a type checker that we know of. Its own note on test helpers matches F2.16.

**P3.4** A "71.5x fewer tokens" figure for graphify circulates in 2026 write-ups. The sources disagree on its corpus: one says 52 mixed files, with 5.4x on a 4-file project. Another says a large monorepo. It gives a range of 6.8x to 49x from sources that it does not name ([thuliobittencourt.com](https://thuliobittencourt.com/graphify-71x-menos-tokens-agente-ia-entender-codigo/), [Towards AI](https://pub.towardsai.net/beyond-the-71x-benchmark-knowledge-graphs-for-coding-agents-graphify-and-rivals-compared-c835177a3b55), paywalled, not read in full). The current README and BENCHMARKS.md do not carry the figure.

An Augment Code post repeats the memory numbers and tells readers to reproduce them on their own corpus. Augment sells a competing context engine ([Augment Code, 2026-07-07](https://www.augmentcode.com/learn/graphify-v099-benchmarks-knowledge-graph)). Not verified: any controlled third-party test of graphify on code structure.

*Implication:* the token claims are not evidence for this repository. Only a local measure is.

### Other code-graph tools for agents

**P3.5** Codebase-Memory parses with tree-sitter into SQLite and serves 14 query tools over MCP. On 31 repositories it gave 83% answer quality, against 92% for an agent that reads files. It used about a tenth of the tokens and 2.1 times fewer tool calls. On graph-native questions, such as caller ranking, it matched or beat the explorer on 19 of 31 languages ([arXiv 2603.27277](https://arxiv.org/abs/2603.27277), abstract read).

*Implication:* an approximate graph trades accuracy for tokens. It is an MCP server, so the operator's rule rules it out as a tool. Its result is still evidence.

**P3.6** Other tools for agents:

- codegraph sends its usage guidance in the MCP start-up response ([codegraph](https://github.com/colbymchenry/codegraph), from P3.3 of the first dossier).
- Serena wraps language servers and gives symbol tools, such as `find_referencing_symbols`, through MCP ([Serena](https://github.com/oraios/serena), described by a third-party listing, not verified).
- Aider's repo map parses with tree-sitter and ranks symbols with PageRank to fit a token budget. The source found is RepoMapper, which says it reimplements Aider's map ([RepoMapper](https://mcpservers.org/servers/pdavis68/RepoMapper)). Aider's own docs were not read.

An independent benchmark found that a graph never beat grep and read on tokens. Its one clear gain was accuracy on structural questions ([code-graph-vs-grep](https://github.com/nefayran/code-graph-vs-grep), first dossier P3.3).

*Implication:* codegraph, Serena and the MCP forms are out by the operator's rule. The repo-map idea (rank by importance for a budget) is a `map` design, not a `callers` design.

### Type-checked options for Go

**P3.7** `gopls` has CLI commands `call_hierarchy`, `references`, `implementation` and `definition`. Each takes a position `file:line:col` or a byte offset. The Go team marks the CLI as experimental. The navigation docs say that the call hierarchy does not find dynamic calls ([gopls navigation](https://go.googlesource.com/tools/+/refs/heads/master/gopls/doc/features/navigation.md)).

gopls also has an MCP server since v0.20.0, marked experimental (third-party listings, not verified against release notes). `gopls` is not installed on this host (C8).

*Implication:* the gopls CLI needs a position, not a name, so a backend must first find the definition. Its MCP form is out by the operator's rule.

**P3.8** `golang.org/x/tools/go/callgraph` gives static, CHA, RTA and VTA call graphs over SSA. Its docs define a sound graph as an over-approximation of all executions, and a more precise graph as a smaller one. CHA adds edges to every type that implements an interface.

The static graph has only static edges ([go/callgraph](https://pkg.go.dev/golang.org/x/tools/go/callgraph), [cha](https://pkg.go.dev/golang.org/x/tools/go/callgraph/cha)).

*Implication:* CHA or VTA can answer "which concrete methods can this interface call reach", which F2.14 leaves out. They need `golang.org/x/tools` in `go.mod`. The static graph of F2.13 needs the standard library only.

## 4. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Measure each backend against a type-checked truth before a choice | Adopt | F2.15: the trail and graphify numbers differ by query type. P3.3, P3.4: published numbers do not measure this. |
| A type-checked Go index for `callers` and `api`, built at the boundary | Adopt | F2.13: about 5 s, the standard library only. F2.9, F2.10: trail fails here. |
| trail-code for `find` and `map` | Adopt for now | Only trail serves `map`. For `find`, graphify `query` has a better top-1 (25/40 against 19/40) at about 10 times the latency (F2.15). The test used exact names, not free words. The trade is open (plan Q5). |
| graphify as a backend in the default route | Not now; a bench-only run | F2.15: below the type checker on `callers` (0.75 recall with a retry, 0.24 without). F2.17: a refresh of about 66–130 s. F2.1: a Python supply chain. |
| A graphify adapter in Go | Defer | YAGNI: build it only when a bench run shows graphify best for a query type. Until then, the bench runs the graphify CLI from outside, as section 1 did. |
| graphify through its MCP server | Reject | The operator's rule of 2026-10-17 forbids MCP. F2.6: the CLI is the only allowed form. |
| `gopls` MCP, codegraph MCP, Serena, Codebase-Memory | Reject | The operator's rule forbids MCP (P3.5, P3.6, P3.7). |
| The `gopls` CLI as the type-checked backend | Reject for now | P3.7: not installed, experimental, takes a position. F2.13: the stdlib checker gives the same static answer. |
| `go/callgraph` CHA or VTA for interface calls | Defer | P3.8: it needs `golang.org/x/tools`. Use it if F2.14's interface gap shows in the usage log. |
| A repo map ranked by PageRank | Defer | P3.6: a `map` idea. No measure of `map` exists yet. |

## 5. Refinements to adopt

- **R1** (F2.13, F2.14): add a `gotypes` backend that serves `callers` and `api` from a type-checked index. It reads a file that the boundary writes.
- **R2** (F2.15, F2.16): set `code_index.backends` to `["gotypes", "trail"]`. The capability route then sends `callers` and `api` to `gotypes`, and `find` and `map` to trail-code.
- **R3** (F2.15, F2.19): add `evolve code bench`, which reruns this measurement against the truth and prints the table. It also runs the grep check of F2.19 on a sample. A backend change must show its numbers first.
- **R4** (F2.2, F2.3, F2.7): if a graphify adapter is built later, it runs the CLI only, with a clean environment and both controls of F2.2. It keeps parsed fields only, and it gives `ErrNotFound` with the candidates for an ambiguous name.
- **R5** (F2.14, F2.22): the `gotypes` answer marks a call through an interface or a function value. A concrete method with no static caller gets the calls through each interface that it implements.
- **R6** (F2.17, C1): the boundary refreshes only the indexes that the route names, and a failed refresh is a warning (ADR-0138 decision 10). graphify refreshes only inside a bench run.
- **R7** (F2.20): the `gotypes` backend takes a package-qualified name for an exact answer. For a bare name with several definitions, each hit names its definition.
