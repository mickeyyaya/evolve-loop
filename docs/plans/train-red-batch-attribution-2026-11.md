# Plan: the integration train checks the CI of its base before it splits a red batch

- **Status:** built, 2026-11-19 (5 of 5 components on main). Earlier status: accepted, 2026-11-18. The design converged after three review rounds (§11).
- **Inbox items:** `integration-train-compares-a-red-batch-with-main-before-it-splits` (AC2, AC3) and `wave-integration-i7-recompose-after-console-merge` (AC3). **Decision:** no ADR. The train keeps its guarantee: a change lands only through a green batch. This plan changes only when the train splits a batch.
- **Related:** [wave-integration-2026-10.md](wave-integration-2026-10.md) (I7, I14, D33), [pr-merge-latency-2026-10.md](pr-merge-latency-2026-10.md).

## 1. Request

The operator, 2026-11-18: "Complete those tasks if it routes to console". The tasks are the merge improvement items that route to the console.

The inbox item (2026-11-16):

> "On a red batch, attribute the failed tests: a failure that main also has at the base is main-owned and must not split the batch; hold the batch and open the main-red stop. Use the existing baseattr verdict. Print one line per step: the step, its members, its CI run and its verdict."

## 2. Facts

| Id | Fact | Evidence |
|---|---|---|
| F1 | A train run refuses a red or a pending main at plan time. AC1 of the item is done. | `internal/integration/train.go:90`, `mainci.go:35-51` `checkMain`; test `TestTrainPlanChecksTheCIOfTheBaseItPlansOn` |
| F2 | Each compose takes `Main.Head` as its base and does not check the CI of that base. Main can move between the plan and a compose. | `train.go:348-372` `batcher.compose` |
| F3 | On a red batch, the train runs the failed jobs again once. A second red splits the batch at once, at each level of the split. | `batchpr.go:111-113`, `batchpr.go:173-191` `Merger.test` |
| F4 | `checkMain` already turns the CI verdict of a commit into a `MainRefusal`. It waits for a pending run in a live train, and `--main-red-fix` overrides a red verdict. | `mainci.go:35-51`; `ghci.go:133` `GhCI.Watch` |
| F5 | The single opener of the main-red stop is in `core` (`OpenMainRedStop`, `MainRedSink`). It writes the stop, files the fix item and emits `CodeMainRedStopOpened`. The fix item is what releases the stop. | `core/main_red_stop_open.go:41-60`, `core/base_red_floor.go:31,206`; `cmd_integration_stop.go:84-94` |
| F6 | A host run of the failed keys can differ from CI: the OS, `-race`, the build tags and the host load all differ. A false main-red verdict parks every lane. | `.github/workflows/go.yml` test steps; design review round 1, H4 |
| F7 | A step line names the PRs, the batch PR and the verdict, not the CI run. `GhCI.Watch` already finds the run id. | `cmd_integration_batch.go:136-151` `stepLine`; `batchpr.go:62-67` `Step`; `ghci.go:134` |
| F8 | `stopCI` embeds the `CI` interface. Only methods of `CI` reach the train through the stop gate. | `stopgate.go:10-21`; `cmd_integration_batch.go:54` |
| F12 | Two frozen ACS predicates use the `CI` contract: `go/acs/cycle2414` implements `CI.Watch`, and `go/acs/cycle2239` calls `GhCI.Watch`. A change to the `Watch` signature changes protected regression evidence. | `go/acs/cycle2414/predicates_test.go:45`; `go/acs/cycle2239/predicates_test.go:199-254` |
| F9 | Inside one train run, a member that a console merge put on main is composed again. `settle` closes only `Plan.Kept` and `Plan.Landed`. | `train.go:167-181` `splitLanded`; `train.go:282-298` `settle`; `batchpr.go:36-40` `Composition` |
| F10 | Merge queues hold a failed batch before they bisect it, so that more evidence can show a transient or a base failure. | Trunk, "pending failure depth" and "batching" docs; Mergify, "batching" (live search, 2026-11-18) |
| F11 | On 2026-11-14 the train bisected for 3 hours on a red main. | memory note `batch_needs_green_complete_main`, batch 177 |

## 3. Goals and non-goals

**Goals:**

