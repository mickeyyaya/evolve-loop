# Plan: console work — build only what a lane cannot build, and stop when the lanes starve

- **Status:** partly built, 2026-11-21 (3 of 5 components on main). Earlier status: proposed, 2026-11-17.
- **Research:** [Console work: when the console builds an item and when it routes the item to a lane](../research/console-build-or-route-2026-11.md). **Decision:** no ADR, because the plan changes no boundary of the loop. It adds rule 6 to [operating-policy.md §2](../operations/operating-policy.md), the rule of record.
- **Related:** [the inbox classifier tiers](inbox-classifier-room-2026-11.md) stamp the author of each console route (D6 there) and decide its reroute (D7 there). This plan says which facts justify a console build, and when the console stops.

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

The operator, 2026-11-17:

> "How do we justify when to continue building the console only feature and when to stop?"

> "Research first and building the doc"

A worked example from the same day. The console built PR #1531, a fix to the integration train. The train is on the protected surface, so a lane must not build it, and the build was justified. In the same hours, wave 177 ran 1 lane of 8, because 180 items had a console route and 1 item was admissible. The console build was justified, but the lanes starved while it ran.

## 2. Facts

| Fact | Research id |
|---|---|
| The lanes ran at 18% of the window. | F1.1 |
| 1 of 288 items was admissible. 180 had a console route. | F1.2 |
| No console route carries a stored protected path. 147 show no protected path and no pipeline kind. 74 have no reason. 71 are features. | F1.3 |
| The route field alone decides the console route. | F1.4 |
| The curator at enforce proposes 0 reroutes. | F1.5 |
| 28 console PRs are open. One batch landed 10. | F1.6 |
| A console build costs two to four review rounds. A lane ships in a median 46.6 LLM minutes. | F1.7 |
| One coarse cell holds 19 to 22 lane items. | F1.8 |
| The policy covers console fixes of integrity defects, not console features. | F1.9 |
| The constraint does only its own work. Gate changes stay out of the hands of the graded agent. Feature work stops on a breach. | P2.1, P2.4, P2.5, P2.6 |

## 3. Goals and non-goals

Goals:

1. Each console build has a reason that a person can check in one step.
2. The console stops feature builds while the lanes starve, and works on the supply.
3. The console finishes its open work before it starts new work.

Non-goals:

- This plan does not change the protected surface, the gates, or who merges.
- It does not change how the route field is stamped. The classifier plan owns that.
- It does not add a time cap on console hours. No verb measures them yet (research §3).

## 4. Candidate designs

| Design | What it does |
|---|---|
| A. No change | The console builds what has a console route, in rank order. |
| B. Capability test only | The console builds an item only when a capability that lanes must not have requires it (R1). |
| C. Capability test and two stop rules | B, and the console starts no feature build while the supply starves (R3) or while its open PRs exceed one batch (R4). |
| D. Time cap | The console spends at most half of its hours on builds (the SRE toil cap). |

Scores: 0 is bad, 2 is good.

| Criterion | A | B | C | D |
|---|---|---|---|---|
| Lane width returns (F1.1, F1.2) | 0 | 1 | 2 | 1 |
| A person can check each console build (goal 1) | 0 | 2 | 2 | 0 |
| Console work in flight stays bounded (F1.6) | 0 | 0 | 2 | 1 |
| Measurable today with current verbs | 2 | 1 | 1 | 0 |
| Cost to adopt | 2 | 2 | 1 | 0 |
| **Total** | 4 | 6 | 8 | 2 |

C wins. B alone does not stop the console when the lanes starve for a reason that is not a route, such as the coarse cell (F1.8). D needs a measure of console hours that does not exist. The stop rules of C read numbers that `evolve wave supply` and `gh pr list` already print. The checked-fact count of B and C waits for K2.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | The console builds an item only for one of three capabilities. They are a protected path, a console-only act, and a defect of operating-policy §1.1. | F1.3, F1.4, F1.9, P2.1, P2.5, P2.6 | "The console is faster": speed is not a capability, and each console hour is lost from merges and reviews (F1.7). |
| D2 | A protected path counts only when the protected check confirms it. Free text in `routed_reason` does not count. | F1.3, F1.4, P2.6 | Trust the reason text: 74 routes have none, and the rest name no checked path. |
| D3 | When lanes lack a verb, a seam or a cell split, the console builds that enabler and routes the item to a lane. | F1.8, P2.3 | Build the item on the console: the next item of that kind stays console-only too. |
| D4 | Before each new console build, the console reads the supply. If admissible items are fewer than the window, it starts no feature build. | F1.1, F1.2, P2.1, P2.4 | A utilization floor alone: utilization lags by one wave. Admissible items predict the next wave. |
| D5 | While the supply starves, the console works on the supply: routes, splits, enablers, merges and integrity defects. | F1.2, F1.5, P2.4 | Stop all console work: a red main or a broken gate still needs the console. |
| D6 | While open console PRs are more than one batch (10), the console starts no new build. It reviews, rebuilds and batches the open ones. | F1.6, P2.2, P2.8 | No limit: 28 PRs wait, and the oldest is 8 days old. A limit of 1: one PR at a time leaves the train half empty. |
| D7 | An unprotected item stays on the console only when it changes the files of an open console PR and needs no new review. The route reason names that PR. | P2.7 | No exception: a split of one small edit costs a lane cycle and a merge order. |
| D8 | A console build in progress stops at the existing limits. More than 3 blocks means a redesign. There is no review round 4. A deferred decision waits for its trigger. | R5, F1.7 | New limits: the existing rules already cover the in-progress case. |

