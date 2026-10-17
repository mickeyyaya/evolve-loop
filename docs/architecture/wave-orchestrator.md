# Wave orchestrator: one component supplies, admits, backfills and oversees every lane

> [ADR-0139](adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md) (Proposed, 2026-10-17) · plan: [wave-orchestrator-2026-10](../plans/wave-orchestrator-2026-10.md) · research: [wave-orchestrator-2026-10](../research/wave-orchestrator-2026-10.md) (F2.1 to F2.24, P3.1 to P3.17, R1 to R8).
>
> This file is the spec of record for how the loop runs its lanes. It holds the census, the sizing, the admission, the pool, the lease, the oversight, the window, the measures and the scale limits. The plan holds the decisions, the components and the order. The ADR holds the decision. Landing stays in [fleet-landing-queue](fleet-landing-queue.md) (ADR-0128, ADR-0134). The events come from [event-channels](event-channels.md) (ADR-0127).
>
> Written in the Issue / Gap / Solution shape.
>
> **Terms used throughout:**
> - **Lane:** one fleet cycle, with its own process and its own worktree.
> - **Slot:** a place for one lane. The width is the number of slots.
> - **Todo:** one unit of work that a lane takes, from the inbox or from a triage decision.
> - **Supply:** the todos that a lane can take now: lane-routable, with their `deps` met, and not claimed.
> - **Cell:** a module package that a todo edits. `internal/admission` computes it from `go list -deps`. The unit of admission.
> - **Hub cell:** one cell that holds the hub packages. Its capacity is 1.
> - **Admissible:** a todo whose cells overlap no cell of a lane in flight.
> - **Lease:** the record that a lane is alive. It holds the last heartbeat and the deadline.
> - **Window:** the number of slots that the orchestrator fills. It is at most `fleet.count`.
> - **Pairwise conflict rate:** the share of landed lanes that needed a rebase or a re-audit at landing.

## Table of contents