- G1. When a batch stays red and the CI of its base is not green, the train does not split. It holds the batch and names the base and its run. (item AC2)
- G2. Each step line names its CI run. (item AC3)
- G3. A member that is on main at a new base is not composed again in the same run. It is settled as landed. (I7 AC3)

**Non-goals:**

- Open the main-red stop from the train. The stop has one opener in `core` (F5). A lane that builds on the red base opens it through the base-red floor. The train refuses, as it refuses at plan time.
- Tell a flaky test from a lane defect when the base is green. With a green base, the split of today stays. A flake history is a later feature.
- Cancel the CI run of an old tip (I7 AC2). D33 gives it to the lane driver I14.

## 4. Candidate designs

| Design | Evidence for "the base is red" | Host CPU | Extra wait | False-hold risk | Change size |
|---|---|---|---|---|---|
| A. Host attribution | `baseattr` runs the failed keys at the base on the host | one `go test` per red batch | minutes | high: the host differs from CI (F6), and a mixed red with a build failure blames main | large: artifact reader, adapter, stop opener |
| B. CI attribution | a dispatched CI run of the failed keys at the base | none | one more CI run | low | large: a workflow input |
| C. Flake history | the failed keys of recent main runs | none | none | medium: history is a pattern, not a cause | needs a store that does not exist |
| D. More reruns | none | none | N CI runs | none, but no gain: a red base still splits | small |
| E. Base CI check | the CI verdict of main at the batch base, through `checkMain` (F4) | none | none, or the rest of a pending base run | low: same jobs and flags as the batch | small: one func port and its wiring |

**Scores** (5 is best):

| Criterion | A | B | C | D | E |
|---|---|---|---|---|---|
| Meets G1 with proof | 3 | 5 | 2 | 1 | 4 |
| Cost in wall time | 4 | 2 | 5 | 2 | 4 |
| No load on the shared host | 3 | 5 | 5 | 5 | 5 |
| Reuses code that exists | 3 | 2 | 1 | 4 | 5 |
| Risk of a false hold | 2 | 4 | 3 | 5 | 4 |
| Total | 15 | 18 | 16 | 17 | 22 |

**Why E wins:** after F1, a batch base is green at plan time. A base goes red only when main moves to a red commit (F2). The CI of that commit answers the question with the same jobs and flags as the batch (F6), and `checkMain` already reads it (F4). E adds no host run, no artifact parser and no second stop opener (F5). E scores lower than B on proof, because a green base with a flaky test still splits. That case stays out of scope.

## 5. Decisions

| Id | Decision | Research id |
|---|---|---|
| D1 | `Merger` gets one func port, `Base func(ctx, base string) error`, beside its other func ports. A nil port keeps the split of today. | F3 |
| D2 | The port runs when the rerun stays red, before the split, with the base of the same loop pass. It runs at each level of a split. | F3 |
| D3 | `Train` wires the port to `checkMain`. A `MainRefusal` stops `Merger.land` with no split. Any other error stops it too. A green base splits as today. | F4 |
| D4 | `--main-red-fix` keeps working: `checkMain` returns the overridden state, so a fix batch splits as before and can land alone. | F4 |
| D5 | A new step kind, `hold`, prints the base and the run of the base. `Step` gets a `Base` field. The exit code is the one of a plan-time main refusal, and the refusal prints its next action. | F7, `cmd_integration_batch.go:83-86` |
| D6 | The run is found once. `GhCI` gets a run log, keyed by the tip. `GhCI.Watch` writes the run id that its one `runID` lookup finds. `Step` gets `Tip` and `Base` fields. The step line and the hold line read the run from the log. The `CI` and `MainCI` interfaces do not change. | F7, F8, F12 |
| D7 | `Composition` gets `OnMain`. `batcher.compose` takes its base from `rev-parse refs/evolve/main` after its fetch, not from `ls-remote`, so the base is a commit in the local repo. It checks each kept member with `Repo.CheckLanded` against that base and moves it to `OnMain`. `Merger.land` adds them to `Outcome.OnMain`, and `settle` closes them on the path of `Plan.Landed`. | F9 |
| D10 | The `hold` step reads the run of its base from the run log. A port error that is not a refusal stops the land with that error and prints a hold step with no run. A tip with no logged run prints `run none`. | round 3, M-a and L-a |
| D9 | A hold emits one signal event through the batch event origin of the train. The hold then shows on the signal surface when no lane runs. | `cmd_integration_stop.go:77` |
| D8 | `EnforceActor` keeps a nil `Base` port and maps `Outcome.OnMain` to merged members. The lane path does not change. D33 lets the console train keep its own path. | plan wave-integration D33 |

