# Wave orchestrator: why waves run one or two lanes, and how fleets of coding agents stay parallel without conflicts (research, 2026-10)

> The plan that uses this research: [wave-orchestrator-2026-10](../plans/wave-orchestrator-2026-10.md). The design: [wave-orchestrator](../architecture/wave-orchestrator.md). The decision record: [ADR-0139](../architecture/adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md).

Date: 2026-10-17. Method:

- a read of the fleet and wave code (`internal/fleet`, `internal/loopwave`, `cmd/evolve/cmd_loop_*.go`);
- the loop logs of the 12 loop runs from `20261016T112047Z` to `20261017T022806Z`;
- `evolve inbox rank --list ready|console|waiting` and `evolve inbox show <id>` for each of the 229 console items;
- the durations of cycles 1900–1920 (directory birth time to the last write, an estimate);
- a simulation of barrier utilization from those durations;
- a live web survey of parallel coding agents, conflict prediction, merge queues, scheduling, supervision and graph partitioning.

Each local claim names its command or file. "Synthesis" marks our own inference.

## Table of contents

- [1. The request](#1-the-request)
- [2. Local evidence](#2-local-evidence)
- [3. Current practice](#3-current-practice)
- [4. Adopt or reject](#4-adopt-or-reject)
- [5. Refinements to adopt](#5-refinements-to-adopt)

## 1. The request

On 2026-10-16 the operator asked: "Set the max cycle per wave to 4". The operator also asked for an orchestrator that oversees every parallel cycle of a wave. On 2026-10-17 the operator asked: "Why the wave cycle number has not configured to 4 parallel running cycle? And did we have the orchestrator oversee all parallel running cycles in the same wave which I requested yesterday".

The operator then gave the scope of this design: "Document and research first and make sure it can scale to large fleet number in parallel". The operator added: "I want to leverage at least 4 lanes at the same time and minimize the confliction by orchestrator design".

## 2. Local evidence

### 2.1 What sets the width of a wave

**F2.1** `.evolve/policy.json` sets `fleet.count` 3, `fleet.min_lanes` 2 and `fleet.plan_source` `triage`. `scheduling` is not set, so it is `wave` (`go/internal/policy/fleet.go:159-235`).
*Implication:* the configured width is 3, not 4, and the scheduler is the barrier.

**F2.2** All 12 loop runs of 2026-10-16 and 2026-10-17 log `wave count 3 -> 2`. Of the 25 bench lines, 23 name `agy-claude` and 2 name `agy` (`grep 'quota bench on CLI family' .evolve/logs/2026101[67]*/loop.log`).
*Implication:* the quota shrink costs one lane in every wave before any plan exists.

**F2.3** `Engine.Size` passes every active bench to the shrink (`go/internal/loopwave/size.go:15`, `:39-49`). `QuotaAwareCount` removes one lane for each benched family (`go/internal/fleet/quota.go:9-39`). The package note says the caller passes "the intersection of the benches with the families the wave needs" ([internal-fleet](../architecture/packages/internal-fleet.md)). The code passes no intersection.
*Implication:* a bench on a family with a fallback in the same chain still costs a lane. `agy-claude` has `claude` after it in the deep and top chains, so its bench removes no real capacity.

**F2.4** Three of the 12 runs log `LOOP_WAVE_EMPTY_PLAN … planned zero lanes (empty backlog)`: `20261016T213952Z`, `20261017T014448Z` and `20261017T022806Z`. The repair then dispatches one lane (`go/internal/loopwave/dispatch.go:102-128`).
*Implication:* in the newest runs, the plan has no work for a second lane.

### 2.2 Where the lanes get their work

**F2.5** The wave plan reads the `triage-decision.json` of the last cycle, prunes it, and widens it from the inbox (`go/internal/loopwave/plan.go:18-25`). Without a decision, it seeds from the inbox (`plan.go:25`). The triage persona commits 1–3 items for each cycle (`agents/evolve-triage.md:31`).
*Implication:* the seed has one item in most waves. The inbox must supply every other lane.

**F2.6** On 2026-10-17, `evolve inbox rank --list ready` shows **0** items. The console list holds 229 items, and the waiting list holds 1.
*Implication:* the inbox supplies no lane at all. This is the first cause of the narrow waves.

**F2.7** The console reasons of the 229 items, from `evolve inbox show <id>`:

| Reason | Items | Source of the reason |
|---|---:|---|
| protected fix surface | 90 | `protectedDerivation`, `go/internal/inboxbatch/consoleroute.go:58-80` |
| `kind:pipeline-repair` | 77 | `pipelineKindDerivation`, `consoleroute.go:49-56` |
| `route:console-manual` | 31 | the item's `route` field |
| `route:console` | 26 | the item's `route` field |
| `kind:pipeline-integrity`, `kind:pipeline-refactor` | 3 | `pipelineKindDerivation` |
| `route:console-sandbox-denied`, `route:console-pipeline-integrity` | 2 | the item's `route` field |

*Implication:* 170 items (74%) go to the console by derivation, not by an explicit route. A change of the derivation rules moves the most supply. §2.6 holds the detail.

### 2.3 How lanes avoid each other today

**F2.8** `Partition` is the live partitioner. It is file-disjoint. A todo that overlaps two buckets waits for a later wave (`go/internal/fleet/partition.go:46-61`). A card with no `files` gets its id as its only file (`go/internal/fleet/triageplan.go:67-70`).
*Implication:* two lanes that edit different files of one package run together, and a card without `files` never conflicts on paper.

**F2.9** `PartitionGraph` partitions by module-internal package sets from `go list -deps` (`go/internal/fleet/packagegraph.go:62`). It has no production caller ([internal-fleet](../architecture/packages/internal-fleet.md), status 2026-10-09).
*Implication:* the package-level partitioner exists, but dispatch does not use it.

**F2.10** ADR-0134 plans admission by cell at dispatch, with a capacity of 1 for the hub cell (`docs/plans/fleet-landing-at-scale-2026-10.md` §5.6, S14). Its research finds about 139 pairwise-disjoint Go lanes in this module (F2.16 of that dossier). S14 is not built.
*Implication:* the admission rule for a large fleet is designed. The dispatch component that runs it does not exist.

**F2.11** Lanes land one at a time under `ship.lock`. A lane that finds a moved `main` rebases and is audited again. The census found 46 such recoveries, at 24.2 min and 1.49 LLM phases each ([concurrent-cycle-landing-2026-10](concurrent-cycle-landing-2026-10.md)).
*Implication:* each conflict that dispatch does not prevent costs about 24 minutes at landing.

### 2.4 How the wave runs its lanes

**F2.12** `Supervisor.Run` starts each lane under a semaphore and returns after `wg.Wait()` (`go/internal/fleet/fleet.go:94`). The wave path builds the supervisor with `Concurrency` and `Launch` only (`go/internal/loopwave/launcher.go:18`). Only `evolve campaign` sets `CycleTimeout` (`go/cmd/evolve/cmd_campaign.go:194`).
*Implication:* a wave has no lane deadline, and one slow lane holds every free slot.

**F2.13** The durations of cycles 1900–1920 range from 8 to 83 minutes, with a median of 31 (estimate: directory birth to the last write). A simulation draws N lanes from these durations 20,000 times. It reports the mean of `sum(durations) / (N × max)`:

| Lanes N | Utilization of a barrier wave |
|---:|---:|
| 2 | 0.80 |
| 4 | 0.66 |
| 8 | 0.56 |
| 16 | 0.49 |

*Implication:* at 4 lanes a barrier wastes about one third of the lane time. The loss grows with N (synthesis).

**F2.14** `RunPool` backfills a lane slot when any lane exits. It picks the highest-priority pending todo that is file-disjoint from every running lane (`go/internal/fleet/pool.go:21`). It runs only when `scheduling` is `pool`, and its `onTransition` hook is `nil` (`go/cmd/evolve/cmd_loop_pool.go:33`).
*Implication:* the rolling pool exists, is off, and reports nothing.

**F2.15** Each lane has its own phase observer, scoped to one cycle (`go/internal/cli/phasecmd/phase_observer.go:77`). `evolve wave watch` polls every 5 s and prints phases, seals and ships (`go/cmd/evolve/cmd_wave.go:27`). The quota state is read once, before the wave (`size.go:15`).
*Implication:* no component sees all lanes of a wave at once. Nothing acts on a stall, a quota wall or an overlap while the wave runs.

**F2.16** `StarvationTracker` counts waves that realize fewer lanes than they size. After K such waves it files one inbox item (`go/internal/fleet/starvation.go:34-54`).
*Implication:* starvation is reported, but nothing repairs the supply.

### 2.5 Summary of the causes

| Cause | Evidence | Lanes lost at width 3 |
|---|---|---|
| No lane-routable supply | F2.6, F2.7 | up to 2 |
| A bench that has a fallback still costs a lane | F2.2, F2.3 | 1 |
| A one-item seed | F2.5 | 0–2, when the inbox is empty |
| The barrier waits for the slowest lane | F2.12, F2.13 | about one third of the lane time |

### 2.6 The console routing in detail

This section holds the evidence of the console-routing investigation of 2026-10-17. An analyzer applied today's rules (`guards/integrity_surface.go` and the `inboxbatch` token rules) to each item. It gave the same split as the live command: 90 protected, 80 pipeline kind, 59 explicit routes and 1 waiting.

**F2.17** The lane-routable supply fell for one month, while the inbox size stayed at 230–240 items. The analyzer applied today's rules to each git snapshot of `.evolve/inbox`:

| Date | Lane-routable items |
|---|---:|
| 2026-09-15 | 41 |
| 2026-09-30 | 45 |
| 2026-10-06 | 32 |
| 2026-10-09 | 18 |
| 2026-10-13 | 14 |
| 2026-10-16 | 4 |
| 2026-10-17 | 1 |

*Implication:* lanes used the lane-safe items, and the console did not drain its own share. A one-time repair of the supply does not hold. The supply needs a producer that runs at each plan.

**F2.18** The protected set is `guards.ProtectedSurfaceManifest`, with 147 fragments (`go/internal/guards/integrity_surface.go:14-164`). It grew from 25 fragments on 2026-08-01 to 147 on 2026-10-17. Its origin is ADR-0064: a cycle edited the gate that graded it, and the audit passed it. ADR-0064 calls the manifest "intentionally NARROW".
*Implication:* the manifest guards a real failure. A remedy must keep each grader protected.

**F2.19** The match is not one rule. `IsProtectedSurface` is a substring match, so the fragment `/go/internal/bridge/` protects all 453 files of that package (`integrity_surface.go:166`). `IsProtectedScope` also routes a directory spelling that only *contains* a protected file (`fragmentInside`, `integrity_surface.go:222`). A protected file spelling is binding, and `route:lane` cannot relax it (`go/internal/inboxbatch/consoleroute.go:31-33`).
*Implication:* a card that names `go/internal/core/` goes to the console, though 85% of the files of `internal/core` are not protected.

**F2.20** The protected share of each package:

| Package | Protected files |
|---|---:|
| `internal/core` | 115 of 748 (15%) |
| `internal/phases` | 78 of 510 (15%) |
| `internal/bridge` | 453 of 453 (100%) |
| `internal/config` | 40 of 40 (100%) |
| `internal/inboxmover` | 42 of 98 (43%) |
| the whole tree | 1,137 of 6,019 (19%) |

Of the 90 protected items, 20 are routed only by ADR-0103 seam fragments, and 11 only by the whole-bridge fragment. 77 of the 90 also name files that are not protected.
*Implication:* most protected items mix a console part and a lane part. A split of each item frees the lane part and keeps the guard.

**F2.21** No code filed the 80 `kind:pipeline-*` items. The only code producer is the halt autofile (`go/cmd/evolve/cmd_loop_escalation.go:101`), and all 80 have an empty `injected_by`. The console filed them, 43 of them between 2026-09-26 and 2026-09-30. Of the 80, 36 have no binding protected file, so `evolve inbox route-lane` already admits them (`consoleroute.go:34-35`).

About 15–18 of them are ordinary code fixes. About 10 change grading logic that the manifest does not name, such as the ship carry and the personas.
*Implication:* a review of the pipeline kind frees 15–18 items with no code change. The grading items stay with the console.

**F2.22** Of the 59 explicit routes, 36 have no binding protected file. About 9 carry only an ownership reason, such as "the stelint package is console-built". 7 came from the dispatchability audit of #756: their fixes cite protected paths that their `files` do not name.
*Implication:* a sweep frees about 6–10 stale routes. The 7 under-declared items must name their protected paths, so that the manifest routes them and not a hand-set route.

**F2.23** Four guards stop a lane at a protected file, whatever the routing (`go/internal/phases/triage/protected_route.go:19-106`, `go/internal/guards/role.go:60`, the build floor of ADR-0064 F37, and `go/internal/phases/ship/integrity.go:11-32`). The ship tripwire fails a `--class cycle` diff that touches the manifest.
*Implication:* the routing decides where work goes. These four guards protect the graders. A change of the routing does not weaken them.

**F2.24** All lane work of cycles 1900–1920 came from the inbox (17 cycles) or from carryover (2 cycles). One cycle had no work, and no cycle took work from its scout. 9 of the 17 inbox items were practice-watch documentation items.
*Implication:* the inbox is the only supply. When the inbox has no lane-routable item, the lanes have nothing.

**The remedies, with the items each one frees** (counted from the real items):

| Id | Remedy | Items freed | Risk | Guarantee kept |
|---|---|---:|---|---|
| S-A | Re-route the pipeline-kind items that have no protected file, with `evolve inbox route-lane`. The grading items stay with the console. | 15–18 | low | the four guards of F2.23 |
| S-B | Sweep the stale explicit routes. The 7 under-declared items name their protected paths. | 6–10, and 7 that route themselves | low | the four guards |
| S-C | Route a directory spelling only when it lies *inside* a fragment, not when it *contains* one. | 8 | low | the ship tripwire judges each file |
| S-D | Split each mixed item into a console part and a lane part, linked by `deps`. | up to 58 | medium: a lane part can need its console part first | the console part stays with the console |
| S-E | Narrow the manifest back to the graders, through an amendment of ADR-0064. | up to 41 | medium to high | the gates, guards, rubrics, sandbox and landing stay protected |
| S-F | A ship class that a lane builds and the console countersigns, for protected files | up to 113 | high: it weakens ADR-0064 | the countersign only |

## 3. Current practice

### 3.1 Parallel coding agents

**P3.1** Co-Coder treats the split of a task as graph partitioning. It builds a static dependency graph, removes hub files, runs Infomap community detection and schedules the groups by dependency.

On DevEval it reached a 68.1% pass rate in 442 s at $0.18. Sequential work reached 56.8% in 800 s at $0.25. Naive file-based parallel work reached 57.7% in 806 s at $0.36: it was no faster and 44% more costly ([arXiv 2606.00953](https://arxiv.org/abs/2606.00953)).
*Implication:* parallel lanes give no gain when the split is by file only. The split must follow cohesion, with the hubs out.

**P3.2** Co-Coder's gain falls as coupling rises. When the graph is one connected group, the partitioner gives one group, and the work becomes sequential ([arXiv 2606.00953](https://arxiv.org/html/2606.00953), limitations).
*Implication:* the width limit comes from the code graph, not from the lane count.

**P3.3** A study of 33,596 agent pull requests in 2,807 repositories replayed three-way merges. The textual conflict rate was 19.8% for two PRs of one agent and 41.7% for PRs of two agents. 84.4% of the conflicted files were source files ([arXiv 2607.04697](https://arxiv.org/abs/2607.04697)).

AgenticFlict found 27.67% over 107,000 simulated merges ([arXiv 2604.03551](https://arxiv.org/abs/2604.03551)). Both count only textual conflicts.
*Implication:* uncoordinated parallel agents conflict in about one pair of four. The pairwise conflict rate is a good measure for our design.

**P3.4** Claim Plane admits a write only when its declared scope is free. Static admission raised the pair pass rate from 23.3% to 50.0%, but it made 96.7% of the runs serial. Dynamic admission made fewer runs serial. But 46 of 90 runs stopped on a scope that the agent did not declare ([arXiv 2608.00947](https://arxiv.org/abs/2608.00947), one author, a preprint).
*Implication:* admission by declared scope is safe but can kill the width. Agents declare too little, so the admission unit must be coarse (a package) and derived, not only declared.

**P3.5** Claude Code agent teams advise 3–5 teammates, with one owner for each file, because "two teammates editing the same file leads to overwrites". Dynamic workflows run 16 agents by default and up to 256, with a stall timeout and at most 5 restarts ([agent teams](https://code.claude.com/docs/en/agent-teams), [workflows](https://code.claude.com/docs/en/workflows), vendor documents).
*Implication:* the vendors keep file ownership and a stall watchdog for each worker. Neither publishes a conflict rate.

**P3.6** Devin's coordinator splits work into child sessions and advises groups of packages that do not conflict. Google Jules caps concurrent tasks at 3, 15 or 60 by plan ([Devin](https://docs.devin.ai/working-with-teams/multidevin), [Jules](https://jules.google/docs/usage-limits/), vendor documents).
*Implication:* the package is the common unit of a non-conflicting split.

### 3.2 Conflict prediction

**P3.7** A classifier over 267,657 merges predicts a safe merge with F1 0.95–0.97, but a conflicting merge with F1 0.57–0.68 only ([arXiv 1907.06274](https://arxiv.org/pdf/1907.06274)).
*Implication:* a predictor can clear a pair as safe. It cannot reliably block a pair. A design must not depend on a predicted conflict.

**P3.8** The survey found no precision or recall data for import-graph distance as a conflict predictor.
*Implication:* our own pairwise conflict rate must measure the package rule.

### 3.3 Merge queues at scale

**P3.9** Uber SubmitQueue calls two changes independent when they touch disjoint sets of build packages. Its conflict analyzer prunes the speculation tree. The 2024 rollout cut CI resources by 53%, CPU by 44% and the P95 wait by 37%. A small change that lands before a large conflicting change cut the landing wait by 74% ([Uber CI costs](https://www.uber.com/us/en/blog/slashing-ci-costs-at-uber/), [bypassing large diffs](https://www.uber.com/en-AU/blog/bypassing-large-diffs-in-submitqueue/)).
*Implication:* package-disjointness is the production test of independence at a large scale.

**P3.10** Zuul's dependent pipeline starts with a window of 20 changes. It adds 1 for each success and halves the window on each failure, for each shared queue ([Zuul gating](https://zuul-ci.org/docs/zuul/9.4.0/gating.html)).
*Implication:* an adaptive window is a simple admission control that follows the failure rate.

**P3.11** Mergify runs batches in parallel when their scopes do not overlap, and grows the batch when every slot is busy ([parallel scopes](https://docs.mergify.com/merge-queue/parallel-scopes), vendor document). bors analysis gives O(E log N) for batches against O(N) for one change at a time, with E failures ([bors forum](https://forum.bors.tech/t/batching-strategy-vs-one-at-a-time/220)).
*Implication:* batches lose when the failure rate is high. LLM lanes fail audits often, so batches stay small.

### 3.4 Scheduling and supervision

**P3.12** kube-scheduler filters the nodes, then scores them. It scores only a part of the nodes: about 50% at 100 nodes and 10% at 5,000 ([scheduler performance](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduler-perf-tuning/)).
*Implication:* at a large fleet, the scheduler must stop at the first K feasible todos, not score all of them.

**P3.13** Work stealing finishes in expected time T1/P + O(T∞) ([Blumofe and Leiserson](https://dspace.mit.edu/handle/1721.1/91874)). The survey found no measured data on the head-of-line cost of a barrier against a rolling pool.
*Implication:* our own simulation (F2.13) is the best evidence for the barrier cost.

**P3.14** Kubernetes nodes send a heartbeat about every 10 s through small Lease objects, with a 40 s grace period ([KEP-589](https://www.kubernetes.dev/resources/keps/589/)). OTP supervisors restart a failed child and escalate to the parent after too many restarts ([OTP supervisors](https://www.erlang.org/docs/25/design_principles/sup_princ.html)).
*Implication:* oversight of each worker is a small lease and a restart rule. Its cost is small next to the tokens of a lane.

**P3.15** Hedged requests send a second copy after the first is slower than about the 95th percentile. In a Bigtable test, this cut the 99.9th-percentile latency from 1,800 ms to 74 ms. It cost about 2% more requests ([The Tail at Scale](https://research.google/pubs/the-tail-at-scale/), numbers from secondary summaries).
*Implication:* a second copy of a lane costs a full lane of tokens and can write `main`. Hedges do not fit lanes (synthesis).

### 3.5 Partitioning theory

**P3.16** A conflict graph colored with k colors runs in k rounds, and each color class runs in parallel. A minimum coloring is NP-hard, so greedy heuristics are the practice ([arXiv 2211.05908](https://arxiv.org/pdf/2211.05908)).
*Implication:* a greedy admission against the cells in flight is the standard answer.

**P3.17** Modularity optimization misses communities smaller than about √(L/2) edges ([Fortunato and Barthélemy](https://arxiv.org/pdf/physics/0607100)). Power-law graphs defeat edge cuts, because a few vertices hold half the edges ([PowerGraph](https://www.cs.princeton.edu/courses/archive/spring16/cos598F/osdi2012-gonzalez-low-gu-bickson-guestrin.pdf)).
*Implication:* hub packages must leave the graph before the split. A hub edit runs alone, as ADR-0134's hub cell does.

## 4. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Admission by package cell, with the hubs in one cell of capacity 1 | Adopt | F2.9, F2.10, P3.1, P3.9, P3.17 |
| A rolling pool that backfills a free slot | Adopt | F2.13, F2.14, P3.13 |
| A lease and a deadline for each lane, with one restart rule | Adopt | F2.12, P3.5, P3.14 |
| A quota shrink only for a family that has no fallback in the chain | Adopt | F2.2, F2.3 |
| Re-route, sweep and the inside-only directory rule (S-A, S-B, S-C) | Adopt now | F2.21, F2.22, F2.19 |
| A supply step that splits a mixed item into a console part and a lane part (S-D) | Adopt | F2.17, F2.20 |
| A narrower manifest (S-E) | Defer to its own amendment of ADR-0064 | F2.18: each fragment needs its own review |
| A countersigned ship class for protected files (S-F) | Reject | F2.18, F2.23: it removes the guard of ADR-0064 |
| The pairwise conflict rate as the main measure | Adopt | P3.3, P3.8 |
| An adaptive admission window, as Zuul | Adopt for the width above 4 | P3.10 |
| A learned conflict predictor that blocks pairs | Reject | P3.7: F1 0.57–0.68 on conflicts |
| Admission by declared files only | Reject | P3.4: agents declare too little |
| Hedged second copies of slow lanes | Reject | P3.15: a copy costs a full lane and writes `main` |
| Batches of many lanes at landing | Reject for now | P3.11: LLM lanes fail often. ADR-0134 owns landing. |

## 5. Refinements to adopt

- **R1** (F2.6, F2.7, F2.17, F2.24): a supply step runs before each plan. It counts the lane-routable items and reports the cause when the supply is below the width.
- **R1a** (F2.19, F2.21, F2.22): the console does S-A and S-B once, and the code does S-C. The estimate was 30–35 items. The measured total is 11 (2026-10-17). S-A and S-B (O2a, #921) moved 3 of 35 candidates, and S-C (O2b) freed 8.
- **R1b** (F2.17, F2.20): the supply step proposes a split of each mixed item (S-D) when the supply is below twice the width. The console approves each split, because a split decides which part is a grader.
- **R2** (F2.2, F2.3): the shrink counts a bench only for a family that is the last in some chain the wave uses.
- **R3** (F2.9, F2.10, P3.9, P3.17): dispatch admits a todo by its package cell, through `PartitionGraph`, with the hub cell at capacity 1. This is ADR-0134 S14.
- **R4** (F2.12, F2.13, F2.14): the rolling pool replaces the barrier. A free slot takes the next admissible todo at once.
- **R5** (F2.12, P3.5, P3.14): each lane holds a lease with a heartbeat and a deadline. A lane past its deadline is stopped and its item is released.
- **R6** (F2.15, P3.12): one orchestrator process reads all lanes through events. It acts on a stall, a quota wall and an overlap, and it scores only the first K admissible todos.
- **R7** (P3.3, P3.8): each wave records its pairwise conflict rate: the share of landed pairs that needed a rebase or a re-audit.
- **R8** (P3.10): above width 4, the admission window grows by 1 for each clean landing and halves on each landing conflict.
