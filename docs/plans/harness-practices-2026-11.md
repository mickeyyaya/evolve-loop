# Plan: the loop proves each fix with a journey test, and a refusal or a nameless red no longer costs hours

- **Status:** proposed, 2026-11-04.
- **Design:** [regression-proof-and-supervision](../architecture/regression-proof-and-supervision.md). **Decision:** [ADR-0159](../architecture/adr/0159-the-loop-keeps-its-integration-train-and-takes-six-practices-from-the-2026-harness-projects.md). **Research:** [harness-practices-2026-11](../research/harness-practices-2026-11.md).

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

The operator, 2026-11-04:

- "make sure for ANY issue we fixed, we must always deploy the relative tests to cover the CUJ to make sure there should never be the same regression happened."
- "do a research online to see if there is a better solution than the one we already built. Also search for the open source projects to see what highest stars projects doing and absort their concept and design to see what we can apply here."
- "again, make sure we have a full research docs, plan, ADR, ...etc written strictly follow the policy under docs/".

## 2. Facts

| Fact | Research id |
|---|---|
| Wave 152 shipped 1 of 9. Wave 153 shipped 0 of 1, with 67.8 minutes of waste. | F3.1 |
| One nameless red ended 2.5 hours of work. Six reruns of the same tree passed. | F3.2 |
| The floor keeps the last 400 bytes and does not retake. The audit tier retakes. | F3.3 |
| The role guard reads a nonzero `cycle_id` as an active cycle, also after the seal. | F3.4 |
| Two refused agents walked the families for 35 minutes in one boundary. | F3.5 |
| Ten CRITICAL findings sat on 5 green, fully covered trees. | F3.6 |
| A default patch-id changes when a peer edits a line 2 lines away. A zero-context patch-id does not. | F3.9 |
| Gas Town uses the same batch-then-bisect design as ADR-0147. | F5.1 |
| Uber's open source queue has no release and needs MySQL. | F5.2 |
| SWE-bench resolves a task only when `FAIL_TO_PASS` and `PASS_TO_PASS` both pass. | F8.1 |
| Agents sometimes edit failing tests even when told not to. | F8.3 |
| One benchmark shows that RTK raises the cost of each solved task by 19%. | F10.1 |

## 3. Goals and non-goals

**Goals**

| Goal | Measure |
|---|---|
| G1. Each fix has a proven journey test | Share of `fix:` commits with a red run and a green run on record |
| G2. A registered journey cannot silently weaken | Each change to a registered test appears in a signal and needs a console commit |
| G3. A refusal costs one stall limit | Time from the denial to the end of the pass: from 35 minutes to at most 6 minutes |
| G4. A floor red names its test | Share of floor reds with a test name or a stated cause: 100% |
| G5. A one-off floor red does not end a cycle | Cycles that end on a red that a serialized retake passes: 0 |
| G6. A composed cycle reads as landed | The peer-edit test of F3.9 passes |
| G7. A move to `enforce` shows its evidence | I9 cites numbers from recent waves |

**Non-goals**

- This plan does not replace ADR-0147. Its components continue under [its own plan](wave-integration-2026-10.md), with the amendment of 2026-11-04 there.
- This plan does not add a tool-output proxy, an outside workflow engine or a desktop interface.
- This plan does not change a persona before the M2 baseline of ADR-0133.

## 4. Candidate designs

| Design | Summary |
|---|---|
| A. Keep and graft | Keep the train. Add a mechanical proof gate and five small fixes in our own code. |
| B. Adopt outside engines | Replace the train with `uber/submitqueue`, and replace checkpoints with Temporal or DBOS. |
| C. Simplify to a serial queue | Drop the batch. Land one deliverable at a time behind a local lock. |
| D. Rules only | Strengthen the briefs and the review prompts. Add no gate. |

Each criterion scores 1 (poor) to 5 (good). "Closes the proof gap" counts twice, because it is the rule of the operator.