Consequences:

- Good: the console spends its hours on merges, gates and the supply. The lanes get the features.
- Good: each console build has a checked line: "D1 capability: protected path `go/internal/integration/train.go`".
- Bad: some console-routed features wait one boundary longer while the console moves them to lanes.
- Bad: the console gives up the speed of one session for a feature that a lane builds more slowly.

What changes our mind: the lanes fail more than half of the items that this plan moves from the console. Then the move costs more than it saves. Then the move pauses, and each failure gets an incident record.

## 6. Components

| Id | Component | Red test first | Lane or owner |
|---|---|---|---|
| K1 | Rule 6 in operating-policy §2: the three capabilities and the two stop rules. | none (a document) | console, this PR |
| K2 | `evolve wave supply` splits `console_route` into items with a checked protected path and items without one. | an item with a console route and an unprotected file counts as `console_route_no_fact` | lane |
| K3 | `evolve wave supply` prints one advice line: `console: build`, `console: feed` (D4) or `console: finish` (D6). | a supply with 1 admissible item in a window of 8 prints `console: feed` | lane |
| K4 | A boundary reroute. The console runs the protected check on the 147 routes without a stored fact. It lists each route with no confirmed path for the operator. An item that needs a console-only act or is a §1.1 defect stays. The rest move to lanes as one inbox PR. | none (an inbox change at the boundary) | console |
| K5 | Admission splits the coarse `cmd/evolve` cell, so two items that edit different files there run at once. Design review first. | two items that edit two different files of `cmd/evolve` both admit | lane, after a design doc |

## 7. Order

1. K1 lands with this design set, so the rule applies from the next boundary.
2. K4 at the next boundary. It uses the census of research F1.3 and the operator's approval. It gives the largest gain at once.
3. K2 and K3, filed as lane items. K3 needs the split of K2.
4. K5 after its design review. It changes admission, which every lane uses. Component C6 of [the concurrent-edit plan](concurrent-edit-layout-2026-11.md) first splits `cmd/evolve` into smaller packages. K5 runs only when the waiting items stay above the target of that plan.

## 8. Verification

| Measure | Before (2026-11-17) | Target after K4 | Command |
|---|---|---|---|
| Admissible items | 1 | at least 8 | `evolve wave supply` |
| Lane utilization | 0.18 | at least 0.6 | `evolve wave status` |
| Console routes without a stored or checked fact | 147 (the protected check did not run) | 0 | `evolve wave supply` after K2 |
| Open console PRs | 28 | at most 10 | `gh pr list --state open` |

The lane failure rate of the moved items is the counter-measure (section 5, "what changes our mind").

## 9. Risks

| Risk | Control |
|---|---|
| A moved item touches a gate path that its `files` list does not name. | The protected check of the lane triage refuses it, and the item returns with a checked path (D2). |
| The console reads the supply once and forgets it. | K3 prints the advice in the verb that the console already runs at each boundary. |
| The stop rule blocks a fix that main needs. | D5 keeps integrity defects and merges as console work while the supply starves. |

## 10. Open questions

- **Q1.** Do the `console-manual` routes that the operator filed by hand stay? This plan treats an operator route as an operator decision, so it stays. The 147 routes have no `routed_by`, so K4 lists them for the operator before the move.
- **Q2.** Is one batch (10) the right limit for D6? The train landed 10 in one batch on 2026-11-16. Measure again after three boundaries.

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| K1 rule 6 in operating-policy §2 | done, 2026-11-17 | [#1532](https://github.com/mickeyyaya/evolve-loop/pull/1532) [`a179b59`](https://github.com/mickeyyaya/evolve-loop/commit/a179b5995) (`docs/operations/operating-policy.md:172`) | none |
| K2 supply splits `console_route_no_fact` | done, 2026-11-17 | cycle 2457 [`7f75fe9`](https://github.com/mickeyyaya/evolve-loop/commit/7f75fe933) | none |
| K3 supply advice line | done, 2026-11-19 | cycle 2519 [`6312414`](https://github.com/mickeyyaya/evolve-loop/commit/6312414d5) (`go/acs/cycle2519/predicates_test.go:20`) | none |
| K4 boundary reroute of console routes | unverified: this step is an inbox change, and no commit names it. The related approval is the backlog review of 2026-11-19. | none | none (draft: console-build-or-route-k4-reroute-record) |
| K5 split of the `cmd/evolve` admission cell | partial: [admission plan](admission-cmd-evolve-cell-2026-11.md) K1 is on main and the `file_cells` step landed (see that plan) | see that plan | cmd-evolve-cell-is-too-coarse-for-8-lanes |

K1 to K3 are on main. K5 depends on the admission plan and has an open item. K4 is unfinished or unrecorded and has no open inbox item. A follow-up draft asks for a measured count of console routes without a checked fact. The target of section 8 is zero.
