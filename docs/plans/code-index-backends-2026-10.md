# Plan: the type checker answers `callers`, and `evolve code bench` picks the backend of each query by measure

- **Status:** accepted, 2026-10-17 (the operator approved it). G1 is built (PR 938). G2 to G5 are open, and each has an inbox item (section 6).
- **Research:** [code-index-backends-2026-10](../research/code-index-backends-2026-10.md). **Design:** [code-index-cli](../architecture/code-index-cli.md), section "Backends and the benchmark". **Decision:** [ADR-0140](../architecture/adr/0140-the-type-checker-answers-callers-and-each-query-goes-to-its-best-measured-backend.md).
- **Extends:** [code-index-cli-2026-10](code-index-cli-2026-10.md) (components K1 to K5). This plan does not change K1 to K5.
- **Decision numbers:** D1 to D11 of this plan are decisions 1 to 11 of ADR-0140, with the same numbers.

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

On 2026-10-17 the operator asked: "Beside trail-code, have we tried graphify github open source for llm to look up the code more efficient? Could we leverage the cli we build for code look up and integrate with graphify to see which one perform better?" Then: "Again, Document all research, finding, decision, and design, architecture in docs following the rule defined under docs/".

On the same day the operator gave a rule for every service: "Forbid any mcp use, we only use cli to commute with other services, if it doesn't exist, we can build cli for that service". This plan applies the rule. Each backend is a CLI or an `evolve` verb. Each MCP option is rejected for this reason alone.

## 2. Facts

| Fact | Research id |
|---|---|
| Against the type checker, trail-code gives `callers` a precision of 0.11 and a recall of 0.32. | F2.15 |
| graphify gives `callers` a precision of 0.93 and a recall of 0.75 with a second call for 18 names. Without it, the recall is 0.24. | F2.15 |
| trail-code matches a name without its case or its type, and misses calls through a package name. | F2.9, F2.10 |
| graphify needs a unique name: 18 of 40 symbols exit 3 on the first call. It has no caller for most calls through an interface. | F2.7, F2.16 |
| The truth agrees with a grep-and-read check on 130 of 130 call sites of 10 symbols. | F2.19 |
| A prototype of the type-checked index gives 1.00 and 1.00 for a qualified name, and 0.44 and 1.00 for a bare name. A query takes about 0.05 s. | F2.20 |
| One call takes about 0.3 s for trail-code, 0.35 s for `grep -rn` and 3.5 s for graphify, at p50 and under a noisy host load. | F2.12, F2.21, F2.6 |
| The graphify build takes about 81 s and 488 MB. The trail build takes 31 s and 239 MB. | F2.4, C1 |
| A stdlib-only type checker writes every static call edge of the module in about 5 s. | F2.13 |
| graphify adds a Python runtime with 30 PyPI packages, writes skill files at each run unless told not to, and prints instructions to the agent. | F2.1, F2.2, F2.3 |
| No published benchmark measures caller precision for graphify. Its token claims disagree on their corpus. | P3.3, P3.4 |
| The operator's rule forbids MCP. | section 1 |

## 3. Goals and non-goals

**Goals**

