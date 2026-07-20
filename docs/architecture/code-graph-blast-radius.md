# Code-Graph & Blast-Radius Context — Design

> **Status:** Design / not yet implemented (authored 2026-07-20). Target: a Go-native code-graph that scopes Scout / Audit / carry-forward context to the *blast radius* of a change, cutting token cost and raising precision.
> **Prior art / attribution:** This design is directly inspired by **[tirth8205/code-review-graph](https://github.com/tirth8205/code-review-graph)** (MIT). We borrow its central idea — a persistent code knowledge-graph queried for *impact radius* instead of re-reading the whole corpus — and re-implement it natively in Go to satisfy this repo's `go_only_all_tooling` invariant. Where we diverge from the source, this document says so explicitly.

## Table of Contents

1. [Request](#request)
2. [Reference & attribution](#reference--attribution)
3. [Problem](#problem)
4. [Goals / Non-goals](#goals--non-goals)
5. [Approaches considered](#approaches-considered)
6. [Chosen architecture](#chosen-architecture)
7. [Data model](#data-model)
8. [Blast-radius algorithm](#blast-radius-algorithm)
9. [Incremental indexing & staleness](#incremental-indexing--staleness)
10. [Integration points (the actual payoff)](#integration-points-the-actual-payoff)
11. [CLI & internal API surface](#cli--internal-api-surface)
12. [Fail-open guarantee](#fail-open-guarantee)
13. [Phased implementation](#phased-implementation)
14. [Testing strategy](#testing-strategy)
15. [Security](#security)
16. [Success metrics](#success-metrics)
17. [Risks & mitigations](#risks--mitigations)
18. [Open questions](#open-questions)
19. [References](#references)

## Request

> Operator, 2026-07-20: *"reference to https://github.com/tirth8205/code-review-graph to build our own feature to optimize the code search / review / scout / … effort and efficiency … write a complete design document for building code-review-graph in this project and add clear reference to the original source."*

Build an evolve-loop-native capability that, given a change (or a task), returns the **minimal set of code that actually matters** — the functions/types/files/tests reachable from the change — so that the Scout, Audit, adversarial-review, and carry-forward phases stop reading whole subsystems and instead read a precise, impact-scoped context.

## Reference & attribution

The source project, **[tirth8205/code-review-graph](https://github.com/tirth8205/code-review-graph)**, establishes the pattern we adopt:

| Source (Python) | This design (Go) |
|---|---|
| Tree-sitter AST extraction (20+ langs) | Tree-sitter via Go bindings (`smacker/go-tree-sitter`); Go-first, then TS/JS/Python |
| Persistent graph in SQLite (`.code-review-graph/`) | SQLite (`modernc.org/sqlite`, pure-Go, CGo-free) under `.evolve/codegraph/` |
| Blast-radius impact analysis (callers/dependents/tests) | Same algorithm — reverse-edge BFS + coverage edges |
| MCP server (30 tools) + GitHub Action | `evolve codegraph` CLI + **internal phase seams** (no MCP needed; phases call in-process) |
| Optional embeddings (sentence-transformers / OpenAI) | Optional hybrid semantic layer (later phase); keyword/graph first |
| Community detection (Leiden) | Deferred — not needed for the scout/audit payoff |
| Claimed ~82× median token reduction, 0.71 impact F1 | Our success target; measured via existing token-telemetry |

**Why not just use the source tool?** Two hard constraints: (1) `go_only_all_tooling` — no Python in the runtime pipeline; (2) evolve phases dispatch through the agent bridge and must not shell into a Python MCP server. A native Go library lets phases call the impact API in-process with no subprocess/network hop. We keep the source's *ideas and data model*, not its code.

## Problem

Today the phases discover context by **breadth**, not by **relevance**:

- **Scout** greps the tree and reads whole files/subsystems to understand a task — most of which is untouched by the eventual change.
- **Audit / adversarial-review** re-read the changed files *and their surrounding context* to judge correctness, again by reading broadly.
- **Carry-forward / merge selection** (see `carryforward_filter.go`) reasons about whether a branch's change is landable, but has no cheap "what depends on this" signal.

This is expensive (tokens ∝ bytes read) and imprecise (the agent may read 30 files and still miss the one caller that breaks). The source project's benchmark — a median 82× token reduction by querying an impact graph instead of the corpus — is the size of the prize.

## Goals / Non-goals

**Goals**
- G1. A persistent, incrementally-updated graph of the repo's symbols and their relationships.
- G2. A fast **impact-radius** query: `changed paths/symbols → {callers, dependents, covering tests}`.
- G3. In-process integration into Scout, Audit/adversarial-review, and the carry-forward filter.
- G4. Measurable token reduction (via token-telemetry) with **no loss of correctness** (Audit must not miss a real caller).
- G5. Strict fail-open: a stale/absent/errored graph degrades to today's grep+read behaviour, never blocks a cycle.

**Non-goals**
- N1. Replacing the LLM's judgment — the graph *scopes* context, it does not decide verdicts.
- N2. Community detection / visualization / GitHub Action (source features not on the critical path).
- N3. A general-purpose LSP. We extract only the edges the phases use.
- N4. Perfect precision. Recall must be ~1.0 (never drop a real dependent); precision is a token-cost optimization.

## Approaches considered

**A — Wrap the Python source tool via subprocess/MCP.**
Fastest to a demo; violates `go_only_all_tooling` and `subagent_dispatch_via_agent_bridge` (phases would shell to Python/MCP). Adds a runtime Python dependency to a Go binary shipped via the marketplace. **Rejected.**

**B — Embeddings-only semantic retrieval (no graph).**
Index files/chunks as vectors, retrieve top-k by similarity. Simple, language-agnostic. But it gives *similar* code, not *impacted* code — it cannot answer "who calls this / what test covers this," which is exactly what Audit needs for recall. Also adds an embedding provider dependency on the hot path. **Rejected as the primary mechanism** (kept as an optional hybrid layer in S6).

**C — LSP / `go/packages` + `callgraph` (Go-only, no tree-sitter).**
For Go specifically, `golang.org/x/tools/go/packages` + `go/callgraph` (CHA/RTA) gives precise call graphs with zero new parser tech. Very accurate for Go. But it is Go-only (the repo is Go today, but Scout reads docs/skills/other langs), it is heavy (loads+type-checks the whole build), and it cannot incrementally update per-file. **Rejected as the general solution; adopted as an optional high-precision Go backend** (see S1 note).

**D — Go-native tree-sitter code-graph + SQLite (CHOSEN).**
Mirrors the source's proven architecture: tree-sitter for fast, incremental, multi-language symbol/edge extraction; SQLite for a persistent, queryable graph; reverse-edge BFS for blast radius. Incremental (per-file, mtime+hash gated), language-extensible, and callable in-process from Go phases. Trades the type-precision of approach C for speed, incrementality, and multi-language reach — acceptable because the graph *scopes* context (recall-biased) rather than proving correctness.

## Chosen architecture

```
                     ┌─────────────────────────────────────────────┐
   repo files ─────► │  Indexer (internal/codegraph/index)         │
   (git-tracked)     │   • tree-sitter parse per file (lang-keyed) │
                     │   • extract nodes + edges                   │
                     │   • mtime+contenthash gate (incremental)    │
                     └───────────────┬─────────────────────────────┘
                                     ▼
                     ┌─────────────────────────────────────────────┐
   .evolve/codegraph/graph.db  ◄──►  │  Graph store (SQLite, pure-Go) │
   (gitignored runtime state)        │   nodes, edges, files, coverage │
                     └───────────────┬─────────────────────────────┘
                                     ▼
                     ┌─────────────────────────────────────────────┐
   changed paths ──► │  Impact analyzer (internal/codegraph/impact)│──► minimal
   (git diff)        │   • map diff → touched nodes                │    context set
                     │   • reverse-edge BFS (callers/dependents)   │    (files+symbols+tests,
                     │   • union covering tests                    │     ranked by distance/risk)
                     └───────────────┬─────────────────────────────┘
                                     ▼
        ┌────────────────────────────────────────────────────────────────┐
        │  Consumers (in-process seams, fail-open):                        │
        │   • Scout   — impact-scoped discovery for bugfix cycles          │
        │   • Audit / adversarial-review — context = blast radius of diff  │
        │   • carryforward_filter — cheap "what depends on this" signal    │
        │   • CLI: evolve codegraph build|update|impact|query|stats        │
        └────────────────────────────────────────────────────────────────┘
```

Package layout (proposed):
- `go/internal/codegraph/` — the library (no phase imports; phases depend on it, not vice-versa).
  - `parse/` — tree-sitter loaders per language, symbol/edge extractors.
  - `store/` — SQLite schema, upsert, query (pure-Go driver `modernc.org/sqlite`).
  - `index/` — the incremental indexer (walk, gate, parse, upsert).
  - `impact/` — blast-radius BFS + ranking.
- `go/cmd/evolve/cmd_codegraph.go` — the `evolve codegraph …` subcommand.
- Consumer seams live in the phases, injected (DI), defaulting to a no-op when the graph is absent.

## Data model

Nodes (SQLite `nodes` table): `id`, `kind` (`func|method|type|const|var|file|test|package`), `name`, `qualified_name`, `file_path`, `start_line`, `end_line`, `lang`, `content_hash`.

Edges (SQLite `edges` table, directed): `src_id`, `dst_id`, `kind`:
- `calls` — function/method call site → callee.
- `imports` — file/package → imported package.
- `implements` / `embeds` — type → interface/embedded type.
- `references` — symbol → referenced type/const (coarse; recall-biased).
- `covers` — test node → symbol it exercises (from test-name heuristics + `covers` comments + coverage profiles when available).
- `defines` — file → symbol.

FTS5 virtual table over `name`/`qualified_name` for keyword lookup. All ids are stable content-addressed keys (`qualified_name`+`kind`) so incremental re-index is idempotent.

**Divergence from source:** we add `covers` from Go's coverage profiles (`coverage.txt`) when present — the repo already generates these in CI — giving higher-fidelity test-coverage edges than name heuristics alone.

## Blast-radius algorithm

```
impact(changedPaths, maxHops):
  seeds = nodes whose file ∈ changedPaths  (or whose span intersects the diff hunks)
  frontier = seeds; radius = {seeds}
  for hop in 1..maxHops:
     next = { src : (src --calls|implements|references--> n) for n in frontier }   # REVERSE edges
     radius ∪= next; frontier = next \ already-seen
  tests = { t : (t --covers--> n) for n in radius }
  return rank(radius ∪ tests)
```

- **Reverse edges** answer "who is affected if this changes" (callers, implementors, referencers).
- **Recall-first:** default `maxHops` is generous and `references` edges are coarse; we would rather include a file that turns out irrelevant than drop a real caller (Audit correctness depends on recall).
- **Ranking** by (hop distance ↑ = less relevant) × (risk: fan-in, test-gap, churn) so a consumer with a token budget can take the top-N.
- Matches the source's `get_impact_radius_tool`; F1 target parity (~0.71) with recall pinned near 1.0.

## Incremental indexing & staleness

- Graph lives at `.evolve/codegraph/graph.db` — **gitignored runtime state** (like `.evolve/runs/`), never committed, per-checkout. It is NOT an evolve *deliverable* (contrast `.evolve/phases/`), so it never trips the tree-diff leak guard.
- `build` does a full walk; `update` re-parses only files whose `(mtime, size)` changed, then verifies by content hash — the source's <2s incremental target.
- A `schema_version` + repo `HEAD` sha are stamped in a `meta` row. On mismatch (schema bump, or a large base jump) the consumer treats the graph as **stale ⇒ fail-open** rather than trusting drifted edges.
- Optional daemon/watch mode is explicitly deferred; the loop calls `update` at cycle start (cheap) instead.

## Integration points (the actual payoff)

1. **Scout (bugfix cycles).** When a cycle has a fault-localization target or a changed-file hint, Scout requests `impact(target)` and reads that set first, expanding only if needed — instead of grepping the subsystem. Feature/greenfield cycles with no seed fall back to today's discovery (fail-open).
2. **Audit / adversarial-review.** The reviewer's context is scoped to `impact(build's diff)` — the changed files **plus their callers and covering tests** — which is exactly the "did this break a caller / is it tested" question, at a fraction of the bytes.
3. **carry-forward filter (`carryforward_filter.go`).** Adds a cheap dependency signal: does the candidate branch touch symbols with high downstream fan-in? Complements the existing 3-way-merge landability check.
4. **Token-telemetry.** Every impact-scoped read is measured against the counterfactual (bytes the phase would have read) so the 82×-style claim is *verified in-repo*, not assumed.

Each seam is DI-injected and defaults off; turning it on is a `policy.json` `workflow.codegraph` block (config, not a Go flag — per `no_feature_flags_use_design_patterns` / `phase_settings_from_config_not_code`).

## CLI & internal API surface

```
evolve codegraph build              # full index of the repo → .evolve/codegraph/graph.db
evolve codegraph update             # incremental re-index of changed files
evolve codegraph impact <paths...>  # print the blast-radius set (files/symbols/tests), ranked
evolve codegraph query <symbol>     # who-calls / callees / covering-tests for one symbol
evolve codegraph stats              # node/edge counts, staleness, last-build sha
```

Internal seam (consumed by phases):
```go
package codegraph
type Impact struct { Files []string; Symbols []Node; Tests []Node; Truncated bool }
type Analyzer interface {
    Impact(ctx context.Context, changed []string, budget Budget) (Impact, error) // err ⇒ caller fails open
}
```

## Fail-open guarantee

This is load-bearing (see `feedback/no_workaround_root_cause_redesign` and the cycle-760..762 destruction lesson): **the code-graph is an optimization, never a gate.** Absent DB, stale schema, parse error, query error, or empty result ⇒ the consumer logs a WARN and uses its pre-existing grep/read path. No cycle may FAIL or block because the graph was unavailable. A regression test asserts each seam's fail-open behaviour.

## Phased implementation

| Phase | Deliverable | Gate |
|---|---|---|
| **S1** | `parse` + `store`: Go tree-sitter extraction → SQLite; `evolve codegraph build|stats`; golden-graph tests on a fixture package | build indexes this repo's Go; node/edge counts stable |
| **S2** | `impact` analyzer + `evolve codegraph impact|query`; reverse-BFS + coverage edges; impact-F1 harness | F1 ≥ ~0.7, recall ≥ 0.95 on a labelled fixture |
| **S3** | Scout seam (impact-scoped discovery, DI, fail-open) + token-telemetry counterfactual | measured token delta on ≥5 bugfix cycles; zero correctness regressions |
| **S4** | Audit / adversarial-review seam (context = diff blast radius) | Audit recall unchanged on a replay of past FAIL cycles |
| **S5** | carry-forward dependency signal | complements landability; no false "landable" flips |
| **S6** *(optional)* | Hybrid semantic layer (embeddings + FTS5) behind the same interface | opt-in; only if S3/S4 show retrieval gaps |
| **S7** *(optional)* | TS/JS/Python extractors for docs/skills/multi-lang scout | per-lang golden tests |

Each phase is a normal evolve cycle: TDD red-first, adversarial audit, and a **wiring proof** that the seam actually fires in the composed phase path (not just unit-green) — per the standing pipeline-first deep-review rule.

## Testing strategy

- **Golden graphs:** a checked-in fixture package with a known symbol/edge set; `build` must reproduce it exactly (catches parser drift).
- **Impact F1 harness:** labelled "change X → these are the true dependents" cases; assert recall ≥ 0.95 and report precision/F1 (the source reports 0.578 precision / 1.0 recall / 0.71 F1 — our floor).
- **Incrementality:** touch one file, assert only its nodes re-index and edges reconcile.
- **Fail-open:** delete/corrupt the DB mid-run; assert each consumer degrades to grep/read with a WARN and the cycle still completes.
- **`-race`** on the indexer (concurrent per-file parse) and the store.

## Security

Parsing is over **repo-tracked source only** (never arbitrary network input); tree-sitter is a pure parser (no code execution). The DB is local, gitignored, per-checkout. No secrets are indexed (node bodies are hashed, not stored verbatim beyond spans). The impact set is advisory context for an LLM phase — it cannot widen a trust boundary (it never grants a phase new write/commit capability). A `security-reviewer` pass is required on the S3/S4 seams since they alter what Audit sees.

## Success metrics

- **Token reduction:** ≥ 10× median on impact-scoped Scout/Audit reads vs the counterfactual (source claims 82×; we set a conservative floor and measure).
- **Correctness:** zero new Audit FAIL-misses on a replay of historical FAIL cycles (recall guard).
- **Build/update latency:** full build < 1s per ~1k files; incremental update < 2s (source parity).
- **Adoption:** Scout + Audit seams live and default-on via `policy.json` after S3/S4 soak.

## Risks & mitigations

| Risk | Mitigation |
|---|---|
| Go tree-sitter bindings (CGo) complicate the CGo-free build | Prefer a pure-Go SQLite driver (`modernc.org/sqlite`); if tree-sitter's CGo is unacceptable, S1 falls back to approach C (`go/packages`+`callgraph`) for Go and defers other langs |
| Graph drift silently degrades recall | schema+HEAD stamp ⇒ stale ⇒ fail-open; golden + F1 tests in CI |
| A new dep expands the supply chain | vendor + pin; the pure-Go path avoids CGo/system libs |
| Over-scoping (agent trusts the graph and misses context) | recall-biased BFS + fail-open + Audit recall replay gate |
| Feature outranks pipeline-integrity work | weighted 0.82 — **below** the 0.85–0.97 pipeline band; drains only after the integrity backlog |

## Open questions

- OQ1. Tree-sitter CGo vs pure-Go `go/packages` for the Go extractor — decide in S1 spike (CGo-free build is a hard repo constraint).
- OQ2. `covers` edge fidelity: name heuristics vs parsing `coverage.txt` vs both — S2 measures which wins on F1.
- OQ3. Where the impact set is injected into a phase prompt (a new handoff-artifact section vs an inline context block) — coordinate with `handoff-artifact-schema.md`.
- OQ4. Do we need per-lane graph isolation under the fleet (each lane's worktree has its own diff)? Likely yes — the graph is per-checkout, so each worktree builds/updates its own `.evolve/codegraph/`.

## References

- **Original source (primary reference):** [tirth8205/code-review-graph](https://github.com/tirth8205/code-review-graph) — persistent tree-sitter→SQLite code knowledge-graph with blast-radius impact analysis, MCP server, incremental indexing (MIT). This design re-implements its core idea in Go.
- Inbox item: `codegraph-blast-radius-context-for-scout-audit-review` (weight 0.82).
- Related in-repo: `carryforward_filter.go` (dependency/landability), `handoff-artifact-schema.md` (where impact context lands), token-telemetry campaign (measurement), `no_feature_flags_use_design_patterns` / `phase_settings_from_config_not_code` (config-driven enablement), `go_only_all_tooling` (why Go-native).
- Tech candidates: `smacker/go-tree-sitter` (bindings), `modernc.org/sqlite` (pure-Go SQLite), `golang.org/x/tools/go/{packages,callgraph}` (Go high-precision fallback).
