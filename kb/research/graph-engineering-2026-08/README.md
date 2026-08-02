# Graph Engineering — what the concept teaches this project, and what to merge

> Research date: 2026-08-02 (console session). Sources at the bottom.
> Companion visual explainer: [`graph-engineering.html`](graph-engineering.html).

## 1. What "graph engineering" means in 2026

The term has converged on a specific practice: **represent an AI application as an
executable graph** whose nodes are agents, tools, deterministic functions,
validators, and human decision points, and whose edges are state transitions,
validation gates, recovery paths, and control boundaries. Design effort goes into
the *edges* — who may hand to whom, what gets checked at each crossing, where a
failure routes — not into ever-larger prompts.

Three load-bearing sub-ideas:

1. **Deterministic/semantic routing split** — route by code when categories are
   exact; spend an LLM only when classification genuinely needs judgment.
2. **Validators as first-class nodes** — gates are graph nodes with defined
   inputs, not prompt suggestions; a compiled graph is immutable per run.
3. **Graph-shaped memory** — agent knowledge stored as *entities and
   relationships*, not flat text chunks. GraphRAG-style multi-hop retrieval
   reports up to ~35% precision gains over vector-only retrieval; the most
   production-validated pattern (Zep) uses **BM25 + embeddings + graph traversal
   with zero LLM calls at retrieval time**.

## 2. The uncomfortable-in-a-good-way finding: the execution half is already ours

Mapping the concept onto evolve-loop shows the **execution graph is not a gap —
it is this project's existing architecture under another vocabulary**:

| Graph-engineering concept | Already here as |
|---|---|
| Executable DAG of typed nodes | Phase spine + orchestrator state machine (`go/internal/core`) |
| Deterministic vs semantic routing | Advisor proposes (LLM), `ClampPlanModelRouting` + typed routing authority dispose (code) — ADR-0074 |
| Validator nodes on edges | EGPS, contract gate, evalgate A–D, spine floor, ship gate |
| Recovery-path edges | Failure-adapter verdicts (PROCEED/RETRY/BLOCK), graduated remediation |
| Pausable, resumable state | Checkpoints at every phase boundary; `--resume` |
| Compile-then-immutable graph | Phase registry + policy resolution at cycle start |

The literature validates choices this repo made empirically (and paid for in
cycles): the deterministic/semantic split is rule 5; validator-nodes is the
gate-wiring-proof doctrine; recovery edges are ADR-0072. **Nothing to merge
here except vocabulary** — and the vocabulary is worth adopting in docs because
it names the design in terms new contributors will recognize.

## 3. The real gap: the knowledge half is a graph nobody traverses

The *memory* side is where the concept has something to teach us. Our durable
knowledge is **graph-shaped data serialized flat and retrieved by grep**:

- **Dossiers** (`knowledge-base/cycles/cycle-N.json`) carry `failure.fingerprint`,
  `defects[]`, `lessons[]`, `carryover[]` — recurrence edges in all but name.
- **Inbox items** carry `connects_to[]`, `deps[]`, `class` — explicit edge
  fields that *no code traverses*. They are written for humans and read by
  nobody at brief time.
- **Scout/triage** rebuild context each cycle from directory listings and
  recent-outcomes files — single-hop retrieval over multi-hop data.
- **The breaker** detects recurrence by fingerprint string collision inside a
  window, when "same fingerprint, different cycles" is a one-edge graph query
  with the whole corpus as its window.

This is precisely the flat-chunks failure mode the graph-memory literature
describes — with the twist that our entities and relationships **already
exist in committed JSON**. We don't need extraction; we need projection.

## 4. What to merge — three slices, one invariant

**Invariant (repo rule, single-source-with-projection):** the committed JSON
files stay the SSOT. The graph is a *derived, read-only projection*, rebuilt
from the corpus — never a second store that can drift. And per rule 5 + the Zep
production pattern: **zero LLM calls at retrieval time**; the LLM consumes the
traversal result, it never performs the traversal.