## 6. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| T1 | The `Base` port and the `hold` step in `Merger.land` (D1, D2, D5) | (a) A fake CI is red twice and the port refuses: no split step, one hold step, the refusal returned. (b) The port is not called on a green verdict or on a flake. (c) With a nil port, the split is unchanged. (d) A hold at the second level of a split keeps the landed first half in `Outcome.Landed`. | console |
| T2 | The wiring in `Train` to `checkMain`, and the override (D3, D4) | Through `Train.Land` with the stop gate on: (a) A red base holds. (b) A base that is pending and then red holds. (c) With `MainRedFix`, the batch splits. | console |
| T3 | The exit code, the hold line and the hold signal in `cmd/evolve` (D5, D9) | A journey through `integration batch` with a fake CI. The hold line names the base and the run of the base. One hold signal event. The exit code is the one of a main refusal. | console |
| T4 | The run log of `GhCI`, the `Tip` and `Base` fields of `Step`, and `stepLine` (D6) | the step line of a train wired as `env.train` wires it names the run that the verdict came from | console |
| T5 | `OnMain` in `Composition`, `Outcome`, `settle` and `EnforceActor` (D7, D8) | A fixture main takes one member between the plan and a later compose, for example the second half of a split. That compose leaves it out. The PR is closed as already landed. A second test moves main between the fetch and the head read: the compose uses the fetched base and does not fail. | console |

Each changed function has 100% coverage. Each test is red before its change.

## 7. Order

1. T4. It changes the `CI` contract, so each later test uses the new fakes.
2. T1, then T2 and T3. T2 wires the port of T1, and T3 prints its step.
3. T5. It is independent of T1 to T4.

All five land in one pull request, because they share the `integration` package.

## 8. Verification

| Goal | Before | After | How it is measured |
|---|---|---|---|
| G1 | a red base splits the batch down to single PRs (3 hours, F11) | the batch holds after one base check | the T2 and T3 tests; the next batch on a red base prints the hold line |
| G2 | step lines name no run | each step line names `run <id>` | the T4 test; the next batch log |
| G3 | a member that main took is cherry-picked again and ejected | the member is closed as already landed | the T5 test |

## 9. Risks

| Risk | Control |
|---|---|
| A pending base run makes a red batch wait. | The wait happens only after a confirmed red, and the split it replaces costs more CI runs. A wait that times out (`ErrRunTimeout`) stops the run with an error and does not split. |
| No lane runs, so no lane opens the stop for a red base. | The refusal prints its next action (D5), and the hold signal shows on the signal surface (D9). |
| A flaky test on a green base still splits the batch. | This is the behavior of today, not a new loss. The non-goals name it. |
| A red base holds good PRs. | The hold opens no stop and parks no lane. The next train run plans again and refuses on the red main (F1). `--main-red-fix` lands the fix alone (D4). |
| The new `CI` method breaks fakes in other packages. | T4 updates each fake in the same commit. `go vet` over every build tag set catches a missed one. |

## 10. Open questions

None.

## 11. Review record

Round 1 (architecture-reviewer and docs-review, both FAIL) judged design A, the host attribution. Its findings and their answers:

| Finding | Answer in this revision |
|---|---|
| C1: the optional run interface is lost through `stopCI` | D6 adds `Run` to `CI` itself (F8). |
| C2: a second opener of the main-red stop | The train opens no stop (non-goal, F5). |
| C3: the threshold blames main for a mixed red | Design E decides on the CI verdict of the base, not on a key match. |
| H1: in shadow, the stop hold does nothing | The hold is a `MainRefusal` of the train, so it does not depend on the stop stage. |
| H2: no route from compose to settle | D7 and D8 add `OnMain` through to `settle` and `EnforceActor`. |
| H3: the attributor reads a stale main ref | Design E uses no attributor. |
| H4: candidate E missing, host run differs from CI | E is scored and chosen. |
| M1: three artifacts, not two | Design E reads no artifact. |
| M2: ports are func fields | D1 is a func field with a nil default. |
| docs-review: the release path is asserted | §9 names it: the next plan refuses, and `--main-red-fix` lands the fix. |