- [1. Goals](#1-goals)
- [2. Architecture](#2-architecture)
- [3. A worked example](#3-a-worked-example)
- [4. The supply census](#4-the-supply-census)
- [5. Sizing](#5-sizing)
- [6. Admission by cell](#6-admission-by-cell)
- [7. The rolling pool](#7-the-rolling-pool)
- [8. The lane lease](#8-the-lane-lease)
- [9. The oversight loop](#9-the-oversight-loop)
- [10. The adaptive window](#10-the-adaptive-window)
- [11. Measures](#11-measures)
- [12. Scale](#12-scale)
- [13. Config keys, signals and commands](#13-config-keys-signals-and-commands)
- [14. Stages](#14-stages)
- [Guards](#guards)
- [Limits](#limits)

## 1. Goals

| Goal | Measure | Target |
|---|---|---|
| G1. At least 4 lanes at once | the median realized width over 3 runs | ≥ 4 when the supply is ≥ 4 |
| G2. Few conflicts | the pairwise conflict rate | ≤ 5% |
| G3. Busy slots | the busy lane time over the slot time | ≥ 0.85 at width 4 |
| G4. One overseer | the share of stalls and deadlines that the orchestrator acts on | 100% |
| G5. Scale | the cost of each step as a function of N, and a soak at N = 16 | stated, and the soak passes |
| G6. Known supply | each plan records its supply and the cause of each refusal | every plan |

The order of the goals is the order of the work. Supply first: an empty supply makes every other goal void. Then conflicts: more lanes without admission only make more conflicts. Then busy slots, oversight and scale.

## 2. Architecture

The orchestrator replaces the fire-and-wait wave on the loop's fleet path. It is deterministic Go in the loop process, and it runs no LLM (ADR-0139 decision 1). It reuses built parts: `fleet.RunPool`, `internal/admission`, `fleet.FreshenSpecs`, `clihealth`, `inboxrank` and the event reader of ADR-0127.

```mermaid
flowchart LR
  subgraph supply[Supply]
    I[inbox rank] --> C[supply census §4]
    T[triage decision] --> C
  end
  C --> A[admission by cell §6]
  S[sizing §5] --> W[window §10]
  W --> P[rolling pool §7]
  A --> P
  P -->|launch| L1[lane 1]
  P -->|launch| L2[lane 2]
  P -->|launch| Ln[lane N]
  L1 & L2 & Ln -->|events| E[(channels: cycle, ship, errors, loop)]
  E --> O[oversight loop §9]
  O -->|free cells, stop lane, pause family| P
  O --> M[measures §11]
  L1 & L2 & Ln -->|ship| Q[landing: ADR-0128, ADR-0134]
  Q -->|ship.landed| E
```

The orchestrator owns four kinds of state. All of it lives in memory, and the run record under `.evolve/waves/` keeps a copy:

| State | Shape | Writer |
|---|---|---|
| the cell table | cell → the lane that holds it | admission (§6), oversight (§9) |
| the lease table | lane → the last heartbeat and the deadline | the lease (§8) |
| the window | one integer | sizing (§5), the window (§10) |
| the measures | counters for each run | oversight (§9) |

## 3. A worked example

This example uses real packages of this repository. `fleet.count` is 4, and `agy-claude` is benched, as on 2026-10-17 (F2.2).

1. **Sizing.** `agy-claude` has `claude` after it in each chain that the wave uses. The bench costs no slot, so the window is 4 (§5).
2. **Supply.** After the routing remedy of the plan (O2a), the census finds 6 admissible todos. Their cells, from `internal/admission`:

| Todo | Files | Cells |
|---|---|---|
| a | `go/internal/inboxrank/inboxrank.go` | `inboxrank` |
| b | `go/internal/quotastate/windows.go` | `quotastate` |
| c | `go/internal/fleet/pool.go` | `fleet` |
| d | `go/internal/inboxrank/explain.go` | `inboxrank` |
| e | `go/internal/sysexec/command.go` | the hub cell |
| f | `docs/operations/wave-operations.md` | none (outside `go/`) |

3. **Admission.** The pool takes the todos in rank order: a, b, c, d, e, f.
   - a, b and c have disjoint cells. Each one takes a slot.
   - d shares `inboxrank` with a. d waits.
   - e takes the hub cell. Its capacity is 1, so e takes the fourth slot.
   - f has no cell in `go/`, so it is always admissible. The window is full, so f waits for a slot.
4. **Backfill.** Lane b lands after 22 minutes. `ship.landed` frees the cell `quotastate`. The pool takes the next admissible todo, f, at once.
5. **A cell frees.** Lane a lands. Its cell `inboxrank` frees, and d becomes admissible. d takes the next free slot.

Today's file partition puts a and d in one lane, because their files differ, and the wave waits for its slowest lane. The orchestrator keeps them apart in time and keeps every slot busy.

## 4. The supply census

### Issue

On 2026-10-17, 0 of 230 inbox items can go to a lane (F2.6). The lane-routable supply fell from 41 on 2026-09-15 to 1 (F2.17). Three runs planned zero lanes (F2.4). The loop finds out only after K starved waves, through one inbox item (F2.16).

### Gap

No step counts the supply before a plan. The plan reads the last triage decision and widens it from the inbox (F2.5). When both are empty, the plan is empty, and the cause is not recorded. The routing rules send 170 items to the console by derivation (F2.7), and nothing tells the console which rule holds which item.

### Solution

Before each plan, the census reads the ranked inbox through `inboxrank.Order` and classifies each item:

| Class | Rule | Source |
|---|---|---|
| `admissible` | lane-routable, `deps` met, not claimed, and its cells are free | `inboxbatch.PartitionConsole`, §6 |
| `waiting_cell` | lane-routable, but a lane in flight holds one of its cells | §6 |
| `waiting_deps` | a `deps` item is open | `inboxrank` |
| `console_route` | an explicit `route: console*` | `inboxbatch.IsConsoleRoute` |
| `console_kind` | a `pipeline-*` kind | `consoleroute.go:49-56` |
| `console_protected` | a protected fix surface | `consoleroute.go:58-80` |

The census writes one record for each plan into the run record, and it publishes `orchestrator.supply` on the `loop` channel. The record holds the count of each class and the ids of the console items with their reasons.

When the admissible count is below the window, the census publishes `LOOP_ORCH_SUPPLY_SHORT` with the counts. When it is below twice the window, the census writes a split proposal for each mixed item. A mixed item names protected files and files that are not protected. The console accepts a proposal with `evolve inbox split <id>` (plan O2c). The split makes two items:

- The **console part** keeps every protected file, the route and the acceptance that a grader judges.
- The **lane part** keeps the files that are not protected. It has `deps` on the console part when it needs the console part first.

The census never routes an item itself. ADR-0074 keeps the routing authority, and the four guards of F2.23 keep the protected files safe.

**The directory rule (built, O2b, 2026-10-17).** A declared directory routes to the console only when it lies inside a protected fragment (ADR-0139 decision 5). `guards.IsProtectedScope` now gives the same answer as `guards.IsProtectedSurface` (`go/internal/guards/integrity_surface.go:180`). Thus `files: ["go/internal/core/"]` is lane work, and `go/internal/bridge/sub/` stays console work.

A protected file spelling stays binding (`go/internal/inboxbatch/consoleroute.go:31-33`). Triage's breaker uses the same predicate, so it refuses a card that names a protected file and accepts the directory. On this tree, `evolve inbox rank --list ready` showed 1 item before the change and 9 after it.

## 5. Sizing

### Issue

Every run of 2026-10-16 and 2026-10-17 shrank the wave from 3 to 2. 23 of the 25 benches named `agy-claude` (F2.2).

### Gap

`Engine.Size` passes every active bench to `QuotaAwareCount`, and each bench costs one slot (`size.go:15`, `quota.go:9-39`). The shrink does not check if the chain has a family after the benched one. `agy-claude` has `claude` after it in the deep and top chains, so a lane on that seat walks to `claude`.

### Solution

**Status: Built (O1, 2026-10-17).** `fleet.ChainAwareBenches` (`go/internal/fleet/quota_chain.go`) gives the benches that cost a slot. `Engine.Size` passes them to `fleet.QuotaAwareCount`.

The sizing step reads each `cli_routing` chain as families. A chain costs one slot when every family in it is benched. The slot goes to the last family of that chain. Thus two chains with the same last family cost one slot. A bench of any other family costs nothing and writes one INFO line. The line names the healthy family that a lane walks to, or says that no lane chain uses the family.

`cliroute.FamilyChains` reads the chains from `default`, `work`, `agents.<agent>.cli` and `tiers.<tier>.clis`. It does not read `after_chain`, so the set is conservative. The `Chains` port of the `Engine` calls it. The caller of `Engine.Size` loads the policy once (`loopwave.LoadRouting`) and passes the `cli_routing` block, so `Size` reads no second snapshot. When the policy is unreadable, or the table gives no chain, every bench costs one slot, as before. An unreadable policy also writes one WARN line, from `LoadRouting`.

Known limits: the step charges one slot for each empty chain. When many lanes share one chain and that chain empties, the shrink undercounts. `fleet.min_lanes` bounds the undercount, and the O6 oversight loop re-sizes on the bench event. A scoped bench (for example `claude@opus`) marks its whole family as down, which overcounts and is the safe direction.

Example: on 2026-10-17, `agy` and `agy-claude` are benched. Each chain of the table ends in `claude`, and `claude` is healthy. The wave keeps all 4 lanes.

The window starts at `fleet.count`, minus the slots that the benches cost, and never below `fleet.min_lanes`. The budget step of `fleetbudget` stays as it is (`size.go:16-36`).

The sizing step runs at the start of each run and again on each `clihealth` bench event (§9). The window can thus shrink while lanes run. A shrink never stops a lane in flight. It only stops the next admission.

## 6. Admission by cell

### Issue

The live partition is file-disjoint (F2.8). File-based parallel agents gain nothing over sequential work (P3.1), and uncoordinated agents conflict in 20–42% of pairs (P3.3). A conflict that reaches landing costs about 24 minutes (F2.11).

### Gap

`PartitionGraph` computes package sets, but nothing calls it (F2.9). ADR-0134 designs admission by cell (S14), but it is not built (F2.10). A card with no `files` gets its id as its only file, so it never conflicts (F2.8).

### Solution

**Built (shadow), 2026-10-17:** [`internal/admission`](packages/internal-admission.md) (plan O3). The admission step is ADR-0134 S14, built for the orchestrator. In the `shadow` stage, the loop runs it after the file partition of each wave. It records each todo that it holds back, and it changes no lane. At the stage `enforce`, the barrier wave does not run, and the pool of §7 admits each todo by this rule.

1. **The edited packages.** `E(t)` is the set of module-internal packages that todo `t` edits: the packages of its Go files. Another path under `go/` maps to its nearest enclosing package. A file outside `go/` maps to no package.
   - Known limit: a todo with only files outside `go/` (docs, scripts) maps to no cell. So does a todo with non-Go files that no package encloses. Such a todo conflicts with nothing, and admission admits it at once. Only the landing proof of ADR-0128 finds a clash between such todos.
2. **The reach.** `D(p)` is the set of module-internal transitive imports of package `p`, from `go list -deps`. It excludes the hub packages. `D(E)` is the union of `D(p)` for each `p` in `E`.
3. **The conflict rule.** Two todos `t1` and `t2` conflict when one of these holds:
   - `E(t1)` and `E(t2)` meet;
   - `E(t1)` meets `D(E(t2))`;
   - `E(t2)` meets `D(E(t1))`;
   - both hold the hub cell.
4. **The hub cell.** A todo that edits a hub package holds the hub cell. The hub cell has a capacity of 1, so two such todos conflict. An import of a hub never conflicts, because `D` excludes the hubs. The global-zone files `go.mod`, `go.sum` and `.evolve/policy.json` (`fleet.IsGlobalZone`) also map to the hub cell, and so does `go/Makefile`.
   - The module-root files `go/go.mod`, `go/go.sum` and `go/Makefile` sit in no package. The hub cell is thus the only cell that holds them.
5. **A todo with no files.** It takes the packages of the `go/internal/<pkg>` paths in its inbox title or summary. When its text names no known package, it takes the hub cell (ADR-0139 decision 8, plan D7). A todo whose only file is its own id (the placeholder of `fleet.TodosFromTriage`) has no files.
6. **The hub set.** A hub is a package that at least `fleet.orchestrator.hub_min_importers` module packages import, directly or transitively. On 2026-10-17 the counts were `sysexec` 151, `log` 127, `atomicwrite` 126, `signalcenter` 122, then `ipcenv` 116. The default 120 gives these four hubs. The transitive count reproduces ADR-0134 research F2.16. The direct count does not.
7. **The test.** A todo is admissible when it conflicts with no lane in flight and with no todo admitted before it. `Admit` records the cells of each decision and the lane or todo that blocks it.
8. **The release.** `ship.landed`, a lane failure or a lane stop removes the lane from the in-flight list (§9).
9. **Rank order.** The step takes the todos in `inboxrank` order. It stops after `scan_k` admitted todos (§12). A todo that waits keeps its rank. It does not lose its place to a later todo of lower rank when its cell frees.

The cell table is the shared view of all lanes in flight that the console has by hand today (F2.15). `evolve wave status` prints it (plan O7).

The cells come from the tree at the run's base. A lane that adds an import adds an edge that the table does not hold. The landing proof of ADR-0128 finds that edge at landing. Admission thus lowers the conflict rate, but it does not replace the landing proof.

**The shadow never delays a launch.** The shadow observation, including the `go list`, runs in its own goroutine while the lanes launch. A panic in it is recovered and logged as one WARN, and the launch is not told. The loop joins the observation after the launch returns. Its line and events thus appear once the lanes are done. The launched specs never change.

**The shadow record.** For each wave, the loop log gets one line: `[loop] wave N: admission shadow (stage S): admission launches A lane(s), the file partition launches F; W todo(s) wait for a cell`. A is the admitted count, capped at the wave count. Each todo that waits gets one `LOOP_ORCH_ADMIT_WAIT` event with `todo`, `rank`, `cells`, `blocked_by`, `blocked_by_lane` and `stage`. The wave run record under `.evolve/waves/` belongs to `evolve wave next`, not to the inner waves of the loop, so it gets no field yet.

**The first measure (2026-10-17, width 4).** The ready list is empty, so both widths are 0. Each of the last 10 triage decisions holds 1 todo, so both widths are 1. The 9 todos of those decisions as one wave give 4 and 4.

In 30 windows of 8 pending items, admission gives 58 lanes against 120. The cause: 120 of the 244 pending items edit `cmd/evolve` or `internal/core`, which import most of the module. The S14 exit takes 3 live runs ([internal-admission.md](packages/internal-admission.md) §Findings).

## 7. The rolling pool

### Issue

A barrier wave waits for its slowest lane. We simulated it with the lane durations of cycles 1900–1920. A barrier of 4 lanes keeps about 66% of the slot time busy, and 16 lanes keep 49% (F2.13).

### Gap

`Supervisor.Run` returns only after `wg.Wait()` (`fleet.go:94`). `RunPool` backfills a free slot, but it runs only under `scheduling: pool`, it uses the file rule, and its `onTransition` hook is `nil` (F2.14).

### Solution

**Built (enforce only), 2026-10-17:** [`internal/orchestrator`](packages/internal-orchestrator.md) `Pool` (plan O4). At the stage `shadow`, the barrier wave runs as before, and the pool does not run. At the stage `enforce`, the loop runs the pool in place of the wave `Supervisor`, whatever `scheduling` says. A policy with `scheduling: pool` at `enforce` gets one WARN at batch start.

The orchestrator runs `RunPool` as its scheduler, with three changes:

1. **Admission.** The pool asks the admission step (§6) for its next todo, in place of its own file rule.
2. **Freshness.** The pool runs `FreshenSpecs` on each todo just before launch, as the wave path does now (`launcher.go:36-45`).
3. **Transitions.** Each transition (`launched`, `exited`, `backfilled`, `idle`) goes to the `loop` channel through `onTransition`, as `LOOP_ORCH_LANE_LAUNCHED`, `LOOP_ORCH_LANE_EXITED`, `LOOP_ORCH_LANE_BACKFILLED` and `LOOP_ORCH_POOL_IDLE`. The console and `evolve wave watch` read the channel.

`RunPool` takes the three changes as hooks of `fleet.PoolConfig` (`Next`, `Prepare`, `Exited`), so the scheduler stays one function. With no hook, it keeps the file rule. The in-flight list of admission is the list of running lanes, so a lane exit frees its cells at once. The module graph loads once for each run. If it does not load, the run admits by the file rule and the loop writes one WARN.

A run ends when the supply is empty and no lane is in flight, or on an operator stop (`evolve loop-stop --wait`). The wave boundary work (merge, `gc`, the code-index refresh, `cli update`) runs at the end of a run, as it does now after a wave. The rule "merge only at the boundary" thus holds unchanged: a run is one boundary-to-boundary unit.

A run has a length limit, `fleet.orchestrator.run_max_lanes`, in lanes launched. It keeps boundaries regular, so merges and binary refreshes still happen. Its default is `4 × fleet.count`.

The operator stop is the brake file of `evolve loop-stop` or a cancelled batch. The pool reads it at each admission. An admission happens only at a run start and at a lane exit, so the check is not a poll. At the run end, the loop prints `[loop] pool N: X/Y lanes ok (orchestrator, target=W)`. The boundary work runs at the next iteration.

**The soak (2026-10-17, goal G5).** `evolve fleet soak --count 16 --pool` ran 64 simulated lanes at `enforce`. The peak in-flight was 16 of 16, and all 64 lanes passed. The run took 43.6 s, with 88.0 lanes a minute and a utilization of 0.91 ([internal-orchestrator.md](packages/internal-orchestrator.md) §Findings).

An earlier run found a cross-lane race on `knowledge-base/cycles/`, which the barrier hides. The tree-diff guard of a lane counted the dossier of another cycle as its own leak. The guard now keys a dossier to its cycle, and the soak ran with no exclusion.

**An empty pool always admits.** When no lane is in flight and the supply is not empty, `fleet.RunPool` launches the top todo even when admission refuses it. Nothing can conflict with an empty set. A refusal there can only end the run with work left.

## 8. The lane lease

### Issue

A wave lane has no deadline (F2.12). The console decides by hand when a stuck lane has run too long (F2.15).

### Gap

`Supervisor` has `CycleTimeout`, but only `evolve campaign` sets it (`cmd_campaign.go:194`). The phase observer of each lane signals `LIVENESS_PHASE_STALLED`, but it kills nothing, and no component reads its signals across lanes.

### Solution

**Built, 2026-10-17:** [`internal/orchestrator`](packages/internal-orchestrator.md) `Leases` (plan O5). The pool at `enforce` holds an `enforce` lease table. The barrier wave at `shadow` holds a `shadow` lease table, so the stage exit is measured on live waves.

Each lane gets a lease when it launches. The lease holds:

- **The heartbeat:** the time of the freshest live signal of that lane. The lease adds no new writer: the lane's existing files are its heartbeat.
  - `RunsHeartbeat` finds the cycle of the lane by its `lane-scope.json`. The heartbeat is the latest time of two kinds. One is the last time stamp of its `signals.ndjson`. The other is the modification time of each `<agent>-pane-watch.json`, `<agent>-events.ndjson` and `<agent>-stdout.log` in the cycle workspace. The newest of these files is a file of the current phase.
  - A long `build` or `go test` phase writes no cycle event for many minutes. But the bridge writes the pane-watch snapshot again each time the pane changes, and the observer writes its heartbeat event each 12 polls (`observerengine.HeartbeatEvery`). Thus a busy phase keeps a fresh heartbeat.
  - When no cycle holds the lane, the heartbeat is unknown. The lease then reports no stall, and it checks again after `stall_minutes`. It does not count from the launch time, because a lane can start before its cycle directory exists.
- **The deadline:** the 95th percentile of the lane time over the last 50 cycles, without the longest lane time. Its floor is `fleet.orchestrator.deadline_floor_minutes` (default 60). The lane time of a cycle is the span of its `signals.ndjson`. The loop computes the deadline once for each batch. On 2026-10-17 the 44 lane times of cycles 1873 to 1928 give 95.1 minutes (plan Q1).

**Why the deadline removes the longest lane.** With 44 samples, the nearest-rank p95 is the third-longest lane time, so the two longest lanes set it. We compared three rules on the same 44 samples (median 39.0 minutes, longest 118.1 minutes):

| Rule | Deadline | Decision |
|---|---|---|
| The p95 of all samples | 98.9 min | rejected: one more long lane moves it to the next outlier |
| The p95 of the samples without the longest lane | 95.1 min | chosen |
| The p90 times 1.25 | 116.2 min | rejected: the factor 1.25 has no measure, and the value is almost the longest lane |

The chosen rule stays a percentile of measured lane times, and one outlier cannot set it. With one sample, the rule keeps that sample.

The oversight loop (§9) acts on a lease:

| Condition | Action |
|---|---|
| No heartbeat for `stall_minutes` (default 30) | publish `LOOP_ORCH_LANE_STALLED`. The lane keeps its slot. |
| The deadline passes | stop the lane through its process-group stop, release its inbox claim, free its cells, and publish `LOOP_ORCH_LANE_DEADLINE`. |
| The lane stops twice for one todo | park the todo with `route: console` and the reason `deadline twice` |

The stage decides the action:

| Stage | A stall | A passed deadline | A lane that exits after its deadline |
|---|---|---|---|
| `shadow` | `LOOP_ORCH_LANE_STALLED` once | `LOOP_ORCH_LANE_DEADLINE` with `action` `recorded`. Nothing stops. | `LOOP_ORCH_LANE_LATE_EXIT` with its `status`: `ok` is a stop that the exit rule of §14 counts |
| `enforce` | `LOOP_ORCH_LANE_STALLED` once | `LOOP_ORCH_LANE_DEADLINE` with `action` `stopped`, then the stop, the claim release and one more try | `LOOP_ORCH_LANE_LATE_EXIT` with `stopped` true. A second stop parks the todo. |

One timer serves all lanes. A heap holds the next time of each lane: the earlier of its deadline and its stall time. The timer is armed for the top of the heap. The deadline timer is the only timer in the orchestrator.

The restart rule follows OTP (P3.14): a todo gets one more try, and a second stop sends it to the console. A stop never runs a second copy of a lane in parallel, because a copy costs a full lane and writes `main` (P3.15).

A stop must not be a broad kill. The orchestrator stops only the lane that its own launch made, and never by a name match. The lease cancels the child context of that launch. The launch of `cmd/evolve` (`execCycleLaunch`, which uses `sysexec.Command`) then sends SIGTERM to that one lane process, and the lane stops its own phase process groups. The package holds no pid and imports no process package. A guard test checks this.

**The dispatch reap after a stop.** The lane reaps its own dispatches when it stops (`bridge/dispatch_reap.go`: a grace time of 2 s and a limit of 30 s). This can take longer than the `WaitDelay` of 10 s of `sysexec`. Go then sends SIGKILL to the lane, and the children of the lane (tmux, the LLM CLI, `go test`) stay alive with no parent. Thus, after a stopped lane returns, the pool calls its `Reap` port one time:

- `cmd/evolve` (`reapLane`, `cmd_loop_lane_reap.go`) finds the cycle of the lane and reads the process table. The dispatch ids of the lane are the `EVOLVE_DISPATCH_ID` tags of that cycle on this project root.
- It runs the sweep of `evolve gc dispatch` (`gc.StaleDispatch` and `proctree.Reaper`, through `gcRun.reapDispatches`), but only for those ids. The proof is the same: the owner of the dispatch is dead, and the process carries the tag or is in the recorded pane tree.
- A lane that was not stopped gets no reap. When no cycle holds the lane, or the process table does not list, the loop writes a WARN, and `evolve gc` stays the backstop.

The port is in `cmd/evolve`, because the orchestrator package imports no `os/exec`, `sysexec` or `proctree`.

## 9. The oversight loop

### Issue

No component sees all lanes of a wave at once (F2.15). The console watches `evolve wave watch`, which polls every 5 s, and it acts by hand.

### Gap

Each phase observer sees one cycle. `wave status` and `wave watch` only print. The quota state is read once, before the wave.

### Solution

The oversight loop is one goroutine in the loop process. It reads the ADR-0127 channels through `internal/events/reader`. It arms the kernel wake, reads to the end of each log, and then waits. It never polls (ADR-0139 decision 11). Each event maps to one action:

| Event (channel) | Action |
|---|---|
| `phase.*`, observer reports (`cycle`) | update the heartbeat of the lane (§8) |
| `cycle.sealed` (`cycle`) | record the verdict. On a FAIL, free the cells of the lane. |
| `ship.landed` (`ship`) | free the cells of the lane, count a clean landing or a conflict (§11), and call the pool to backfill |
| `GIT_FLEET_REBASE_NEEDED` and the ADR-0128 tier events (`ship`) | count a landing conflict for the window (§10) |
| a `clihealth` bench or unbench (`errors`) | run the sizing step again (§5). A family with no fallback stops taking new lanes. |
| `LIVENESS_PHASE_STALLED` (`errors`) | publish `LOOP_ORCH_LANE_STALLED` once for the lane |
| an operator stop (`loop`) | stop all admission. Lanes in flight finish. |
| the lease deadline timer | §8 |

The deadline needs a timer, because "nothing happened" is not an event. The loop holds one timer for the nearest deadline, not one timer for each lane.

The loop observes and acts only on its own lanes. It does not edit a lane's worktree, its verdict or its ship. The landing stays with ADR-0128 and ADR-0134.

## 10. The adaptive window

### Issue

At a large width, a fixed window hides a rising conflict rate. Each conflict at landing costs about 24 minutes (F2.11).

### Gap

The width is a fixed `fleet.count`, and only quota benches change it.

### Solution

**Deferred (plan O8).** The window has no effect at width 4. It starts when `fleet.count` goes above 6, with the ADR-0134 group commit. At or below width 4, the window is the sized count of §5. Above width 4, the window follows Zuul's rule (P3.10):

- It starts at `fleet.orchestrator.window.start` (default 4).
- It grows by 1 for each clean landing, up to the sized count.
- It halves on each landing conflict, down to `fleet.orchestrator.window.min` (default 4).

The window never stops a lane in flight. A smaller window only delays the next admission.

## 11. Measures

**State: Built (O7, 2026-10-17).** `evolve wave status --json` reports these values for the current run (`measures`) and for each of the last 3 runs (`runs`, the oldest first). The pure package `internal/wavemeasure` computes them from facts that already exist. The command computes them each time it runs. It writes no record and starts no timer. Package note: [internal-wavemeasure.md](packages/internal-wavemeasure.md).

| Measure | Definition | Source |
|---|---|---|
| `width_sized` | the width after the quota shrink and the budget | the last `loop.wave` record with `fields.width_sized` in the run, else `fleet.count` (`width_source`: `signal` or `policy`) |
| `width_realized` | the time-weighted median of the lanes in flight over the run time. Idle time counts as width 0. | the lane intervals |
| `utilization` | the busy lane time over `width_sized` times the run time | the lane intervals |
| `conflict_rate` | the landed lanes that needed a rebase or a re-audit at landing, over all landed lanes (`conflicts`, `landed`) | `recoverFromShipError` calls `recordShipError` before it recovers. A lane that rebased and then landed thus still holds the event: a `ship.error` with `SHIP_GIT_FLEET_REBASE_NEEDED`, `SHIP_GIT_FLEET_REBASE_CONFLICT` or `SHIP_AUDIT_BINDING_HEAD_MOVED` in the lane's stream |
| `admission_waits` | the todos that waited for a cell. It is 0 until O3 registers its code. | the count of `LOOP_ORCH_ADMIT_WAIT` in the run |
| `deadline_stops` | the lanes that the lease stopped | not built: it arrives with O5, which emits `LOOP_ORCH_LANE_DEADLINE` |
| `supply` | the class counts of the last census (§4) | not built: O6 adds it |

The facts:

- **The run window.** It starts at the `started_at` of the wave record. It ends at `ended_at`, else at the next record's `started_at`. A current run whose loop is not live ends at the mtime of its loop log. `wave next` uses the same rule to close a wave.
- **The one exception to "reads state and signals".** `wave status` reads the mtime of the loop log for the run end. It reads the metadata only, never the text. A live run ends at the clock.
- **A lane interval.** It is the span of the cycle's signals: from the first event to `cycle.sealed`. The phase times of the cycle dossier widen it. A lane with no seal and no dossier is in flight, and it ends at the end of the run window.
- **The sized width.** The loop writes one INFO `loop.wave` record at each sizing, with `fields.width_sized` and `fields.desired`, to `.evolve/signals.ndjson`. Before O7, no record held the sized width. The loop log has the line `wave count X -> Y`, but `wave status` reads state and signals, never the text log. A run from before O7 thus shows `width_source: policy`.

The conflict rate is the main measure of G2 (P3.3, P3.8). No outside source gives a good predictor of conflicts, so the loop measures its own rate.

### The baseline before O4

Measured on 2026-10-17 on the runtime plane, read-only, with the O7 binary (`evolve wave status --json --project-root <runtime>`):

| Wave | Cycles | `width_sized` (source) | `width_realized` | `utilization` | `conflicts` / `landed` |
|---|---|---|---|---|---|
| 118 | 1923 | 3 (policy) | 1 | 0.32 | 0 / 1 |
| 119 | 1924 | 3 (policy) | 1 | 0.32 | 0 / 1 |
| 120 | 1925 | 3 (policy) | 1 | 0.33 | 0 / 1 |

The loop log of each of the three runs has `wave count 3 -> 2`, because `agy-claude` was benched. With the sized width 2, the utilization is 0.48 to 0.49. Each run fell back to one sequential cycle (`LOOP_WAVE_DISPATCH_FAILED`: fewer than 2 file-disjoint todos). Thus the realized width is 1, and no lane landed on a moved `main`.

## 12. Scale

This section gives the cost of each step as a function of the lane count N and the limit of N with its cause.

### The cost of each step

| Step | Cost for each call | Calls | Reason |
|---|---|---|---|
| Census (§4) | O(I) for I inbox items | one for each plan | one pass over the ranked inbox |
| Cells of a todo (§6) | one `go list -deps ./...` for the module, 0.8 s for 303 packages | one for each plan | `LoadModuleGraph` loads the whole graph once |
| Admission test (§6) | O(c) map lookups for c cells of a todo | at most `scan_k` todos for each free slot | the scan stops at the first `scan_k` admissible todos (P3.12) |
| Release (§9) | O(c) | one for each lane end | |
| Event handling (§9) | O(1) for each event | one for each event | the channels push. No poll tick. |
| Deadline timer (§8) | O(log N) | one for each lane | one timer on a heap of deadlines |
| Memory | O(N + cells) | | the cell table and the lease table |

No step compares all pairs of lanes. The cost thus grows linearly with N, not with N². At N = 200 with `scan_k` 32 and about 10 cells for each todo, one admission is about 320 map lookups (synthesis).

### The limits of N

| Limit | Value | Cause | What raises it |
|---|---|---|---|
| Disjoint Go cells | about 139 lanes | the import graph: at most 139 packages with no edge between any two (ADR-0134 research F2.16) | work outside `go/`, which has no cell. Smaller hub packages. |
| The serial landing | about 6 lanes | one landing takes about 5 minutes (ADR-0134 research §2.1). The median lane takes 31 minutes (F2.13). Little's law gives 31 / 5 ≈ 6 lanes before landings queue (synthesis). | ADR-0134: content-keyed verification and group commit |
| Host CPU | 4 lanes without a budget (F12.1). An estimate of 8–12 lanes with the lane CPU budget. | this host has 14 CPUs. Each lane's build floor and audit run `go test` over the module. | the lane CPU budget (below). Lanes on more hosts, a later plan. |
| Host memory | above the CPU limit | one agy phase uses about 350 MB, and one Claude Code phase about 1.5 GB, on a 64 GB host (2026-10-17, `ps`) | none needed before the CPU limit |
| Quota | changes each hour | the benches of §5 | more families in each chain |

The first limit that binds is the serial landing, at about 6 lanes. A width of 4 is inside it. A width above 6 needs ADR-0134's group commit. The orchestrator then raises its window through §10, and the conflict rate shows if the cells hold.

### The lane CPU budget

Each `go test` that a lane runs gets one share of the host CPUs. The share is `procs = max(1, host CPUs / fleet width)`. The lane gives the share to the child three times: `-p procs`, `-parallel procs` and `GOMAXPROCS=procs` in the child environment.

**F12.1** Wave 122 (2026-10-17) was the first wave with 4 lanes. Cycle 1931's ship floor ran `go test -json -count=1 -timeout 20m ./internal/core` at 634% CPU (`ps`, 21:02). The load average was 15–18 on the 14 CPUs of the host.
*Implication:* one lane floor alone can fill the host, so the host CPU limit of the table above binds at width 4, not at 8–12.

**F12.2** On the same day, the same load caused two Claude REPL boot timeouts (exit 80). They were the build of cycle 1928 and the `doctor live` run of the boundary (inbox `boot-timeout-reported-as-all-families-exhausted`).
*Implication:* an oversubscribed host fails the phases of all lanes, not only the slow lane.

**F12.3** `go test` sets `-p` and `GOMAXPROCS` to the host CPU count when the caller does not set them. Before this budget, only the integration tier of `ciparitygate` passed `-p`, with a fixed value of 4. At width 4 it thus asked for 16 packages.

The formula divides the CPUs among the lanes. The floor of 1 keeps a lane alive on a host with fewer CPUs than lanes. A worked example: 14 CPUs at width 4 gives `procs` 3, so the 4 lanes ask for 12 CPUs.

`GOMAXPROCS` is necessary because the two flags do not bound the CPU use:

- `-p` bounds the number of packages that build and run at the same time. `-parallel` bounds the number of parallel tests in one test binary. The two multiply: a budget of 3 can still run about 9 test goroutines.
- Only `GOMAXPROCS` bounds the threads that run Go code. It bounds the compiler, the linker and the CPU use inside each test binary.

The width is `EVOLVE_FLEET_WIDTH` (`ipcenv.FleetWidthKey`). Each fleet launch path sets it to the effective width after the shrink ([internal-fleet](packages/internal-fleet.md)). The lane process reads it from its own environment, because each lane is a separate process. A console run has no width, so the budget changes nothing, and the integration tier keeps its 4.

The package `internal/lanebudget` ([its note](packages/internal-lanebudget.md)) owns the formula, and each site that builds a `go test` argv goes through it. `TestEveryGoTestArgvGoesThroughTheLaneBudget` fails on a new site that does not. `releasepreflight` is exempt: it runs only on the console, before a release.

The budget does not make a lane faster. It makes the lanes share the host, so the load stays below the CPU count. The live check is the load average during the test floors of a wave at width 4 (`uptime` and `evolve wave status`).

## 13. Config keys, signals and commands

### Config keys

All keys are under `.evolve/policy.json` `fleet`, decoded by `policy.FleetConfig()`. The `orchestrator` block is strict-decoded: an unknown key, an unknown `stage` or a negative number refuses the policy. Built on 2026-10-17: `stage`, `scan_k`, `hub_min_importers`, `deadline_floor_minutes`, `stall_minutes` and `run_max_lanes`.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `count` | int | 4 (was 3) | the largest window |
| `min_lanes` | int | 2 | the smallest window after sizing |
| `orchestrator.stage` | `shadow` \| `enforce` | `shadow` | §14 |
| `orchestrator.scan_k` | int | 32 | the admissible todos that admission scans for each free slot |
| `orchestrator.hub_min_importers` | int | 120 | the importer count (direct or transitive) that makes a package a hub (§6) |
| `orchestrator.deadline_floor_minutes` | int | 60 | the floor of the lane deadline |
| `orchestrator.stall_minutes` | int | 30 | the heartbeat gap that reports a stall |
| `orchestrator.run_max_lanes` | int | `4 × count` | the lanes that one run launches before its boundary |
| `orchestrator.window.start` | int | 4 | §10 |
| `orchestrator.window.min` | int | 4 | §10 |

`scheduling` goes away when the orchestrator is at `enforce`: the orchestrator always uses the pool. A policy with `scheduling` set then gets one WARN.

### Signals

| Code | Channel | When |
|---|---|---|
| `orchestrator.supply` | `loop` | after each census |
| `LOOP_ORCH_SUPPLY_SHORT` | `loop` | the admissible count is below the window |
| `LOOP_ORCH_ADMIT_WAIT` | `loop` | a todo waits for a cell. Once for each todo. |
| `LOOP_FLEET_POLICY_INVALID` | `loop` | `.evolve/policy.json` does not load at batch start (for example a misspelt `fleet.orchestrator` key), so the loop runs sequential until it is fixed |
| `LOOP_ORCH_LANE_LAUNCHED` | `loop` | §7: the first fill of a run launched a lane. INFO. |
| `LOOP_ORCH_LANE_EXITED` | `loop` | §7: a lane exited and freed its slot and its cells. INFO. |
| `LOOP_ORCH_LANE_BACKFILLED` | `loop` | §7: a freed slot took the next admissible todo. INFO. |
| `LOOP_ORCH_POOL_IDLE` | `loop` | §7: a slot stays free after a lane exit. INFO. |
| `LOOP_ORCH_LANE_STALLED` | `errors` | §8 |
| `LOOP_ORCH_LANE_DEADLINE` | `errors` | §8 |
| `LOOP_ORCH_LANE_LATE_EXIT` | `loop` | §8: a lane exited after its deadline. INFO. |
| `LOOP_ORCH_LANE_RECOVERY_FAILED` | `errors` | §8: the claim release or the park after a stop failed |
| `LOOP_ORCH_FAMILY_PAUSED` | `loop` | §9: a family with no fallback stops taking new lanes |
| `LOOP_ORCH_WINDOW` | `loop` | §10: the window changes |

Each code goes into the Signal Center registry.

### Commands

| Command | What it does |
|---|---|
| `evolve wave status [--json]` | adds the measures of §11 and the cell table |
| `evolve wave supply [--json]` | runs the census (§4) read-only and prints its classes and its split proposals |
| `evolve inbox split <id>` | the console accepts a split proposal (§4) |

## 14. Stages

| Stage | What the orchestrator does | Exit |
|---|---|---|
| `shadow` | The barrier wave runs as now. The orchestrator computes the census, the sizing, the admission and the leases. It records each action that it computes, and changes nothing. | 3 runs: the admission costs at most 10% of the width. No computed deadline stop hits a lane that passed later. |
| `enforce` | The orchestrator runs the lanes. | the soak at N = 16 with the simulated driver passes, and the conflict rate is at most 5% over 3 runs |

The stage key is one key for the whole orchestrator (ADR-0139 decision 16). Each component lands in shadow first, so each part has a measure before it acts.

## Guards

| Rule | Guard | State |
|---|---|---|
| The orchestrator runs no LLM. | a structure test: the orchestrator package imports no bridge and no driver package | proposed (O6) |
| The orchestrator never polls. | `TestOrchestrator_NeverPollsAndNeverStopsALaneByItself` bans `time.Sleep`, `time.Tick` and `time.NewTicker` in `internal/orchestrator` | built (O4, O5) |
| Two lanes in flight never hold one cell. | `TestAdmit_ATodoInABusyCellWaits` and `TestAdmit_TwoAdmittedTodosNeverConflict` | built (O3) |
| The hub cell admits one lane. | `TestAdmit_TheGlobalCellAdmitsOne` | built (O3) |
| The shadow changes no lane. | `TestAdmit_ShadowRecordsTheWaitAndChangesNothing` | built (O3) |
| A stop is never a broad kill. | the stop cancels the launch context of that lane only (`TestLease_AnEnforcedDeadlineStopsOnlyThatLaneThroughItsInjectedStop`), and the guard test refuses `syscall.Kill`, `os.FindProcess`, `os/exec`, `sysexec` and `proctree` in the package | built (O5) |
| The shadow lease keeps the barrier. | `TestWave_AShadowLeaseKeepsTheBarrierLaunchesExactlyAsToday` | built (O5) |
| The census never routes an item. | `TestSupply_RecordsTheCauseOfEachRefusal` checks that the inbox files do not change | proposed (O6) |
| A split keeps every protected file in the console part. | `TestInboxSplit_TheConsolePartKeepsEveryProtectedFile` | proposed (O2c) |
| A bench with a fallback keeps its slot. | `TestQuotaAwareCount_ABenchWithAFallbackKeepsTheLane` | proposed (O1) |

## Limits

- **Landing.** The orchestrator does not land. ADR-0128 and ADR-0134 own the queue, the tiers and the group commit. Above about 6 lanes, the width needs their group commit (§12).
- **Edges added in a lane.** Cells come from the tree at the run's base. An import that a lane adds is found at landing, not at admission (§6).
- **Todos outside `go/`.** A todo whose files are all outside `go/`, or that has unmapped non-Go files, maps to no cell. It conflicts with nothing, so admission cannot hold it back (§6).
- **Accuracy of the cells.** The package graph of `go list -deps` misses runtime coupling: shared files under `.evolve/`, shared test data and generated outputs. The landing proof of ADR-0128 covers these (fleet-landing-queue §7).
- **Supply.** The census measures the supply and proposes splits. It does not create work from a goal. A goal-split planner phase is plan design D, which is deferred (plan Q3).
- **Hosts.** One host runs all lanes. The CPU limit of 8–12 lanes is an estimate. The soak at N = 16 measures it.
- **What a test cannot prove.** A soak with the simulated driver proves the scheduling and the cost of each step. It does not prove the conflict rate of real LLM lanes. Only live runs measure that.
