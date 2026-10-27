# Plan: a cycle stops on a blocker that the lane cannot fix, and a small prescribed fix costs one patch, not one build

- **Status:** proposed, 2026-10-27. Items 2 and 3 have two builds. §11 compares them and gives the parts that move onto the build on `main`.
- **Decision:** [ADR-0158](../architecture/adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md). **Research:** [cost-aware-repair-2026-10](../research/cost-aware-repair-2026-10.md). **Case:** [cycle 2035](../incidents/2026-10-27-cycle-2035-repair-loop.md). **Design of record for the rungs:** [retry-ladder.md](../architecture/retry-ladder.md).

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
- [11. Status of the build](#11-status-of-the-build)

## 1. Request

The operator, on 2026-10-27:

> "Do the deep dive and write this 2035 example in the docs, research the best strategy and policy we could apply to improve this problem."

> "I want to take cycle 2035 as an example that we waste all our token for that small changes higher level model can fix immediately, I want to make sure audit will involving fix and trigger another review for it's changes. I need to balance to cost and output otherwise, this project is no use."

Decisions that the operator took before this plan:

- `retry.contract_correction_retries` is 1 ([#1169](https://github.com/mickeyyaya/evolve-loop/pull/1169)).
- The retry ladder L5 and L6 get an `audit_fix` rung in the decider ([#1173](https://github.com/mickeyyaya/evolve-loop/pull/1173)).
- The auditor fixes small prescribed findings in its loaded session. A different fresh seat reviews only that patch, the gates run again, and one escalation then a stop follow.
- A fix of a protected file is a held proposal ([ADR-0157](../architecture/adr/0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md)).

## 2. Facts

| Fact | Research id |
|---|---|
| Cycle 2035 used 160.1 LLM minutes and 87.4 M cache-read tokens, and it did not ship. | F1.1 |
| 65% of those minutes came after the first audit, which already named the protected blocker. | F1.2 |
| The blocker M1 is on the fence, and two deterministic gates run it. No lane diff can pass. | F1.3 |
| The repair decision reads only the failure class. It has no lane-fixability input. | F1.4 |
| A repair round restarts tdd and build, also for a fix of one test helper. | F1.5 |
| Build is 70% of the minutes and 77% of the cache-read tokens. | F1.6 |
| Two floor refusals cost two new dispatches with the full prompt. | F1.7 |
| Deterministic gates found the blockers. The audits found none of F1, F2 or the empty tests. | F1.8, F1.9 |
| An empty test satisfied a guard. | F1.10 |
| The lane deadline is the 95th percentile of lane times. It grows with each long failure. | F1.11 |
| Triage admitted the same item ten times for 628 LLM minutes. | F1.12 |
| The cycle spent about 41 LLM minutes after the console took its item. | F1.13 |
| The wave cost 101.2 LLM minutes for each shipped cycle. | F1.14 |
| The `audit_fix` decision exists at `shadow`, with no handler. | F1.16 |
| Practice: classify an error before a retry, stop on a failed precondition, and budget the retries. | P2.6 to P2.9 |
| Practice: self-repair gains decay fast, and the quality of the feedback decides the gain. | P2.11 to P2.14 |
| Practice: the reviewer applies a small exact patch, and a second person reviews it. | P2.1 to P2.4 |

## 3. Goals and non-goals

| Goal | Measure | Target |
|---|---|---|
| G1. Less cost for each shipped cycle | LLM minutes and tokens for each shipped cycle, over one wave | from 101.2 to 75 LLM minutes or less |
| G2. No repair round that cannot converge | repair rounds that start with a hand-off finding that blocks | 0 |
| G3. A small prescribed fix costs a patch, not a build | LLM minutes of an `audit_fix` round | 15 or less, against about 50 for a tdd and build round |
| G4. No loss of quality | escaped HIGH defects found after a ship; the ship rate | no rise; no fall |
| G5. No new way to game a guard | empty tests in changed test files | 0 |

The order of the goals is the order of the operator: cost first, then output. G4 and G5 are limits that the cost cuts must not break.

Non-goals:

- This plan does not change the fence. ADR-0157 owns the fence and the held proposals.
- This plan does not add a new model tier. The retry ladder owns the climb (ADR-0142).
- This plan does not remove the audit. The audit changes its focus.

## 4. Candidate designs

Each design is scored 1 to 5 on each criterion, and 5 is the best. "Build cost" is the cost to build the design, so 5 means cheap. The 2035 column is an estimate of the LLM minutes of cycle 2035 under the design, from the sum of `llm-calls.ndjson` over time.

| Id | Design | 2035 (LLM min, estimate) | Cost per shipped cycle (×2) | Quality (×2) | Safety | Build cost | Weighted sum |
|---|---|---|---|---|---|---|---|
| C0 | Today, with #1169 and #1173 at `shadow` | 160.1 | 1 | 2 | 4 | 5 | 15 |
| C1 | The `audit_fix` rung only | 160.1 | 2 | 3 | 3 | 3 | 16 |
| C2 | The lane-fixability check only, at triage and before each round | 12.3 at triage; 55.8 after round 1 | 4 | 3 | 5 | 4 | 23 |
| C3 | A hard spend limit for each cycle only (90 LLM min) | 90.1 | 3 | 2 | 5 | 4 | 19 |
| C4 | A higher model for each repair round (the cascade alone) | 160 or more | 1 | 3 | 4 | 4 | 16 |
| C5 | C2 + C1 + C3, with the builder self-check, the vacuous-test guard, the final-diff audit and the supersede signal, all at `shadow` first | 12.3 | 5 | 4 | 4 | 2 | 24 |

How each score comes from the facts:

- **C0.** The cost stays at F1.14. The quality score is low, because the audits missed F1, F2 and the empty tests (F1.9).
- **C1.** In cycle 2035, `ChooseAuditFix` declines in round 1 and in round 3 with `unprescribed` (F1.17). Thus C1 alone saves nothing in this case. In a case with one small lane finding, it replaces about 50 LLM minutes of tdd and build with a patch and a review (estimate). The safety score is 3, because the auditor writes code. The second seat (P2.2, P2.4) is the control.
- **C2.** The companion console item existed at triage (F1.12). The estimate is that a triage check stops the cycle after 12.3 LLM minutes. If the triage check misses, the check before round 1 stops it at 55.8, because the audit proved M1 red by an overlay (D3). The item goes to the console, where it took 44 minutes. The quality score is 3, because C2 does not make the audit find F1 or F2.
- **C3.** If the limit had applied, it stops the cycle at 20:29Z, after audit round 2 (F1.15). The estimate is a cut of about 70 LLM minutes, but the limit also stops a good cycle that runs long. Thus its quality score is low.
- **C4.** A stronger model does not reach a protected file (F1.3). It costs more for each round. The research (P2.10) supports a climb only on a measured capability failure, which the ladder already does.
- **C5.** C2 stops cycle 2035 at triage. C1 handles the cases that C2 lets through. C3 is the backstop for each case that the other checks miss. The guard, the audit focus and the self-check raise the quality. The build cost is the highest, so the order in §7 puts the cheapest high-value parts first.

**Why C5 wins.** C2 gives the largest single cut and is cheap, and C5 contains C2. C5 adds the parts that the operator asked for: the audit fix and a second review. It also adds the parts that close the quality gaps of F1.9 and F1.10. C5 wins by one point over C2, so the order builds C2 first. If the shadow wave shows that C1 does not fire, the plan stops after C2 and C3 (see §10).

**Worked example.** With C5 at `enforce`, the triage of cycle 2035 sees the open item `retro-minting-files-through-inbox-add-lane-ac-5-console`. Its `files` are in the fence, and its id starts with the id of the lane item. Triage defers the lane item until the console item is consumed, after 12.3 LLM minutes. The estimate of the wave cost for each shipped cycle is then (506.1 − 160.1 + 12.3) / 5 = 71.7 LLM minutes, against 101.2. It assumes the same five ships.

## 5. Decisions

| Id | Decision | Research id |
|---|---|---|
| D1 | Sort each finding into a lane finding or a hand-off finding, before triage admits an item and before each repair round. | R1, R3 |
| D2 | A hand-off finding cites a fence path (`guards.IsProtectedSurface`) or names an open console item. | F1.3, F1.17 |
| D3 | A hand-off finding that blocks stops the cycle at once, with salvage. A finding blocks when a deterministic gate fails on it, the audit proves a red test on it, or it is HIGH or above. | R2, P2.8 |
| D4 | `ChooseAuditFix` judges only the lane findings. | F1.17 |
| D5 | The auditor applies the patch in its loaded session, inside the files that the prescriptions cite. | R4, P2.1 |
| D6 | A fresh seat of another family reviews only the patch, and the gates run again. | R4, P2.2, P2.4 |
| D7 | One escalation follows a failed patch review: a raised build with the L6 digest. Then stop. | R5, P2.14 |
| D8 | A spend limit for each cycle: LLM minutes, fresh tokens, cache-read tokens and audit rounds. | R6, P2.6, P2.13 |
| D9 | The lane deadline stays as a hang guard, from the times of shipped lanes only. | F1.11 |
| D10 | The build session runs the handoff floor checks before it closes. | R7, F1.7 |
| D11 | A guard refuses an empty test body in a changed test file. | R8, P2.19 |
| D12 | The audit takes the gate results as facts, reviews the final diff and asks if each fix is honest. | R9, P2.21 |
| D13 | A live cycle stops at its next phase boundary when the console claims its item. | R10, F1.13 |
| D14 | `evolve wave status` prints the cost for each shipped cycle, the rounds and the convergence rate. | R11, P2.16 |
| D15 | Each component starts at `shadow`. It moves to `enforce` after one wave on measured evidence and an operator flip. | ADR-0155 decisions 17, 18 |

## 6. Components

The inbox items carry the full acceptance. The rank is the build order of §7.

| Rank | Item id | Component | Red test first | Route |
|---|---|---|---|---|
| 1 | `cost-per-shipped-cycle-measure` | D14: cost and rounds in the wave measures | a wave with 2 shipped and 1 failed cycle prints the sum of the LLM minutes divided by 2 | lane |
| 2 | `repair-fixability-handoff` | D1 to D4 before each repair round | a failure block with a fence-path finding and a gate FAIL returns a stop, not a repair round | lane |
| 3 | `triage-defers-item-with-pending-console-companion` | D1, D2 at triage and admission | a lane item with an open console companion is deferred, with the reason | lane |
| 4 | `cycle-spend-limit-stop-and-handoff` | D8 | a cycle above `max_llm_minutes` at a phase boundary records `CYCLE_SPEND_LIMIT`; at `enforce` it stops with salvage | lane |
| 5 | `vacuous-test-guard` | D11 | `func TestX(t *testing.T) {}` in a changed test file fails the guard | console-manual |
| 6 | `audit-fix-rung-handler` | D5 to D7, through `core.RungHandler` | a chosen `audit_fix` applies a cited patch, dispatches a fresh review seat and runs the gates | lane |
| 7 | `audit-reviews-the-final-diff` | D12 | the audit prompt carries the gate results as facts and the honesty question | console-manual |
| 8 | `builder-selfcheck-before-close` | D10 | a build that closes with no GREEN self-check for its tree gets the findings in the same session | console-manual |
| 9 | `cycle-stops-when-console-takes-its-item` | D13 | a console claim of the item of a live cycle stops the cycle at the next boundary with `superseded` | lane |
| 10 | `lane-deadline-from-shipped-lanes` | D9 | the deadline from 3 shipped and 2 long failed lanes ignores the failed lanes at `shipped` | lane |
| 11 | `wire-cost-aware-repair-call-sites` | D3, D5, D8, D13 | with the fixability stage at `enforce`, cycle 2035 stops after audit round 1 as `handed-off` | console-manual |

Items 5, 7, 8 and 11 are console-manual, because each one changes a fence path. The paths are `go/acs/regression/` (`integrity_surface.go:37`), the audit persona and rubric (`:44`, `:159`) and the build floor (`:88`). Item 11 changes the phase loop and the cycle seal in `internal/core` (`:87`, `:99`, `:100`) and the audit phase (`:140`).

Items 2, 4, 6 and 9 build pure packages, which are lane work. Their call sites are on the fence, so item 11 wires each package when it lands.

## 7. Order

1. `cost-per-shipped-cycle-measure`. The other parts need a before and after number.
2. `repair-fixability-handoff`. It is the largest cost cut for its size (C2).
3. `triage-defers-item-with-pending-console-companion`. It stops the repeat of F1.12.
4. `cycle-spend-limit-stop-and-handoff`. It is the backstop for each miss of 2 and 3.
5. `vacuous-test-guard`. It is cheap and closes a way to game a guard now.
6. `audit-fix-rung-handler`. #1173 is on main. It needs item 2 to remove hand-off findings first.
7. `audit-reviews-the-final-diff`. It goes with item 6, because the patch review uses the same focus.
8. `builder-selfcheck-before-close`. It needs a way to send findings into a live session (§10, Q2).
9. `cycle-stops-when-console-takes-its-item`.
10. `lane-deadline-from-shipped-lanes`. Item 4 is the cost stop, so this item only makes the hang guard tighter.
11. `wire-cost-aware-repair-call-sites`. The console wires each pure package of items 2, 4, 6 and 9 as soon as that package lands. Without the wiring, no package changes a cycle.

After items 1 to 4 and their wiring (item 11) run one wave at `shadow`, the operator flips them to `enforce`. Items 6 to 10 follow the same rule. Item 5 enforces at once, with its baseline.

## 8. Verification

| Goal | Before (measured) | After (how to measure) |
|---|---|---|
| G1 | 101.2 LLM minutes for each shipped cycle, waves 141 and 142 | `evolve wave status` measures (item 1), one wave at `enforce` |
| G2 | 2 repair rounds with a hand-off finding that blocks in cycle 2035 | count of cycles sealed `handed-off` with the reason `handoff_blocker`, against the repair rounds that start with such a finding |
| G3 | about 50 LLM minutes for the tdd and build of a repair round (cycle 2035, round 2: 51.6) | LLM minutes of each `audit_fix` round in `llm-calls.ndjson` |
| G4 | 2 HIGH regressions escaped the audits of cycle 2035 | HIGH findings of the console reviews after a ship; the ship rate of the wave |
| G5 | 3 empty tests in cycle 2035 | the guard of item 5 on each changed test file |

At `shadow`, each component records what it does at `enforce`. The shadow wave proves two numbers before a flip:

- The cut: the LLM minutes after each `would` stop.
- The false stops: the `would` stops on a cycle that then shipped. The target is 0.

## 9. Risks

| Risk | Control |
|---|---|
| The fixability check stops a cycle that the lane can win. | It stops only on a hand-off finding that blocks. The shadow wave counts false stops before the flip. |
| The auditor writes a large or wrong patch. | The `audit_fix` bounds of #1173 (3 files, 40 lines, 10 code lines, no new production logic). A fresh seat of another family reviews the patch, and the gates run again. |
| The auditor judges its own patch. | The review seat is a new session of another family. It reads only the patch and the prescriptions. |
| The spend limit stops a good long cycle. | The limit is 1.5 to 2 times the largest shipped value (F1.15). The stop writes salvage, so the console can land the work. |
| The empty-test guard refuses a valid placeholder. | A baseline of today's offenders. A test with `t.Skip` and a reason passes. |
| A protected fix escapes the fence through `audit_fix`. | A fence path is a hand-off finding (D2). Under ADR-0157 enforce, a patch on a fence path is a held proposal. |
| More stops mean more console work. | The stop moves work to the place where it converges. The console needed 44 wall minutes (its tokens are not recorded), where the lane spent 104 LLM minutes and did not ship (F1.2). |

## 10. Open questions

- **Q1.** Does `audit_fix` fire often enough to pay for its build? The shadow wave of #1173 records `LADDER_AUDIT_FIX`. If it records `would` on fewer than 1 cycle in 20, build items 1 to 4 and their wiring (item 11), and stop.
- **Q2.** Can the bridge send findings into a live build session before it closes? If not, item 8 becomes a hard rule in the build contract: no handoff without a GREEN self-check receipt for the tree.
- **Q3.** Which family reviews an `audit_fix` patch when the chain has only one family that is healthy? The proposal is to decline the rung and take the repair round of today.
- **Q4.** The spend limit counts fresh tokens and cache-read tokens apart. A cost in money needs a price for each family, which the policy does not have. The plan [token-telemetry-2026-10](token-telemetry-2026-10.md) proposes these weights as policy data (D8).

## 11. Status of the build

Items 2 and 3 have two builds:

- Cycle 2039 landed item 2 on `main` as commit `13ec27e4c`. It added the package `go/internal/fixability`, the key `workflow.repair.fixability.stage` and the ACS `go/acs/cycle2039`, which pins the API.
- The console built items 2 and 3 and their part of item 11 in [#1188](https://github.com/mickeyyaya/evolve-loop/pull/1188), with a different `go/internal/fixability`. [#1189](https://github.com/mickeyyaya/evolve-loop/pull/1189) (item 4) is on top of #1188.

The console keeps the build on `main`, because it shipped and the ACS pins it. The branch `feat/adr0158-fixability-gaps` moves each behaviour of #1188 that `main` does not have onto the API of `main`. It adds no second package and does not change the ACS of cycle 2039. The branch `feat/cycle-spend-limit-v2` moves #1189 onto that branch.

| Behaviour | Acceptance | `main` (cycle 2039) | #1188 | Result |
|---|---|---|---|---|
| Lane and handoff findings, the severity rule, the red paths, the salvage pointer, the stage key | item 2, criteria 1 to 5 | yes | yes | `main` stays |
| A gate failure that names a cited path, the package of a cited Go file, or a test of the finding blocks | D3, "a deterministic gate fails on it" | no | yes | moved: `Input.GateFailures` |
| The words "tests fail" or "predicate failed" in the defect block | D3, "the audit proves a red test"; round 1 has no gate failure | no; only a red path from the caller | yes | moved |
| A console item id must be whole | D2 | no; a substring matches | yes | moved |
| The fence is an input function | not in the acceptance | no; `guards.IsProtectedSurface` | yes | `main` stays; `core` gets the check through `core.WithRepairFixability`, because `guards` imports `core` |
| The name of the rule that blocks, and JSON tags on the findings | not in the acceptance | no | yes | not moved |
| `Companions` and `OpenConsole` | item 3, criteria 1 to 3 | no | yes | moved into `go/internal/fixability` |
| Triage, the census, the wave seed and `evolve inbox batches` defer a lane item with a console companion | item 3, criterion 4 | no | yes, in `inboxmover.RankLaneMenu` | moved into `inboxmover.PartitionLaneMenu`, so the no-work check and `evolve inbox rank` agree with triage |
| The check before a repair round: `REPAIR_HANDOFF`, `repair-handoff.json`, the `handed-off` seal, `HANDOFF_ROUTE`, no code FAIL count, the resume path | item 11 for items 2 and 3 | no | yes | moved |
| `evolve wave status` prints `handed-off (<reason>: <blockers>)` | §8, G2 | no | yes | moved |
| The wave measures and PQ5 count the stops, the `would_stop` decisions and the false stops | §8, the shadow wave numbers | no | yes | moved on `feat/cycle-spend-limit-v2`, because it needs item 1 ([#1184](https://github.com/mickeyyaya/evolve-loop/pull/1184)) |

Item 3 names `go/internal/admission/admission.go` as its file. #1188 puts the check in `inboxmover.RankLaneMenu`. This branch puts it in `inboxmover.PartitionLaneMenu`, because all readers of the lane menu call it. The readers are triage, the census, the wave seed, the no-work check, `evolve inbox batches` and `evolve inbox rank`.