- **B1:** for a package-qualified Go name, `evolve code callers` gives the static callers with a precision and a recall of at least 0.95. The measure is against the truth, and the truth must agree with the grep check of F2.19 on its sample.
- **B2:** each query type goes to the backend that measures best for it, and the measure is repeatable by one command.
- **B3:** the agents see no change in the commands, the output format or the exit codes.
- **B4:** no backend uses MCP, writes outside its index, or tells the agent what to do.
- **B5:** the live gain is measured through the K3 usage log before and after the route change. K3 is [PR #931](https://github.com/mickeyyaya/evolve-loop/pull/931), open on 2026-10-17.

**Non-goals**

- A runtime call graph. The truth is static (F2.14).
- A new `evolve code` command, such as `path` or `def`. Open question Q2.
- graphify's document, paper and image pass. It needs an LLM, and the bench is code only.
- A graphify adapter now. It waits for a bench result (D11).

## 4. Candidate designs

### A. Keep trail-code only

The route stays `["trail"]`.

- **Offers:** no new code. A call costs about what `grep -rn` costs (F2.12, F2.21). One index to refresh.
- **Weakness:** `callers` is mostly noise for common names and for interface methods (F2.9, F2.16).

### B. graphify as the first backend

A graphify adapter answers all queries except `map`. The route is `["graphify", "trail"]`.

- **Offers:** EXTRACTED and INFERRED tags, so the agent can see a guess. It finds some injected calls that trail-code misses (F2.8). The best top-1 for `find` on exact names (25/40, F2.15).
- **Weakness:** a common name exits 3, and the second call needs the file (F2.7). A call through an interface has a recall of 0.06 (F2.16). Each call is about 10 times slower (F2.6). It adds a Python supply chain and needs two environment controls (F2.1, F2.2).

### C. A type-checked `gotypes` backend for `callers` and `api`, with trail-code for `find` and `map`

The boundary writes a type-checked index. The route is `["gotypes", "trail"]`.

- **Offers:** exact static callers for a qualified name, and exact declarations. The standard library only. About 5 s at the boundary (F2.13, F2.20).
- **Weakness:** no measure picks the backends, so a later change is a judgment again. A bare name merges homonyms (F2.20). It shows the interface method, not the concrete one (F2.14).

### D. A capability route that sends each query type to its best measured backend

The same `gotypes` backend as C. `evolve code bench` measures every candidate backend against the type-checked truth, and checks the truth with grep. The route holds the best backend of each query type. graphify is a bench-only run of its CLI, with no adapter until it wins a query type.

- **Offers:** everything of C. Each later change of the route is a measured claim. A new engine enters through one bench run first, and through an adapter only if it wins.
- **Weakness:** the bench verb is more work than C.

### Scored comparison

Each criterion has a score from 1 (bad) to 5 (good). The weight is in brackets. A score marked "e" is an estimate, and the notes give its method.

| Criterion (weight) | A. trail only | B. graphify first | C. gotypes + trail | D. measured route |
|---|:-:|:-:|:-:|:-:|
| `callers` accuracy, B1 (×3) | 1 | 3 | 4 | 4 |
| `find` and `api` accuracy (×2) | 3 | 3 | 4 | 4 |
| Latency for each call (×2) | 5 | 1 | 5e | 5e |
| Build and refresh at the boundary (×1) | 4 | 2 | 4 | 4 |
| Supply chain and host needs, B4 (×2) | 3 | 1 | 4 | 4 |
| Effort and risk (×2) | 5 | 3 | 3 | 2 |
| Output tokens for the agent (×1) | 3 | 2 | 4 | 4 |
| A repeatable measure for the next change, B2 (×2) | 1 | 1 | 2 | 5 |
| **Weighted total (out of 75)** | **44** | **31** | **56** | **60** |

Notes on the scores:

- C and D score 4, not 5, for `callers`: a qualified name gives 1.00 and 1.00, and a bare name gives 0.44 and 1.00 (F2.20).
- The latency of C and D is an estimate (5e). At p50, the prototype query took 0.044–0.047 s and the start of `evolve` took 0.019 s. The sum of about 0.07 s was not measured as one run (F2.20).
- The token score of C and D comes from the prototype output: a median of 245.5–267 bytes, against 304.5 for trail-code (F2.15).
- C and D score 4 for the supply chain: trail-code still needs Node for `find` and `map`. graphify runs only on the bench host, never on the agent path.
- The `find` and `api` score of C and D is the trail score plus one, because `api` becomes exact.
- B scores 3 for `callers`: 0.93 and 0.75 with a second call, but 0.06 for interface methods (F2.16). It scores 3 for `find` and `api`: the best top-1 for `find`, but 0.59 recall for `api`.

**The winner is D.** The decisive criterion is B2. C and D give the same answers today. Only D makes the next route change a measured claim, and the operator's question was "which one perform better". D costs the bench verb. The bench is also the regression test of the route, so it pays for itself on the first change.

**What each alternative offers.** A is the fallback, because it needs no work. B is the right design only if graphify's call graph were exact and fast, and F2.15 shows that it is neither. C is the content of the route today, and D adds the measure.

**What changes our mind.** We revisit D if no backend change happens in the 3 months after G3 lands. The bench then has no use, and C is enough. We revisit the route when a bench run shows another backend better on a query type, at a latency that Q5 accepts.

## 5. Decisions

| Id | Decision | Because (findings) | Rejected options and why |
|---|---|---|---|
| D1 | A `gotypes` backend serves `callers` and `api` from a type-checked index. | F2.9, F2.10, F2.13, F2.15, F2.19 | The `gopls` CLI: not installed, experimental, takes a position (P3.7). |
| D2 | The `gotypes` index uses the standard library only (`go/parser`, `go/types`, `go list`). | F2.13 | `go/callgraph`: it adds `golang.org/x/tools` (P3.8). |
| D3 | The boundary builds the `gotypes` index after the trail refresh. A phase reads it only. | ADR-0138 decision 7, F2.13 | A build for each call: 3–5 s for each query (F2.13). |
| D4 | A package-qualified name (`router.Digest`, `loopwave.Launcher.Run`) gives an exact answer. For a bare name with several definitions, each hit names its definition. | F2.20 | A merge of all homonyms with no label: a precision of 0.44, as trail-code does (F2.9). |
| D5 | A `gotypes` hit names its kind: `static`, `interface` or `func-value`. A concrete method with no static caller gets one line for each interface that it implements: "implements `Bridge.Launch`: 5 calls through it". | F2.14, F2.22 | An empty answer: it hides the calls through the interface. |
| D6 | The default route becomes `["gotypes", "trail"]`. | F2.15, F2.20 | `["graphify", "trail"]`: F2.15, F2.6. |
| D7 | `evolve code bench` measures each named backend against the type-checked truth, and runs the grep check of F2.19 on a sample. It prints the table of F2.15. | F2.15, F2.19, P3.3 | Published benchmarks: none measures callers (P3.3). A truth with no check: circular for `gotypes` (F2.14). |
| D8 | A change to the default route carries a bench table in its pull request. | B2 | A judgment: F2.15 shows that the same tool differs by query type. |
| D9 | No backend uses MCP. A service without a CLI gets an `evolve` verb. | the operator's rule | gopls, codegraph, Serena and Codebase-Memory through MCP (P3.5–P3.7). |
| D10 | The boundary refreshes only the indexes that the route names, and a failed refresh is a warning. graphify refreshes only inside a bench run, never at the boundary. | F2.17, ADR-0138 decision 10 | A graphify refresh at the boundary: about 66–130 s for each wave (F2.17). |
| D11 | graphify stays a bench-only run of its CLI. A Go adapter is built only when a bench shows graphify best for a query type. The adapter then follows research R4. | F2.15, F2.1 | An adapter now: YAGNI, graphify wins no query type today. |

## 6. Components

Each component starts with its red test. The engineering bar is `evo:engineering-craft`. New code has 100% coverage.

| Id | Component | Red test first | Owner |
|---|---|---|---|
| G1 | D1, D2, D4, D5: `go/internal/adapters/codeindex/gotypes`. `Build(moduleDir, out)` writes the compact index of F2.20. `Backend` reads it and serves `callers` and `api`. `Capabilities()` is `{Callers, API}`. | `codeindextest.Run` on a fixture module of three packages, with two functions of one bare name, one interface call and one function-value call. It fails until a qualified name gives only its own callers and a bare name names each definition. Each hit must have the right kind, and the concrete method must get its "implements" line. `find` and `map` give `ErrUnsupported`. **Built 2026-10-17:** [internal-adapters-codeindex-gotypes](../architecture/packages/internal-adapters-codeindex-gotypes.md). `Build` takes 3.7–8.2 s and 434–461 MB, and `callers` takes 0.031–0.037 s at p50. | a lane |
| G2 | D3, D10: the boundary step `code-index gotypes`, a step of `wave next` after the K2 refresh. **Amended on 2026-10-26 by K6:** the step comes after the `sync-main` refresh ([code-index-cli.md](../architecture/code-index-cli.md) §4). It writes `dev/code-index-gotypes/index.gob`, beside the index worktree and outside every git tree. | `wave next --dry-run` lists the step after the trail refresh. A failed build prints a warning, the boundary goes on, and the last good index stays. | a lane, after K2. Open: on 2026-10-27, `gotypes.Build` has no production caller. Inbox item `code-index-backends-g2-boundary-gotypes-build`. |
| G3 | D7, D8: `evolve code bench [--backends a,b] [--symbols N]`. It builds the truth from the index worktree and samples symbols by a hash of their position. It runs each backend through `Backends.Search`, and runs the grep check of F2.19 on 10 symbols. It prints precision, recall, p50, p95, bytes and failures, with the method of F2.15. Open: inbox item `code-index-backends-g3-bench-verb` (2026-10-27). | A fake backend with two known false positives and one false negative gives the exact precision and recall. A fixture where the truth misses a real call makes the grep check fail the run. | a lane |
| G4 | D6: set the policy key `code_index.backends` to `["gotypes", "trail"]`, with the G3 table in the pull request. Open: inbox item `code-index-backends-g4-default-route-gotypes` (2026-10-27). | `buildCodeBackends` with the new default routes `callers` to `gotypes` and `find` to trail (a test on the factory map). | the console |
| G5 | B5: one more field in the K3 record: the backend that answered. Open: inbox item `code-index-backends-g5-usage-log-backend` (2026-10-27). | A call through the route writes `backend=gotypes` for `callers` and `backend=trail` for `find`. | a lane, after PR #931 |

**Deferred.** A graphify adapter and its refresh are not components of this plan (D11). If a G3 run shows graphify best for a query type, a new plan adds them. Until then, the bench runs the graphify CLI from outside, with the controls of research F2.2, as section 1 of the research did.

## 7. Order

The order is G1, G3, G2, G4, G5.

1. G1 comes first, because every later step needs the `gotypes` backend.
2. G3 comes next, so that the route change of G4 has its table.
3. G2 makes the `gotypes` index current at each boundary. G4 must not land before it.
4. G4 changes the route, with the G3 table as its proof.
5. G5 adds the backend to the usage log, for the live A/B of section 8.

## 8. Verification

**Before and after**, with the method of F2.15:

| Measure | Before (2026-10-17, trail-code) | After (target, `gotypes` route) |
|---|---|---|
| `callers` precision, qualified name | 0.11 | at least 0.95 |
| `callers` recall, qualified name | 0.32 | at least 0.95 |
| `callers` precision, bare name | 0.11 | measured by G3, each hit named by its definition |
| `api` recall | 0.77 | at least 0.95 |
| agreement of the truth with the grep check | 130/130 (F2.19) | 100% on the G3 sample |
| `callers` p50 | about 0.3 s | at most 0.3 s (estimate: about 0.07 s, F2.20) |
| `callers` output, median | 304.5 bytes (≈ 76 tokens, estimate) | measured by G3 (prototype: 245.5–267 bytes) |
| Boundary time added | 0 s | at most 15 s |

**The live A/B.** The K3 usage log ([PR #931](https://github.com/mickeyyaya/evolve-loop/pull/931)) records each call with its command, phase and working directory. G5 adds the backend. The trial runs 3 waves with the route `["trail"]` and then 3 waves with `["gotypes", "trail"]`. The change is a policy edit, so there is no new flag. For each arm, compare:

- the calls per phase, and the rate of exit 1 (no result) for `callers`;
- the build tokens and the audit pass rate, as G3 of [code-index-cli-2026-10](code-index-cli-2026-10.md) measures them.

The target: the exit-1 rate of `callers` falls, and the tokens and the pass rate do not get worse. The A/B is small, so it shows a direction, not a proof.

## 9. Risks

| Risk | Control |
|---|---|
| The supply chain of a PyPI tool: a lookalike package or a bad release (F2.1). | graphify runs only in a bench run, never on the agent path (D10, D11). Pin `graphifyy==<version>` and its wheel sha256, and install it into its own venv, never globally. |
| The graphify query log records each question in `~/.cache` (F2.2). | Each bench run of graphify sets `GRAPHIFY_QUERY_LOG_DISABLE=1` and `GRAPHIFY_NO_AUTO_REFRESH=1`, and clears the environment. |
| Approximate edges: an INFERRED edge names the wrong function (F2.8), and trail-code merges names (F2.9). | D6 sends `callers` to the type checker. D4 names the definition of each hit, and D5 marks the kinds that a static answer cannot resolve. |
| The truth is itself wrong, so the bench scores a wrong answer as right. | D7: the grep check runs in each bench. F2.19 found 0 missed sites in 130. The check does not cover interface methods yet (F2.19). |
| The type checker fails on a broken tree. | G2 is a warning step. The last good index stays, and the `gotypes` backend gives `ErrAbsent` with no index, so the agent uses grep. |
| The `gotypes` index is at `main`, not at the cycle's edits. | The same rule as ADR-0138: the prompt says that the index shows `main`. |
| The boundary needs about 1.7 GB more memory for about 5 s (F2.13). | G2 runs after the merges, not in parallel with a lane. |
| The timings are noisy: the host load average was 12 to 43 (F2.15). | G3 prints the load average beside each table. Compare by order of size. |

## 10. Open questions

- **Q1:** does `gotypes` also serve `find` for an exact identifier? It answers a definition exactly, but `find` takes free words. We propose not now: the capability route picks one backend for each query, not for each argument.
- **Q2:** does `evolve code` get a `def` or a `path` command? graphify has `path`. No phase has asked for it. The K3 log decides.
- **Q3:** does the bench also score `map`? There is no type-checked truth for a directory summary. We propose that the bench reports `map` as "not scored".
- **Q4:** does `gotypes` add a CHA mode for the concrete targets of an interface call (P3.8)? D5 already gives the reverse: the interface calls of a concrete method, with `go/types.Implements` and no new library. A CHA mode comes only if the K3 log shows calls for an interface method followed by a grep for its implementations.
- **Q5:** what latency can an agent accept for a better answer? No measure gives it yet. For `find`, graphify `query` has a better top-1 than trail-code (25/40 against 19/40), at about 10 times the latency (F2.15). A grep costs about 0.35 s (F2.21). We propose to measure it after PR #931 lands: in the K3 log, compare the follow-up greps and reads after `find` calls of each backend. Until then, the route keeps trail-code for `find`, because the `find` test used exact names, not free words.