Round 2 (architecture-reviewer WARN, docs-review PASS) found 0 CRITICAL, 1 HIGH, 3 MEDIUM and 2 LOW. Each prescribed fix is in this revision:

| Finding | Answer in this revision |
|---|---|
| H1: two lookups of the run; the hold has no run | D6: the run is found once and carried to the step and the hold lines. The amendment below changes the carrier to a run log. |
| M1: no release signal when no lane runs | D9 and §9. |
| M2: the landed check reads `refMain`, not the base | D7 checks against the base SHA; T5 moves main between the fetch and the head read. |
| M3: four test cases not named | T1 (b, c, d) and T2 (b). |
| L1: a base wait that times out | §9. |
| L2: wrong citation | F8. |

Round 3 (architecture-reviewer, a fresh seat, WARN) confirmed that D6 closes H1 and that D9 and the new test cases close M1 and M3. It found 1 new HIGH, 1 MEDIUM and 2 LOW. The convergence rule allows no fourth round, so this revision applies each prescribed fix, and the code review of the build checks them:

| Finding | Answer in this revision |
|---|---|
| H1: the T5 fixture fails on a git error, because the `ls-remote` base is not in the local repo | D7 takes the base from the fetched ref (option a). T5 tests a member that main takes after the plan, and a move between the fetch and the head read. |
| M-a: the run of the hold step; a port error that is not a refusal | D10. |
| L-a: `Result.Run` on error paths | D10. |
| L-b: the status line names round 1 only | The status line. |

**Amendment before the build.** A read of the implementers found F12: a new `Watch` signature changes two frozen ACS predicates. D6 now keeps the interfaces and adds a run log to `GhCI`. The run is still found once (round 2, H1), and the code review of the build checks the change.

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| T1 `Base` port and `hold` step | done | [#1566](https://github.com/mickeyyaya/evolve-loop/pull/1566), [`13df90a`](https://github.com/mickeyyaya/evolve-loop/commit/13df90ad7); `go/internal/integration/batchpr.go:49`, `:62`; `base_hold_test.go` | none |
| T2 wiring in `Train` to `checkMain` | done | [#1566](https://github.com/mickeyyaya/evolve-loop/pull/1566), [`13df90a`](https://github.com/mickeyyaya/evolve-loop/commit/13df90ad7); `go/internal/integration/train.go:294`; `train_base_hold_test.go` | none |
| T3 exit code, hold line and hold signal | done | [#1566](https://github.com/mickeyyaya/evolve-loop/pull/1566), [`13df90a`](https://github.com/mickeyyaya/evolve-loop/commit/13df90ad7); `go/cmd/evolve/cmd_integration_batch.go:146`; `cmd_integration_hold_test.go` | none |
| T4 run log of `GhCI`, `Tip` and `Base` fields | done | [#1566](https://github.com/mickeyyaya/evolve-loop/pull/1566), [`13df90a`](https://github.com/mickeyyaya/evolve-loop/commit/13df90ad7); `go/internal/integration/runlog.go` | none |
| T5 `OnMain` in `Composition`, `Outcome`, `settle`, `EnforceActor` | done | [#1566](https://github.com/mickeyyaya/evolve-loop/pull/1566), [`13df90a`](https://github.com/mickeyyaya/evolve-loop/commit/13df90ad7); `go/internal/integration/batchpr.go:39`, `enforce_actor.go:106`; `onmain_test.go` | none |

All five components landed in one pull request. The bookkeeping of item `wave-integration-i7-recompose-after-console-merge` came in with [#1605](https://github.com/mickeyyaya/evolve-loop/pull/1605), [`fd36f33`](https://github.com/mickeyyaya/evolve-loop/commit/fd36f33348a9c2098c4f280e667f5bff7594e254). The item `integration-train-compares-a-red-batch-with-main-before-it-splits` is still in the open inbox, although its work is on main. It needs a consume at the next boundary. Nothing is left to build. Verification rows G1 to G3 need a live red-base batch, so they are unverified.
