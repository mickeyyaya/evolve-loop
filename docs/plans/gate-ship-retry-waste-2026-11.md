# Plan: lanes spend their time on work, not on gates that wait, repeat or fail for host reasons

- **Status:** partly built, 2026-11-21 (6 of 8 components on main). Earlier status: proposed, 2026-11-16.
- **Research:** [Gate, ship and retry waste](../research/gate-ship-retry-waste-2026-11.md). **Decision:** no ADR for Stage 1, because it changes no boundary and no guarantee. Stage 2 (the scoped re-check at `enforce`) gets an ADR, because it changes what a gate proves.
- **Specs that change:** the package notes of each package that a component changes (section 6).

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

The operator, 2026-11-16, after the wave-174 health readings:

> "Good finding, then working on the improvement tasks and designs to cut down the wasted time on the gate, ship, retries, ultrathink"

Terms used throughout:

- **Lane time.** The wall-clock time of one lane from dispatch to seal.
- **Re-check.** The composed-tree gates that a lane runs after a peer lands and its fast-forward merge fails.
- **CPU share.** The `-p` and `GOMAXPROCS` value that `lanebudget` gives each `go test` of a lane.
- **Base attribution.** A check that runs a failed test on main alone, to learn if main or the lane owns the failure.
- **Scoped re-check.** A re-check that tests only the packages that both the lane and the new main commits can affect.

## 2. Facts

| Fact | Research id |
|---|---|
| Gates and ship take 62% of lane time in waves 173 and 174. The LLM takes about 37%. | F1.1 |
| Each lane gets the host CPUs divided by the pool limit: 14 / 8 = 1 CPU. Only 2.3 to 3.1 lanes were live. A lone lane still spends 55 minutes in the composed gates. | F1.2 |
| The verify lock serializes the build-floor coverage run and the audit ACS suite. The waits took 3.14 hours in wave 173 and 7.65 hours in wave 174. | F1.3 |
| The re-check after main moved cost 8.25 hours in wave 174. Four serial `make` gates take a median of 55 minutes. 4 of 12 re-check windows collided with the next peer landing. | F1.4 |
| The importer backstop dies at its 20-minute cap at 1 CPU. The kill then goes to an LLM re-audit and the same retry on an unchanged tree: about 5.3 hours and 2 audited lanes lost. | F1.5 |
| All 4 composed-gate declines of wave 174 were the leak guard, not a failed test. The composed gates have no base attribution, so each red became an LLM re-audit: 4.44 hours. | F1.6 |
| Correction rounds of the build floor cost 10.0 hours. Cheap static checks report only after the long tests. | F1.7 |
| A passing audit goes back to Build for an explanation-doc fix, then a full re-audit: 3.34 hours. | F1.9 |
| The repo-contract gate runs on each ship before the cheap audit-binding check: 2.88 hours. | F1.10 |
| Ship attempts for each shipped cycle: 2.18. From the first audit PASS to the landing, the re-check takes 52% and the re-audit 20%. | F1.11 |
| The integration train at `enforce` removes `FLEET_REBASE_NEEDED` but not `HEAD_MOVED`, and it adds a median wait of 45 minutes. | F1.12 |
| The failure detector counts only LLM minutes, so host waste does not show. | F1.13 |
| Retry loops cost 18.6 hours, 38% of lane time, in wave 174. | F1.14 |

*Implication:* one setting (F1.2) slows every gate in the list. Three routes send host or main failures to an LLM that cannot fix them (F1.5, F1.6, F1.9). The race after a peer lands (F1.4) is real, but it is long because each gate is slow.

## 3. Goals and non-goals

Goals, each measured in section 8:

1. Gates and ship take 35% or less of lane time.
2. A host or main failure never starts an LLM re-audit of an unchanged lane tree.
3. Ship attempts for each shipped cycle fall from 2.18 to 1.3 or fewer.
4. Host waste shows in the readings of each boundary.

Non-goals:

- No gate proves less at `enforce` without an ADR and shadow evidence. Stage 1 changes when and how fast a gate runs, not what it proves.
- No speculative batching. It pays only when the cycle failure rate is under about 10% (research §2), and ours is far above that.
- The integration train stays at `shadow`. F1.12 shows that `enforce` adds a wait and keeps `HEAD_MOVED`.

## 4. Candidate designs

