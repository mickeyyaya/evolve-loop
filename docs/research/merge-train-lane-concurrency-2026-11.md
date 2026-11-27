# Merge train and lane concurrency: why wave 191 ran one lane, and what to change (research, 2026-11)

> The design of the train: [ADR-0147](../architecture/adr/0147-the-wave-orchestrator-owns-integration-and-decides-when-the-wave-reaches-main.md), [integration-driver](../architecture/integration-driver.md) and [integration-driver-enforce](../architecture/integration-driver-enforce.md). The admission rules: [ADR-0143](../architecture/adr/0143-admission-holds-only-shared-edits-and-the-ship-gate-owns-api-drift.md) and [internal-admission](../architecture/packages/internal-admission.md). Earlier research: [wave-integration-2026-10](wave-integration-2026-10.md), [merge-concurrency-2026](merge-concurrency-2026/README.md) and [admission-cell-grain-2026-11](admission-cell-grain-2026-11.md). No plan uses this dossier yet. Section 6 lists the decisions that a plan must take.

Date: 2026-11-27. Base: `origin/main` 3e48919e4. Method: the wave status, the supply census, the signal store and the git log of `main`. Also the GitHub run list, code reads with `path:line`, and a live web search. Each local claim names its command or file. "Synthesis" marks our own inference.

## Table of contents

