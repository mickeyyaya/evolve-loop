# qmd and the knowledge base: adopt the tool, borrow the ideas, or change nothing (research, 2026-11)

> The related work: [incident-knowledge-base-2026-11](incident-knowledge-base-2026-11.md) (the records and the graph), [code-index-backends-2026-10](code-index-backends-2026-10.md) (the backend pattern) and [cell-context-index-2026-10](cell-context-index-2026-10.md) (the earlier rejection of a vector store). No plan exists yet. Section 7 gives a staged plan that the operator can turn into one.

Date: 2026-11-23. Method:

- a live read of the qmd repository: README, manifest, releases, commits and issues (links in section 9);
- a read of our code, docs and corpus in the `origin/main` tree of 2026-11-23, with `path:line` for each claim;
- a count of the corpus with `ls`, `wc` and `du`.

We did not install or run qmd. Each qmd number is a claim of the qmd authors. It is not a measurement of ours. "Synthesis" marks our own inference. "Not verified" marks a claim that we cannot check.

## Table of contents

1. [Request](#1-request)
2. [Terms used throughout](#2-terms-used-throughout)
3. [What qmd is](#3-what-qmd-is)
4. [Where our knowledge base needs retrieval](#4-where-our-knowledge-base-needs-retrieval)
5. [The rules that limit an adoption](#5-the-rules-that-limit-an-adoption)
6. [The approaches, scored](#6-the-approaches-scored)
7. [Recommendation and staged plan](#7-recommendation-and-staged-plan)
8. [Open questions for the operator](#8-open-questions-for-the-operator)
9. [Sources](#9-sources)

## 1. Request

The console asked for a decision on how, and if, evolve-loop can apply `tobi/qmd` or its ideas to the knowledge-base design. The current architecture must stay. Three approaches are the minimum: adopt qmd as an external CLI behind an adapter, borrow the ideas in native Go, and change nothing.

## 2. Terms used throughout

| Term | Meaning |
|---|---|
| BM25 | A ranking function for text. It scores a term by its rarity (IDF) and its frequency, and it corrects for document length. |
| FTS5 | The full-text search module of SQLite. It gives BM25 ranking. |
| Vector search | A search by the distance between embeddings. An embedding model maps text to a list of numbers. |
| RRF | Reciprocal rank fusion. It merges ranked lists. A document scores the sum of `1/(k+rank)` over the lists. |
| Rerank | A second pass in which a model re-scores the top candidates. |
| Query expansion | A model writes extra queries (keyword and semantic forms) from the user query. |
| Recall at k | The part of the wanted documents that are in the top k results. |
| MRR | Mean reciprocal rank. It is the mean of `1/rank` of the first wanted result. |
| The bridge | `evolve agent run` and the bridge package. It is the one path by which an LLM runs. |
| Lesson recall | The call that gives the orchestrator the lessons for the next plan (F4.1). |

## 3. What qmd is

**P3.1** qmd is "an on-device search engine for everything you need to remember". It is a CLI for markdown notes, transcripts, docs and knowledge bases, and it also has an MCP server. The license is MIT ([README](https://github.com/tobi/qmd), [package.json](https://raw.githubusercontent.com/tobi/qmd/main/package.json)).
*Implication:* the license allows use. The MCP server does not count for us (section 5).

**P3.2** The full query has these steps ([README](https://github.com/tobi/qmd)):

1. A query-expansion model writes typed sub-queries: `lex` (keywords, to BM25), `vec` and `hyde` (to vector search).
2. The original query goes to both backends with a weight of 2.
3. BM25 runs on SQLite FTS5. Vector search uses cosine distance.
4. RRF merges the lists with `k=60`. It adds +0.05 for rank 1 and +0.02 for ranks 2 and 3.
5. The top 30 go to an LLM reranker. A blend sets the score: 75/25 (retrieval/reranker) for ranks 1 to 3, 60/40 for 4 to 10, 40/60 below.

Three commands expose the tiers: `search` (BM25 only), `vsearch` (vectors only) and `query` (the full pipeline). `--no-rerank` skips the reranker.
*Implication:* the BM25 tier needs no model. The vector tier needs an embedding model. Only the full tier needs generative models.

**P3.3** The models run locally through `node-llama-cpp` as GGUF files, which download from Hugging Face on first use to `~/.cache/qmd/models/` ([README](https://github.com/tobi/qmd), [search summary](https://unpkg.com/@tobilu/qmd@latest/README.md)).

| Role | Model | Size |
|---|---|---|
| Embedding | embeddinggemma-300M-Q8_0 | about 300 MB |
| Rerank | qwen3-reranker-0.6b-q8_0 | about 640 MB |
| Query expansion | qmd-query-expansion-1.7B-q4_k_m (fine-tuned) | about 1.1 GB |

The total is about 2.0 GB of model files (synthesis: the sum of the three sizes). The models stay in VRAM across requests.

The contexts are disposed after 5 minutes of idle time, and a rebuild costs about 1 second. The README gives no latency benchmark, no RAM figure and no first-run time ([README](https://raw.githubusercontent.com/tobi/qmd/main/README.md)).
*Implication:* the cost per query is not documented. We must measure it ourselves. Not verified: the RAM use.

**P3.4** Storage is one SQLite file, `~/.cache/qmd/index.sqlite`. Project-local indexes live in `.qmd/`. The tables are `collections`, `path_contexts`, `documents`, `documents_fts` (FTS5), `content_vectors`, `vectors_by_collection` (sqlite-vec) and `llm_cache` ([README](https://github.com/tobi/qmd)). The SDK needs an explicit `dbPath`.
*Implication:* the index is a derived file. We can place it under `.evolve/` and rebuild it from the markdown.

**P3.5** Chunking: about 900 tokens with 15 percent overlap. The cut point is scored by structure: H1 100 down to H6 50, code fence 80, horizontal rule 60, blank line 20. Code blocks stay whole. `--chunk-strategy auto` adds tree-sitter cuts for TypeScript, JavaScript, Python, Go and Rust ([README](https://github.com/tobi/qmd)).
*Implication:* the heading-aware cut suits our dossiers, which use `## ` sections and tables.

**P3.6** The CLI has `collection add|list|remove`, `context add|list|rm`, `update`, `embed`, `status`, `search`, `vsearch`, `query`, `get`, `multi-get`, `bench`, `doctor` and `trust`. `--format json` gives `docid`, `score`, a `qmd://collection/path` file URI, a snippet and the context. `--explain` adds the per-list scores and the RRF terms. A collection is a directory with a glob, and a context is a text note for a path prefix ([README](https://github.com/tobi/qmd)). `qmd bench` reports precision at k, recall, MRR and F1 for the bm25, vector, hybrid and full modes.
*Implication:* the JSON output and `bench` fit a shadow measurement.

**P3.7** The runtime is Node 22 or later, or Bun 1.0 or later. The manifest lists 13 dependencies. Three are native: `better-sqlite3`, `node-llama-cpp` and `sqlite-vec`, with platform packages for macOS and Linux ([package.json](https://raw.githubusercontent.com/tobi/qmd/main/package.json)). macOS needs Homebrew SQLite for extensions ([README](https://github.com/tobi/qmd)).
*Implication:* qmd is a Node tool with native add-ons. We already run the trail-code binary, which is also Node (the third-party convention names it, `docs/conventions/third-party-dependencies.md:13`).

**P3.8** Maturity and activity:

- The repository shows 31.4k stars, 811 commits, 135 open issues and 133 open pull requests ([repository](https://github.com/tobi/qmd)).
- The releases run from v1.1.5 (2026-03-07) to v2.9.0 (2026-11-17), with a stable SDK in v2.0.0 ([releases](https://github.com/tobi/qmd/releases)).
- The last commits are from 2026-11-15 to 2026-11-20. They fix embedding batches, lock tests and daemon races ([commits](https://github.com/tobi/qmd/commits/main)).
- Since v2.5.1 the npm release is published from GitHub Actions with npm Trusted Publishing (OIDC), not a long-lived token ([releases](https://github.com/tobi/qmd/releases)).

*Implication:* the project is active and popular, but it changes fast. Eight months moved it from 1.1 to 2.9.

**P3.9** Open issues that touch our use ([issues](https://github.com/tobi/qmd/issues)):

| Issue | Meaning for us |
|---|---|
| #1088: after the 5-minute idle dispose, queries take 5 to 27 minutes on the CPU backend | A CPU-only host can stall. We run one process for each call, so the model must load each time. Not verified: our host. |
| #1133: read-only commands take a write lock while `qmd update` runs | Search blocks during an update. We must not update while an agent searches. |
| #1078: a changed document is embedded again in full | A dossier edit pays the full embed cost of that file. |
| #1102: chunking time grows quadratically with break points | Large files cost more than linear time. |
| #1122 and #1077: project-local config can be trusted implicitly, and `mcp` and `bench` bypass the trust gate | The trust gate is young. We must not rely on it. |

**P3.10** Security facts:

- The models download from Hugging Face on first use. The README does not document an offline mode or a variable to switch it off ([README](https://raw.githubusercontent.com/tobi/qmd/main/README.md)).
- A project-local `.qmd/index.yml` can carry `update` commands and custom model URIs. The README says it "travels with a `git clone`". Those fields need `qmd trust` approval, and they are skipped in a non-interactive run ([README](https://raw.githubusercontent.com/tobi/qmd/main/README.md)).
- A collection update hook runs through `bash -c` in the collection directory, and a non-zero exit aborts the run (same source).
- The v2.8.3 release added the trust gate and a check of the `Origin` and `Host` headers for the HTTP server ([releases](https://github.com/tobi/qmd/releases)). The HTTP endpoints have no authentication.
- A Socket.dev scan of the package was not readable (HTTP 403). Not verified: the supply-chain score.

*Implication:* the first model download is a network step that we cannot pre-approve in a hermetic cell. The native add-ons and 13 dependencies widen the supply chain. We can control both with a pre-staged cache and a pinned version, but that is work.

## 4. Where our knowledge base needs retrieval

**F4.1** Lesson recall is the one retrieval path that the Go controller runs. `recallForPlan` builds a query from the last failure summary, its class and the goal words, and calls `kb.Lookup` (`go/internal/core/routing_dispatch.go:185-197`).

The orchestrator holds the `research.KB` interface and gets a `FileKB` (`go/cmd/evolve/cmd_cycle.go:667`). The result is a list of one-line digests for the plan.
*Implication:* this call is deterministic Go. It runs before any LLM. It is the main place where a better ranker changes the outcome.

**F4.2** `FileKB.Lookup` has no index.

For each call it lists the files under each root, parses each one, and scores each lesson (`go/internal/research/filekb.go:29-70`). The score is a count of query tokens that are in the lesson text, plus 2 for a matching failed step, times the confidence (`go/internal/research/filekb.go:72-93`).

The tokenizer keeps lowercase letters and digits of 3 or more characters (`go/internal/research/kb.go:62-77`). The score has no IDF, no length correction, no stemming and no phrase match. 

It returns the top 5 by default (`go/internal/research/filekb.go:22`). The orchestrator passes a recall `k` from policy data (`go/cmd/evolve/cmd_cycle.go:667`), so the live `k` can differ.
*Implication:* a common word counts as much as a rare one. Synthesis: a long lesson that repeats common words can outrank a short exact match. We have no measurement of this yet.

**F4.3** The default roots are `knowledge-base/research/`, `.evolve/instincts/lessons/`, `docs/research/`, `docs/retrospectives/` and `docs/incidents/` (`go/internal/research/kb.go:58`). `Lookup` reads files that parse as lessons. The package has named tests for incident files, and it skips legacy records and index files (`go/internal/research/filekb.go`, test `TestFileKB_RecallsIncidentLessonAndSkipsLegacyAndIndex`).
*Implication:* a dossier without lesson fields is not a result. Most of `docs/research/` is outside the recall today. Synthesis from the parse rule; we did not run a probe.

**F4.4** The corpus today in the `origin/main` tree (commands `ls`, `du`):

| Store | Size | Used by |
|---|---|---|
| `docs/research/` | 182 entries, 6.7 MB | Read by hand, and by the recall roots |
| `docs/incidents/` | 129 entries, 1.4 MB | `incidentkb` and the graph |
| `docs/retrospectives/` | 9 entries | The recall roots |
| `knowledge-base/cycles/` | 3,560 files (a `.json` and an `.md` for each cycle), 24 MB | `evolve kb graph` |
| `docs/research-index/` | 99 dated entries, rendered into `docs/research-index.md` (938 lines) | People and agents that scan the index |

*Implication:* the whole text corpus is about 32 MB. SQLite, a Go loop or `grep` can serve it. It is small, so the cost of an index is low and the gain must come from rank quality, not from speed.

**F4.5** The graph, `evolve kb graph neighbors|recurrences`, links cycles, fingerprints, defects, lessons, items and incident records (`go/internal/kbgraph/kbgraph.go:77`, `go/cmd/evolve/cmd_kb.go:15-16`). `incidentkb` parses incident records with a typed schema and has `List` and `Check` (`go/internal/incidentkb/yaml.go:29`). Both are exact lookups by id or by fingerprint.
*Implication:* a known fingerprint does not need a ranker. The need is for a free-text question ("have we seen a stalled lane after a rebase?") that has no key.

**F4.6** The code index is `evolve code callers|api|find|map` behind the `codeindex.Backend` interface (`go/internal/codeindex/backend.go:36`). The design says that `find` is the query where "a text or embedding search is better ... when the agent knows only some words" (`docs/architecture/code-index-cli.md:148`). The earlier research rejected a vector store for cell bodies, because a cell has tens of files (`docs/research/cell-context-index-2026-10.md:117`).
*Implication:* the pattern for an outside engine exists, and the rejection was for one narrow case. It does not cover the knowledge base.

**F4.7** The older research-tool design says to "try `kb-search.sh` first" (`docs/architecture/research-tool.md:35`). The script is in `legacy/`, and the same document says that its hook was removed (`docs/architecture/research-tool.md:58`). The default path variable in that document still names three roots.
*Implication:* agent research recall is a mix of an old script, direct file reads and the research index. Synthesis: nothing ranks across the whole corpus for an agent.

**F4.8** Summary of the needs and today's method:

| Need | Today | Gap |
|---|---|---|
| Lesson recall for the plan | Token overlap, top 5 (F4.2) | No IDF, no length correction |
| Incident recall by fingerprint | Exact graph lookup (F4.5) | None for the exact case |
| Incident recall by symptom text | Lessons path only, if the file parses (F4.3) | Free text over the records |
| Research recall | Index scan, `grep`, file reads (F4.7) | No ranked search over 182 dossiers |
| Cycle dossier recall | Graph by id (F4.5) | No text search over 1,780 dossiers |
| Code `find` | trail-code (F4.6) | Out of scope for this dossier |

## 5. The rules that limit an adoption

| Rule | Source | Effect on qmd |
|---|---|---|
| No LLM starts another LLM. Every LLM query goes through the bridge. | `AGENTS.md` hard rules; `docs/operations/operating-policy.md` | The query-expansion and rerank models of qmd are LLMs that run outside the bridge. Only the BM25 and embedding tiers are clear of the rule. The reading of "LLM query" for an embedding model is a question for the operator (section 8). |
| CLI only, never MCP. | `docs/conventions/cli-only-no-mcp.md` | Use `qmd search|vsearch|query --format json` only. Never `qmd mcp`. |
| The Go binary is the only controller. | `AGENTS.md` | The adapter runs qmd. A phase agent never starts it by hand. |
| One adapter package for each outside library or binary. | `docs/conventions/third-party-dependencies.md:7` | One package, as `adapters/codeindex/trail` does. |
| Isolation: a hermetic environment and pipeline-composed inputs. | `docs/architecture/llm-isolation.md` (the builder `cellEnv` in `go/internal/bridge/hermetic_env.go`) | qmd reads `~/.cache` and `~/.config` by default. The adapter must set the cache and config paths and pre-stage the models. A cell must not trigger a download. |
| No environment control values. No feature flags. | `AGENTS.md` | Choose the backend with policy data, as `code_index.backends` does (`docs/architecture/code-index-cli.md:134`). |
| One home for each belief. | `AGENTS.md` | One ranker for lesson recall. A second index of the same files needs a derived, rebuildable file with one writer. |

## 6. The approaches, scored

Each criterion scores from 1 (bad) to 5 (good). The criteria are fit with the rules of section 5, cost (host and upkeep), expected gain for our needs (F4.8), risk and work. Low risk and low work score high.

| Approach | Fit | Cost | Gain | Risk | Work | Sum |
|---|---|---|---|---|---|---|
| A. qmd as an external CLI behind an adapter (full pipeline) | 1 | 1 | 4 | 2 | 3 | 11 |
| A2. qmd as an adapter, lexical and vector tiers only | 3 | 2 | 4 | 3 | 3 | 15 |
| B. Native Go: BM25 and RRF over our own index | 5 | 5 | 3 | 4 | 3 | 20 |
| C. No change | 5 | 5 | 1 | 4 | 5 | 20 |
| D. B first, with a measured gate, then A2 only if the gate shows a gap | 5 | 4 | 4 | 4 | 3 | 20 |

### A. Adopt qmd, full pipeline

- **Fit 1.** Query expansion and rerank are local LLMs that run outside the bridge (P3.2, section 5). A 2 GB model cache, a first-use download from Hugging Face and a Node runtime break the hermetic rule. Much staging work is the only fix. Synthesis.
- **Cost 1.** About 2.0 GB of models (P3.3), a Node 22 runtime, three native add-ons (P3.7), and a model load for each CLI call. A daemon avoids the load, but it is an MCP or HTTP server, and we do not use one.
- **Gain 4.** The best quality in the survey: hybrid plus rerank. Not verified on our corpus.
- **Risk 2.** Fast change (P3.8), open issues on idle stalls and locks (P3.9), a young trust gate (P3.10).
- **Work 3.** An adapter package, a pre-staged model cache, a pinned version, an index lifecycle and a conformance suite.

### A2. Adopt qmd, lexical and vector tiers only

The adapter calls `qmd search` and `qmd vsearch`, never `qmd query`. The Go side fuses the two lists with RRF. No generative model runs.

- **Fit 3.** The generative-model objection is gone. The embedding model remains, and the Node, download and isolation points of A remain.
- **Cost 2.** About 300 MB of model, plus Node and the native add-ons.
- **Gain 4.** Vector search adds meaning matches that BM25 cannot find. Synthesis: for a small English corpus of dossiers with technical terms, the gain over a good BM25 is unknown until measured.
- **Risk 3.** The same supply-chain points, a smaller surface.
- **Work 3.** As in A, less the rerank staging.

### B. Borrow the ideas in native Go

The repository has two Go libraries (`gopkg.in/yaml.v3` and `pgregory.net/rapid`, `go/go.mod:5-7`). Neither gives text search. So B uses the standard library only.

- **What B builds.** A derived index file under `.evolve/` (rebuildable, one writer). It has a BM25 index over fields, with the heading-aware chunking of P3.5. It has RRF (`k=60`) to merge the lists of lexical match and of graph neighbors from `kbgraph`. It keeps the `research.KB` interface, so the orchestrator does not change (`go/internal/core/orchestrator.go:218`).
- **Fit 5.** Go only, deterministic, no LLM, no new library, no download. It sits behind an interface that exists.
- **Cost 5.** CPU and a small file. The corpus is 32 MB (F4.4).
- **Gain 3.** IDF and length correction fix the main fault of F4.2. Fields (a symptom field weighs more than a body) and the graph add precision. There are no vectors, so a query with no shared words still fails.
- **Risk 4.** We own the ranker. A bad weight is a quiet quality loss, so the shadow gate in section 7 is needed.
- **Work 3.** A tokenizer with a stemmer or a prefix rule, the index, the chunker, RRF, tests, the gate and the docs.

BM25 and RRF are public, short algorithms, so the port is small. Synthesis: the effort is in the measurement set and the weights, not in the code.

### C. No change

- **Fit 5, cost 5, work 5.** Nothing moves.
- **Gain 1.** The fault of F4.2 stays. Recall is also limited to files that parse as lessons (F4.3), and 182 dossiers and 1,780 cycle dossiers stay out of a ranked search (F4.8).
- **Risk 4.** No new risk, but a known weakness remains, and we have no number for its size.

C is right if the measurement of section 7 shows that today's recall finds the wanted lesson in the top 5 on nearly all cases.

### D. B first, then qmd as a measured comparator

D is B plus a measurement set. The set comes first, and a decision on vectors follows from it. Each stage can stop. The operator can run qmd on a scratch host, as a comparator only. It never enters the pipeline until it wins on our set and the open questions of section 8 are answered.

- **Fit 5.** Same as B, with the adapter path kept open behind the same interface.
- **Cost 4.** The measurement set needs a few hours of labelling.
- **Gain 4.** B's gain, and a proof of the extra gain from vectors.
- **Risk 4.** The gate controls the main risk.
- **Work 3.** B, plus the set and the comparison.

### The decisive difference

B, C and D tie on the sum, so the sum does not decide. Gain and evidence decide.

C has no evidence. B fixes a known fault, but its gain is a guess. D is B with a gate that turns the guess into a number. A and A2 break the bridge and isolation rules, or add cost for a gain that nobody measured. So D wins.

### Worked example

The orchestrator plans after a failure. The summary reads "audit failed: lane stalled after a rebase" and the class is `code-audit-fail`. The terms are `audit`, `failed`, `lane`, `stalled`, `after`, `rebase` and the class tokens (`go/internal/research/kb.go:16-30`). Today each matched term adds 1, whatever its rarity (`go/internal/research/filekb.go:80-82`).

So `audit` counts as much as `stalled`. With BM25, `stalled` and `rebase` are rare in the corpus and carry most of the score. A lesson about a stalled lane after a rebase moves up.

The graph list adds the lessons of past cycles with the same class. RRF merges the two lists. Synthesis: we have not run this. Stage 0 of the plan tests it.

## 7. Recommendation and staged plan

**Recommendation: approach D.** Keep the architecture. Build a measurement set first. Then build BM25 and RRF in Go behind `research.KB`, in shadow. Recall in the pipeline stays lexical (operator answer to Q1). Do not adopt qmd now.

| Id | Decision | Because | Rejected and why |
|---|---|---|---|
| D1 | Do not adopt qmd in the pipeline now. | F4.1: the main recall path runs in the controller before any LLM. P3.2, section 5: the generative tiers run outside the bridge. P3.3: about 2.0 GB of models. | A: breaks the bridge and isolation rules. |
| D2 | Fix the ranker in Go first. | F4.2: the score has no IDF and no length correction. The corpus is small (F4.4). | C: leaves the known fault. |
| D3 | Measure before any vector step. | F4.8: the gap is free-text recall, and we have no gold set. P3.3 gives no latency figure. | A2: closed by the operator answer to Q1. |
| D4 | Keep one `research.KB` interface and choose the ranker by policy data. | Section 5: no feature flag, one home for each belief. `docs/architecture/code-index-cli.md:134` shows the pattern. | A second recall path beside the first. |

Stages, each with its proof:

| Stage | Work | Proof of done |
|---|---|---|
| 0. Gold set | Write 40 to 60 queries from past plans with the lessons, incident records and dossiers that a bridge seat marks as wanted (answer to Q4). Take the first set from `evolve kb graph recurrences`, where a repeat failure shows a lesson that was needed. Store the set as data under `docs/`. | The set exists, with a count. Today's `FileKB` has a measured recall at 5 and MRR on it. |
| 1. Shadow ranker | Add a BM25 and RRF ranker in a new package. It reads the same roots as `FileKB`. The orchestrator runs it beside `FileKB`, logs both lists, and uses only the `FileKB` result. | The log has both lists for each plan. Recall at 5 and MRR for each ranker on the set. |
| 2. Decide the switch | A data rule: the new ranker replaces the old when it wins recall at 5 by an operator margin and loses no old hit. A boundary applies it. | The flip record, and a rollback on regression. |
| 3. Widen the corpus | Add dossiers, incident records and cycle dossiers as chunked, field-weighted entries. Add `evolve kb search <text>` as a CLI verb, so that an agent and a person use the same ranker. | The verb has JSON output and a conformance test. |
| 4. Scratch-host comparator, optional | Only if stage 2 shows a gap on queries with no shared words. On a scratch host only, run `qmd search`, `qmd vsearch` and `qmd bench` on the same set. Pin the versions and set telemetry off. This is a benchmark only. No adapter path exists while Q1 stands. Item: `scratch-host-comparator-bench`. | A table of recall at 5 and MRR, and the latency and RAM that we measure. The operator decides. |

What changes our mind: if stage 0 shows that recall at 5 is already high (the operator sets the bar), choose C.

If stage 4 shows a large gain from vectors, the result is a finding for the operator only. Do not build an adapter while the answer to Q1 stands. A new operator decision on Q1 must come first.

Limits of this research: we did not run qmd, so its quality, speed and RAM on our corpus are not measured. We did not verify the supply-chain score. We have no gold set yet, so the gain of B is an estimate. The README figures are the claims of the authors.

Consequences of D:

- Good: a fix for a known ranking fault with no new library, no model and no network step. A number for the gain.
- Cost: a few hours of labelling, and a ranker that we own and must keep.
- Risk: a poor gold set misleads the decision. The set grows from real misses. The bridge seat labels the set with no human check (operator answer to Q4), so the measurement rests on the seat's judgement.

## 8. Open questions for the operator

1. Is an embedding model an "LLM query" under the bridge rule? If yes, only lexical ranking is allowed in the pipeline, and A2 is closed. If no, A2 stays open but the isolation work remains.
2. May the hermetic cell read a pre-staged local model cache? The cache is about 300 MB for embeddings only.
3. What recall at 5 on the gold set is "good enough" to choose C?
4. Who labels the gold set? The operator, or a bridge seat with the operator checking a sample?
5. If stage 4 wins, do you accept Node 22 and three native add-ons on the host, as for trail-code?
6. Is a scratch-host run of qmd for stage 4 allowed, with the model download, outside the repository?

### Operator answers (2026-11-23)

- **Q1: closed.** An embedding model is an LLM query under the bridge rule. Recall inside the pipeline stays lexical (BM25 and RRF). Approach A2 is closed.
- **Q2: moot.** Q1 closed A2, so no model runs in the hermetic cell and no model cache is needed. The question reopens only if Q1 changes.
- **Q3: still open.** The operator has not set the recall at 5 bar that makes C good enough. Stage 0 gives the first number to judge.
- **Q4: answered.** A bridge seat labels the gold set. No human checks a sample, at the operator's choice. Consequence: the measurement rests on the seat's judgement. A labelling fault in the seat reaches every recall figure, and nothing in the process finds it.
- **Q5: moot.** Q1 closed A2 and stage 4 has no adapter path, so the host needs no Node 22 and no native add-ons for the pipeline.
- **Q6: answered.** qmd can run on a scratch host only. The run is a benchmark only, with pinned versions and telemetry off. Item: `scratch-host-comparator-bench`.

## 9. Sources

Outside sources (read on 2026-11-23):

- qmd repository and README: <https://github.com/tobi/qmd>
- README raw text: <https://raw.githubusercontent.com/tobi/qmd/main/README.md>
- Package manifest (version 2.9.0, MIT, Node 22): <https://raw.githubusercontent.com/tobi/qmd/main/package.json>
- Releases: <https://github.com/tobi/qmd/releases>
- Commits: <https://github.com/tobi/qmd/commits/main>
- Open issues: <https://github.com/tobi/qmd/issues>
- Published README on npm (search result): <https://unpkg.com/@tobilu/qmd@latest/README.md>
- Changelog (HTTP 503 at read time, not read): <https://github.com/tobi/qmd/blob/main/CHANGELOG.md>
- Socket.dev package page (HTTP 403, not read): <https://socket.dev/npm/package/@tobilu/qmd>

Local sources are cited as `path:line` in the text. The repository tree is `origin/main` of 2026-11-23.