### Slice 1 — `evolve kb graph` (projector + query)
A Go projector reads `knowledge-base/cycles/`, `.evolve/inbox/` (live +
consumed), and lesson files, and materializes typed nodes
(`cycle · defect · lesson · item · class · fingerprint · file · commit`) with
typed edges (`recurs-as · consumed-by · connects-to · depends-on · touches ·
fixed-by`). Query surface: `evolve kb graph neighbors <id> --hops 2`,
`evolve kb graph recurrences <fingerprint|class>`. No new daemon, no DB — an
in-memory build over the corpus (hundreds of nodes, milliseconds).

### Slice 2 — brief-time traversal for scout/triage
Where the triage brief today gets a directory listing, it gets the k-hop
neighborhood of the batch's items: prior consumed items of the same class,
their squash SHAs, the files they touched, the lessons attached to their
failures. This is the GraphRAG lesson applied at the one point in our pipeline
where context precision converts directly into pass-rate (brief quality was
the #1 lever in the July stability research).

### Slice 3 — recurrence as a graph query
The identical-fingerprint breaker keeps its window semantics for *halting*, but
`recurs-as` edges over the full corpus feed a weekly report (`evolve kb graph
recurrences --since <cycle>`) so a class recurring across batches — the
apicover class hit **six times** before a human counted them by hand — surfaces
after the *second* instance, mechanically.

### Explicitly not merged
- **No graph database, no vector store** — corpus is small; projection wins.
- **No LangGraph adoption** — the Go orchestrator *is* the executable graph;
  swapping engines buys vocabulary at migration cost.
- **No LLM-written edges** — edges come from fields already written by gated
  pipeline stages; extraction-by-LLM would reintroduce the drift the SSOT rule
  exists to prevent.

## 5. Risks

- **Projection rot** — mitigated the way the dossier schema now is: a drift
  test binding projector field reads to the Go structs (the `TestSchema_NoDrift`
  precedent).
- **Brief bloat** — traversal output must fit the deep-tier artifact budget;
  cap hops (2) and nodes (≤40) at the projector, not in the prompt.
- **Half-written corpus** — ~90 pre-fix dossiers carry the retro mislabel and
  early dossiers lack `failure` blocks; the projector must treat absent edges
  as absent, never synthesized (the fail-open-visible doctrine).

## Sources

- [Graph Engineering for AI Agents: A Complete Guide in LangGraph — Analytics Vidhya, Jul 2026](https://www.analyticsvidhya.com/blog/2026/07/graph-engineering/)
- [Graph-Augmented Large Language Model Agents: Current Progress and Future Prospects — arXiv 2507.21407](https://arxiv.org/pdf/2507.21407)
- [MemGraphRAG: Memory-based Multi-Agent System for Graph RAG — arXiv 2606.00610](https://arxiv.org/html/2606.00610v1)
- [GraphSearch: An Agentic Deep Searching Workflow for Graph RAG — arXiv 2509.22009](https://arxiv.org/pdf/2509.22009)
- [Memory in the Age of AI Agents — arXiv 2512.13564](https://arxiv.org/pdf/2512.13564)
- [AI Memory vs RAG vs Knowledge Graph: Enterprise Guide 2026 — Atlan](https://atlan.com/know/ai-memory-vs-rag-vs-knowledge-graph/)
- [AI Agent Memory Architectures — Zylos Research, Apr 2026](https://zylos.ai/research/2026-04-05-ai-agent-memory-architectures-persistent-knowledge/)
- [LangGraph as a DAG: Rethinking Data Pipeline Orchestration — Medium](https://medium.com/@srikrishnan.tech/langgraph-as-a-dag-rethinking-data-pipeline-orchestration-f089ccea175b)
- [Building Production-Ready AI Agents with LangGraph — Ranjan Kumar](https://ranjankumar.in/building-production-ready-ai-agents-with-langgraph-a-developers-guide-to-deterministic-workflows)
