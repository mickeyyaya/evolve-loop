# Plan: a host sentinel inside the loop finds host trouble between seals and holds new lanes under a disk floor

- **Status:** built at shadow, 2026-11-21, after design review round 3. The console scope decision moves the scoring of false stops and the evidence path of the stage flip to the follow-up item `host-sentinel-stage-evidence` (section 8).
- **Inbox item:** `host-sentinel-inside-the-loop` (rank 41, console-owned).
- **Depends on:** nothing that blocks the build. The mid-wave run reuses `gcRun.cacheScope` (D5). [Cycle recycle and gc check](cycle-recycle-and-gc-check-2026-11.md) X2 (the `<hash>-d` trim) improves the result but does not block it. When the check registry of that plan (X1a to X1e) lands, it takes over `cacheScope` as the mid-wave list.
- **Related plans:** [fleet host-adaptive window](fleet-host-adaptive-window-2026-11.md), [gc closes the legacy gap](gc-closes-the-legacy-gap-2026-11.md).
- **Package notes:** [internal-gc](../architecture/packages/internal-gc.md), [internal-gcpolicy](../architecture/packages/internal-gcpolicy.md), [internal-orchestrator](../architecture/packages/internal-orchestrator.md), [internal-fleet](../architecture/packages/internal-fleet.md), [internal-gateregistry](../architecture/packages/internal-gateregistry.md), [cmd-evolve](../architecture/packages/cmd-evolve.md). **Stage rules:** [shadow stage contract](../architecture/shadow-stage-contract.md).

## Table of contents