| Criterion (weight) | 1. Enforce the integration train | 2. Fast gates first, then a scoped re-check (chosen) | 3. A FIFO ship ticket only |
|---|---|---|---|
| Hours saved in a wave like 174 (3) | 1: removes some rebases, adds 45 min waits | 3: all gates faster, three LLM loops gone | 2: ends collisions, gates stay slow |
| Integrity risk (3) | 2: a new merge path at `enforce` | 3: Stage 1 proves the same; Stage 2 waits for shadow evidence | 3: none |
| Cost to build (2) | 1: 5 ports unbuilt, compose has no gate | 2: about 7 small components, mostly existing code | 2: one new lock |
| Evidence before each step (2) | 1 | 3: each component has a measure | 2 |
| **Weighted total** | **14** | **30** | **22** |

Design 2 wins. A slow gate makes every other waste larger: the re-check window, the collision rate and the deadline kill. So the first change is the CPU share. The FIFO ticket stays in the plan, but only after the re-check is short. With a short re-check the ticket costs little, and with a long re-check it makes lanes wait in line.

What changes our mind: after Stage 1, ship attempts can stay above 1.6. Then collisions dominate, and the FIFO ticket moves ahead of the scoped re-check.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | The CPU share of a `go test` uses the live lanes at its start, not the pool limit. The pool writes `{pool pid, live width}` atomically to one file under the plane `.evolve` directory at each lane start and end. A registered `ipcenv` protocol key names that file to each lane. `lanebudget` ignores the file when the pool pid is dead and uses the spawn width. The audit retake rebuilds its `-p` after it takes its lock. | F1.2, F1.4, F1.5, F1.8 | A host token pool now: more parts. Measure the oversubscription of D1 first (section 8). Raise the cap of each gate: it hides the slow test and keeps the lost time. A new environment variable outside `ipcenv`: the hard rules allow only registered protocol keys. |
| D2 | A gate killed at its deadline is a capacity failure. It retries once with the full live share and never routes to an LLM re-audit of an unchanged tree. | F1.5, F1.14 | Keep `precondition-reaudit`: the tree did not change, so the LLM has nothing to fix. |
| D3 | A red composed gate first runs its failing packages on main alone. A failure on main parks the lane under the main-red stop. Only a failure that main does not have goes to a re-audit. | F1.6, research P2 (base attribution) | Re-audit each red: four of four reds in wave 174 needed no LLM. |
| D4 | The early fast-forward check of ship (`early_fast_forward.go`) also covers a lane audited off its lane base. So a binding error stops before the repo-contract gate. The composed gates write a receipt of the packages they passed on the rebased tree. The importer backstop removes those packages from its targets when its tree is the receipt tree. | F1.10 | Reuse through `scanShadow`: its key holds the whole-tree id, so a rebase never matches (F1.10). Run the gate twice: the same tree and the same package give the same result. |
| D5 | Stage 2: a passing audit with only an explanation-doc finding gets the small prescribed fix in place (ADR-0158). The fix must re-seal the explanation snapshot (`explanationdocs.SealResult`) and write a typed rebind record. That record proves that the new tree differs from the audited tree only in the explanation document and its handoff. | F1.9 | Stage 1: without the re-seal and the rebind, the ship gates refuse the lane (`verifyNativeExplanation`, `CodeIntegrityTreeDrift`). A Build round: 3.34 hours for a doc fix. |
| D6 | The build floor runs in two stages. Stage A runs every cheap static check and reports all its failures together. Stage B runs the long tests only when stage A is green. | F1.7 | Stop at the first red: it hides a second residual, and ADR-0158 collects all residuals of one round. Keep the order: a lint red waits for 20 minutes of tests. |
| D7 | The failure detector and the readings count host minutes beside LLM minutes. A sampler records the host load for each gate span. | F1.13 | Only LLM minutes: host waste stays invisible. |
| D8 | The scoped re-check starts at `shadow`. It computes the scoped set, still runs the full re-check, and records each failure outside the scoped set as an escape. | F1.4, research P2 (conflict analysis) | `enforce` at once: an import graph misses some interactions (tests that read files, global inputs). |
| D9 | The FIFO ship ticket, the shared build cache and a full post-landing suite on main wait for the numbers of Stage 1. | F1.4, F1.11, research P2 | Build them now: their value depends on the gate time that Stage 1 changes. |

## 6. Components

Tier column: the tier that `gateclass.TierOf` gives the files of the component (ADR-0155 decision 15). T0 is a path of the protected manifest. A lane builds T1 and T2 work since ADR-0157. A T0 component stays with the console until the admission of held proposals reaches `enforce`.

