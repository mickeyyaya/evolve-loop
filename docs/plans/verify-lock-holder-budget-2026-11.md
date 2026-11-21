# Plan: the run that holds the host verification lock gets the host CPU budget

- **Status:** proposed, 2026-11-21.
- **Research:** none. The facts below come from the live wave 185 and the code. **Decision:** none. The rule is one formula in one package.
- **Related:** [internal-verifylock](../architecture/packages/internal-verifylock.md), [internal-lanebudget](../architecture/packages/internal-lanebudget.md), [signal-codes](../architecture/signal-codes.md).

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

The inbox item `verify-lock-holder-gets-the-host-budget`, from the operator on 2026-11-21:

> "The run that holds the host verification lock gets the host CPU budget, and lock waits feed the fleet window."

The operator braked the fleet until this lands. This plan covers two parts: the budget of the holder, and one signal for each acquire. The fleet window input and the seat-health label are follow-up items.

A worked example. The host has 14 CPUs and the fleet has 8 lanes. Lane A holds the lock and runs the coverage pass. It runs `go test -p 1 -parallel 1` with `GOMAXPROCS=1`. Seven other lanes wait at the lock, and each waits up to 15 minutes.

## 2. Facts

| Id | Fact | Source |
|---|---|---|
| F1 | One hub flock serializes the coverage pass of the build floor and the ACS suite. The wait bound is `MaxWait`, 15 minutes. After it, the run goes unserialized and writes `verify-single-flight-unavailable`. | `go/internal/verifylock/verifylock.go:26,36-44` |
| F2 | The two callers are `firstCoverPass` and `acquireSuiteLock`. | `go/internal/core/build_floor_reviewer.go:329`, `go/internal/acssuite/acssuite.go:159` |
| F3 | Each lane gets `max(1, hostCPUs/width)` for `-p`, `-parallel` and `GOMAXPROCS`. | `go/internal/lanebudget/lanebudget.go:48-64` |
| F4 | The coverage pass takes its argv and its environment through `lanebudget.Args` and `lanebudget.Env`. So the one holder runs at 14/8 = 1 CPU. Seen live on 2026-11-21: `go test -p 1 -parallel 1 -tags integration -coverprofile ./cmd/evolve`. | `go/internal/core/build_floor_reviewer.go:296,393` |
| F5 | The per-package unit runs of the floor take no lock and keep the lane budget. This is the decision of 2026-10-21. | `docs/architecture/packages/internal-verifylock.md` |
| F6 | Wave 185 had 8 lanes and 0 of 9 shipped. It had 75 `verify-single-flight-unavailable` events in cycles 2582 to 2590 (first count: 73). Wave 184 had 4 to 5 lanes: 6 events. | the console, 2026-11-21 |
| F7 | No event states how long an acquire waited. The wait is not measured. | `go/internal/dispatchevents/events.go:171` |
| F8 | A second lock, `integration-tier.lock`, serializes the retakes of the integration tier. The audit gate takes it in `retake` and the build floor takes it in its floor retake. The first try of each runs unlocked at the lane share, by design. In wave 185 the audit gate ran `-race -p 1 -parallel 1` and eight audit FAIL signals report the 10 minute test timeout of `go/cmd/evolve` (601 s). The retake lock timed out in cycles 2586 and 2589. | the console, 2026-11-21; `go/internal/ciparity/tierscope.go:11-13`, `go/internal/phases/audit/ciparitygate/tier.go:95-101`, `go/internal/core/build_floor_retake.go:118-124` |

*Implication of F3, F4 and F6:* the lock lets one run work while the others wait, and the one run uses 1/14 of the host. The lock holds the queue for the length of a 1 CPU run. The wait bound then expires, and the runs go unserialized anyway.

*Implication of F5:* only the locked run can use more than the lane share. The other runs of the other lanes still need their lane share.

## 3. Goals and non-goals

Goals:

1. The run that holds the lock gets a CPU budget of the host less the share of the other lanes. At width 8 on 14 CPUs it gets 7, not 1.
2. Each acquire emits one signal with the wait time.
3. The budget rule has one home: `lanebudget`.

Non-goals:

- The integration-tier retakes under `integration-tier.lock` (F8). See decision D8.
- No change to the wait bound or to the lock file.
- No change to the unlocked per-package runs.
- The lock wait as an input of the fleet window, and the seat-health label: two follow-up inbox items.