- [1. Request](#1-request)
- [2. Facts](#2-facts)
- [3. Goals and non-goals](#3-goals-and-non-goals)
- [4. Candidate designs](#4-candidate-designs)
- [5. Decisions](#5-decisions)
- [6. Components and files](#6-components-and-files)
- [7. Red tests](#7-red-tests)
- [8. Rollout](#8-rollout)
- [9. Risks](#9-risks)
- [10. Open questions and follow-ups](#10-open-questions-and-follow-ups)

## Terms used throughout

- **Sentinel.** A background task of the loop process. It samples the host, emits signals and sets the floor hold.
- **Sample.** One reading of four values. These are the minimum free disk, the Go cache size, the owner-dead tmux servers and the orphan processes that the seal sweep leaves behind (D13).
- **Mid-wave run.** One in-process call of `gcRun.cacheScope`, the Go cache trim and the temp sweep. It judges each resource by its own proof and cannot touch a live lane.
- **Boundary-only check.** A gc check that `cacheScope` does not run. It runs at a wave boundary.
- **Live sweeps.** The sweeps that already run while lanes run. These are `gcOrphanSessions`, `sweepSealedCycleGC` and `laneReap` (F3.1 to F3.3).
- **Floor hold.** A state in which the pool starts no new lane. Running lanes continue.
- **Gc lease.** The file lock `.evolve/gc.lock` that each `evolve gc` run takes (`go/cmd/evolve/cmd_gc.go:112`).

## 1. Request

The operator decision of 2026-11-04 in the inbox item: "add the rule that it should detect the system and tmux status in the background when evo loop / wave / cycle is running". The item asks for these parts:

- a sentinel in the loop with typed `HOST_*` signals;
- mid-wave-safe gc scopes;
- a disk floor that holds new lanes;
- a shadow stage first;
- a `wave status` line.

## 2. Facts

**F1.1** The item records a wave of 9 to 13 hours. It records a host with 273 MB free and a 138 GB Go cache. It records 324 orphan processes of 3 to 6 hours age (inbox item `source` field). The item dates from 2026-11-04.

*Implication:* a boundary-only gc cannot protect a wave. Trouble grows between boundaries. Part of the 2026-11-04 evidence is old: sweeps that ran later cover it (F3.1 to F3.3).

**F1.2** On 2026-11-20 the disk fell to 12 GB free (2.6%), and four cycles failed for it ([cycle recycle plan](cycle-recycle-and-gc-check-2026-11.md), F1.1). The Go cache held 56.1 GB and 282 orphan tmux servers lived (same plan, F2.1 and F2.5).

*Implication:* most of this evidence lies outside the reach of a sentinel. The 282 servers are test sockets under `TMUX_TMPDIR`, which `ReapOrphanSockets` cannot see until X8. Of the 53 GB cache, 35.9 GB sits in the `<hash>-d` dirs that `TrimGoCache` skips until X2. The salvage leaves (187 GB) are not a mid-wave scope: K5 owns them.

**F2.1** No sentinel exists. A search of `go/internal/orchestrator`, `go/internal/gc`, `go/internal/policy` and `go/internal/signalcenter` finds one `HOST_*` code: `HOST_DISK_LOW`. It is defined at `go/internal/signalcenter/event.go:218`, registered under `ModuleHost` in `go/internal/signalcenter/registry.go:60`, and emitted by the cycle-start disk guard (`go/cmd/evolve/cmd_cycle_start.go:70-71`). Its fields are `free_pct`, `pause_pct`, `path` and `outcome`. The doc row is `docs/architecture/signal-codes.md:243`. The load sampler `startHostPressure` (`go/cmd/evolve/cmd_loop_hostpressure.go:143`) is the only host task in the loop.

*Implication:* a second `RegisterCode` of `HOST_DISK_LOW` with other fields records a registry Conflict (`registry.go:82-87`), and a reader of the old fields will break. The sentinel needs its own disk code.

**F2.2** The load sampler reads load, memory and swap, and it caps the admission window. It runs only when `Adaptive.Enabled` is set and a window exists (`cmd_loop_hostpressure.go:144-147`). It writes `host-pressure.ndjson` to the path of `hostSamplesPath` (`:104`), which is the run log dir when `--log` is set. It emits `LOOP_ORCH_HOST_PRESSURE`. The checked-in policy has no adaptive block (`.evolve/policy.json:131-138` holds only stage, admission and window), so `resolve()` returns a zero `AdaptiveConfig` there (`go/internal/policy/fleet_adaptive.go:90-93`). The JSON key of the load limit is `load_per_core_high` (`:19`).

*Implication:* load is built and owned by the sampler. The sentinel reads no load and adds no load code. But in the checked-in configuration nothing watches load, so C13 adds `fleet.orchestrator.window.adaptive` with stage `shadow`. The shadow stage does not cap the window (`fleet_adaptive.go:96`).

**F2.3** The loop starts the oversight task and the load sampler in `orchestratedRun`, and each stops with a `defer` (`go/cmd/evolve/cmd_loop_orchestrator.go:128-130`). `startOversight` runs `over.Run` in a goroutine and joins it with a `WaitGroup` on stop (`:135-162`). `poolRunner` selects `orchestratedRun` only when `fleet.orchestrator.stage` is `enforce`. Otherwise `rollingPool` runs, with no oversight, no sampler and no window (`go/cmd/evolve/cmd_loop_window.go:169-175`). Both runners call `fleet.RunPool` (`cmd_loop_pool.go:43`, `go/internal/orchestrator/pool.go:89`). `orchestratedRun` passes `Halted: b.poolHalted` (`cmd_loop_orchestrator.go:71-73`), and `rollingPool` passes no `Halted` (`cmd_loop_pool.go:43`).

*Implication:* a task of the same shape stops with its owner. A sentinel inside `orchestratedRun` works only at orchestrator stage `enforce`. The sentinel must start in `dispatchPool`, above the runner choice (D1).

**F2.4** A failed read goes through `Decide`, which yields `HostBlind` (`cmd_loop_hostpressure.go:50-55`), and the loop emits `LOOP_HOST_PRESSURE_BLIND` (`go/internal/orchestrator/orchestrator.go:40`). A failed write of the record warns once (`appendSample`, `:73-81`). Each host tool call runs under `hostToolTimeout` of 800 ms (`:20`), but `hostRun` builds its context from `context.Background()` (`:124`). A stop during a sample does not kill the child process.

*Implication:* the pattern "a failed read never stops the loop" exists. The sentinel copies it, and it runs each child under its own context so that a stop cancels the child.

**F3.1** `gcOrphanSessions` reaps orphan sessions and owner-dead sockets while lanes run. It has no lease gate. It runs at each cycle start (`go/cmd/evolve/cmd_cycle.go:360`), after each cycle (`cmd_loop_control.go:218`) and at loop start (`cmd_loop.go:157`).

*Implication:* the tmux socket scope of the item is already a per-cycle sweep. It is not a sentinel scope.

**F3.2** `sweepSealedCycleGC` runs at each seal (`cmd_cycle.go:280`, `cmd_loop_sealed_gc.go`). It calls `gc.SweepSealedCycle` (`go/internal/gc/cycleseal.go:40`). That sweep kills the ppid-1 orphans of the sealed cycle and releases its sealed lanes. `gc.mode` is `enforce` in the checked-in policy. It landed on 2026-11-11, after the item evidence of 2026-11-04. `sealHold` (`cycleseal.go:61`) keeps a paused, resumable cycle.

*Implication:* the orphan processes of a cycle that seals are already reaped at its seal. A sentinel scope for them re-runs a live sweep. Only a cycle that never seals can leave orphans that no live sweep owns.

**F3.3** `laneReap` runs `StaleDispatch` on the dispatches of a stopped lane (`cmd_loop_lane_reap.go:22-55`).

*Implication:* the dispatch processes of a stopped lane are also covered.

**F3.4** `evolve gc --only cache` runs the Go cache trim and the pipeline temp sweep. It touches no worktree, branch, session or process (`go/cmd/evolve/cmd_gc.go:32`, `cmd_gc_scope.go:14-28`). It trims only when disk free is under `gc.cache_trim_free_gb` (`cmd_gc_scope.go:30-45`).

*Implication:* the Go cache and the temp dirs are the resources that no per-cycle sweep owns. A trim gets its own trigger today. The sentinel must not add a second trigger.

**F3.5** `TrimGoCache` skips each `<hash>-d` directory entry (`go/internal/gc/gocache.go:88`). After a trim, `evolve gc` reported 20 GB while `du` measured 53 GB ([cycle recycle plan](cycle-recycle-and-gc-check-2026-11.md), F2.1).

*Implication:* the cache scope under-trims until X2. The sentinel must re-measure and must not claim success from the reported size.

**F3.6** The process proofs have two cases each. `FinishedCycleOrphans` needs parent pid 1, a cwd under a `cycle-N` worktree, no live lease and a closed cycle or a gone tree (`go/internal/gc/processes.go:44-68`). `StaleDispatch` needs the project root in the process env, and then either a dead owner or parent pid 1 with a closed cycle (`go/internal/gc/dispatch_processes.go:36-50`). The dead-owner case checks neither the cycle nor the parent pid (`:45-46`).

*Implication:* a live lane never matches `FinishedCycleOrphans`. But a dispatch whose owner died while its cycle is held (paused, resumable) matches the dead-owner case of `StaleDispatch`. A mid-wave run of that proof kills it.

**F3.7** The documented rule is in `docs/architecture/cli-contract.md` §4.6, D10 and F9.3 (lines 373-380). While a run lease is live, `gc` skips sessions, sockets, cycle processes and dispatch processes. The file sweeps run, because they judge each resource. The `liveRunDirs` message (`cmd_gc.go:92-104`, printed at `:101`) says the same. The skip gate is at `:119`. The live sweeps of F3.1 and F3.2 already run the sessions, sockets and sealed-cycle processes while a lease is live, each with its own proof.

*Implication:* the rule reads wider than the code. The sentinel must not widen it further. Section 6 amends it in one place (C11).

**F3.8** `takeGCLease` acquires `.evolve/gc.lock` with a context deadline. A wait of 0 refuses at once with exit 3 (`cmd_gc.go:34` for the flag, `:87` for the refusal). The refusal prints one line to `stderr` on each try (`:86`).

*Implication:* a try-lock exists. "Skip, do not wait" needs no new lock code.

**F3.9** The code of the gc check steps lives in package main. These are `gcRun.cacheScope` (`cmd_gc_scope.go:14`), `goCache` (`cmd_gc.go:401`), `pipelineTemp` (`:428`), `gcGoCacheDir` (`:491`), `takeGCLease` (`:80`) and the dispatch wiring in `cmd_gc_dispatch.go`.

`gcRun.step` runs each step under `context.WithoutCancel` with its own timeout (`cmd_gc.go:166`), and `TrimGoCache` takes no context (`go/internal/gc/gocache.go:50`). The full sweep list is `gcRun.project` (`cmd_gc.go:239-256`). `internal/gc` does not import `swarm`.

*Implication:* the mid-wave run belongs in `cmd/evolve`, beside `cacheScope`. A run in `internal/gc` needs a large move or a new `gc` to `swarm` to `bridge` edge. A stop does not cancel a step in progress, so the sentinel needs its own budget and a context check between the two steps (D6).

**F3.10** The recycle plan proposes one registry of gc checks (D10, X1a to X1e). That plan has status "proposed" and has no review yet (`cycle-recycle-and-gc-check-2026-11.md:3`). The list of steps that are safe during a wave exists today in one place: `gcRun.cacheScope` runs exactly K7 and K8 (`cmd_gc_scope.go:14-28`). The documented contract says that `--only cache` is safe while lanes run (`cmd_gc.go:32`; `runtime-reference.md:566`).

*Implication:* a `MidWave` attribute on a registry that does not exist yet adds a dependency, not a capability. The sentinel reuses `cacheScope` and waits for no registry.

**F4.1** `Window.admitted()` is `min(size, ceiling)` (`go/internal/orchestrator/window.go:46`). `clamp` touches only `w.size` (`:64-66`, `:72`). `SetCeiling(0)` therefore gives `Size() == 0` already. The host-pressure controller is the one owner of the ceiling (`:54-56`).

*Implication:* a second writer of the ceiling overwrites the host-pressure value. That is the reason for a separate hold state. It is not the clamp.

**F4.2** The pool reads the width from `width() = max(1, min(limit, Window()))` (`go/internal/fleet/pool.go:168-173`). `RunPool` loops only while `len(p.running) > 0` (`:82`). Its `select` waits on `completions` and `kick` only, with no `ctx.Done()` case (`:83-94`). `fill` returns `full` when the width is 0 and emits no transition (`:150-152`), and `open()` gates each launch (`:208`).

A window of 0 still starts one lane when none run, because of the `max(1, ...)`. If that floor goes, a pool with no running lane returns at once, and each pending todo gets `ErrNeverLaunched`. That is a stop, not a hold.

Nothing kicks the intake when a hold clears.

`BrakeEngaged` is a file stat (`go/internal/loopchain/inbox.go:55-58`) and fires no event. The loop `ctx` ends only on SIGINT and SIGTERM. `wave stop --wait` waits only for run leases (`cmd_wave_stop.go:17`, `:33`). A brake therefore cannot end a held pool with 0 lanes unless the pool checks the stop clause again on a timer.

`rollingPool` sets no `Kick` channel (`go/cmd/evolve/cmd_loop_pool.go:43`). In `orchestratedRun`, the pool builds its own intake and passes `in.Wake()` as `Kick` (`go/internal/orchestrator/pool.go:84`), which a sentinel built in `dispatchPool` cannot reach. `PoolHeld` exists, but only as the `refused` signal of a file-claim conflict (`fleet/pool.go:191-198`). `idleCause` knows `CauseHostPressure` only through `HostCapped()` (`go/internal/orchestrator/idle_cause.go:51-70`), so a disk hold emits `LOOP_ORCH_POOL_UNDERFILLED` WARN.

`idleCause` also checks `Pending == 0` before the host causes (`:59-62`), and the underfill exclusion skips only `CauseHostPressure` (`go/internal/orchestrator/pool.go:138`). The console rule reads that WARN as a reason to stop the loop.

*Implication:* the hold must live in the fleet pool, with its own wait state, its own kick and its own idle cause (D9).

**F4.3** Three settings now control "enough disk to start work". (1) `preflight.min_free_gib` makes `diskSpaceHalt` stop a wave launch (`cmd_loop_window.go:156-168`). Its default is 1 GiB (`go/internal/policy/preflight_config.go:9`).

(2) The cycle-start guard pauses a cycle start under a percent and gives up after `max_pause_s` (`go/internal/looppreflight/cyclestart.go:39-93`). The policy key `preflight.disk_guard` holds only `max_pause_s` (`preflight_config.go:22-24`). The 5% is the compiled `defaultDiskPausePercent` (`go/internal/looppreflight/disk_percent.go:10`), built at `cmd_cycle_start.go:59`.

(3) The gc policy has the absolute floors `disk_floor_gb` and `disk_warn_gb` (`go/internal/gcpolicy/gcpolicy.go:18-19`). The default floor is 10 GB (`:84-92`).

*Implication:* a bytes guard that falls back to `min_free_gib` lowers the default floor from 10 GB to 1 GiB. It also leaves the `free_pct` and `pause_pct` fields of `HOST_DISK_LOW` with no meaning. The hold reads only the gc keys (D8). The guard stays as is.

**F5.1** `swarm.ReapOrphanSockets` kills a server when the owner pid in the socket name is dead (`go/internal/swarm/reap_orphans.go:123`).

*Implication:* a tmux count must use the same predicate as the reaper, or the count and the release disagree.

**F6.1** The signal registry has a `ModuleHost` module (`go/internal/signalcenter/event.go:86`; `:89` is the `knownModules` map). `RegisterCode` takes a module and a description. `orchestrator.Publish.emit` hard-codes `Module: ModuleLoop`, and `BelongsTo` requires the module prefix, so a `HOST_` code cannot go out through it.

*Implication:* the host codes need their own publisher with `ModuleHost`, outside `orchestrator.go`.

**F7.1** The stage contract is live code. `gatecontract.Shadow` is at `go/internal/gatecontract/shadow.go:48`. The seeds are in `go/internal/gateregistry/seed_stages.go`. The tests `TestEveryStageKeyHasRegistryEntry`, `TestNoStringTypedStageField`, `TestEverySignalCodeHasNonTestEmitter` and `TestShadowSignalCarriesWouldFields` (`stage_conformance_test.go:256-283`) walk the typed policy schema. A new stage key with no seed, a string type, no emit site or no `would_*` fields is a new violation, and CI fails. `gateregistry` is on the protected surface (`go/internal/guards/integrity_surface.go:238`).

**F7.4** The evidence tools only read and link. `stageevidence.FileChain` reads a chain file (`go/internal/stageevidence/chainfile.go:18`) and `Link` chains a record (`score.go:172`). `ScorerFor` ignores its key and returns the default scorer (`score.go:144-146`). `Record` and `Outcome` are keyed by cycle (`score.go:66-83`). `DecisionVersion` hashes each module-local transitive dependency of the closure (`stageevidence/version.go:31`).

*Implication:* nothing appends sentinel decisions today, and `evolve stage verify` cannot score them. The item must add an appender and a `ScorerFor` case (C11). The closure of the stage must stay small. A large closure changes the decision version with almost every merge, and two clean waves cannot share one version (D2).

*Implication:* each new stage key needs a seed, a typed field, an emit site and `would_*` fields in this item (C10).

**F7.2** The wave-start disk check and the cycle-start guard read disk through one function with two builds. The file `disk_unix.go` builds for darwin and linux (`go/internal/looppreflight/disk_unix.go:1`). The file `disk_other.go` is the non-unix stub (`:1`).

*Implication:* the reader works on both CI hosts. It has no macOS-only path.

**F7.3** `gc` reads disk on `os.TempDir()` and the project root (`cmd_gc_disk.go:14`). The Go cache dir (`gcGoCacheDir`) can sit on a third volume.

*Implication:* the sentinel samples the minimum free space over the three volumes.

### Outside research

**P1** The Kubernetes kubelet monitors disk, memory and inodes. Before it evicts a pod for disk pressure, it runs container and image garbage collection first. It has a `eviction-pressure-transition-period` to stop a condition from flapping ([node-pressure eviction](https://kubernetes.io/docs/concepts/scheduling-eviction/node-pressure-eviction/)). One report notes that an image GC high threshold close to the hard eviction threshold lets pressure start before GC runs ([kubernetes issue 124868](https://github.com/kubernetes/kubernetes/issues/124868)).

*Implication:* run the cheap, safe cleanup first, and set the cleanup threshold well above the hold threshold. Add a transition period so a signal does not flap.

**P2** The Go build cache has no size cap. The standard `go clean -cache` removes the entire cache, and a request for a disk limit has been open since 2019 ([go issue 29561](https://go.dev/issue/29561), [go clean](https://manpages.ubuntu.com/manpages/noble/man1/go-clean.1.html)).

*Implication:* a size bound is our job. The trim must work by age and size, and it must not run `go clean -cache` on a host where lanes build.

**P3** A GitLab project cleans its Go build cache at build start when the cache is over a set size ([gitaly merge request 4765](https://gitlab.com/gitlab-org/gitaly/-/merge_requests/4765)).

*Implication:* "trim over a threshold, with a policy value" is current practice. It is the shape of the cache check.

### Root cause, restated

The gap is detection and response between seals, for resources that no per-cycle sweep owns: the Go cache, the temp dirs and the disk floor. Sessions, sockets and sealed-cycle processes already have live sweeps (F3.1 to F3.3). So the sentinel does three things. It detects the three resources. It runs `cacheScope` for the cache and the temp dirs. It holds new lanes when free disk is under the floor.

## 3. Goals and non-goals

| Goal | Measure |
|---|---|
| G1. Find trouble during a wave | One `HOST_*` signal for each threshold crossed, with the value and the threshold |
| G2. No flood | A repeat of the same code inside `repeat_s` emits nothing |
| G3. Release only what is safe | At enforce, only `cacheScope` runs. A live lane keeps its temp dirs and cache entries. A live test server keeps its socket dir |
| G4. Never wait on gc | A held gc lease makes the sentinel skip the release |
| G5. Protect new work | Free disk under the floor holds new lanes. Running lanes continue. A hold is never reported as underfill. A hold ends with a stop or a cancel, and never ends the pool on its own |
| G6. Stop clean | The sentinel ends with the pool and leaves no process, also a child of an in-flight sample. A failed sample warns and the loop goes on |

Non-goals: the sentinel reads no load and sets no load cap (the load sampler owns both). It runs no boundary-only check. It changes neither the cycle-start guard nor `preflight.min_free_gib`. It skips the sequential path and the wave fallback route (`cmd_loop_driver.go:186-200`), which launches lanes with no hold. It leaves foreign caches alone ([cycle recycle plan](cycle-recycle-and-gc-check-2026-11.md), F2.8) and reaps nothing that a live sweep owns.

## 4. Candidate designs

- **A. External daemon.** A launchd or cron job runs `evolve gc --only cache` on a timer.
- **B. In-process task with a pure decision core.** A goroutine in the loop samples on a timer. A pure function maps the sample to findings. The task emits signals, runs the mid-wave checks through one in-process call, and sets the floor hold in the fleet pool.
- **C. Extend the load sampler.** Add disk and cache readings to `hostSampler`, and use its ticker.
- **D. Event only.** Run the safe checks on each cycle seal and read no timer.
- **E. Extend the cycle-start guard.** The cycle-start guard reads the bytes floor. No sentinel task holds the pool.

Scores: 2 is good, 1 is partial, 0 is bad.

| Criterion | A | B | C | D | E |
|---|---|---|---|---|---|
| Stops with the loop and leaves no process (acceptance 5) | 0 | 2 | 2 | 2 | 2 |
| Skips a held gc lease, never waits (acceptance 3) | 1 | 2 | 2 | 2 | 0 |
| Sees a wave with no seals (the 9 to 13 hour case) | 2 | 2 | 2 | 0 | 1 |
| No feature flag and one home for each setting | 1 | 2 | 1 | 2 | 2 |
| Floor hold reaches the pool (acceptance 4) | 0 | 2 | 2 | 1 | 1 |
| Load decision stays in one place | 2 | 2 | 0 | 2 | 2 |
| Works on macOS and Linux with Go 1.23 | 1 | 2 | 2 | 2 | 2 |
| Test seam for a live-lane proof (acceptance 2) | 0 | 2 | 1 | 2 | 0 |
| Covers a lane that is already running | 0 | 2 | 2 | 2 | 0 |
| **Total (of 18)** | 7 | 18 | 14 | 13 | 10 |

Why B wins. A runs outside the loop. It cannot stop with the loop, and it cannot reach the pool. C ties the sampler to `Adaptive.Enabled` (F2.2), and it puts two decisions in one function. D cannot see a long wave with few seals (F1.1).

E is the simplest path to acceptance 4. But it acts only at a cycle start. It cannot trim the cache and has no release step. It sees no trouble while all lanes run. B keeps the idea of E: the start-work floor has one home (D8). E as a whole is rejected, because the guard counts percent (F4.3).

B has a coupling too. It needs a sentinel start that does not depend on the orchestrator stage. F2.3 shows that B placed in `orchestratedRun` depends on `fleet.orchestrator.stage`, as C depends on `Adaptive.Enabled`. D1 starts it in `dispatchPool` so that both runners are covered.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Design B: a `hostsentinel` goroutine that `dispatchPool` starts for each pool run, above the runner choice. It stops when the pool run returns. Before the runner starts, `dispatchPool` reads only the free disk, the one input of the hold. The other readings come on ticks. The hold goes through one value, `fleet.PoolConfig.Hold`, that `dispatchPool` gives to both runners. | F2.3, F4.2 | A: no stop with the loop. C: couples to `Adaptive.Enabled`. D: blind to a quiet wave. B in `orchestratedRun`: couples to the orchestrator stage and leaves `rollingPool` with no sentinel. A full first sample before the runner: the cache walk (30 s budget) and the `lsof` listing delay each pool start. A first sample inside the goroutine: `RunPool` fills at once (`fleet/pool.go:79`), so lanes start before the hold is set. |
| D2 | A pure core `hostsentinel.Decide(sample, config, state) (findings, state)`. Package `internal/hostsentinel` is a leaf: the sample type, `Decide`, the codes and the publisher. It imports only `signalcenter`. The scorer and the record builder sit in `internal/hostsentinel/evidence` and add only `stageevidence`. The seed closure does not include them (C10). The host reader sits in `cmd/evolve` (C6) with injected functions. | F2.4, F7.4 | The reader in `internal/hostsentinel`: it imports `looppreflight`, `gc` and `swarm`, and `FinishedCycleOrphans` needs `WorktreeOptions`, which only `cmd/evolve` builds. The decision version then changes with almost every merge. |
| D3 | The sentinel wakes on `interval_s` only. It has no seal wake. | F3.2: `SweepSealedCycle` already runs at each seal | A seal wake: a duplicate of a live sweep. |
| D4 | Thresholds live in the `gc` policy block: new `sentinel` sub-block. Keys: `interval_s` (60), `repeat_s` (900), `orphan_servers_max` (20), `orphan_processes_max` (20), `hold_max_s` (7200), `release_budget_s` (300), `no_progress_bytes` (1 GiB) and the two stages of D10. The hold reads `disk_floor_gb` and `disk_warn_gb`. The cache check reads `go_cache_max_gb`. The sentinel reads no load. | F4.3, F2.2, one home for each setting | A new copy of each threshold: two homes drift. A load reading: it duplicates the load sampler. |
| D5 | The mid-wave run is `takeGCLease(root, 0, ...)` followed by `gcRun.cacheScope(root)`, called in process. `cacheScope` serves both `--only cache` and the sentinel. The recycle registry takes over `cacheScope` as the mid-wave list when it lands. | F3.4, F3.10 | A `MidWave` attribute on the registry of the recycle plan: that plan has no review, so the build waits for an unknown time. A typed table in `internal/gc`: a third list. |
| D6 | The run sits in `cmd/evolve` (C7), beside `cacheScope`. The sentinel passes a context with a budget of `release_budget_s` (D4). The mid-wave call uses a cancellable step variant, and the children run under that context (detail after this table). The skip when the lease is held goes out as one deduplicated `HOST_RELEASE_SKIPPED` signal. The run does not print the `takeGCLease` refusal line. | F3.8, F3.9 | `RunMidWave` in `internal/gc`: a large move. Shelling out to `evolve gc --only cache`: a second process and no typed report. A reuse of the `takeGCLease` stderr line: it prints on each try (`cmd_gc.go:86`). A second step list in `cmd_gc_midwave.go`: it breaks D5. |
| D7 | The mid-wave run has no process check. This narrows the operator fix on purpose. The sentinel detects owner-dead tmux servers and orphan processes, and it emits signals. The rule of `cli-contract.md` D10 and F9.3 stays. Section 6 amends that text to name the live sweeps. | F3.1 to F3.3, F3.6, F3.7 | A process release with the `StaleDispatch` proof: its dead-owner case kills a held cycle. A reap of `FinishedCycleOrphans` (`processes.go:54-68`): it is safe, but it repeats the seal sweep. |
| D8 | The hold reads only `gc.disk_floor_gb` (floor) and `gc.disk_warn_gb` (clear). `preflight.min_free_gib` stays the wave-launch halt. It runs before any lane exists, and it has its own meaning: no wave starts under 1 GiB. The cycle-start guard stays unchanged. A guard on bytes is a separate item (section 10). | F4.3 | A fallback of `disk_floor_gb` to `min_free_gib`: it makes a second home and lowers the default floor from 10 GB to 1 GiB. A guard on bytes in this item: it leaves `free_pct` and `pause_pct` of `HOST_DISK_LOW` with no meaning. |
| D9 | The floor hold is a state of the fleet pool, not of the window. `PoolConfig.Hold` has `Held() bool` and `Wake() <-chan struct{}`, and the sentinel owns both. `open()` returns false while held, and `width()` stays unchanged. The loop condition, the `select` cases and the wake step are in the list after this table. | F4.1, F4.2, P1 | `SetCeiling(0)`: a second writer of the ceiling. A `Kick` on the intake: `rollingPool` has none, and `orchestratedRun` builds its own intake. A loop condition with no stop clause: a stop while held with no lane never returns. A stop clause that only a select case checks: a brake fires no event, so the pool must wake on a timer. |
| D10 | Two stages, one for each decision. `gc.sentinel.hold.stage` (default `shadow`) governs the hold and its false stops. `gc.sentinel.release.stage` (default `shadow`) governs the call of the mid-wave run and its conflicts. The signals go out at both stages. At `shadow` the sentinel sets no hold and runs no release. K7 and K8 already delete under `evolve gc`, so only the sentinel trigger is gated. | [shadow stage contract](../architecture/shadow-stage-contract.md), F3.10 | One stage for both: a false stop of the hold blocks the release flip, and the reverse. A per-check stage: the registry is not built. |
| D11 | Each finding maps to the one release that can address it. Only a free disk under `disk_warn_gb` and a cache over `go_cache_max_gb` start the run. The server and orphan findings emit signals only, and the run has a cooldown of `repeat_s`. A no-progress backoff stops a run that frees little (detail after this table). | F3.4, F3.5 | A run on any finding: each run lists about 170k cache files and runs a host-wide `lsof` (`go/internal/gc/openpaths.go:18`, `:41`). A persistent orphan finding repeats that work all wave. A cooldown only: until X2 the cache stays over the cap (F1.2), so each `repeat_s` starts a run that frees almost nothing. |
| D12 | The codes of the sentinel are in the table after this one. `HOST_DISK_LOW` stays with the cycle-start guard and its fields. The codes and a `ModuleHost` publisher live in `go/internal/hostsentinel`. | F2.1, F6.1 | One code for two emitters: the field sets differ. A `HOST_` code through `orchestrator.Publish.emit`: it sets `ModuleLoop`. |
| D13 | Free disk is the minimum over `os.TempDir()`, the project root and the Go cache dir (F7.3). The server count uses `ReapOrphanSockets` (F5.1). The orphan count uses `FinishedCycleOrphans` (F3.6): what the seal sweep leaves behind. It runs on each 10th tick, because the `lsof -d cwd` listing can exceed the child budget on a loaded host. The cache size is measured after a trim and on each 10th tick, with a 30 s budget. Each child runs under the sentinel context. | F2.4, F5.1, F3.5, F3.6, F7.3 | A per-tick walk of the cache: it breaks the 800 ms tool budget. A count of all tmux servers: live lanes own many. A new predicate for cycles that never seal: no evidence yet that the set matters (section 10). |
| D14 | The sentinel writes each sample to `host-sentinel.ndjson`, with the path rule of `hostSamplesPath`. `wave status` runs in a separate process. It reads the dir from `wave.Record.LogPath` (`go/internal/wave/record.go:33`), shows the last line of the sentinel file, and the last line of `host-pressure.ndjson` for load. | F2.2 | A new store, and a second load reader. |
| D15 | The dedupe key is the code. The state holds the last emit time for each code. A finding repeats after `repeat_s`. A second band repeats it at once: twice the limit for a size or count, and half the floor for free disk. A crossing of `disk_warn_gb` emits `HOST_SENTINEL_DISK_WARN` under the dedupe rule. | G2, P1 | No dedupe: a 60-second timer floods the signal log. |
| D16 | If the hold stays longer than `hold_max_s`, the sentinel emits `HOST_LANE_HOLD_STUCK` (INCIDENT) once and keeps the hold. It never clears a hold while free disk is under the floor. The hold clears at `disk_warn_gb`. While the hold is on, each tick sends `Wake()` (D9). | F4.3, P1 | Reuse `DiskPauseMax` and clear: that lifts the hold with the disk still under the floor. |
| D17 | The pool emits `PoolFloorHold` with a `Held bool` field, once when the hold starts and once when it clears. `idleCause` returns `CauseDiskFloor` first when `t.Held` is true. The underfill exclusion at `go/internal/orchestrator/pool.go:138` also skips `CauseDiskFloor`. `CauseDiskFloor` joins `IdleCauses`. The transition has its own code (detail after this table). | F4.2 | Reuse `PoolHeld`: it means a file-claim refusal. A check after `Pending == 0`: an empty list then reports `supply_empty`. |
| D18 | The sentinel writes the hold decision for each lane, not for each interval. In shadow, a lane launch while `would_hold` is true writes one `Kind: decision` record. Its `Cycle` is the cycle of that lane. A **false stop** of one record is `would_hold && Outcome.Shipped && free_bytes >= floor_bytes/2`. The other rules are in the detail after this table. | F7.4 | An interval record with no lane cycle: `Scorer.Classify` works on one record (`score.go:116-119`). A new `Kind`: `Score` counts only `KindDecision` (`score.go:256`). The dispatch inside `stageevidence`: the `evidence` package imports `stageevidence`, so an import cycle follows. A scorer that reads the lease files at verify time: the leases are gone by then. |
| D19 | The checked-in policy gains `fleet.orchestrator.window.adaptive` with `{"stage": "shadow"}`, so the load sampler runs. A policy test pins that the block exists. | F2.2 | An explicit decision to leave load out: the request names the system status, and the code exists. |

Codes of the sentinel. The Kind `loop.wave` is the only Kind that `evolve wave watch` prints (`cmd_wave_status.go:349-353`). C9 extends the filter to print INCIDENT as well as WARN. `signalcenter` has only INFO, WARN and INCIDENT (`event.go:21-23`).

| Code | Kind | Severity | Fields |
|---|---|---|---|
| `HOST_SENTINEL_DISK_FLOOR` | `host.sentinel` | WARN | `free_bytes`, `floor_bytes`, `warn_bytes`, `path` |
| `HOST_SENTINEL_DISK_WARN` | `host.sentinel` | INFO | `free_bytes`, `warn_bytes`, `path` |
| `HOST_GOCACHE_OVER` | `host.sentinel` | WARN | `cache_bytes`, `limit_bytes` |
| `HOST_ORPHAN_SERVERS_HIGH` | `host.sentinel` | WARN | `count`, `limit` |
| `HOST_ORPHANS_HIGH` | `host.sentinel` | WARN | `count`, `limit` |
| `HOST_RELEASED` | `host.sentinel` | INFO | `freed_bytes`, `cache_bytes` (re-measured) |
| `HOST_WOULD_RELEASE` | `host.sentinel` | INFO | `scopes`, `free_bytes`, `cache_bytes` |
| `HOST_WOULD_HOLD` | `host.sentinel` | INFO | `would_hold`, `free_bytes`, `floor_bytes` |
| `HOST_RELEASE_SKIPPED` | `host.sentinel` | INFO | `reason` |
| `HOST_LANE_HOLD` | `loop.wave` | WARN | `free_bytes`, `floor_bytes`, `running` |
| `HOST_LANE_HOLD_STUCK` | `loop.wave` | INCIDENT | `held_s`, `free_bytes` |
| `HOST_SENTINEL_BLIND` | `host.sentinel` | WARN | `reason` |

Detail of D6, D11, D17 and D18:

- D6: `gcRun.step` wraps each step in `context.WithoutCancel` (`cmd_gc.go:166`). The mid-wave call therefore uses a step variant that the context can cancel, and `cacheScope` checks the context between its two steps. The step list stays one (D5).
- D6: a stop waits for a `TrimGoCache` walk that already runs, because `TrimGoCache` takes no context (`gocache.go:50`).
- D11: after a run that frees fewer than `no_progress_bytes`, the same finding starts no run until its reading grows.
- D17: the new code is `LOOP_ORCH_POOL_FLOOR_HOLD` under `ModuleLoop`, in `orchestrator.go`. It joins `transitionCodes` (`orchestrator/pool.go:52-59`) and the `idleOrHeld` test (`:125`). `HOST_LANE_HOLD` cannot serve, because `Publish.emit` stamps `ModuleLoop`. `evolve signals codes generate` rewrites `signal-codes.md`.
- D18: the record fields are `would_hold`, `free_bytes`, `floor_bytes` and the live lease ids. The release decision is one `Kind: decision` record for each `would_release`.
- D18: a record whose cycle has no chained `Outcome` does not count toward readiness.
- D18: the key-to-scorer dispatch lives in `cmd/evolve` `verifyStageEntry` (`cmd_stage.go:95`, `:208`). `stageevidence.ScorerFor` stays unchanged.

Detail of D9:

- The loop runs while `len(p.running) > 0 \|\| (held() && !capped() && !p.stopped() && ctx.Err() == nil)`. A pool at `MaxLaunches` with the hold on cannot launch again, so it does not wait.
- The `select` gains a `ctx.Done()` case and a `Wake()` case. After the `ctx.Done()` case fires once, the loop sets its channel to nil, as it does for `kick` (`fleet/pool.go:87-90`). Without that step, a cancel with running lanes spins the loop at full CPU until the lanes drain.
- While the hold is on, the sentinel sends `Wake()` on each tick. The pool then checks the stop clause again within `interval_s`, and a brake or a `Halted` result ends a held pool with 0 lanes.
- `dispatchPool` passes `Halted: b.poolHalted` to both runners. `rollingPool` receives it as well.
- A wake runs `fill`, then `resupply`.
- `cacheScope` keeps its own trim rule `gc.cache_trim_free_gb` (F3.4). D13 keeps the sentinel off that rule.

The chain for D7:

- *Fact* F3.2 and F3.3: sealed-cycle processes, sockets and stopped-lane dispatches already have live sweeps.
- *Fact* F3.6: the dead-owner case of `StaleDispatch` does not check the cycle, so it can kill a held cycle.
- *Implication:* a mid-wave process release gains little and risks a held cycle.
- *Decision:* detect only, and keep the rule of `cli-contract.md`.
- *Consequence:* `cacheScope` touches no process at all, so no held cycle can lose one. A later item may add a process check only on shadow evidence of a residue after the live sweeps.
- *Change of mind:* a shadow signal count of `HOST_ORPHANS_HIGH` that stays high after two seals of the live sweeps justifies that later item.

### Worked example

The wave runs for 6 hours. At minute 361 the sentinel reads these values:

- minimum free disk 8 GB (floor 10 GB, warn 20 GB);
- Go cache 41 GB (limit 20 GB);
- 3 owner-dead servers and 3 orphan processes.

1. `Decide` finds two crossings: `HOST_SENTINEL_DISK_FLOOR` (8 GB, floor 10 GB) and `HOST_GOCACHE_OVER` (41 GB, limit 20 GB).
2. At shadow, the sentinel emits the two signals and one `HOST_WOULD_RELEASE` with `scopes`, `free_bytes` and `cache_bytes`, and one `HOST_WOULD_HOLD`. It appends the stage records. It sets no hold.
3. At enforce of the release stage, it tries the gc lease with wait 0. The lease is free. It runs `cacheScope` under its budget. It emits `HOST_RELEASED` with the freed bytes and the re-measured cache size.
4. At enforce of the hold stage, free disk is still under the floor. The hold stops new lanes and `HOST_LANE_HOLD` goes out. The 4 running lanes finish, and the pool does not return. At 21 GB free the hold clears and the sentinel signals `Wake()`. The pool fills, calls `resupply` if the list is empty, and starts a lane.

## 6. Components and files

| Id | Component | File (new or changed) |
|---|---|---|
| C1 | Pure decision core, sample type, dedupe state, codes and the `ModuleHost` publisher. It imports only `signalcenter`. | new `go/internal/hostsentinel/decide.go`, `signals.go` |
| C2 | Policy block `gc.sentinel`, its defaults and the two typed stage fields | changed `go/internal/gcpolicy/gcpolicy.go`, `go/internal/policy/gc_for.go` |
| C3 | The hold in the fleet pool: `PoolConfig.Hold`, the `open()` gate, the loop condition, the `ctx.Done()` and `Wake()` cases, `resupply` on a wake, and `PoolFloorHold` | changed `go/internal/fleet/pool.go` |
| C4 | The idle cause: `idleCause` checks `t.Held` first, `CauseDiskFloor` joins `IdleCauses`, and the underfill exclusion skips it | changed `go/internal/orchestrator/idle_cause.go`, `go/internal/orchestrator/pool.go`, `go/internal/orchestrator/orchestrator.go` |
| C5 | Both runners pass `Hold` and `Halted` from `dispatchPool` to `fleet.PoolConfig` | changed `go/cmd/evolve/cmd_loop_pool.go`, `go/internal/orchestrator/pool.go` |
| C6 | Host reader in `cmd/evolve`: minimum free disk, cache size, owner-dead server count and orphan count. It runs each child under the sentinel context. It takes injected functions. | new `go/cmd/evolve/cmd_loop_sentinel_reader.go` |
| C7 | Mid-wave run: lease with wait 0, then `cacheScope` under a budget. The file adds a cancellable step variant, and `cacheScope` checks the context between its steps. The step list stays in `cacheScope`. `freed_bytes` is the free-disk delta, and the loop logs get the gc output through buffered writers. | new `go/cmd/evolve/cmd_gc_midwave.go`, changed `go/cmd/evolve/cmd_gc_scope.go`, `go/cmd/evolve/cmd_gc.go` |
| C8 | Sentinel shell: goroutine, ticker, the ndjson record and the stage records. It reads free disk before the runner starts. It sends `Wake()` on each tick while held. It keeps the no-progress backoff. Start and stop in `dispatchPool`. | new `go/cmd/evolve/cmd_loop_sentinel.go`, changed `go/cmd/evolve/cmd_loop_window.go` |
| C9 | Last sample in `wave status`, and the `wave watch` filter that prints INCIDENT and the hold line | changed `go/cmd/evolve/cmd_wave_status.go` |
| C10 | Stage seeds `gc.sentinel.hold.stage` and `gc.sentinel.release.stage` in `seed_stages.go`: signals `HOST_WOULD_RELEASE` (release seed) and `HOST_WOULD_HOLD` (hold seed), the emit-site func and the `would_*` fields. The closure is `internal/hostsentinel` only. The scorer stays outside it. `DecisionVersion` hashes the whole closure (`version.go:163-187`), and `stageevidence` has 33 module-local dependencies. Its own fixture and golden tests pin it. `PolicySubtree` covers `gc.sentinel` and the three reused gc keys. | changed `go/internal/gateregistry/seed_stages.go`, the policy schema files |
| C11 | Stage record builder and `Score`. The key-to-scorer dispatch sits in `verifyStageEntry`. The enforce actor of each stage sits behind a port. The outcome and seal writer of the stage contract (E2, E3) is not built. It is a dependency of each flip, not of the build. | new `go/internal/hostsentinel/evidence/score.go`, changed `go/cmd/evolve/cmd_stage.go` |
| C12 | Docs: scope table, the amended rule and the packages. Files: `wave-operations.md`, `cli-contract.md` §4.6 D10 and F9.3, the `liveRunDirs` message (`cmd_gc.go:101`), five package notes and the new `internal-hostsentinel.md`. Run `evolve signals codes generate` for `signal-codes.md`. | changed and new docs |
| C13 | The adaptive shadow block in the checked-in policy, and its policy test. The checked-in `gc.disk_floor_gb` is 25 and `gc.disk_warn_gb` is 40. Both sit above the 5% guard line, about 24 GB on this 483 GB volume (`disk_percent.go:10`). A clear then never launches lanes into the guard pause. | changed `.evolve/policy.json` |

Corrections of the terms. The old Terms called worktrees and run dirs boundary-only. That is wrong: `SweepSealedCycle` releases sealed lanes mid-wave, and the file sweeps judge each resource. The term boundary-only now means "`cacheScope` does not run it". The amended text of `cli-contract.md` says so in one place.

Protected surface. C3, C5, C8 and C10 change the pool path, the stage seeds and the policy schema. `internal/gateregistry` (`go/internal/guards/integrity_surface.go:238`) and `.evolve/policy.json` (`:55`) are on the protected surface. `gcpolicy.go` and `gc_for.go` are not. The item stays protected through `gateregistry`, so the review needs full seats, an adversarial seat and a security seat (operator decisions of 2026-11-03).

CI parity. The reader uses `looppreflight.DiskFreeBytes`, which has a unix build (`disk_unix.go`, darwin and linux) and a non-unix stub (`disk_other.go`). The tmux count and the process list go through `sysexec.RunFunc`, so a test injects a fake and runs the same on Linux. The code uses no feature newer than Go 1.23 (`GOTOOLCHAIN=go1.23.12`). No setting reads the environment.

## 7. Red tests

Each test in the first table fails on the current tree and is written first. The guard tests in the second table pass today and stay as guards.

| Test | Package | What it proves | Acceptance |
|---|---|---|---|
| `TestDecide_EmitsOneFindingForEachCrossedThreshold` | `hostsentinel` | A sample over the disk, cache, server and orphan limits gives four findings. Each has the value and the limit. | 1 |
| `TestDecide_ARepeatInsideRepeatSecondsEmitsNothing` | `hostsentinel` | A second `Decide` at `now+repeat_s-1` gives no finding. At `now+repeat_s` it gives one. A value of twice the limit, or a free disk of half the floor, repeats at once. | 1 |
| `TestDecide_APersistentOrphanFindingStartsNoRelease` | `hostsentinel` | An orphan finding that stays over 10 ticks emits signals and starts zero releases. A disk under warn starts one and then waits for `repeat_s`. | 2 |
| `TestMidWave_LeavesLiveResourcesInPlace` | `cmd/evolve` | The fixture has four live resources: a held temp dir, a fresh cache entry, a mapped `<hash>-d` dir and a live socket dir. The run keeps all four. It removes a stale entry. | 2 |
| `TestMidWave_RunsOnlyCacheScope` | `cmd/evolve` | The run calls the cache step and the temp step and no other `gcRun` step. | 2 |
| `TestSentinel_ShadowStageReleasesNothing` | `cmd/evolve` | At shadow the fake mid-wave run has zero calls, and one `HOST_WOULD_RELEASE` is emitted with the scope names. | 2 |
| `TestSentinel_AHeldGCLeaseSkipsAndDoesNotWait` | `cmd/evolve` | The test holds the real flock of `gc.lock` from a second file descriptor. The sentinel runs no step and emits one `HOST_RELEASE_SKIPPED`. A second tick inside `repeat_s` emits none. | 3 |
| `TestMidWave_AStopDuringARunReturnsInTheBudget` | `cmd/evolve` | A stop during a fake release returns inside the budget and cancels the child. | 5 |
| `TestPool_HoldWithNoRunningLaneStartsNothingAndDoesNotReturn` | `orchestrator` | Through `orchestrator.Pool.Run`: with 0 running lanes and the hold set, nothing launches and the pool does not return. After the clear and a wake, a lane launches. | 4 |
| `TestPool_ACancelWhileHeldWithNoLaneReturns` | `fleet` | A `ctx` cancel while held with 0 lanes returns the pool. | 5 |
| `TestPool_HaltedWhileHeldWithNoLaneReturns` | `fleet` | `Halted` while held with 0 lanes returns the pool. | 5 |
| `TestPool_ABrakeEngagedWhileHeldWithNoLaneReturnsAfterTheNextTick` | `cmd/evolve` | Under `orchestratedRun` and under `rollingPool`, with a fake ticker: set the hold with 0 lanes, then engage the brake file. The pool returns after the next tick and launches nothing. | 5 |
| `TestPool_ACancelWithTwoRunningLanesReturnsAfterBothComplete` | `fleet` | A cancel with 2 running lanes returns the pool after both completions. The `ctx.Done()` case is handled once. | 5 |
| `TestPool_ACappedPoolWhileHeldWithNoLaneReturns` | `fleet` | A pool at `MaxLaunches` with the hold on and 0 lanes returns. | 5 |
| `TestPool_AClearWakesThePoolUnderBothRunners` | `cmd/evolve` | A clear starts a lane under `rollingPool` and under `orchestratedRun` with `Admit == nil`. With an empty pending list, the wake calls `resupply`. | 4 |
| `TestPool_HoldLetsTwoRunningLanesFinish` | `orchestrator` | With 2 running lanes and the hold set, both finish and no third starts. | 4 |
| `TestPool_AHoldReportsDiskFloorNotUnderfill` | `orchestrator` | A hold with an empty pending list emits `PoolFloorHold` and idle cause `disk_floor`. It emits no `LOOP_ORCH_POOL_UNDERFILLED` and no `supply_empty`. | 4 |
| `TestSentinel_DiskUnderTheFloorHoldsAndAboveWarnClears` | `cmd/evolve` | A disk at floor-1 sets the hold. A disk between floor and warn keeps it. A disk at warn clears it and signals `Wake()` once. A hold past `hold_max_s` emits one `HOST_LANE_HOLD_STUCK` and keeps the hold. | 4 |
| `TestSentinel_TheFirstSampleHoldsBeforeTheFirstLaunch` | `cmd/evolve` | Free disk at floor-1 when the pool starts launches zero lanes. The start reads only free disk. | 4 |
| `TestSentinel_ANoProgressReleaseStartsNoSecondRunUntilTheReadingGrows` | `cmd/evolve` | A run that frees under `no_progress_bytes` blocks the next run for the same finding until the cache reading grows. | 2 |
| `TestMidWave_AStopBetweenTheStepsSkipsTheTempStep` | `cmd/evolve` | A cancel after the cache step runs no temp step. The step list is the one of `cacheScope`. | 5 |
| `TestPool_TheFloorHoldTransitionHasARegisteredLoopOrchCode` | `orchestrator` | `PoolFloorHold` maps to `LOOP_ORCH_POOL_FLOOR_HOLD` in `transitionCodes` and counts in `idleOrHeld`. | 4 |
| `TestSentinel_StopsWithTheLoopAndLeavesNoTask` | `cmd/evolve` | After the stop function returns, the `WaitGroup` has returned, the fake ticker is stopped, and an in-flight fake `RunFunc` has seen its context cancel. No goroutine count is read. | 5 |
| `TestSentinel_AFailedSampleWarnsOnceAndTheLoopGoesOn` | `cmd/evolve` | A reader that fails 3 times gives one `HOST_SENTINEL_BLIND` and one warning. The next good sample acts. | 5 |
| `TestSentinel_RunsUnderBothPoolRunners` | `cmd/evolve` | With the orchestrator stage at `shadow` (the `rollingPool` path) and at `enforce`, a hold stops new lanes in each. | 4 |
| `TestMidWave_RemeasuresTheCacheAfterTheTrim` | `cmd/evolve` | A fake trim reports 20 GB and the fake measure says 53 GB. The report shows 53 GB. | 1 and 2 (F3.5) |
| `TestSentinelNdjsonPath_FollowsTheHostSamplesPathRule` | `cmd/evolve` | With `--log` set, the sentinel file sits in the run log dir, as `host-pressure.ndjson` does. | 6 |
| `TestWaveStatus_ShowsTheLastHostSentinelSample` | `cmd/evolve` | `wave status` reads the dir from `wave.Record.LogPath`, prints the last ndjson line and handles a missing file. | 6 |
| `TestWaveWatch_PrintsTheHoldLineAndTheStuckIncident` | `cmd/evolve` | `wave watch` prints `HOST_LANE_HOLD` (WARN) and `HOST_LANE_HOLD_STUCK` (INCIDENT). | 6 |
| `TestStageSeeds_PinTheDescriptorOfEachPopulatedRow` (extended) | `gateregistry` | The test loops over the existing entries only (`stage_conformance_test.go:528`). A reverse check is added: each `want` key must have an entry. The test then fails until the seeds of both new keys exist, each with its closure, subtree and `would_*` fields. | 6 |
| `TestStageVerify_ScoresAFixtureChainOfTheSentinel` | `stageevidence` | `evolve stage verify` on a fixture chain of each key gives the expected counts of conflicts and false stops. | 6 |
| `TestScore_CountsConflictsAndFalseStops` | `evidence` | A `would_release` whose record lists a live lease id counts as a conflict. A `would_hold` record whose cycle shipped with free disk at or over half the floor counts as one false stop. | 6 |
| `TestScore_AWouldHoldRecordWithNoOutcomeIsNotReady` | `evidence` | A `would_hold` record with no chained `Outcome` gives not ready and no false-stop measurement. | 6 |
| `TestEnforceActor_ReachesTheHoldThroughThePort` | `evidence` | A spy port records one hold call at enforce and none at shadow. | 6 |
| `TestTheCheckedInPolicy_HasTheAdaptiveShadowBlock` | `policy` | The checked-in policy has `fleet.orchestrator.window.adaptive` with stage `shadow`. | 1 |
| `TestSentinel_CUJ_ThroughTheRealLoopEntry` | `cmd/evolve` | A loop run with a fake host reader emits the signals through the real signal center. This is the regression test through the real entry point. | all |

| Guard test | Package | What it keeps |
|---|---|---|
| `TestEveryStageKeyHasRegistryEntry` and the three siblings of `stage_conformance_test.go` | `gateregistry` | They pass today. They stay green with the two new seeds: a registry entry, a typed field, a non-test emit site and the `would_*` fields. |

Acceptance 6 maps to three gates. The gate `evolve docs ste-lint --strict` checks the docs. The gate `cover changed` checks 100% coverage of each changed function. The comment floor (`go/internal/commitgate/comments_added.go`) checks the rule of no comments.

The policy test `TestTheCheckedInPolicy_EnforcesTheStagesTheOperatorMoved` stays unchanged. It asserts that each listed value is `enforce`, and the two new keys are not on its list while they are at `shadow`.

## 8. Rollout

Both stages start at `shadow` (D10). In shadow, the sentinel does the following:

1. It emits the `HOST_*` signals, the `HOST_WOULD_RELEASE` signal and the `HOST_WOULD_HOLD` signal.
2. It writes the ndjson sample.
3. It sets no hold and runs no release.

The build of 2026-11-21 stops at shadow. It does not append stage records, and `Score` does not exist yet. So `evolve stage verify` cannot recompute a count for these two keys, and it makes no claim about them. The code of the hold and of the release exists and has tests. Nothing enables it.

The follow-up item `host-sentinel-stage-evidence` builds the evidence path. Its scope is:

- The launch hook that gives the cycle number of each lane to the sentinel. The sentinel then writes one `Kind: decision` record for each lane launched while `would_hold` is true (D18).
- The key-to-scorer dispatch in `verifyStageEntry`, and the scorer in `internal/hostsentinel/evidence` (C11).
- The outcome and seal writer of the stage contract (E2 and E3), and the enforce actor behind its port with a spy test.
- The criteria to flip each stage: two clean waves in shadow under one decision version. The release needs zero conflicts. The hold needs zero false stops.
- A **conflict** is a `would_release` record that lists a live lease id. - A **false stop** is a `would_hold` record whose lane cycle shipped PASS with `free_bytes` at or over half the floor.

Until that item lands, an operator must not set either stage to `enforce`. The boundary gc stays unchanged.

## 9. Risks

| Risk | Control |
|---|---|
| A mid-wave run removes a resource of a live lane. | The live-resource test, the shadow conflict count, and the fact that `cacheScope` touches no worktree, session or process. |
| The hold stops the pool when disk is low for a reason that gc cannot fix, for example a large run dir. | `HOST_LANE_HOLD` names the free bytes. `HOST_LANE_HOLD_STUCK` pages at `hold_max_s`. The hold stays under the floor (D16). |
| A hold with no running lane never ends, also on a stop or a brake. | The loop condition of D9 has a stop clause, and each tick of a held sentinel sends `Wake()`. Red tests cover cancel, halt, brake, cap and clear. |
| The hold starts after the first lanes. | The first sample runs before the runner starts (D1). |
| The sample is slow (a walk of a large cache) and delays the loop. | The sentinel runs in its own goroutine. The cache is measured every 10th tick with its own budget. Each child runs under the sentinel context. |
| A persistent finding that the run cannot fix repeats heavy work. | D11 maps each finding to its release, and the run has a cooldown and a no-progress backoff. |
| The disk guard owns the band between the hold floor and its pause line. | C13 sets the floor and warn above the guard line (25 GB and 40 GB). |
| Two sentinels run when two loops share a host. | The gc lease serializes the release. The hold is local to each loop. |
| The under-trim of F3.5 hides the real cache size. | D13 re-measures. X2 fixes `TrimGoCache` for the `<hash>-d` entries. |
| The decision version changes with each merge. | The package is a leaf (D2). |
| A new policy key breaks an old `policy.json`. | The block is optional. An absent block means the defaults, with both stages `shadow`. |

Limits. The sentinel does not free a disk that other programs fill. A test cannot prove the `statfs` figure of a real volume. The design does not cover caches outside the pipeline, the salvage leaves (K5), the test sockets (X8) or the sequential path.

## 10. Open questions and follow-ups

- **Q1.** Does the hold need a maximum time? Decided in D16: it emits an incident at `hold_max_s` and keeps the hold.
- **Q2.** Is the orphan count limit a number or a ratio of the live lanes? Assumption: a number, with the default 20.
- **Q3.** Does the gh HTTP cache scope join the mid-wave run? The recycle plan holds it as report-only (check K11). Assumption: not in this item.

Follow-ups. Each is a separate item with its own red tests and stage.

- **FU1.** A cycle-start guard on bytes. It needs a new reading of `HOST_DISK_LOW` fields, and a decision on the default floor.
- **FU2.** A predicate for the orphans of a cycle that never sealed: ppid 1, the cycle tree exists, no live run lease and a stale heartbeat. Build it only on shadow evidence that `HOST_ORPHANS_HIGH` stays high after two seals (D7).
- **FU3.** A move of the mid-wave list to the check registry of the recycle plan, when X1a to X1e land.
- **FU4.** `host-sentinel-stage-evidence`: the evidence path of the two stages (section 8).