| Id | Component | Red test first | Tier | Owner |
|---|---|---|---|---|
| S1 | Live-width CPU share (D1): the pool writes the width file; `lanebudget.Current` reads it; the audit retake rebuilds its flags after its lock. | `TestCurrent_ALoneLaneGetsTheHostCPUs`, `TestCurrent_ReadsTheLiveWidthNotThePoolLimit`, `TestCurrent_ADeadPoolFileFallsBackToTheSpawnWidth`, `TestPool_WritesTheLiveWidthAtLaneStartAndEnd`, `TestRetake_RebuildsTheShareAfterTheLock` | T2 (`lanebudget`, `fleet`, `ipcenv`) and T0 for the retake (`ciparitygate` is on the manifest) | console first: it gates the rest |
| S2 | Capacity class for a deadline kill, with one retry and no re-audit (D2). | `TestRecovery_ADeadlineKillOnAnUnchangedTreeRetriesWithoutReaudit`, `TestRecovery_ASecondKillParksTheLaneWithACapacityCode` | T0 (`ship/repocontract.go`, `core/ship_recovery.go`) and T2 (`router/recovery.go`) | console |
| S3 | Base attribution of a red composed gate (D3). | `TestComposedGate_ARedThatMainAlsoFailsParksTheLane`, `TestComposedGate_ARedOnlyTheLaneHasGoesToReaudit`, `TestComposedGate_ABaseRunErrorKeepsTheRouteOfToday` | T0 (`ship/base_red.go`, `baseattr`) | console |
| S4 | The early check covers each binding, and the composition receipt reaches the importer backstop (D4). | `TestEarlyFastForward_ALaneNotAuditedOnItsBaseStopsBeforeTheRepoContractGate`, `TestImporterBackstop_SkipsThePackagesOfTheReceiptTree`, `TestImporterBackstop_AReceiptOfAnotherTreeSkipsNothing` | T0 (`ship/ship.go`, `ship/repocontract.go`) | console |
| S6 | Two-stage build floor (D6), in `build_floor_reviewer.go` and the check order of `cmd_cycle_config.go`. | `TestBuildFloor_StageAReportsAllStaticFailuresTogether`, `TestBuildFloor_AStageARedSkipsTheTestRun` | T0 | console |
| S7 | Host minutes and host load in the readings (D7). | `TestVerdict_CountsHostMinutes`, `TestSampler_RecordsLoadForEachGateSpan` | T2 | lane-eligible |
| S8 | Scoped re-check at `shadow` with an escape record (D8). | `TestScopedSet_IsTheIntersectionOfBothReverseClosures`, `TestScopedSet_AGlobalInputSelectsAll`, `TestScopedSet_ATreeReadingTestJoinsWhenItsInputChanged`, `TestShadow_AFailureOutsideTheSetIsAnEscape` | T1 or T2 (a new package and the composition wiring) | console: its evidence decides Stage 2 |
| S5 | Stage 2: explanation-doc fix in place with a re-seal and a rebind record (D5). | `TestExplanationCorrection_AnInPlaceFixReSealsTheSnapshot`, `TestExplanationCorrection_TheRebindRecordAllowsOnlyTheDocument`, `TestExplanationCorrection_ACodeFindingStillGoesToBuild` | T0 (`explanationdocs`, the ship explanation gate) | console, Stage 2 |

Each component keeps 100% coverage of the functions that it changes. Each test goes through the real entry point. A lane-eligible component goes to the inbox with no console route, so the loop builds it. The tier of each file is checked again with `gateclass.TierOf` before the build.

## 7. Order

1. **S1 first.** It shortens every other gate, so the later numbers measure the new speed.
2. **S2 and S3 next.** They stop the three LLM loops that cannot fix anything.
3. **S4 and S6** in any order. They are independent. S5 waits for Stage 2, because it needs the re-seal and the rebind record.
4. **S7 with S1,** so the first wave after S1 already reports host minutes.
5. **S8 after S1 to S3,** at `shadow`.
6. **Measure 2 waves, then decide Stage 2.** Stage 2 holds three parts:
   - the scoped re-check at `enforce`, with its own ADR and a full post-landing suite on main as the safety net;
   - the FIFO ship ticket;
   - the shared build cache.

## 8. Verification