## 4. Candidate designs

Score each criterion from 1 (poor) to 3 (good).

| Criterion | A: host less the other lanes | B: K slots, host / K | C: the full host | D: live load reading |
|---|---|---|---|---|
| Holder speed | 3 | 2 | 3 | 2 |
| The other lanes keep their share | 3 | 2 | 1 | 2 |
| One home, no new constant | 3 | 1 | 3 | 1 |
| Size of the change | 3 | 1 | 3 | 1 |
| Testable without a real host | 3 | 2 | 3 | 1 |
| Total | 15 | 8 | 13 | 7 |

- **A.** The holder gets `hostCPUs - (width - 1) * laneShare`, never below the lane share. The other lanes run unlocked package runs at their lane share. So the holder gets what they leave. Results: width 8 gives 7, width 4 gives 5, width 1 gives 14.
- **B.** The lock has K slots. Each holder gets host / K. It needs a new lock design, a new constant K and a new slot protocol. It also lets K suites run at the same time. A suite on a loaded host gave false REDs (the batch halt in the verifylock note).
- **C.** The holder gets all CPUs. It is the simplest, and it oversubscribes the host when the other lanes run package units. Width 8 gives 14 + 7 = 21 CPUs of demand.
- **D.** The holder reads the load average and takes the free CPUs. It makes the argv depend on a reading that changes while the run starts. A test needs a fake load source, and the reading lags the real load.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | The holder budget is `HolderProcs(hostCPUs, width)`. A width below 1 gives 0. Else it is `max(laneShare, hostCPUs - (width - 1) * laneShare)`. `Holder()` reads the width like `Current()` does, through one shared `laneWidth()`. | F3, F4, F5, and the scores of A | B: new lock design and a new constant. C: demand of 21 CPUs at width 8. D: unstable input. |
| D2 | The holder share is an explicit value. The caller asks `lanebudget.ShareFor(held)` after the acquire. It gets `Holder()` when it holds the lock, else `Current()`. It passes the value to the existing seams: `ArgsFor`, `EnvFor` and `scopeShare`. | F2, F5, and review finding H1: a run that did not get the lock never gets the value | A context mark: ACS has no context to carry it, and `Current()` stays a second way to read the share. A global flag: it also lifts the unlocked runs. |
| D3 | A console run (no fleet width) keeps a share of 0. | F3: a console run has no width | A holder share for the console: the console has no budget to lift. |
| D4 | `verifylock.Observer.AcquireWithin` measures the wait and emits one `HOST_VERIFY_LOCK_WAIT` signal in module `host`. Fields: `source`, `wait_ms`, `outcome` (`acquired` or `unserialized`). `AcquireAt` and `Acquire` stay free of it. The free `AcquireWithin` becomes the unexported body behind the observer, so each bounded acquire emits the signal. | F7. The module `host` exists and its prefix fits the code. | A new module `verify`: a closed-set entry for one code. A `dispatchevents` event: it is an abnormal-events log, not the signal center. |
| D5 | The floor uses `ReviewInput.Signals`. The ACS suite takes `Options.Signals`, and the audit phase passes its own center. A center is built only at the two process roots, and a guard test pins that. A manual `evolve acs` run passes no center and emits no signal. | Review finding H2: a nil center emits nothing, and a unit test with an injected center passes. The test of the audit phase proves the wiring. | A center built inside `acssuite`: the root guard test refuses it. A new field from the composition root of `evolve acs`: no production lock holder runs there. |
| D6 | The existing `verify-single-flight-unavailable` event stays. | The wave statistics of F6 read it. | Replace it: it breaks the baseline of the acceptance check. |
| D7 | No shadow stage, and a measured wave with a revert threshold. See section 8. | The change can move a verdict through `-parallel` (review finding H3). | A shadow of the budget: a shadow compares verdicts, and the budget has no second verdict to compare. |
| D8 | Three paths take a lock. Two take the verify lock: the floor coverage pass and the ACS suite. This change gives them the holder share. The third is the retake of the integration tier under `integration-tier.lock`, in the audit gate and in the build floor. It keeps the lane share, and a follow-up item owns it. | F8: the tier lock is a second lock with its own holder. A holder of each lock at the host budget asks for more than the host. The first try, which has the 601 s timeouts, runs without a lock, so the holder rule does not reach it. | The same rule for the tier lock now: two holders at 7 CPUs plus the other lanes ask for 20 CPUs on 14. One budget over both locks needs its own design. |