| Criterion | A | B | C | D |
|---|---:|---:|---:|---:|
| Closes the proof gap (×2) | 5 (10) | 1 (2) | 1 (2) | 2 (4) |
| Closes the failures of F3.2 to F3.5 | 5 | 2 | 2 | 1 |
| Keeps one Go binary on one host | 5 | 1 | 5 | 5 |
| Keeps the work already built | 5 | 1 | 2 | 5 |
| Throughput at our floor length | 4 | 5 | 1 | 4 |
| Change size | 3 | 1 | 3 | 5 |
| **Total (of 35)** | **32** | 12 | 15 | 24 |

Basis of the scores:

- **Failures:** A fixes each one. B and C fix only the merge part, and D changes only text.
- **Built work:** B replaces most parts, C drops the batch parts, and A and D add to them.
- **Throughput:** B speculates in parallel. A batches and D keeps today's process, each at about the same rate. C holds a lock for each floor.
- **Change size:** B adds services. A and C need new code. D changes only text.

**A wins.** B has the highest throughput on paper, but F5.2 shows no release, and F7.1 shows that resume already works. C is the simplest, and F5.6 shows that its lock caps throughput under a long floor. D is the cheapest, and F3.6 and F3.7 happened with the rule already in place.

What A takes from the others: from B, the content-keyed idea (the zero-context identity) and the later partition stage (F5.3). From C, the PID-liveness rule. From D, the rule stays in each brief as the first line of defense.

## 5. Decisions

These ids belong to this plan. "integration plan D10" and "integration D22" name the decisions of the [integration plan](wave-integration-2026-10.md).

| Id | Decision | Research id |
|---|---|---|
| D1 | Keep the integration train of ADR-0147. | F5.1, F5.2, F5.4 |
| D2 | A `fix:` commit carries a `Fix-Journey` trailer: a registered journey, `test:<name>` from a lane, or `none` with a reason. | F8.1, F8.5 |
| D3 | A gate runs the named test without the fix and with the fix. | F8.1, F8.2 |
| D4 | The registry and the tests that it names are on the protected surface. | F8.3, F8.5 |
| D5 | A cycle is active when its phase is live, or when its owner lease is live or unreadable. | F3.4, F5.6, F7.4 |
| D6 | A stall after a guard denial is terminal for the chain and the pass. | F3.5, F6.2, F6.5 |
| D7 | The floor shares the audit scope, retakes once serialized, and keeps each failed test name. | F3.2, F3.3, F9.1, F9.3 |
| D8 | The landed identity uses a zero-context patch-id. | F3.9, F5.3 |
| D9 | Each agent seat has one named health state in `evolve wave status`. | F6.4 |
| D10 | A move to `enforce` cites recent measured evidence. The operator decides. | F6.6 |
| D11 | A removal test runs after the M2 baseline, one component at a time. | F13.1 |
| D12 | A token change is judged by the cost of each shipped cycle, and it keeps the cached prefix. | F10.1 |

## 6. Components

The route follows the rule of ADR-0147 §6: an item that touches a gate file is `console-manual`. Each item has an inbox entry with the same id.

| Id | Component | Red test first, as a journey | Owner |
|---|---|---|---|
| R1 | `role-guard-derives-the-active-cycle` | A cycle aborts, the boundary self-heal agent writes its own output, and the wave starts. A live owner still gets the refusal. | console-manual |
| R2 | `guard-denial-ends-the-dispatch-chain` | An agent is denied and stalls. The chain stops after one stall limit, the second diagnosis does not start, and the signal says refused. | console-manual |
| R3 | `build-floor-keeps-audit-rules` | A floor run fails one test once. The retake passes, the cycle goes on, and a signal names the test. A second red names the test and fails. | console-manual |
| R4 | `landed-identity-zero-context` (also a precondition of I12) | A peer edits a line 2 lines from a cycle change. The composed cycle reads as landed. | console-manual |
| R5 | `journey-registry-and-trailer` | A row that names a missing test fails the guard. A `fix:` commit with no trailer gets a shadow signal. | console-manual |
| R6 | `regression-proof-gate` | A fix with a test that passes without the fix gets `NOT_PROVEN`. A real proof gets `PROVEN`. A compile failure gets `INCONCLUSIVE`. | console-manual |
| R7 | `agent-seat-health-states` | A pane that waits at its prompt with no deliverable reads `blocked`. A dead session reads `zombie`. | lane |
| R8 | `enforce-evidence-for-integration` | I8 prints the evidence of the last two shadow waves, and I9 refuses to close with fewer. | console-manual |
| R9 | `harness-removal-test` | Design only until M2. It names one component, one wave window and one measure. | operator |

