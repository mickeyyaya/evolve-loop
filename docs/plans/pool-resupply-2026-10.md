# Plan: a free pool slot takes the work that became ready after the run started

- **Status:** built, 2026-10-19.
- **Design:** [wave-orchestrator §7](../architecture/wave-orchestrator.md#7-the-rolling-pool) and [§8](../architecture/wave-orchestrator.md#8-the-lane-lease). **Decision:** [ADR-0139](../architecture/adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md) (this plan amends O4 and O5). **Research:** [wave-orchestrator-2026-10](../research/wave-orchestrator-2026-10.md) and §2.2 below.
- **Package notes:** [internal-fleet.md](../architecture/packages/internal-fleet.md), [internal-orchestrator.md](../architecture/packages/internal-orchestrator.md).

**Terms used throughout:**

- **Run:** one call of `fleet.RunPool`, from one boundary to the next boundary.
- **Backlog:** the todos that the run holds. Before this plan, the plan source gave the backlog one time, at the run start.
- **Resupply:** one more call of the plan source in the same run, after a lane exit leaves a slot free.
- **Deadline stop:** the lease stops a lane past its deadline (`LOOP_ORCH_LANE_DEADLINE`, action `stopped`).

## 1. Request

The operator, 2026-10-19, after wave 130: "Items that become dispatchable mid-wave are never supplied." And: "a todo whose lane exited by `LOOP_ORCH_LANE_DEADLINE` must not be re-queued at once in the same wave."

## 2. Facts

### 2.1 Local evidence

| Id | Fact | Source |
|---|---|---|
| F1 | `RunPool` gets `backlog []Todo` one time. Each backfill reads only that list. | `go/internal/fleet/pool.go:56` on `origin/main` |
| F2 | The loop gives the backlog from `productionPoolPlanFn(..., loopwave.PoolSupply(fc), ...)` before the run, and never again. | `go/cmd/evolve/cmd_loop_window.go:184`, `go/cmd/evolve/cmd_loop_pool.go:28-42` |
| F3 | Wave 130, 02:28Z to 05:27Z: the pool ran live 1 of target 4. It published `LOOP_ORCH_POOL_IDLE` after each exit. `evolve inbox rank --list ready` showed 23 ready items. | operator measure, 2026-10-19 |
| F4 | The handoff items H2 to H5 became ready when H1 shipped in cycle 1952. The run did not hold them, because the plan ran before cycle 1952. | operator measure, 2026-10-19 |
| F5 | A rolling-pool run lasts while one lane lives. Thus no boundary came to plan again for 3 hours. | §7 of the spec: "A run ends when the supply is empty and no lane is in flight" |
| F6 | After a deadline stop, `Pool.recover` returned true on the first stop. `PoolConfig.Exited` then put the same todo at the head of the queue, and the pool relaunched it at once. | `go/internal/orchestrator/pool.go:126-127`, `go/internal/fleet/pool.go:105` on `origin/main` |
| F7 | That relaunch had no event for 76 minutes. | operator measure, 2026-10-19 |
| F8 | The plan source keeps a claim whose holder has a live lease. So a second plan in the same run does not release the claims of lanes in flight. | `go/internal/inboxmover/holder.go:42` (`Keeps`), `holder.go:93` (`live` → `HolderLive`) |
| F9 | The run length limit `run_max_lanes` defaults to `4 × fleet.count`, 16 lanes at width 4. | `go/internal/policy/fleet_orchestrator.go:66-69` |
| F10 | The lease table of the loop lives for the whole batch, one table for each stage. Its stop count for a todo thus lives across runs. | `go/cmd/evolve/cmd_loop_orchestrator.go:36-46` |

*Implication:* the slot stays free because the run cannot see new supply (F1, F2), and the run does not end, because one lane lives (F5). The fix must bring new supply into the run, not shorten the run.

### 2.2 Current practice (live search, 2026-10-19)

| Id | Practice | Source | Lesson for the pool |
|---|---|---|---|
| P1 | A Temporal worker polls its task queue only when a slot is free. The slot, not a timer, starts the poll. | [Temporal: worker performance](https://docs.temporal.io/develop/worker-performance), [worker capacity](https://docs.temporal.io/troubleshooting/worker-capacity.md) | Ask the source for work when a slot frees, and only then. |
| P2 | Celery advises `worker_prefetch_multiplier = 1` with `acks_late` for long tasks. A deep prefetch holds tasks behind a long task while other processes idle. | [Celery: optimizing](https://docs.celeryq.dev/en/main/userguide/optimizing.html) | A run that takes its whole backlog at the start is a deep prefetch. Take a small supply, and take more on demand. |
| P3 | In the Kubernetes fine-parallel work-queue Job, each worker takes the next item, and loops until the queue is empty. | [Kubernetes: fine parallel processing](https://kubernetes.io/docs/tasks/job/fine-parallel-processing-work-queue) | The worker pulls from the live queue, not from a list fixed at Job start. |
| P4 | Kubernetes `backoffLimitPerIndex` gives each index its own retry budget, and a failed index goes to `status.failedIndexes`. | [Kubernetes v1.33 blog](https://kubernetes.io/blog/2025/05/13/kubernetes-v1-33-jobs-backoff-limit-per-index-goes-ga/) | Count retries for each todo, and route a todo past its budget to a separate list (the console). |
| P5 | In Kafka share groups (KIP-932), a released record goes back to the available state for a later delivery. A record past the delivery-count limit is archived. | [KIP-1222](https://cwiki.apache.org/confluence/spaces/KAFKA/pages/388893095/KIP-1222+Acquisition+lock+timeout+renewal+in+share+consumer+explicit+mode) | A stopped todo is released to the queue, not redelivered at once to the same consumer. The second stop parks it. |

## 3. Goals and non-goals

Goals:

- G1. A free slot takes a todo that became ready after the run started, in the same run.
- G2. The new todo passes the same filters as the first plan: the rank, the console-route filter, the protected-path filter and admission by cell.
- G3. An empty inbox costs one plan call for each lane exit, and no spin.
- G4. A todo that a deadline stop ended does not launch again in the same run.

Non-goals:

- The rolling pool of `scheduling: pool` at stage `shadow` keeps its fixed backlog. It has no launch limit, so a resupply can make its run endless.
- The run length stays `run_max_lanes`. This plan does not change the boundary.

## 4. Candidate designs

| Id | Design | How a new todo reaches a slot |
|---|---|---|
| A | A `Resupply` port on `PoolConfig`. After a lane exit leaves a slot free, the pool calls the plan source again and adds the todos that it does not hold. | at the next lane exit |
| B | End the run when a slot stays free and the supply is short. The boundary plans again. | after every lane in flight ends |
| C | A timer in the pool plans again each N minutes. | up to N minutes later |
| D | Plan a larger backlog at the start, with items that wait on `deps`. The pool checks the `deps` before each launch. | at the next lane exit, if the item was in the inbox at the start |

Scores, 1 (bad) to 5 (good):

| Criterion | Weight | A | B | C | D |
|---|---|---|---|---|---|
| Time until a free slot is used (G1) | 3 | 5 | 1 | 3 | 4 |
| One plan path for all filters (G2) | 3 | 5 | 5 | 5 | 2 |
| Push, not poll (G3) | 2 | 5 | 5 | 1 | 5 |
| Size of the change | 1 | 4 | 3 | 3 | 2 |
| **Weighted total** | | **44** | 31 | 31 | 30 |

**Why A wins.** A is the Temporal rule (P1): a free slot asks for work. It re-runs the same `PlanFn` path, so the rank, the route filter, the protected-path filter and admission by cell all apply (G2). It runs only on a lane exit, so it needs no timer (G3).

- B keeps the run correct, but a free slot waits for the slowest lane. That is the barrier that §7 removed (66% busy at width 4, research F2.13).
- C adds the only second timer in the orchestrator and delays work by up to N minutes.
- D cannot see an item that is filed after the start. For an item that waits on `deps`, D must copy the `deps` rule of `inboxmover` into the pool. The plan then has two dispatchability rules.

*What changes our mind:* one plan call can take more than a few seconds. Then a resupply on each exit delays the settle of the next exit. Then the plan call moves to its own goroutine, with the result sent back on a channel.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | `fleet.PoolConfig.Resupply func(held []Todo) []Todo`. The pool gives the port a copy of each todo that it holds. `settle` calls it when the fill after an exit leaves a slot free and the pool is open (no stop, under `MaxLaunches`). | F1, F3, P1 | a call on each `Kick`: the oversight kicks on a landing, not on a free slot |
| D2 | The pool adds only the todos whose `ID` it does not hold. It holds each `ID` of the start backlog and of each resupply. | F8, G4 | dedupe in the port: the port does not know which todos the pool took |
| D3 | The results grow with the backlog. `results[i]` is the result of the i-th todo that the pool held, and `Result.Index == i`. A lane goroutine sends its result on the completion channel, and only the run loop writes the results slice. | a goroutine that writes a slice that the run loop grows is a data race | a fixed results slice: it has no place for a resupplied todo |
| D4 | `planAndRunPool` and `dispatchPoolIteration` no longer return their backlog. No caller read it, and after D3 it did not match the results. | D3 | keep it: a caller that indexes the results by that backlog reads the wrong todo |
| D5 | The loop gives the resupply only to the orchestrator pool. The rolling pool ignores it. | F9 | the rolling pool too: it has no `MaxLaunches`, so its run can never end |
| D6 | A plan error in a resupply writes one `[loop] WARN: orchestrator: pool N resupply: ...` line and adds nothing. The slot waits for the next exit. | AGENTS.md rule 12 (fail loudly) | end the run on the error: one bad read of the inbox then costs the lanes in flight |
| D7 | Each added todo publishes `LOOP_ORCH_POOL_RESUPPLIED` (INFO, fields `todo`, `live`, `target`). | F7: an operator must see the pool act | no event: the console cannot tell a resupply from a backfill |
| D8 | A deadline stop releases the claim. The todo does not go back in the queue of the same run. `PoolConfig.Exited` is removed. A later run can take the todo again. The second stop of the todo in the batch parks it with the reason `deadline twice`. | F6, F7, F10, P4, P5 | relaunch at once (the old rule): it repeated the stall for 76 min. Park on the first stop: one slow day sends good work to the console. |
| D9 | The resupply plans the same wave at the depth `PoolSupply + len(held)`. The top items by rank can include held todos: pending todos and todos that a deadline stop released. The extra depth leaves room for at least `PoolSupply` new todos. | the architecture review of this change, 2026-10-19: a fixed depth lets the held todos fill the cut | an exclusion list inside the plan: it changes `loopwave` and `triagecap` for the same result |

## 6. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| R1 | `fleet.PoolConfig.Resupply`, `PoolResupplied`, the `known` set and the result channel (`pool.go`) | `TestRunPool_AResupplyAfterAnExitFillsTheFreeSlotsBackToTheTarget`, `TestRunPool_AResupplyNeverAddsAnIDTheBacklogAlreadyHolds`, `TestRunPool_AnEmptyResupplyEndsInIdleWithOneCallForEachExit`, `TestRunPool_EachResuppliedTodoIsNamedByOneTransition`, `TestRunPool_AClosedPoolNeverAsksForResupply` | this change |
| R2 | `orchestrator.Pool.Resupply`, `CodePoolResupplied`, `recover` with no re-queue | `TestPool_ResupplyFeedsTheFreedSlotFromThePort`, `TestLease_ALanePastItsDeadlineIsReleasedAndNotRelaunchedInTheSameRun`, `TestLease_ATodoStoppedInTwoRunsIsParkedForTheConsole` | this change |
| R3 | `poolPlanAt`, `poolSupplyFn`, the resupply of `planAndRunPool`, `orchestratorResupply` (`cmd/evolve`) | `TestPlanAndRunPool_TheResupplyPlansTheSameWaveDeeperByTheHeldTodos`, `TestOrchestratorResupply_PassesThePlanAndWarnsOnAPlanError`, `TestOrchestratedRun_TheResupplyFeedsTheFreedSlot`, `TestRollingPool_KeepsTheFixedBacklogAndNeverAsksForResupply` | this change |

## 7. Order

1. R1 first, because R2 and R3 call the port.
2. R2 next, because the deadline rule and the port meet in `Pool.Run`.
3. R3 last, because it wires the plan source into the port.

## 8. Verification

| Goal | Before | After | Measure |
|---|---|---|---|
| G1 | a run with 2 todos and a target of 4 peaks at 2 live lanes | the same run peaks at 4 when the resupply gives 3 todos | `TestRunPool_AResupplyAfterAnExitFillsTheFreeSlotsBackToTheTarget` |
| G3 | no call | one call for each exit, then `idle` | `TestRunPool_AnEmptyResupplyEndsInIdleWithOneCallForEachExit` |
| G4 | the stopped todo launched again at once | the next todo launches; the stopped todo has one result | `TestLease_ALanePastItsDeadlineIsReleasedAndNotRelaunchedInTheSameRun` |
| G1, live | live 1 of 4 for 3 hours (F3) | `LOOP_ORCH_POOL_RESUPPLIED` after an exit, and live back to the target while the inbox has admissible items | `evolve wave watch` on the first run after the merge |

## 9. Risks

| Risk | Control |
|---|---|
| A slow plan call delays the settle of the next exit. | The completion channel has one place for each slot, so a lane never blocks on its send. The "what changes our mind" line of §4 gives the next step. |
| A resupply adds a todo whose cell is busy. | The pool admits through `Next` (admission by cell), as for every pending todo. A held todo waits. |
| The run never ends while the inbox has items. | `run_max_lanes` (F9) ends the run, and `Resupply` is not called when the pool is closed. |
| A todo that the run released after a deadline stop comes back in the resupply. | D2: the pool holds its `ID`, so it does not launch again in the same run. |

## 10. Limits

- The plan source releases stale claims (`releaseStaleClaims`) at each call. A resupply thus writes to the inbox in the run. The release keeps each claim whose holder has a live lease (F8), so the lanes in flight keep their claims.
- On the inbox seed path (no triage decision of a prior cycle), the plan needs at least 2 file-disjoint items. A resupply with one ready item then writes one WARN and adds nothing. Mid-run, the last cycle has a triage decision, so the widen path runs, and it has no such floor.
- The resupply call runs on the run loop. A slow plan delays the settle of the next exit (§9).

## 11. Open questions

- Q1. Does the rolling pool of `scheduling: pool` need a launch limit, so it can take a resupply too? Not now: no policy runs it at stage `shadow` (`.evolve/policy.json` sets stage `enforce`).
- Q2. The plan source reads the triage decision of the last cycle. Mid-run, the last cycle is a lane of the same run. The widen then fills from the inbox (`WidenNarrowDecision`). We accept this, because the same path runs at a boundary.
