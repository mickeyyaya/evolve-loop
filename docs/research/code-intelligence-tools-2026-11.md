# Code-intelligence tools: which one fits evolve-loop best (research, 2026-11)

> The related work: [code-index-backends-2026-10](code-index-backends-2026-10.md) (the first bench of trail-code, graphify and a type checker) and [code-index-cli-2026-10](code-index-cli-2026-10.md) (why a CLI and not MCP). Also the spec [code-index-cli](../architecture/code-index-cli.md) and the plan [code-index-backends plan](../plans/code-index-backends-2026-10.md). The knowledge-base peer study is the qmd study `docs/research/qmd-knowledge-base-2026-11.md` (PR #1732, not on main yet). This dossier does not repeat it.

Date: 2026-11-23. Method:

- a live read of the public pages, APIs and papers of each tool (links in section 8);
- a read of our code and docs in the `origin/main` tree of 2026-11-23 (`2c8fc5fe9`), with `path:line` for each local claim;
- the measured numbers of the earlier bench, which we cite and do not redo.

We installed and ran no tool for this dossier. Each outside number is a claim of the vendor or of a third party. It is not a measurement of ours. "Synthesis" marks our own inference. "Not verified" (NV) marks a claim that we cannot check. Two stars and release counts were re-read on 2026-11-23 from the GitHub API (graphify and trail-code, section 8).

## Table of contents

1. [Request](#1-request)
2. [Terms used throughout](#2-terms-used-throughout)
3. [Facts: our code index and our constraints](#3-facts-our-code-index-and-our-constraints)
4. [The tool landscape](#4-the-tool-landscape)
5. [What retrieval helps an agent](#5-what-retrieval-helps-an-agent)
6. [The strategies, scored](#6-the-strategies-scored)
7. [Recommendation, staged plan and risks](#7-recommendation-staged-plan-and-risks)
8. [Sources](#8-sources)

## 1. Request

The console asked for a dossier that compares trail-code, graphify and the other open-source code-intelligence and knowledge tools. It must decide which tool fits this project best "while keeping its current architecture". The console reviews and ships it.

The question that this dossier decides:

> Which tool, or which mix of tools, gives our agents the best code navigation (find, callers, refs, api, map) and the best knowledge-base recall? The rules of section 3.2 apply.

Code navigation and the knowledge base are two questions. Section 7 answers each one.

## 2. Terms used throughout

| Term | Meaning |
|---|---|
| Truth set | 40 symbols in 10 files. The Go type checker (`go/types`) gives the true callers of each one ([code-index-backends](code-index-backends-2026-10.md), F2.15). |
| Precision, recall | Precision is the part of the answers that are right. Recall is the part of the true answers that the tool finds. |
| Tree-sitter | A parser that builds a syntax tree without type information. It cannot tell two functions with one name apart. |
| Type-checked index | An index that comes from a compiler front end. It knows the type and package of each name. |
| LSP, gopls | A language server and the Go language server. `gopls` has experimental CLI verbs. |
| SCIP, Kythe | Two formats for a compiler-made code index. |
| Trigram index | A text index of 3-character pieces. It makes a regex or substring search fast. |
| Repo map | A short ranked list of the key symbols of a repository. It fits a token budget. |
| BM25, RRF | A text ranking function, and a method to merge ranked lists. The peer study defines both. |
| Graph RAG | Retrieval that uses a graph that an LLM extracts from text. |
| Backend | One implementation of `codeindex.Backend` behind `evolve code`. |
| The bridge | `evolve agent run` and `go/internal/bridge`. It is the one path by which an LLM runs. |
| Shadow | A mode in which a new component runs beside the old one. It logs its answer. The old answer is the one that the pipeline uses. |

## 3. Facts: our code index and our constraints

### 3.1 The index today

**F3.1** `evolve code` has four read-only verbs: `callers`, `api`, `find` and `map`. A `Backend` interface serves them. It has the methods `Callers`, `API`, `Find` and `Map`, and also `Name` and `Capabilities`. Its errors are `ErrNotFound`, `ErrAbsent` and `ErrUnsupported` ([code-index-cli](../architecture/code-index-cli.md):60-69, :112-119).

*Implication:* a new tool is a new `Backend`. The agents see no change (goal G5, `docs/architecture/code-index-cli.md:31`).

**F3.2** Two backends are built. The `trail` adapter wraps the trail-code CLI (`go/internal/adapters/codeindex/trail`). The `gotypes` adapter type-checks the module with the standard library only (`go/internal/adapters/codeindex/gotypes`). graphify has no adapter. It ran in a bench only ([code-index-cli](../architecture/code-index-cli.md):413-435).

*Implication:* the pattern for a third backend exists, and the conformance suite and the bench exist too.

**F3.3** The live route is `["trail"]`. The compiled default is `defaultCodeIndexBackend = "trail"` (`go/internal/policy/policy_codeindex.go:10`). The file `.evolve/policy.json` has no `code_index` key. Item G4, "default route `callers` to gotypes", is "not started" (`docs/plans/code-index-backends-2026-10.md`:207-213). The route is a capability route: the first backend that serves a query answers it, with no failover ([code-index-cli](../architecture/code-index-cli.md):89-94, :134-136).

*Implication:* the best backend that we have built does not serve any query today.

**F3.4** The measured quality, against the truth set (host load average 12 to 22, so times are noisy). Source: [code-index-backends](code-index-backends-2026-10.md):158-181, F2.15.

| Query and backend | Precision | Recall | p50 (s) | p95 (s) | Note |
|---|---|---|---|---|---|
| trail `callers` | 0.11 | 0.32 | 0.29 | 0.45 | 7 wrong, 4 absent. It matches a name without case or type (F2.9). |
| trail `callers --out` | 0.04 | 0.21 | 0.29 | 0.44 | It misses calls through an injected function (C7). |
| trail `api` | 0.99 | 0.77 | 0.26 | 0.28 | Exported symbols only: 0.52 and 0.72. |
| trail `find` (definition) | top-1 19/40 | found 28/40 | 0.85 | 1.04 | 12 not found. |
| graphify `callers` (with retry) | 0.93 | 0.75 | 3.50 | 3.89 | 18 of 40 ambiguous. 45% of call edges are INFERRED. |
| gotypes prototype, qualified name | 1.00 | 1.00 | 0.044 | 0.095 | A bare name scores 0.44 and 1.00 (F2.20). |

Build costs: graphify takes 80.9 s, 1.86 GB peak and a 488 MB graph (F2.4-F2.6). trail-code takes 31 s for a full build (6,004 files, 51,347 symbols, 134,377 edges) and about 2 s for an increment (C1 of [code-index-cli-2026-10](code-index-cli-2026-10.md):28). The shipped gotypes backend builds in 3.7 to 8.2 s, peaks at 434-461 MB, and keeps a 5.4 MB index (`docs/architecture/code-index-cli.md`:423-425). The truth program checks 974 packages in 3.1 s (F2.13).

*Implication:* for `callers`, the type checker wins on quality and on latency. This is a measurement on our own code, not a vendor claim.

**F3.5** The truth is the static call graph from the same checker that `gotypes` uses, so a score against it alone is circular. An independent grep check of 10 symbols agreed (F2.19, [code-index-backends](code-index-backends-2026-10.md):154, :229). The check has no interface methods ([code-index-cli](../architecture/code-index-cli.md):485).

*Implication:* a new backend must also pass a second, independent set (section 7, stage 0).

**F3.6** Agents barely use the index. In the 7 days to 2026-10-26 `evolve code usage` counted 2 calls, both from the console (`docs/architecture/code-index-cli.md:203`). Twenty agy phases made 0 calls through MCP (C3 of [code-index-cli-2026-10](code-index-cli-2026-10.md):34). A protected-file fence blocks the persona text for the commands (item K4a). The token gain is not proven (`docs/architecture/code-index-cli.md:511`).

*Implication:* a better backend raises the quality of each call. It does not raise the number of calls. Uptake needs its own work (item K4a).

**F3.7** trail-code (installed 0.30.5, bin `trail`) writes into the tree that it indexes: `.gitignore`, `.ignore` and a `graft/` cache of about 238 MB. So only the detached worktree `dev/code-index` is written, and only at a boundary (F4.6, `docs/architecture/code-index-cli.md:209`, :216). The adapter must strip an instruction that the tool prints to the agent, and must set `DO_NOT_TRACK=1` (`docs/architecture/code-index-cli.md:156-171`).

*Implication:* any tool of this type needs the same wall: a dedicated index tree, a boundary refresh and an output filter.

**F3.8** The module has 548,299 Go lines in 8,850 files without `go/vendor`, and 569,595 lines with it (`find go -name '*.go' | xargs wc -l`, 2026-11-23). The module needs Go 1.23 and has two libraries: `gopkg.in/yaml.v3` and `pgregory.net/rapid` (`go/go.mod:5-7`).

*Implication:* the repository is about 0.55 million Go lines, not one million. Every "1M lines" claim of a vendor is an upper bound for us.

### 3.2 The hard constraints

Any tool must pass all of these. The ids C1 to C9 are used in the scores of section 6.

| Id | Constraint | Source |
|---|---|---|
| C1 | CLI only, never MCP. When a tool has no CLI, build one. | `AGENTS.md`:402; `docs/conventions/cli-only-no-mcp.md`:1-12 |
| C2 | Every LLM call goes through the bridge. No LLM starts or directs another. | `AGENTS.md`:97; `docs/architecture/llm-isolation.md`:61 |
| C3 | The Go binary `evolve` is the single controller. A tool sits behind it. | `docs/architecture/llm-isolation.md`:1, :65 |
| C4 | Hermetic cells: a closed environment allowlist, writes only in the assigned tree, no ambient control variables. | `docs/architecture/llm-isolation.md`:331-350 |
| C5 | Adapters only: one package for each outside library or binary, with a structure test. | `docs/conventions/third-party-dependencies.md`:7-17; `go/test/structure/binary_test.go`:26 |
| C6 | Runs on macOS and Linux CI (ubuntu with Go 1.23 and 1.27, `macos-latest`). | `.github/workflows/go.yml`:18, :151, :231 |
| C7 | The main language is Go (about 548k lines). Docs and shell scripts also exist. | F3.8 |
| C8 | No telemetry and no network at query time. The tool output must not instruct the agent. Only the boundary writes the index. | `docs/architecture/code-index-cli.md`:156-171, :185, :258 |
| C9 | A new backend passes the conformance suite and carries a bench table against the truth. A new tool carries its docs. | `docs/architecture/code-index-cli.md`:128-139, :474 |

Two more rules apply to the code that we write: no comments, and 100% coverage of new code. They limit the work of a native build (section 6).

### 3.3 The retrieval needs

| Need | Question | Served today by |
|---|---|---|
| Find | Where is the code for "this idea"? | trail `ask` (top-1 19/40) |
| Callers and refs | Who calls this symbol? What does it call? | trail (0.11 precision). gotypes is built and unused. |
| Api | What are the signatures of this file? | trail `skeleton`. gotypes is built and unused. |
| Map | What is the structure of this directory? | trail `map` only |
| Knowledge-base recall | Which lesson, incident or dossier applies to this plan? | `research.KB` (peer study) |

### 3.4 A worked example

Run `evolve code callers NewFromFS`. The truth has 37 callers, most of them `profiles.NewFromFS(...)`. trail-code exits 1 with "no result", because it does not match a call through a package name (F2.10). The gotypes prototype, asked for `profiles.NewFromFS`, returns all of them in 0.044 s (F2.20). A tree-sitter graph cannot fix this fault. The fault is in the lack of types, not in the tool.

## 4. The tool landscape

Each table has one row for each tool. "NV" means not verified. "(vendor)" marks a figure that the authors report. Each row cites its page in section 8 by the link in the first column. Stars and dates are as read on 2026-11-23.

### 4a. Agent code indexes

| Tool | License | Runtime | Build method | LLM at index or query | Query surface | Go quality | Incremental | Hermetic | Maturity | Known issues |
|---|---|---|---|---|---|---|---|---|---|---|
| [trail-code](https://github.com/trailhq/Trail-Code) | MIT | Node (TypeScript) | tree-sitter, wasm grammars. `build --deep` adds LLM passes | No for the map. Yes for `--deep`. | CLI and MCP | Name-only. Our bench: callers 0.11 / 0.32 (F3.4). | Yes, content hash | Writes into the indexed tree. Telemetry unless `DO_NOT_TRACK=1`. | 11,457 stars, 271 open issues. Created 2026-07-03. Two npm maintainers. | Fast churn: 0.23 to 0.30 in weeks. An upgrade notice broke our exit code. `init` wires hooks and MCP. |
| [graphify](https://github.com/Graphify-Labs/graphify) | Apache-2.0 (plus MIT files) | Python 3.11 to 3.14 | tree-sitter for code, about 40 languages. LLM only for documents and images. | No for code. Yes for documents. | CLI, skill and optional MCP | Listed. Our bench: callers 0.93 / 0.75 with a retry (F3.4). | `--update`, `watch`, git hooks. Issue #857 on stale edges: status NV. | Code-only runs offline | 152,360 stars, 1,692 open issues and PRs (API). Release v0.9.118 on 2026-11-23. | 80.9 s, 1.86 GB build. 45% of call edges INFERRED. The PyPI name is `graphifyy`. Typosquat risk. |
| [CodeGraph](https://github.com/colbymchenry/codegraph) | MIT | Rust kernel, npm | native parse, 20 languages | No | CLI (many verbs) and MCP | "Full". 96.6% cross-file on gin (vendor). NV on our code. | Watcher, sync | Local. Optional telemetry. | 78,495 stars, v1.6.4 (2026-11-20) | SQLite lock errors on network disks. The agent falls back to grep without `init`. |
| [codebase-memory-mcp](https://github.com/DeusData/codebase-memory-mcp) | MIT | C, one binary | 164 tree-sitter grammars plus hybrid type resolution | No generative LLM. Bundled embeddings. | MCP, plus one-shot CLI | Hybrid type resolution covers Go. NV on our code. | Watcher | Local. A `curl \| bash` installer. | 49,071 stars, v0.12.1 (2026-10-16) | A format change in 0.11 forced a reindex. |
| [CodeGraphContext](https://github.com/CodeGraphContext/CodeGraphContext) | MIT | Python | tree-sitter | No | CLI and MCP | Listed. NV. | `cgc watch` | Embedded graph stores | 4,400 stars | Six storage backends. Go depth NV. |
| [code-graph-rag](https://github.com/vitali87/code-graph-rag) | MIT | Python 3.12+ | tree-sitter plus `go/types` for Go | LLM turns natural language into a graph query | CLI and MCP | Full, `go/types`. NV on our code. | Partial | Needs an LLM provider for questions | 5,399 stars, v0.7.79 (2026-11-21) | 276 open issues. Memgraph or Neo4j are options. |
| [GitNexus](https://github.com/abhigyanpatwari/GitNexus) | PolyForm Noncommercial 1.0.0 | Node | tree-sitter | No (optional) | CLI, MCP, web UI | Coverage varies | `analyze --watch` | `analyze` writes `AGENTS.md` and `CLAUDE.md` into the repo | 48,754 stars | The license is not open source. It also edits our rule files. |
| [Potpie](https://github.com/potpie-ai/potpie) | Apache-2.0 | Python | NV | NV | CLI-first | NV | NV | Needs `potpie login` for managed features | 5,798 stars, v2.0.1 (2026-08-31) | Too little is verified. |

**F4.1** All eight tools build the call graph from a tree-sitter parse, except code-graph-rag (`go/types` for Go) and codebase-memory-mcp (a hybrid type resolution). The rest cannot separate two functions with one name.

*Implication:* the root of our measured trail-code fault (F3.4) is in the method, so a swap to another tree-sitter tool is not a fix. graphify scores better because it resolves names on the graph, but it pays 80.9 s and 1.86 GB.

**F4.2** The agent code indexes ship an MCP server first and a CLI second. Most install hooks or edit agent config (trail `init`, GitNexus `analyze`, CodeGraph `init`).

*Implication:* each one needs the wall of F3.7 and an output filter. None is free of C1, C4 and C8 work.

**F4.3** All benchmarks of these tools are vendor reports. Examples: trail-code "46% fewer tool calls" and 27/50 against 33/50 on SWE-bench Verified; CodeGraph "88% fewer tool calls" (median over 7 repositories); codebase-memory-mcp "10x fewer tokens". No independent replication was found. None measures caller accuracy.

*Implication:* we treat them as hypotheses. Our measured caller accuracy (F3.4) is the only accuracy number in this section.

### 4b. Compiler-accurate indexes

| Tool | License | Runtime | Build method | LLM | Query surface | Go quality | Incremental | Hermetic | Maturity | Known issues |
|---|---|---|---|---|---|---|---|---|---|---|
| our [gotypes](../architecture/code-index-cli.md) | ours | Go, stdlib only | `go list`, `go/parser`, `go/types` | No | `evolve code` | Exact for static calls. 1.00 / 1.00 with a qualified name. | Rebuild at a boundary, 3.7 to 8.2 s | Yes | Built (G1 to G3, G5) | Go only. Host platform only. Calls only. No concrete target for an interface call. |
| [gopls CLI](https://pkg.go.dev/golang.org/x/tools/gopls) | BSD-3-Clause | Go | The Go type checker through LSP | No | CLI verbs `references`, `implementation`, `call_hierarchy`, `symbols`. Also `gopls mcp`. | Native, the best | In-process cache for one run | It can run `go` and contact the module proxy. NV. | v0.24.0 (2026-10-14) | The CLI doc calls the interface "experimental". It takes a position, not a name. It reloads packages on each run. Start-up cost NV. |
| [scip-go](https://github.com/scip-code/scip-go) with [SCIP](https://github.com/sourcegraph/scip) | Apache-2.0 | Go | `go/packages` | No | A `.scip` file. No query CLI. We must write the reader. | Compiler-grade | Whole index. Incremental NV. | Shells out to `go` | SCIP 893 stars, v0.10.0 (2026-09-03). scip-go 74 stars. | Stdlib navigation needs manual indexing. |
| [Kythe](https://github.com/kythe/kythe) | Apache-2.0 | Go, C++, Java; Bazel | Compiler extractors and indexers | No | Tools and an xrefs service. No MCP. | Go extractor and indexer | NV | Needs a build of the code under Bazel | 2,171 stars, v0.0.76 (2026-07-16) | Heavy. It needs a build extraction for each build. |

**F4.4** gopls, scip-go and Kythe all give exact Go answers. Each needs the `go` tool or Bazel at run time.

Each has a limit for us. gopls takes a position, and we have a name. scip-go gives a file and no query layer. Kythe needs Bazel. Our `gotypes` already gives the same static answer from the standard library with no new dependency (F2.13, F2.20).

*Implication:* for Go, `gotypes` is the compiler-accurate option that fits C3 and C5 at zero cost. The others are checks and comparators, not replacements. The earlier study rejected the gopls CLI for the same reasons ([code-index-backends](code-index-backends-2026-10.md):322).

### 4c. Lexical and structural search

| Tool | License | Runtime | Build method | LLM | Query surface | Go quality | Incremental | Hermetic | Maturity | Known issues |
|---|---|---|---|---|---|---|---|---|---|---|
| [zoekt](https://github.com/sourcegraph/zoekt) | Apache-2.0 | Go | Trigram index. ctags ranks symbols. | No | CLI, JSON and gRPC | Text only. Symbol rank needs ctags. | Re-index a whole repo. Shard rebuild. | Local | 2.1k stars. No GitHub releases. | Index is 2 to 3 times the corpus (Sourcegraph blog). No 1M-line Go figure found. |
| [probe](https://github.com/buger/probe) | Apache-2.0 | Rust, Node SDK | ripgrep plus tree-sitter blocks, BM25 | No (only the `agent` command) | CLI and MCP | `.go` supported. Quality NV. | None needed (no index) | Local | 742 stars. Release candidate line (`v0.6.0-rc347`). | Release churn. |
| [ast-grep](https://github.com/ast-grep/ast-grep) | MIT | Rust | tree-sitter pattern match | No | CLI, YAML rules | Go listed | None (no index) | Local | 16.7k stars | Structural queries only. No natural language. |
| [universal-ctags](https://github.com/universal-ctags/ctags) | GPL-2.0 | C | Definition tags | No | CLI, tag files | A parser exists. Quality NV. | `-a` append only | Local | 7.3k stars | Definitions only. No references. The license is copyleft. |
| [aider repo map](https://aider.chat/docs/repomap.html) | NV on the page | Python | tree-sitter tags and a PageRank graph | No | Inside aider only | Go credited | Per-file cache by mtime | Local | Part of aider | No standalone CLI. A `ZeroDivisionError` issue (#1536). |
| [repomix](https://github.com/yamadashy/repomix), [code2prompt](https://github.com/mufeedvh/code2prompt) | MIT | Node, Rust | Pack files into one prompt | No | CLI (both also offer MCP) | Weak for Go | Not applicable | Local | 29.4k, 7.8k stars | They pack and do not retrieve. A repository of 0.55 million lines cannot fit a context. |

**F4.5** Only `ast-grep`, `probe` and `zoekt` are good CLI tools with no model. None is type-aware. zoekt is the only Go-native one. Its trigram index makes a regex search of a large tree fast. A third-party report puts the Linux kernel (about 545 MB) at about 160 s to index on one thread ([Sourcegraph](https://sourcegraph.com/blog/zoekt-memory-optimizations-for-sourcegraph-cloud/)). We have no timing for our tree.

*Implication:* these tools replace `grep`, not `callers`. They address `find` and `map` at best. Today agents already run `grep` and `rg` through the shell (C5 of [code-index-cli-2026-10](code-index-cli-2026-10.md):40), so the gain is speed and ranking, not a new ability. Synthesis.

**F4.6** The aider repo map is the best budgeted yield at 8K tokens in one study (section 5, F5.3). It has no standalone CLI. A Go port needs tree-sitter or the `go/ast` package. For Go, `go/ast` and `go/doc` give the exported symbols without a new library.

*Implication:* a native `map` for Go is a small job on `go/parser` and the existing `gotypes` index. Synthesis.

### 4d. Knowledge-base and memory tools

The peer study covers [qmd](https://github.com/tobi/qmd) in depth. This table adds the others and stays short.

| Tool | License | Runtime | Build method | LLM at ingest or query | Query surface | Go quality | Incremental | Hermetic | Maturity | Known issues |
|---|---|---|---|---|---|---|---|---|---|---|
| qmd | MIT | Node 22 or Bun | SQLite FTS5 plus vectors | Local models at query time | CLI and MCP | Not Go | Yes | Needs staged models | See the peer study | See the peer study |
| [GraphRAG](https://github.com/microsoft/graphrag) | MIT | Python | LLM extracts a graph | Yes, heavy. "Can be expensive". | CLI | Not Go | NV | No | 36.7k stars. "Largely in maintenance mode". | Nondeterministic extraction |
| [LightRAG](https://github.com/HKUDS/LightRAG) | MIT | Python 3.11+ | One LLM call for each chunk | Yes | REST, UI, Python | Not Go | NV | An air-gap guide exists | 40.8k stars, 191 open issues | The server binds all interfaces with auth off by default. A new embedding model forces a full re-embed. |
| [Cognee](https://github.com/topoteretes/cognee) | Apache-2.0 | Python | Graph, vector and SQL stores | Optional | CLI, Python, REST, MCP | Not Go | NV | Models download on first run | 33.5k stars, v1.6.1 | 754 open issues |
| [Graphiti](https://github.com/getzep/graphiti) | Apache-2.0 | Python 3.10+ | LLM entity and edge extraction | Yes | Python library | Not Go | NV | Needs Neo4j, FalkorDB or Neptune | 33.0k stars | Small models give invalid JSON. |
| [mem0](https://github.com/mem0ai/mem0) | Apache-2.0 | Python, npm | LLM extraction | "Requires an LLM" | CLI, Python | Not Go | NV | No | 69.4k stars, 506 open issues | The benchmark numbers come from the managed platform. |
| [Letta](https://github.com/letta-ai/letta) | Apache-2.0 | NV | An agent runtime | NV | TUI, server, SDK | Not Go | NV | NV | 25.5k stars | The wrong category: it is not a document index. |
| [txtai](https://github.com/neuml/txtai) | Apache-2.0 | Python 3.11+ | Sparse and dense index plus graph | No by default | Python, FastAPI, MCP | Not Go (a Go binding for the API) | NV | Local | 13.7k stars | Heavy Python stack. No CLI seen. |
| [Bleve](https://github.com/blevesearch/bleve) | Apache-2.0 | Go | BM25 and tf-idf, kNN, hybrid RRF | No | Go library | Go-native | Yes | Local. Determinism NV. | 11.3k stars, 236 open issues | A library, not a CLI. It adds a `go.mod` entry. |
| [SQLite FTS5](https://sqlite.org/fts5.html) and [sqlite-vec](https://github.com/asg017/sqlite-vec) | Public domain; Apache-2.0 or MIT | C | `bm25()` built in. A `vec0` table for vectors. | No | Our own CLI | Through cgo or a Go binding | Yes | `bm25` is deterministic for a fixed corpus | sqlite-vec 8.3k stars, "pre-v1" | FTS5 needs a build flag in some builds. Tokens over 32,768 bytes are cut. |
| [Basic Memory](https://github.com/basicmachines-co/basic-memory) | AGPL-3.0 | Python 3.12+ | Markdown files plus a SQLite index | No documented | MCP and CLI | Not Go | NV | Telemetry and a 24-hour auto-update by default | 4.7k stars | AGPL is a license risk for embedding. |

**F4.7** Graph-RAG tools (GraphRAG, LightRAG, Graphiti, mem0) call an LLM for each chunk at ingest. mem0 "requires an LLM", and the GraphRAG README warns that indexing "can be an expensive operation". Our corpus is markdown and JSON in git with stable ids.

*Implication:* they break C2 (the bridge) and the deterministic gates. The corpus has structure that we already parse (`kbgraph`, peer study), so an LLM extraction adds cost and noise. Synthesis.

**F4.8** The remaining fits are a lexical index (FTS5, Bleve or our own BM25) with an optional vector layer. The peer study scored a native Go BM25 and RRF ranker highest and set qmd as a later comparator.

*Implication:* this dossier adopts that result. Section 7.2 gives the knowledge-base recommendation in one paragraph.

## 5. What retrieval helps an agent

Each item names its source and its limit.

**F5.1** Agentic search with grep and file reads was enough for one SWE-bench agent ([Augment write-up](https://jxnl.co/writing/2025/09/11/why-grep-beat-embeddings-in-our-swe-bench-agent-lessons-from-augment/)). The same write-up lists limits: it scales poorly to large code, handles natural language badly, and SWE-bench repositories are small. Anthropic's own lead said that agentic search "outperformed everything. By a lot" after an early RAG try. He gave "vibes" and some internal benchmarks as the evidence ([Latent Space](https://www.latent.space/p/claude-code)). This is vendor-stated and not published as a benchmark.

**F5.2** Dense retrieval wins less than it looks. A study of 91 repositories in six languages, Go included, found that much of the dense advantage comes from benchmark choices. Generate-then-grep (an LLM writes likely identifiers) lifts BM25 to match dense in every Python pool. No retriever predicts the winner for one query.

The authors advise a fusion of dense and BM25 ([arXiv 2611.20455](https://arxiv.org/abs/2611.20455)). The study measures retrieval, not task success. We read only the abstract.

**F5.3** In Agent Retrieval Bench (427 samples, 25 repositories) no retrieval family dominates. A repo map gave the best budgeted yield at 8K tokens. Agent logs missed every gold file on 27 to 35% of samples ([arXiv 2607.24882](https://arxiv.org/abs/2607.24882)). Abstract only.

**F5.4** Vendor results favor semantic search plus grep. Cursor reports +12.5% answer accuracy on average, and +2.6% code retention on repositories with 1,000 or more files ([Cursor](https://cursor.com/blog/semsearch)). A search-database vendor reports 65% against 87% file precision in a 50-task test, with the read window as a confound ([AI Engineer talk](https://ai.engineer/talks/zKk7sDMGDEQ-benchmarking-semantic-code-retrieval-on-claude)). Both are vendor data from sellers of an index.

**F5.5** Graph retrieval is not a clear win. A systematic study found that neither RAG nor GraphRAG dominates, and a combination helps ([arXiv 2502.11371](https://arxiv.org/abs/2502.11371)). A search summary of GraphRAG-Bench reports 60.92 for reranked vector RAG against 49.29 for Microsoft GraphRAG on fact retrieval (not opened; NV). For long-memory questions, grep gave higher accuracy than vector retrieval on a 116-question subset, and the result depends on the agent harness ([arXiv 2605.15184](https://arxiv.org/abs/2605.15184)). FalkorDB reports 71.48 against 55.39 for its own graph product (vendor).

**F5.6** Graph tools help structural questions more than tokens. This is the independent reading that our own spec records (`docs/architecture/code-index-cli.md`:511, P3.3). Our graphify bench shows the cost side: a graph with 82% import edges and 45% inferred call edges (F2.5).

*Implication of F5.1 to F5.6:* retrieval that is exact, cheap and local pays. A heavy graph with an LLM shows no clear gain. Synthesis: for a typed language such as Go, an exact symbol index plus fast lexical search covers most needs. The gains from embeddings are modest and mostly vendor-reported. They are largest on huge repositories with vague queries.

We have 0.55 million lines. We also have a measured fault (F3.4) that no study addresses: the callers answer of the current default is wrong 89% of the time.

## 6. The strategies, scored

### 6.1 Criteria

Each criterion scores from 1 (bad) to 5 (good). The sum has a maximum of 25.

| Criterion | What it measures |
|---|---|
| Fit | The constraints C1 to C9 of section 3.2. A break of C1, C2 or C4 scores 1 or 2. |
| Quality | The measured or expected answer quality for our needs (F3.4). A vendor claim scores 3 at most, because we did not measure it. |
| Cost | The host cost: runtime, memory, index time and disk. |
| Risk | The supply chain, the churn of the tool and the license. Low risk scores high. |
| Work | The effort to build, test and document. Low work scores high. |

### 6.2 Code navigation

| Id | Strategy | Fit | Quality | Cost | Risk | Work | Sum |
|---|---|---|---|---|---|---|---|
| S1 | No change: trail-code stays the default for all four verbs | 4 | 1 | 5 | 3 | 5 | 18 |
| S2 | Keep trail, and make gotypes the default for `callers` and `api` (item G4). trail keeps `find` and `map`. | 5 | 5 | 5 | 4 | 4 | 23 |
| S3 | Add graphify behind the `code_index` adapter as a third backend | 2 | 3 | 1 | 2 | 2 | 10 |
| S4 | Add CodeGraph or codebase-memory-mcp (CLI mode) as a third backend | 2 | 3 | 3 | 2 | 3 | 13 |
| S5 | Build natively: gotypes, plus a trigram search (a zoekt-style index) and a Go repo map. Replace trail. | 4 | 4 | 3 | 3 | 2 | 16 |
| S6 | S2 now, then S5 for `find` and `map` only if a gold-set gap shows | 5 | 5 | 4 | 4 | 3 | 21 |

The reasoning behind each row:

- **S1.** Fit 4: trail passes the rules with the wall of F3.7. Quality 1: callers precision 0.11 and recall 0.32 (F3.4). The tool is also a single-vendor package with fast churn (section 4a). Work 5: nothing to build.
- **S2.** Fit 5: no new tool, no new runtime, the adapter and the route exist (F3.1 to F3.3). Quality 5: 1.00 and 1.00 with a qualified name, and 0.044 s p50 (F3.4). Risk 4: gotypes is Go only and counts calls only, and trail still answers `find`, `map` and the other languages. Work 4: one route entry in policy data, a fallthrough rule and a bench table (C9).
- **S3.** Fit 2: a Python runtime with about 30 dependencies, an 80.9 s build, 1.86 GB of memory, no failover, and a PyPI typosquat risk. Quality 3: 0.93 and 0.75 with a retry, so below the type checker (F3.4). Cost 1: 3.5 s p50 for each query. The earlier study rejected it for the default route ([code-index-backends](code-index-backends-2026-10.md):318).
- **S4.** Fit 2: a new runtime or binary, and a `curl | bash` installer for one of them, and telemetry options. Quality 3: all numbers are vendor claims and none was measured on our code (F4.3). Risk 2: young tools with fast change, and a forced reindex on a format change.
- **S5.** Fit 4: Go only, but zoekt as a library enters `go.mod`, and ctags is GPL. Quality 4: expected, not measured. Cost 3: a trigram index of 2 to 3 times the corpus. Risk 3: we own the code, with the no-comment and 100% coverage rules. Work 2: a tokenizer, an index, a ranker, a repo map, tests and docs.
- **S6.** The same as S2 plus a gate. Cost and work are a little higher than S2. Its extra value is a stop rule for S5, so that we do not build S5 on a guess.

**The decisive difference.** S2 has the highest score (23 of 25). It is also the only strategy whose key number is a measurement on our own code, not a vendor claim. S6 is S2 with a gated next step. It scores 21 because of the extra work that it plans.

S2 and S6 do not conflict: stages 2 and 3 of section 7.3 are the S6 gate. S3 and S4 pay for a new runtime, and gotypes already beats their accuracy. S5 builds what we cannot yet show that we need.

### 6.3 Knowledge base

The peer study scored its approaches on the same five criteria. We map them here and add two that this dossier covers.

| Id | Strategy | Fit | Quality | Cost | Risk | Work | Sum |
|---|---|---|---|---|---|---|---|
| K1 | qmd, full pipeline, behind an adapter (peer A) | 1 | 4 | 1 | 2 | 3 | 11 |
| K2 | Native Go BM25 and RRF, then a gated vector comparator (peer D) | 5 | 4 | 4 | 4 | 3 | 20 |
| K3 | Graph RAG (GraphRAG, LightRAG, Graphiti, mem0) | 1 | 2 | 1 | 2 | 2 | 8 |
| K4 | Bleve as a library behind one adapter, BM25 and kNN | 4 | 3 | 4 | 3 | 3 | 17 |
| K5 | No change (peer C) | 5 | 1 | 5 | 4 | 5 | 20 |

K1, K2 and K5 take the peer scores. K3 scores 1 on fit. Each tool calls an LLM at ingest (F4.7), and it needs a graph database or a Python stack. K4 is a fair option. Bleve is Go-native with BM25, kNN and hybrid RRF (section 4d).

But it adds a `go.mod` library that needs one adapter package. The peer study found that the work is in the gold set and the weights, not in the ranker code.

K2 and K5 tie, and the peer study broke the tie on evidence. K2 turns a guess into a number with a gate. K5 has no evidence.

## 7. Recommendation, staged plan and risks

### 7.1 The recommendation

| Id | Decision | Because (findings) | Rejected options and why |
|---|---|---|---|
| D1 | For code navigation, take S2. Route `callers` and `api` to `gotypes`, keep trail for `find` and `map`. S2 depends on the answer to open question 6: the spec says "no failover", and S2 needs a fall to trail. The follow-up item "Fallthrough rule for an absent gotypes index" proposes a policy-data rule. | F3.4: 1.00 / 1.00 against 0.11 / 0.32. F3.3: the backend is built and idle. F4.4: no other compiler-accurate tool fits C3 and C5 at zero cost. | S1: leaves a known 89% error rate. S3, S4: a new runtime for a lower or unmeasured accuracy (F3.4, F4.3). |
| D2 | Do not adopt graphify, CodeGraph, codebase-memory-mcp, GitNexus, Potpie or CodeGraphContext in the route now. Keep them as bench-only comparators. | F4.1, F4.2: a new runtime and an agent-config wall for each. F4.3: no independent number. The GitNexus license is non-commercial. | S3, S4: see section 6.2. |
| D3 | Do not replace `find` and `map` yet. Measure first, with a gold query set. | F3.4: trail `find` is 19/40 top-1, which is weak. But F5.1 to F5.4 show no clear gain for an index over agent grep, and F3.6 shows that agents barely call the index. | S5 now: it builds on a guess. |
| D4 | For the knowledge base, take the peer recommendation (K2): a native BM25 and RRF ranker in shadow behind `research.KB`. Use qmd or Bleve only as a measured comparator. | F4.7: graph RAG breaks C2. F5.2, F5.5: BM25 and grep are competitive with dense retrieval, and no graph approach dominates. Peer study section 7. | K1, K3: break the bridge and isolation rules. K4 now: a library before a gold set. |
| D5 | Keep one `Backend` interface and one `research.KB` interface. Choose by policy data, never by a flag. | C3, C5, C9. The route pattern is in `docs/architecture/code-index-cli.md`:134. | A second path beside the first. |

Consequences of D1 to D5:

- Good: the callers error rate falls from 89% to about 0 on the truth set. There is no new dependency and no new runtime. A query is faster: 0.044 s p50 for the prototype, against 0.29 s for trail (F3.4).
- Bad: `callers` is Go only, host platform only, and counts calls only. A method value is not a call, and an interface call shows the interface, not the concrete target (F3.5). trail answers other languages.
- Bad: a route change is a policy change. A policy-gate edit needs the operator (memory rule: gate and config changes need the operator).

What changes our mind:

- If agent `grep` finds the right symbol for nearly all gold `find` queries, change nothing for `find` and `map`. Drop S5.
- If a second Go-only tool shows better precision on the independent check of stage 0, add it as a comparator, not as a replacement.
- If trail-code drops Go support or breaks the CLI exit codes again, route `find` and `map` to the native stage-3 build at once.

### 7.2 The recommendation in two lines

- **Code navigation.** S2 (23 of 25): `gotypes` for `callers` and `api`, trail-code for `find` and `map`. Measure a native `find` and `map` in shadow, and build it only if the gold set shows a gap (S6).
- **Knowledge base.** K2: the native BM25 and RRF ranker in shadow behind `research.KB`, with the gold set of the peer study. qmd stays a comparator. No graph RAG tool.

### 7.3 The staged plan

Each stage can stop. A stage starts when the earlier stage has its proof.

| Stage | Work | Proof of done |
|---|---|---|
| 0. Second truth | Add an independent check set for `callers`: 40 symbols that include interface methods, methods with a package-qualified call, and generic functions. Check 10 of them by hand or by the grep check of F2.19. | The set exists as data under `docs/`, with a count. The gotypes score on it is recorded. |
| 1. Shadow route | Set `code_index.backends` to `["gotypes", "trail"]` in policy data. Add the fallthrough of the G4 item: trail answers when no gotypes index exists. Run `evolve code bench`. Put its table in the pull request (C9). | The bench table. The usage log (`code-calls.ndjson`) shows the `backend` field on each call (item G5). |
| 2. Gold query set | Write 40 `find` queries in plain words with the expected symbol, and 15 `map` queries. Mark each with the wanted file. Take the first set from real agent shell history: `grep` and `rg` calls in cycle logs. Score trail, the `grep` baseline and, on a scratch host only, the comparators (CodeGraph, graphify `query`). | A table of top-1 and found-in-5 for each tool, and a latency column. |
| 3. Gate | Apply the flip criteria below. If `find` or `map` shows a gap, build a native version behind the same interface: a `go/ast` repo map and a trigram or BM25 `find`. | The flip record and a rollback rule. |
| 4. Persona text | Unblock item K4a, so that agents learn the commands. Measure the uptake in the usage log. | The call count of phase agents is above 0 for a week. |
| 5. Knowledge base | Follow the stages 0 to 4 of the peer study. | See the peer study. |

**Flip criteria** (synthesis; the thresholds are a proposal for the operator to set):

1. On both truth sets, `gotypes` callers reach 0.95 precision and 0.95 recall with a qualified name. They lose no hit of trail.
2. p95 latency of a `callers` query through `evolve code` is 0.2 s or less.
3. The conformance suite passes with the `gotypes`-first route on macOS and on Linux CI.
4. The boundary refresh of the gotypes index succeeded on each of the last 20 boundaries.
5. A native `find` or `map` beats trail by an operator margin on top-1 (trail today: 19 of 40). It finds at least as many in the top 5. Its p95 is 1.0 s or less.
6. A rollback rule exists. Any drop below the bar on a later bench returns the route to `["trail"]` at the next boundary.

### 7.4 Risks

| Id | Risk | Control |
|---|---|---|
| R1 | The truth is circular: gotypes and the truth use the same checker (F3.5). A shared fault is hidden. | Stage 0: an independent second set with interface methods, and a grep check. |
| R2 | gotypes sees the host platform only and misses cgo and `GOOS` files. | The fallthrough to trail for a not-found answer. A bench row for a build-tagged file. |
| R3 | Agents still do not call the index (F3.6). A better backend then changes nothing. | Stage 4: the persona text and a usage metric. Treat a call count of 0 as a failure of the plan. |
| R4 | trail-code churn or an upstream change of license or product. Signs: fast releases, an upgrade notice that broke an exit code, a push toward hooks and an LLM key. | Pin the version. Keep the adapter conformance suite. The native fallback of stage 3. |
| R5 | A tool prints an instruction to the agent, or writes outside its tree (F3.7). | Keep the strip filter and the dedicated index tree for every new tool. |
| R6 | A vendor claim is read as a fact. | Section 4 marks each such figure. Only a bench on our truth and gold sets moves a tool into the route. |
| R7 | Our `callers` answer is a static graph: no runtime graph, no concrete target of an interface call. | State the limit in the output text. Keep CHA or VTA as an open question (plan Q4). |

### 7.5 Open questions for the operator

1. Do you accept the flip criteria of section 7.3, such as 0.95 on the truth sets and 0.2 s p95?
2. May a scratch host outside the repository run CodeGraph or graphify for stage 2?
3. Is `gopls` allowed as a stage 0 checker only, not in the route? It needs `go` and can contact the module proxy.
4. Do you want a native `find` and `map`, or is `grep` through the shell enough?
5. Who writes the gold query sets: you, or a bridge seat with you checking a sample?
6. How must the route fall through when the gotypes index is absent or stale (exit 2 or 21)? The plan says "fall to trail". The spec says "no failover". The two must be reconciled by a rule in policy data.

### 7.6 Follow-up items

| Title | Acceptance |
|---|---|
| Default route `callers` and `api` to gotypes (G4) | `code_index.backends` is `["gotypes","trail"]` in policy data. A bench table is in the pull request. |
| Second truth set for the code index | A data file with 40 symbols, interface and qualified-call cases, and the gotypes score. |
| Gold query set for `find` and `map` | 40 `find` and 15 `map` queries with wanted files, stored as data. |
| Bench comparators for CodeGraph and graphify `query` | `evolve code bench` prints their rows, run on a scratch host. No adapter lands. |
| Fallthrough rule for an absent gotypes index | A conformance test shows that `callers` falls to trail, with a visible warning. |
| Persona text for the code commands (K4a) | Scout, builder and debugger texts name the verbs. The usage log shows phase calls. |
| Native `find` and `map` decision record | A dated record that cites the stage 2 table and chooses build or stop. |
| Knowledge-base shadow ranker | The stages of the peer study 0 and 1 are done. |

### 7.7 Limits of this research

- We did not install or run any tool. Every outside number is a claim of the authors or of a third party.
- No tool has a timing on a tree of our size. The "1M lines" figures of vendors do not apply to 0.55 million lines.
- We did not verify these points:
  - Go call resolution of graphify, CodeGraph and codebase-memory-mcp;
  - graphify issue #857, and the build method of Potpie;
  - Kythe storage and incrementality, and the incremental mode of scip-go;
  - the last commit of zoekt, and the start-up cost of the gopls CLI;
  - the full text of three 2026 arXiv papers (abstracts only).
- Several paper figures came from search summaries and carry "NV" in section 5.
- The earlier bench ran on a loaded host (load average 12 to 22), so its times are an upper bound.
- trail-code star counts and issue counts differ between its README page and the API. We use the API (11,457 stars, 271 open issues).

## 8. Sources

Outside sources, read on 2026-11-23. Cells that say "API" were re-read from the GitHub API on that date.

Code indexes and graphs:

- trail-code repository: <https://github.com/trailhq/Trail-Code> ; API: <https://api.github.com/repos/trailhq/Trail-Code> ; npm: <https://registry.npmjs.org/@trailhq/trail-code>
- graphify repository: <https://github.com/Graphify-Labs/graphify> ; API: <https://api.github.com/repos/Graphify-Labs/graphify> ; docs: <https://docs.graphify.com/> ; a third-party review: <https://ai.dosa.dev/blog/code-knowledge-graph-tools-ai-coding-agents>
- CodeGraph: <https://github.com/colbymchenry/codegraph>
- codebase-memory-mcp: <https://github.com/DeusData/codebase-memory-mcp>
- CodeGraphContext: <https://github.com/CodeGraphContext/CodeGraphContext>
- code-graph-rag: <https://github.com/vitali87/code-graph-rag>
- GitNexus: <https://github.com/abhigyanpatwari/GitNexus>
- Potpie: <https://github.com/potpie-ai/potpie>
- Kythe: <https://github.com/kythe/kythe>
- SCIP and scip-go: <https://github.com/sourcegraph/scip> ; <https://github.com/scip-code/scip-go>
- gopls: <https://pkg.go.dev/golang.org/x/tools/gopls> ; CLI doc: <https://github.com/golang/tools/blob/master/gopls/doc/command-line.md>

Search and packing tools:

- zoekt: <https://github.com/sourcegraph/zoekt> ; memory note: <https://sourcegraph.com/blog/zoekt-memory-optimizations-for-sourcegraph-cloud/>
- probe: <https://github.com/buger/probe>
- ast-grep: <https://github.com/ast-grep/ast-grep>
- universal-ctags: <https://github.com/universal-ctags/ctags>
- aider repo map: <https://aider.chat/docs/repomap.html>
- repomix: <https://github.com/yamadashy/repomix> ; code2prompt: <https://github.com/mufeedvh/code2prompt>

Knowledge-base and memory tools:

- qmd: <https://github.com/tobi/qmd> (the peer study has the detail)
- GraphRAG: <https://github.com/microsoft/graphrag>
- LightRAG: <https://github.com/HKUDS/LightRAG>
- Cognee: <https://github.com/topoteretes/cognee>
- Graphiti: <https://github.com/getzep/graphiti>
- mem0: <https://github.com/mem0ai/mem0>
- Letta: <https://github.com/letta-ai/letta>
- txtai: <https://github.com/neuml/txtai>
- Bleve: <https://github.com/blevesearch/bleve>
- SQLite FTS5: <https://sqlite.org/fts5.html> ; sqlite-vec: <https://github.com/asg017/sqlite-vec>
- Basic Memory: <https://github.com/basicmachines-co/basic-memory>

Evidence on retrieval:

- Augment on SWE-bench: <https://jxnl.co/writing/2025/09/11/why-grep-beat-embeddings-in-our-swe-bench-agent-lessons-from-augment/>
- Claude Code on Latent Space: <https://www.latent.space/p/claude-code>
- Embeddings against grep: <https://arxiv.org/abs/2611.20455>
- Agent Retrieval Bench: <https://arxiv.org/abs/2607.24882>
- RAG against GraphRAG: <https://arxiv.org/abs/2502.11371>
- Is grep all you need: <https://arxiv.org/abs/2605.15184>
- Cursor semantic search: <https://cursor.com/blog/semsearch>
- AI Engineer talk (turbopuffer): <https://ai.engineer/talks/zKk7sDMGDEQ-benchmarking-semantic-code-retrieval-on-claude>

Local sources are cited as `path:line` in the text. The repository tree is `origin/main` at `2c8fc5fe9` (2026-11-23). The knowledge-base peer study is on the branch `console/research-qmd-knowledge-base`.
