# Plan: turn software-quality concepts into measured indexes that the Signal Center publishes

- **Status:** proposed, 2026-10-21.
- **Design:** [llm-quality-indexes](../architecture/llm-quality-indexes.md). **Decision:** [ADR-0153](../architecture/adr/0153-quality-is-a-panel-of-separate-indexes-with-intervals-that-the-signal-center-publishes.md). **Research:** [llm-quality-indexes-2026-10](../research/llm-quality-indexes-2026-10.md).

## Table of contents

1. [Request](#1-request)
2. [Facts](#2-facts)
3. [Goals and non-goals](#3-goals-and-non-goals)
4. [Candidate designs](#4-candidate-designs)
5. [Decisions](#5-decisions)
6. [Components](#6-components)
7. [Order](#7-order)
8. [Verification](#8-verification)
9. [Risks](#9-risks)
10. [Open questions](#10-open-questions)

## 1. Request

The operator, 2026-10-21:

> "Build all necessary docs and I want to turn those concepts into measurable indexes for llm."

The brief adds that the indexes must evolve for a non-deterministic, LLM-driven loop, and must not copy standard metrics as they are. Two more requests came on the same day:

> "Those indexes should be monitored in the unified signal center which is designed to monitor the overall system health."

> "Make sure everything is well documented and explained."

## 2. Facts

| Fact | Research id |
|---|---|
| One wave has 4 to 16 cycles; the ship-rate interval of one wave is 40 to 70 points wide. | F2.3 |
| Production data gives no repeated tries of one item, so pass^k needs a replay harness. | F2.4 |
| About one cycle in six ends for a system reason, not an item reason. | F2.5 |
| Three PRs had 100% coverage and a test that did not probe the change; a zero-miss rule had an empty denominator. | F2.7 |
| The independent review finds a HIGH or CRITICAL defect on 41.8% [32.6, 51.7] of PRs. | F2.8 |
| All 234 audit calls ran on one model family. | F2.9 |
| `run.json` `final_verdict` disagrees with the seal in cycle 2024. | F2.2 |
| Hidden tests cut a measured cheat rate from 54% to near zero (secondary). | P3.3 |
| A 10-point MDE on the ship rate needs about 382 cycles per arm. | P4.3 |
| The Signal Center has closed module and kind sets and a code registry; listeners observe and never decide. | spec §5 |

## 3. Goals and non-goals

Goals:

- Each quality concept has an index with a formula, a data source, an interval and a stage.
- Every index publishes through the Signal Center, and nowhere else.
- No index can be raised by the agent that it measures.
- No stage flip happens on noise.
- Each index is explained for a reader who did not see the request.

Non-goals:

- A composite quality score.
- A new dashboard or a new store.
- A gate in this plan. Each gate is a later, separate decision.
- Mutation testing on process-kill code.
- An LLM call in any index computation.

## 4. Candidate designs

| Id | Design |
|---|---|
| A | One composite score: a weighted sum of code, test, pipeline, judge and outcome terms. |
| B | A layered panel: separate indexes in five layers, each with an interval and a stage, published through the Signal Center. |
| C | Outcomes only: lagging indexes such as escape rate, churn and revert rate. |
| D | An external SaaS or a standard metric suite as it is (for example a code-quality vendor or the DORA four). |
| E | Panel B with its own store and dashboard. |

Scores are 1 (poor) to 5 (good). The weights reflect the goal order of the spec (§3).

| Criterion (weight) | A | B | C | D | E |
|---|---|---|---|---|---|
| Resists a game by the measured agent (3) | 1 | 4 | 4 | 2 | 4 |
| States its uncertainty (2) | 1 | 5 | 4 | 1 | 5 |
| Gives early warning (2) | 3 | 5 | 1 | 3 | 5 |
| One monitoring home (2) | 3 | 5 | 4 | 1 | 1 |
| Fits non-deterministic runs (2) | 1 | 5 | 3 | 1 | 5 |
| Cost: no LLM, no new service (1) | 4 | 4 | 5 | 1 | 2 |
| **Weighted total (out of 60)** | **23** | **56** | **41** | **19** | **46** |

Why B wins:

- **A** loses on the game criterion. A weighted sum invites a game of its cheapest term, and a move in one layer hides behind another (Goodhart). It also has no honest interval: the terms have different units and samples.
- **C** loses on early warning. An escape shows up 14 or more days after the ship. In a loop that ships 5 items a wave, that is many waves late. Outcome indexes stay in B as the final check.
- **D** loses on fit. Vendor suites assume a human author, give no interval, and need a new service against the CLI-only rule. The DORA four measure delivery, not the gap between a check and its intent.
- **E** loses only on the monitoring home, which the operator set. B keeps all of the rest of E.

## 5. Decisions

| Id | Decision | Research id |
|---|---|---|
| D1 | Quality is a panel of 21 separate indexes in five layers. There is no composite score. | §4, P3.3 |
| D2 | Every rate carries a Wilson 95% interval. Every median carries a cluster bootstrap interval by wave. A census carries none. | P4.1, P4.2, F2.3 |
| D3 | A reading breaches only when its whole interval is on the bad side of the threshold. | P4.3, F2.3 |
| D4 | Each index has a stage: `shadow`, `report` or `gate`. An index with a baseline starts at `report`, and the others at `shadow`. A flip follows the sample rule of spec §4.6. | P4.3, P3.2 |
| D5 | The Signal Center is the only monitoring home: module `quality`, kinds `quality.reading`, `quality.breach`, `quality.warning`, and one breach code per index with a threshold. | operator request |
| D6 | The Signal Center only carries readings. Computation is deterministic Go in `internal/qualitypanel`. It is a new package: `internal/qualityindex` already holds the review-score grammar. A gate stays a separate, explicit decision. | signal-center-design §3 |
| D7 | The collector runs at the wave boundary and on request, off the critical path of a lane. | F2.5 |
| D8 | Two channels: `quality.breach` (lossless) and `quality.reading` (best effort). | [event-channels.md](../architecture/event-channels.md), [event-notification-protocol-2026-10](event-notification-protocol-2026-10.md) |
| D9 | `evolve status` and the dashboard snapshot show the readings as a panel next to loop and CLI health. | operator request |
| D10 | Indexes read only data that the measured agent cannot write. No index feeds the prompt of the agent that it measures. | P3.3, R3, R8 |
| D11 | A rule that claims zero misses also emits its denominator; an empty one emits `QUALITY_VACUOUS_DENOMINATOR`. | F2.7 |
| D12 | pass^k comes from a replay harness through `evolve agent run`, not from production cycles. | F2.4, P3.1 |
| D13 | Mutation never runs on code that signals a process. | house rule |

## 6. Components

Each component is one console inbox item with route `console-manual`. Each one starts with a RED test from its acceptance.

| Id | Inbox item | What it builds | Red test first | Depends on |
|---|---|---|---|---|
| Q1 | `quality-index-collector` | `internal/qualitypanel` with the Wilson, bootstrap, MDE and breach test. Module `quality`, its kinds and codes. `evolve quality report`, the boundary step, the channel routes and the stage policy. | a reading with a known k and n emits `quality.reading` with the exact Wilson bounds | none |
| Q2 | `pipeline-rates-with-intervals` | PQ1, PQ3 (`evolve quality compare`), PQ4 and CQ2 readers over waves, cycles and ship commits | a cycle-2024 shape (PASS in `run.json`, FAIL seal) counts as not shipped | Q1 |
| Q3 | `complexity-ratchet` | CQ1 on changed functions and the tree, CQ3, CQ4, CQ5 readers | a changed function with cyclomatic 11 lowers the changed-function reading | Q1 |
| Q4 | `outcome-churn-revert-recovery` | OQ2, OQ3, OQ4 readers | a fixture repo with one rewritten line gives the exact churn | Q1 |
| Q5 | `mutation-efficacy-on-changed-functions` | TQ1 and TQ3 in shadow, with the kill-path exclusion | a function that calls `syscall.Kill` is never passed to gremlins | Q1 |
| Q6 | `revert-sensitivity-probe` | TQ2 in shadow | a hunk whose test does not probe it counts as insensitive | Q1 |
| Q7 | `escape-and-false-pass-links` | the `escapes` inbox field, the `Fixes-Ship` trailer, an inbox lint WARN, OQ1 and JQ2 | a fix item whose source names a commit and has no link gets one lint WARN | Q1 |
| Q8 | `judge-calibration-seeded-defects` | the seeded corpus, JQ1, JQ3 with a structured severity field, JQ4 | a seeded defect absent from a judge report counts as missed | Q1, ADR-0152 |
| Q9 | `pass-k-replay-harness` | PQ2 through `evolve agent run` on a frozen item list | 3 tries with 2 passes give pass^2 = 1/3 | Q1, ADR-0152 |
| Q10 | `held-out-acceptance-share` | the hand-off hash of acceptance checks and TQ4 | a check that the builder changed after hand-off is not held out | Q1 |
| Q11 | `quality-health-panel` | the "Quality" block in `evolve status` and `Snapshot.Quality` in the dashboard | the block prints one line per `report` index with its interval and no total | Q1, Q2 |

## 7. Order

1. **Q1** first, because every other component emits through it.
2. **Q2**, because the pipeline rates have data today and answer the most frequent question.
3. **Q11**, so that the operator sees the first readings in `evolve status`.
4. **Q3** and **Q4**, which read data that exists.
5. **Q7**, because OQ1 and JQ2 need its links before data can accumulate.
6. **Q5** and **Q6**, the test indexes, which need new tooling runs at the boundary.
7. **Q8**, **Q10** and **Q9**, which need new corpora, hashes and replay runs.

## 8. Verification

| Goal | Before | After | How measured |
|---|---|---|---|
| Each concept has an index | 0 indexes | 21 in the spec | spec §1 table |
| One monitoring home | 0 quality signals | each computed index visible in `evolve signals tail --kind quality.reading` | Q1 acceptance |
| No self-written source | not checked | no index reads a self-report; tree indexes read `main` after the landing; CQ5 is the stated exception | spec per-index source row |
| No flip on noise | not checked | each flip cites its MDE | ADR amendments |
| Baselines exist | none | 12 indexes with a baseline, 9 marked no data | spec §1 table |

## 9. Risks

| Risk | Control |
|---|---|
| Readings flood the root stream. | `quality.reading` is best effort; one reading per index per window. |
| A breach is read as a verdict. | Spec §5.6 and the runbook state that a breach is a call to look. No code path reads a breach. |
| The boundary takes longer. | Mutation (Q5) runs only on changed functions and has a time budget; a timeout emits `quality.warning`. |
| The review directory moves. | `QUALITY_SOURCE_ABSENT` fires, and the path is in policy. |
| An index becomes a target. | D10: no index in a builder prompt; each index names its game and counter. |
| A mutant kills host processes. | D13 and the Q5 red test. |

## 10. Open questions

- **OQ-1** Does the auditor floor on Claude (`claudeFamilyFloor`) stay? JQ4 measures it but does not change it.
- **OQ-2** Which items form the PQ2 replay set? Proposal: 10 items from the last 30 days, 5 shipped and 5 failed, frozen at their base commits.
- **OQ-3** Do the review files move into Git, so that JQ3 has a tracked source?