| Goal | Measure (source) | Before (wave 174) | Target after Stage 1 |
|---|---|---|---|
| G1 | Gates and ship as a share of lane time (`wave latency`) | 0.62 | 0.35 or less |
| G1 | Median composed re-check (`wave latency`) | 55 min | 20 min or less |
| G2 | LLM re-audits after a host or main failure (signals) | 4 lanes, 2 lost | 0 |
| G3 | Ship attempts for each shipped cycle | 2.18 | 1.3 or fewer |
| G4 | Host minutes in the readings | absent | present for each wave |
| S1 risk | Host load and memory pressure with live-width shares (S7 sampler) | not sampled | load under 2 times the CPU count |
| S8 | Escapes of the scoped set in shadow | none measured | 0 in 2 waves before Stage 2 |

The health readings of each boundary hold these numbers.

## 9. Risks

| Risk | Control |
|---|---|
| A lane that starts late oversubscribes the CPUs that early lanes took. | The share is fixed only for one `go test`. S7 samples the load. A token pool replaces D1 if the load passes 2 times the CPU count. |
| The capacity retry hides a real test hang. | One retry only. A second kill parks the lane with a capacity code and a signal. |
| Base attribution blames main for a lane failure. | The base run uses the same packages and the same flags on main alone. A base-run error keeps the route of today. |
| The scoped set misses an interaction. | It stays at `shadow` until 2 waves show 0 escapes. Global inputs and tree-reading tests select the full set. |
| The in-place doc fix changes code. | It applies only to a finding on an explanation document. A code finding still goes to Build. |

## 10. Open questions

1. Must the full post-landing suite on main (research P2, TAP) come before the scoped re-check at `enforce`, or with it?
2. The `-count=1` rule exists because tests read files through `..`. The registry of tree-reading tests lists them now. Can the shared build cache come back for the other packages?
3. Does the verify lock (F1.3) stay after S1, or can the coverage run take a live share too?

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| S1 live-width CPU share | done, 2026-11-17 | [#1499](https://github.com/mickeyyaya/evolve-loop/pull/1499) [`7d48417`](https://github.com/mickeyyaya/evolve-loop/commit/7d4841722) (`go/internal/lanebudget/lanebudget_test.go`, `go/internal/fleet/pool_livewidth_test.go`, `go/internal/phases/audit/ciparitygate/tier_retake_share_test.go`) | none |
| S2 capacity class for a deadline kill | done, 2026-11-17 | [#1500](https://github.com/mickeyyaya/evolve-loop/pull/1500) [`ee9432a`](https://github.com/mickeyyaya/evolve-loop/commit/ee9432ac2) (`go/internal/core/ship_recovery_capacity_test.go`) | none |
| S3 base attribution of a red composed gate | done, 2026-11-17 | [#1513](https://github.com/mickeyyaya/evolve-loop/pull/1513) [`ac8cb42`](https://github.com/mickeyyaya/evolve-loop/commit/ac8cb424c) (`go/internal/core/composed_gate_attribution_test.go`) | none |
| S4 binding check first and composition receipt | not started: none of the three named tests exists in `go/` | none | gate-ship-s4-binding-first-receipt |
| S6 two-stage build floor | done, 2026-11-17 | [#1502](https://github.com/mickeyyaya/evolve-loop/pull/1502) [`d195127`](https://github.com/mickeyyaya/evolve-loop/commit/d195127e0) (`go/internal/core/build_floor_stages_test.go`) | none |
| S7 host minutes and load sampler | done, 2026-11-16 | cycle 2428 [`415232e`](https://github.com/mickeyyaya/evolve-loop/commit/415232ea8) (`go/internal/cyclespend/hostminutes_test.go`) | none |
| S8 scoped re-check at shadow | done, 2026-11-17 | cycle 2450 [`ced71cd`](https://github.com/mickeyyaya/evolve-loop/commit/ced71cdd2) (`go/internal/scopedset`), wired at `go/cmd/evolve/cmd_composition_shadow.go:25` | none |
| S5 explanation-doc fix in place (Stage 2) | not started: no `TestExplanationCorrection_` test exists | none | none (draft: gate-ship-s5-explanation-fix-in-place) |

Stage 1 is on main except S4, which has an open item. S5 and the Stage 2 parts (scoped re-check at `enforce`, FIFO ticket, shared build cache) wait for two waves of measures. I found no record of the section 8 measures, so the gate for Stage 2 is unverified. S5 has no inbox item, so I wrote a draft. The plan names `TestPool_WritesTheLiveWidthAtLaneStartAndEnd`, but the test file `pool_livewidth_test.go` may use another name. That name is unverified.