1. [Request](#1-request)
2. [Terms used throughout](#2-terms-used-throughout)
3. [The answer first](#3-the-answer-first)
4. [Local evidence](#4-local-evidence)
5. [Current practice](#5-current-practice)
6. [Candidate approaches](#6-candidate-approaches)
7. [Adopt or reject](#7-adopt-or-reject)
8. [Refinements](#8-refinements)
9. [Limits and open questions](#9-limits-and-open-questions)

## 1. Request

The operator asked on 2026-11-27, after wave 191 ran one lane of five:

> "Why do we design the merge train that could only run one lanes and didnt support multiple agent running at the same time? The merge train is designed to relax the merge pressure on the main and reduce heavy CI loading, loop agent should be able to finish its task and merge to the integration branch designed for the merge purpose. Deep dive and ultrathink the design to see what we can improve"

## 2. Terms used throughout

- **Lane:** one cycle that runs in the pool of a wave. `fleet.count` is 5, so a wave has five lane slots.
- **Admission:** the rule that decides which inbox items can take a lane at the same time (`go/internal/admission`).
- **Cell:** a key that admission holds for a lane in flight. A package, a file, the hub, or an id family `family:<first word of the id>`.
- **Self-land:** a lane rebases onto `main` and pushes its own commit. This is the path at stage `shadow`.
- **The train:** the integration driver of ADR-0147. A lane delivers its change, and the driver composes, tests and merges batches.
- **Stage:** `fleet.integration.stage`. At `shadow` the train only records. At `enforce` the train is the one writer of `main` for the loop.
- **Eject:** the train removes a member from a batch after a conflict or a red gate, and the lane gets the diff that it met.

## 3. The answer first

The train is not a one-lane design. ADR-0147 decision 1 makes every lane deliver to the train. Decision 6 flushes a batch at a cap of 4 members or after 45 minutes (`go/internal/integration/cap.go:6-7`). Many lanes can finish and merge through the train in one wave.

Wave 191 ran one lane for three reasons outside the train:

1. The console started the wave with `--only` three items. All three are train items.
2. The three items edit the same packages: `internal/integration` and `internal/phases/ship`.
3. The three ids start with `train-`, so the id-family rule holds them in one cell.

The pressure on `main` that the request names is real. It comes from stage `shadow`, where each lane still self-lands. The train removes that pressure only at stage `enforce`, which is one item and one operator ruling away (F4.10).

## 4. Local evidence

**F4.1** Wave 191 runs at width 1 of 5 and utilization 0.20. The top idle cause is `admission_refused` (`evolve wave status`, 2026-11-27 00:10Z). The wave runs `--only train-write-once-defence,train-production-eject-ports,train-production-promoter` (boundary step 18 of `evolve wave next`, 2026-11-26 21:42Z).
*Implication:* the four idle slots had no item that the wave allowed.

**F4.2** The supply census lists the cells of the train items as `family:train,internal/integration,internal/phases/ship` (`evolve wave supply`, 2026-11-27). The package cells alone hold the three items apart.
*Implication:* the train items are serial by their code, not only by their names. A different rule cannot run them in parallel.

**F4.3** `idFamily` takes the text before the first hyphen of an item id (`go/internal/admission/admission.go:305-311`). `sharedEdit` includes `sameFamily` (`admission.go:161-163`), and the optimistic rule calls `sharedEdit` (`admission.go:102-106`). So the family rule holds in every admission mode.
*Implication:* two items of one family wait for each other even when their files are disjoint.

**F4.4** The family rule exists because of cycle 1484. A `context-fill` lane shipped a contract test that turned its sibling lane red, and the two lanes edited disjoint files ([internal-admission](../architecture/packages/internal-admission.md), "Id family").
*Implication:* the rule guards against coupling between siblings that no file cell sees. That is a failure of composition, and the train tests composition (ADR-0147 decision 4).

**F4.5** From 2026-11-14 to 2026-11-26 the pool wrote 176 `LOOP_ORCH_POOL_UNDERFILLED` events with a cause (`.evolve/signals.ndjson`, field `cause`).

| Cause | Events |
|---|---|
| `bench` (a quota bench of a CLI) | 77 |
| `family_held` | 75 |
| `run_cap` | 16 |
| `admission_refused` | 4 |
| `supply_empty` | 4 |

The health readings name `family_held` as the top idle cause of wave 173, with a share of 0.815 ([readings 2026-11-16](pipeline-health-readings/2026-11-16/readings.md) §5).
*Implication:* the family rule is one of the two largest causes of empty lanes. The other cause, `bench`, is a quota limit and not an admission rule.

**F4.6** Of 301 inbox items, 30 are admissible. The console owns 209: 73 by a route fact, 42 by a route with no fact, 27 by kind and 67 by a protected file. 43 wait for a cell and 19 wait for a dependency (`evolve wave supply`, 2026-11-27).
*Implication:* a wave with no `--only` still draws on 30 items. The console owns two thirds of the backlog, and the console is one seat.

**F4.7** `main` took 314 first-parent commits from 2026-11-18 to 2026-11-26, a mean of 35 for each day (`git log origin/main --first-parent --since=2026-11-18`). Of the commits since 2026-11-18, 110 are lane self-lands (`evolve-cycle`) and 14 are merges of `origin/main`.
*Implication:* at `shadow` each lane moves `main`, and each move restarts the base of every other lane.

**F4.8** The latest 100 runs of required CI on `main` span 2026-11-20 to 2026-11-27. The mean run took 25 to 34 minutes on each day, and 11 runs failed (`gh run list --branch main --workflow required.yml --limit 100`).
*Implication:* the CI load on `main` scales with the number of self-lands, not with the number of batches.

**F4.9** Cycles 2600 to 2657 wrote 46 `ORCHESTRATOR_SHIP_RECOVERY_PATH` events in 29 of 53 cycles. The paths are 33 `rebind`, 9 `reaudit`, 3 `rebuild` and 1 `end` (`.evolve/runs/cycle-26*/signals.ndjson`).
*Implication:* more than half of the lanes met a moved `main` at ship. Twelve of those cost an audit round or a build round.

**F4.10** At stage `shadow`, the train composed 33 deliveries and ejected 1 (signals `SHIP_INTEGRATION_COMPOSED` and `SHIP_INTEGRATION_EJECTED`, cycles 2600 to 2657). The enforce actor holds the ports `Requeue` and `Briefs` (`go/cmd/evolve/cmd_loop_driver.go:148-156`). `EnforceActor.Missing` still lists `Promote` (`go/internal/integration/enforce_actor.go:144`), and the item `train-production-promoter` adds it.
*Implication:* the shadow eject rate is low, about 3%. The train needs one item, the rollout journey and the flip to `enforce`.

**F4.11** The driver takes `integration.DefaultFlushPolicy()`, a cap of 4 and a maximum age of 45 minutes (`cmd_loop_driver.go:123`, `cap.go:6-7`). `MaxCap` is also 4 (`cap.go:5`). `policy.json` has no key for these values.
*Implication:* the operator cannot tune the batch size or the merge frequency without a code change, and the cap cannot grow above its start value.

**F4.12** ADR-0148 decision 6 makes each PR a boundary PR below stage `enforce`. A code change thus reaches `main` only at a wave boundary, except through a lane self-land.
*Implication:* at `enforce` the train is the first path that merges code to `main` inside a wave with a batch test. ADR-0147 names this an amendment that the operator decides (plan Q4).

**F4.13** In wave 174 the gates took 34% of the lane time, and host work took about 62% (`docs/research/pipeline-health-readings/2026-11-16/readings.md` §5.3). Each lane runs the build floor on the whole of each changed package. The `cmd/evolve` package run on darwin is near its 30-minute limit ([incident 2026-11-26](../incidents/2026-11-26-cmd-evolve-darwin-suite-at-the-30m-limit.md)).
*Implication:* the lane gates and the batch CI test the same packages again. At `enforce` the batch is the place for the heavy suite.

## 5. Current practice

**P5.1** Zuul tests queued changes in parallel with speculative execution. Each change is tested on the assumption that the changes ahead of it pass. A window, tuned like TCP flow control, bounds the number of changes under test ([Zuul project gating](https://zuul-ci.org/docs/zuul/9.4.0/gating.html)).

**P5.2** Uber SubmitQueue builds a conflict graph of pending changes and commits independent changes in parallel ([Keeping master green at scale, EuroSys 2019](https://blog.acolyer.org/2019/04/18/keeping-master-green-at-scale/)). A 2025 follow-up runs only the most likely speculative builds and reports about 53% less CI resource use ([CI at Scale, arXiv 2501.03440](https://arxiv.org/html/2501.03440v2)).

**P5.3** Trunk runs parallel merge queues. Each pull request goes to a lane from its impacted build targets. Lanes with disjoint targets do not wait for each other ([Trunk parallel queues](https://docs.trunk.io/merge-queue/optimizations/parallel-queues)). Batch, merge method and concurrency are settings of each queue.

**P5.4** The CAID study of multi-agent coding finds that branch and merge is central to parallel agents. Conflicts often show only at integration time, and the fix is a revision of one agent's work ([arXiv 2603.21489](https://arxiv.org/pdf/2603.21489)).

**P5.5** The 2026-07 sweep of this repository measured a 27.7% conflict rate on 142,000 agent pull requests. It found change size the main lever ([merge-concurrency-2026](merge-concurrency-2026/README.md), finding 5). It also found that at 3 to 10 lanes, prefix speculation beats a batch with a bisect (finding 3).

*Synthesis:* each of the four surveyed systems admits overlapping changes and tests them together before the merge. None of the four holds a change by the name of its ticket.

## 6. Candidate approaches

Each approach starts from today: stage `shadow`, self-land, admission with the family rule, and waves started with `--only` the head items.

| Id | Approach | Width | Load on `main` | Risk of a red `main` | Cost of a wasted lane | Build cost |
|---|---|---|---|---|---|---|
| A | Keep today | Low (F4.1, F4.5) | High: 35 moves and about 12 CI runs a day (F4.7; F4.8, 100 runs in 8 days) | Medium: 11 red runs in 8 days (F4.8) | Medium: 12 costly recoveries (F4.9) | None |
| B | Finish `enforce`, keep the admission rules | Low, as today | Low: one move for each batch | Low: the batch test runs before the merge | Low | Small: one item and the journey (F4.10) |
| C | B, plus admission that knows the stage, plus a focus mode for waves | High: the family rule and part of the package rule move into the train | Low | Low: the batch test holds | Medium: more ejects, each a lane round | Medium |
| D | C, plus parallel trains by affected packages and the heavy suite in the batch only | Highest | Lowest: one CI run for each batch, and lighter lane gates | Low, if the affected-package graph is correct | Medium | High |

*Synthesis:* B is the precondition for C and D, and it is already in flight. C gives most of the width for a moderate cost. D pays off only when the pool runs more than five lanes, or when the batch CI becomes the bottleneck.

## 7. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| A batch test before the merge, one owner of `main` (P5.1, P5.2) | Adopt (it is ADR-0147) | F4.7 to F4.9 measure the cost of self-land. |
| Admit overlapping changes, test them together (P5.1 to P5.4) | Adopt at `enforce` | F4.10: the shadow eject rate is 3%. F4.4: the family rule guards a composition failure that the batch test catches. |
| Hold a change by the name of its item | Reject at `enforce` | None of the four surveyed systems does it (synthesis). It costs 75 underfill events in 13 days (F4.5). |
| Parallel queues by impacted targets (P5.3) | Defer | Five lanes and a cap of 4 give one batch at a time. Revisit above five lanes. |
| Speculative builds with a success model (P5.2) | Reject | The earlier sweep rejected a trained model at 3 to 10 lanes (merge-concurrency-2026, "What not to import"). |
| A window that grows on green and halves on red (P5.1) | Adopt | ADR-0147 decision 8 already halves the cap on red. The growth on green is missing. |

## 8. Refinements

Each refinement names its findings. A plan decides each one.

| Id | Refinement | Findings | Owner of the decision |
|---|---|---|---|
| R1 | A wave focus mode. The head items take the first lanes, and admissible items fill the free lanes. Today `--only` excludes every other item. | F4.1, F4.6 | The operator: it changes the standing rule to run waves `--only` the head features. |
| R2 | Finish `enforce`: `train-production-promoter`, `train-write-once-defence` and `train-rollout-gate-and-enforce-journey`. Then the operator flips the stage. | F4.10, F4.12 | In flight; the flip is the operator's. |
| R3 | At `enforce`, the id-family rule leaves admission. Siblings of one family compose in one batch, and the acceptance predicates of both run on the composed tree. At `shadow` first, record each family hold that the new rule admits, and the result of its shadow compose. | F4.3, F4.4, F4.5, F4.10 | A plan, then the operator moves the stage of the rule. |
| R4 | At `enforce`, a package cell holds only an edit of the same file. The file grain already holds for leaf packages ([admission-cell-grain-2026-11](admission-cell-grain-2026-11.md)). Extend it to every package, and measure the eject rate. | F4.2, F4.10 | A plan, with a stop condition on the eject rate. |
| R5 | The batch cap, `MaxCap` and the maximum age become `policy.json` data. The cap grows by one on a green batch, up to `MaxCap`, and halves on a red batch. | F4.11, P5.1 | A plan. |
| R6 | At `enforce`, a lane runs its gates on its changed packages and the packages that import them. The whole suite, with the darwin set, runs once for each batch. | F4.8, F4.13 | A plan. ADR-0128 holds the tiers. |
| R7 | At `enforce`, a lane can build an item of a protected file. The batch of such a member is a console-review batch: the console reviews it, and does not build it. | F4.6 | The operator: it changes who can edit a protected surface. |
| R8 | Parallel trains by affected packages. | P5.3 | Deferred until the pool runs more than five lanes. |

*Worked example (synthesis):* with R1 and R2, wave 191 runs `train-production-promoter` in one lane and four admissible items in the other lanes. Each lane delivers to the train. After 45 minutes, or at four members, the driver composes a batch, runs the batch CI once, and fast-forwards `main`. No lane rebases at ship, and `main` takes one CI run for the batch, not four.

## 9. Limits and open questions

- **The eject rate is a biased sample.** The 1 eject in 33 composes comes from lanes that admission kept apart. Relaxed admission raises the rate. R3 and R4 must start at `shadow` and stop above a rate that a plan sets.
- **An eject costs a lane round.** A cycle in wave 190 took 61.9 LLM minutes to its verdict, at the median (`evolve wave status`, wave 190). An eject wastes that time, so R3 and R4 trade width against rework.
- **The flip moves `main` inside a wave.** That amends the rule that code merges only at a boundary (ADR-0147, Consequences). The operator decides it (Q2 below).
- **Supply is the second limit.** R1 to R4 widen the use of the 30 admissible items. Only R7 changes the 209 items that the console owns.
- **The `bench` cause is out of scope.** Quota benches cost 77 underfill events (F4.5). Routing and quota pacing own that cause.

Open questions for the operator:

1. Q1: Do waves keep `--only` the head features, or take R1, the focus mode with backfill?
2. Q2: After the rollout journey passes, does the operator flip `fleet.integration.stage` to `enforce`? Then the train merges inside a wave.
3. Q3: Does R7 move protected-surface builds from the console to the lanes, with a console review of the batch?