**The first journeys to register.** Each one is an issue that this period fixed.

| Journey | Issue | Proof today |
|---|---|---|
| A cloud launch reaches the real CLI | `--` separator (#1286) | 4 hand runs; no test against recorded real output |
| A paused cycle resumes through a lane | Sequential resume (#1280) | `cmd_cycle_resume_concurrent_test.go` |
| A failed cycle leaves the boundary able to start a wave | F3.4 | None; R1 adds it |
| One family runs one lane, in any launch order | Flake of F3.8 | `TestPool_TwoTodosOfOneFamilyRunOneAfterTheOther` |
| A fenced verdict reads as its real verdict | #1250 | `agent_verdict_test.go` |
| A batch test tests real packages | I3b finding (F3.6) | Fixtures with the production shape; no real adapter |

## 7. Order

1. **R1, then R2.** Today they hold up the boundary of each wave after a failed serial cycle.
2. **R3.** It stops the largest measured waste (F3.1, F3.2).
3. **R4.** I2 and later components depend on the identity, so fix it first. I12 and I13 run beside it ([amended order](wave-integration-2026-10.md#10-amendment-of-2026-11-04-console-prs-enter-the-train)).
4. **R5, then R6.** The gate reads the registry and the trailer.
5. **R7.** It is a read-only view, and a lane can build it at any time.
6. **R8.** It waits for I8, which waits for I10.
7. **R9.** It waits for the M2 baseline.

## 8. Verification

| Goal | Before (2026-11-04) | After | How |
|---|---|---|---|
| G1 | 0 fixes with a gate proof | At shadow, a report on each `fix:` commit; at enforce, 100% | `FIX_PROOF_*` signals |
| G2 | No registry | Each row resolves | The registry guard test |
| G3 | 35 min | At most 6 min | Signal timestamps |
| G4 | The name is lost (F3.3) | 100% | Floor reports in `audit-fail-reason.json` |
| G5 | 1 cycle in wave 153 | 0 | `FLOOR_RETAKE_*` signals |
| G6 | Fails (F3.9) | Passes | R4 test |
| G7 | No stated bar | Two clean shadow waves, with numbers in I9 | I8 output |

Each component has 100% coverage on each changed function, a test that is red without its fix, and the loop-equivalent review. The docs of this set pass `evolve docs ste-lint --strict` and the docs-review skill.

**What a test cannot prove.** The gate proves that a test depends on the fix. It does not prove that the test covers the whole journey. A review seat still judges that.

## 9. Risks

| Risk | Control |
|---|---|
| A new gate blocks good fixes | Stage `shadow` first. `INCONCLUSIVE` does not block. |
| Authors write `Fix-Journey: none` to skip the gate | A review seat reads each reason. A signal counts them. |
| A retake hides a real intermittent fault | One retake only. Each one emits a signal with the test name, and `evolve ci classify` reads the recurrence. |
| The guard allows a write while an orphan agent runs | Only a terminal phase plus a dead lease proves it. Any doubt keeps the refusal. |
| The red run loads the host during a wave | One package only, under the build pool cap. |
| The zero-context patch-id matches an unrelated commit with the same text | The trailer decides first. The patch-id only confirms. |

## 10. Open questions

| Id | Question | Our proposal |
|---|---|---|
| Q1 | What evidence does a move to `enforce` need? | **Decided by the operator, 2026-11-04:** two consecutive shadow waves with no disagreement between the composed tree and the landed tree. |
| Q2 | Which commits does the gate cover at enforce? | Each `fix:` commit from the console and from a lane. A lane names its test, and the console adds the row at the boundary. |
| Q3 | Do we add `testscript` for journey tests? | No for now. The `e2e_*_test.go` files work. Review after 10 journeys. |
| Q4 | Does a mix of families in one wave raise conflicts? | Unknown. F5.7 does not show this. Record the family pair on each ejection. |
| Q5 | Do we add branch protection on `main`? | **Decided by the operator, 2026-11-04:** yes, once the train is live. See [the steps](wave-integration-2026-10.md#branch-protection-on-main). |