*Consequence of D1:* the holder budget is never below the lane share. It is at most the host. A lane at width 8 gets 7 CPUs for the coverage pass, 7 times the old value. The share sets `-p`, `-parallel` and `GOMAXPROCS` to the same number. So the product of the flags is a cap per flag, not a CPU count. The same model holds today at width 2, where a lane gets 7.

*Condition that changes the decision:* a measured wave shows a load above the host size during a locked run. Then add a reserve term for the LLM sessions. D1 has no reserve, because no measurement of it exists.

## 6. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| C1 | `lanebudget.HolderProcs`, `Holder`, `ShareFor`, and the shared `laneWidth` | `TestHolderProcs_*`: width 8 on 14 CPUs gives 7, width 4 gives 5, width 1 gives 14, a width below 1 gives 0. `TestShareFor_*`: held gives the holder share, not held gives the lane share | `go/internal/lanebudget` |
| C2 | `verifylock.Observer.AcquireWithin`: wait time and one signal | `TestObserver_*`: a held lock gives a wait above 0 and one signal. An unserialized run gives `outcome=unserialized` | `go/internal/verifylock` |
| C3 | The floor passes the share to `coverPass` and `coverTestArgs`. The retake outside the lock keeps the lane share. | The real runner: width 8 and the lock held, the argv of the holder has `-p 7` and `GOMAXPROCS=7`. The retake has `-p 1`. A run that lost the lock has `-p 1`. | `go/internal/core` |
| C4 | The ACS suite passes the share to `discoverGoLane` and `scopeShare`, and takes its center from `Options` | The same test through `Run`. A test through `Run` counts one signal on the given center. A test through the audit phase counts one signal on the audit center. | `go/internal/acssuite` |
| C5 | The code `HOST_VERIFY_LOCK_WAIT` and the generated signal codes file | `evolve signals codes check` | `go/internal/signalcenter`, `docs/architecture/signal-codes.md` |
| C6 | Three follow-up inbox drafts, and one to retire the older event | none: data | the console |

## 7. Order

1. C1 first. It is the one home of the rule. The other components only call it.
2. C5 and C2 next. The signal needs the code before the observer emits it.
3. C3 and C4 last. They use C1 and C2 and they carry the acceptance tests.

## 8. Verification

| Goal | How it is measured |
|---|---|
| G1 holder budget | The tests of C3 and C4 run the real floor runner and read the argv and the environment. Width 8, lock held: `-p 7 -parallel 7`, `GOMAXPROCS=7`. |
| G2 one signal for each acquire | The tests of C2 count the signals of one acquire: 1. |
| G3 one home | The tests of `HolderProcs` pin the formula. Callers receive the value from `ShareFor` and never compute it. |
| G4 effect on a wave | One measured wave at width 8: fewer than 73 `verify-single-flight-unavailable` events in 9 cycles. The sum of `wait_ms` is the new baseline. |
| G5 verdict risk | The same wave: the count of floor retakes that turn green, and the red count of the ACS suite, against wave 185. A rise of either is the revert line. |

**Rollout.** This change has no shadow stage. A shadow stage compares a new verdict with the old one before the new one acts.

The budget has no second verdict to compare. It gives a CPU count to a `go test` run. But the count can move a verdict: `-parallel 7` runs tests at the same time that ran one by one before. A test with shared state can go red. The floor retake runs at the lane share, so it can hide such a red. The ACS suite has no retake.

The control is G5, plus one run of the floor and the ACS suite at the holder share on the host before the fleet starts. A revert undoes the change.

## 9. Risks

| Risk | Control |
|---|---|
| The holder and the unlocked runs together exceed the host. | D1 subtracts the share of the other lanes. The LLM sessions are the open part (section 10). |
| A run that failed to get the lock reads the holder share. | D2 gives the value only to a caller that holds the lock. C3 tests the lost lock. |
| The ACS acquires emit nothing in production. | D5 builds the center in `acssuite`. C4 counts the signal through `Run`. |
| More parallel tests turn a verdict red. | D7 and G5. |
| The signal write fails and breaks the lock. | `Center.Emit` has no return value and accepts a nil center. The lock never depends on it. |

## 10. Open questions

1. Q1. Do the LLM sessions need a reserve? Measure the load during a locked run of wave 186 first.
