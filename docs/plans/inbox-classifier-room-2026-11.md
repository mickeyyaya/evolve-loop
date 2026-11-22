# Plan: the loop gets more room to improve itself, one verified tier at a time

- **Status:** built, 2026-11-17 (Tier A on main; Tier B and C not started by design). Earlier status: proposed, 2026-11-16. Tier A is approved for a build; Tier B and Tier C wait for the measurements of section 8.
- **Research:** [Self-evolving loops: how much the loop may change itself, and how it heals](../research/self-evolving-loop-autonomy-2026-11.md). **Decision:** no ADR for Tier A, because it changes no boundary. Tier B gets an ADR that amends [ADR-0155](../architecture/adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md) and [ADR-0157](../architecture/adr/0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md).
- **Specs that change:** the package notes of `internal/cycleoutcome`, `internal/committedset`, `internal/inboxbatch`, `internal/inboxcurator`, `internal/inboxmover`, `internal/phases/triage` and `internal/triagedecision`.

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

The operator, 2026-11-16:

> "I want you to review the Todo tasks and redesign the classifier to give the evo loop more room to 'improve itself', research the latest self improvement and self healing loop design online and learn as much as we can and compare with the adrs and design in our docs to learn what we should do to expand its capabilities"

> "Balance the design and performance on the self evolve and healing loop, the tradeoff for the pragmatic solution."

> "Build the docs first and tests and others"

Terms used throughout:

- **Console route.** An item that only the operator console builds. `inboxbatch.ConsoleRouted` decides it.
- **Fact.** A cause that code can check: a protected file, a pipeline kind, or a commit on main. Free text is not a fact.
- **No-work lane.** A lane whose triage commits no item. `cycleoutcome.ApplyNoWork` then routes the item that the lane drew.
- **Tier.** A step of this rollout, not a level of autonomy. Tier A cleans the routes. Tier B admits T0 items to lanes as held proposals. Tier C earns autonomy for each surface from its record.
- **T0, T1, T2.** The levels of [ADR-0155](../architecture/adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md) decision 15, as `gateclass.TierOf` computes them. T0 is each path inside the protected fence. T1 is the other pipeline code. T2 is the item code. Since ADR-0157, a lane builds T1 and T2 work, and a protected item is T0 work.
- **Landed claim.** A no-work answer of triage that names a commit as the delivery of the item.

## 2. Facts

| Fact | Research id |
|---|---|
| One function decides the route. No policy key sets the console kinds. | F1.1 |
| 221 of 268 open items are console routes. 69 routes have no fact behind them. | F1.2 |
| The console route took almost the whole queue twice before (256 of 258, and 0 of 230 lane items). | F1.3 |
| Four no-work lanes in wave 174 stopped the launch of wave 175. Two drew items already on main. | F1.4 |
| The no-work path routes each answer of triage to the console, with no check of main. | F1.5 |
| A closed-drop vocabulary exists, but only the post-ship path reads it. | F1.6 |
| The curator refuses a reroute for each item with free text in `routed_reason`. The curate run of 2026-11-16 made 0 reroute proposals. | F1.7 |
| Ship retry loops, one long failed cycle and the quota bench cost more than the classifier. | F1.8 |
| ADR-0064, ADR-0155 and ADR-0157 already hold a frozen kernel and held proposals. | F1.9 |

One more measurement, made for this plan on 2026-11-16 from the runtime inbox. The loop made 29 of the console routes through the no-work path (`routed_reason` starts with `lane triage (cycle`):

| Answer of triage | Items | Correct target |
|---|---|---|
| `dropped` or `escalate_block`, `protected-surface` | 21 | console, but the protected path is only text |
| `skip_shipped`, or `dropped` with `already-shipped` | 4 | consumed at the boundary, when the commit is on main and changes a path of the item |
| `dropped` with `out-of-scope` or `stale` | 4 | a lane proposal for the operator |

The other 144 console routes came from the operator or from intake. Tier A does not change them.

*Implication:* Tier A frees about 8 items and stops a class of no-work lanes. The large room is in the protected items. Only Tier B opens that room.

## 3. Goals and non-goals

Goals:

1. A no-work lane never sends an item to the console when a checked commit says the item is done. A lane never draws that item again.
2. Each console route that the loop makes carries a fact, or it gets a lane proposal.
3. Tier B starts only with the numbers of section 8, so the next step rests on evidence.

Non-goals:

- Tier A does not change the protected manifest, the gates or the ledger.
- Tier A does not move an operator route. The operator decides each proposal.
- Tier A does not make the wave faster. The ship retry loop and the quota bench are separate work (F1.8).

## 4. Candidate designs

| Criterion (weight) | 1. Full risk model now | 2. Three tiers, cheap first (chosen) | 3. Reroute by hand only |
|---|---|---|---|
| Lane room in 2 waves (3) | 3: all tiers at once | 2: Tier A hygiene, Tier B next | 1: one pass, then drift again |
| Cost to build and run (3) | 1: risk scores, trust records, replay harness | 3: three small changes on existing verbs | 3: no code |
| Integrity risk (3) | 1: a large change to the fence at once | 3: no boundary change in Tier A | 3: none |
| Evidence before the next step (2) | 1: no baseline | 3: measured per tier | 1: no measure |
| Stops the root cause (2) | 3 | 3: the routes carry facts | 0: the same routes come back |
| **Weighted total** | **25** | **37** | **23** |

Design 2 wins. Our volume is about 20 cycles in each wave. That volume is too small to train a risk score or to give each surface a record. See research sections 2 to 4. Design 3 does not stop the cause: the no-work path makes the same routes again in the next wave (F1.5).

What changes our mind: Tier A can show no change in the no-work lanes after 2 waves. Then the cause is not the route, and we look at the plan of triage first.

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | `ApplyNoWork` checks a landed claim and fails closed. The commit must be an ancestor of main and must change a declared path of the item. A git error means "not landed". | F1.4, F1.5, R1 | Trust the text of triage: an agent can name any commit on main (research: agents attack their grader). Reuse `shaLandedOnMain`: it fails open on a git error. |
| D2 | A checked claim does not route the item and does not count as a hand-off. `ApplyNoWork` stamps the sha in place on a loop-owned field `landed_sha`. The lane menu does not offer an item with that stamp. | F1.4, F1.5 | Consume the item inside `ApplyNoWork`: a move of a tracked inbox file during a wave is not a stamp. Then `inboxstamps.Classify` blocks `sync-main`, as in cycle 2000 (2026-10-20). |
| D3 | `evolve inbox curate` at the boundary consumes an item with a `landed_sha` after it checks the claim again. The consume is deterministic. | F1.6, R1 | Leave it to the operator: the claim is a fact that code checks, so a person adds no check. |
| D4 | The sha comes from `skip_shipped.git_sha`. For a `dropped` answer with `already-shipped: <sha>`, a new reader takes the token after the colon. `ClosedDroppedIDs` reads only the tag. | F1.6 | A new field in the triage contract: two producers write the reason, the agent and the host. The git check already makes the token checkable. |
| D5 | For a `protected-surface: <path>` answer in `dropped[]` or `escalate_block[]`, `ApplyNoWork` stamps the path into the loop-owned `protected_files` through the existing `addItemProtectedFiles`. It stamps only a path that the wired protected predicate confirms. | F1.4, R3 | Write `files`: the curator owns that field (`inboxbatch/fieldowner.go`). Read only `escalate_block[]`: 14 of the 21 refusals come through `dropped[]`. Stamp an unchecked path: an agent can name a path outside the fence. |
| D6 | The route verbs stamp a loop-owned field `routed_by`: `loop` for the no-work path and the curator, `operator` for `evolve inbox route-console`. | F1.7, R2 | Infer the author from `routed_cycle`: the console verb also takes a cycle. Set it with `evolve inbox edit`: that verb refuses each loop-owned field. |
| D7 | `canRerouteToLane` follows one table. `routed_by` loop with no fact: a lane proposal. `routed_by` operator: no proposal. No `routed_by` and a reason with the no-work prefix: as loop. Other old items: the rule of today. | F1.2, F1.7, R2 | Ignore the old items: the 29 measured loop routes stay on the console. Teach the curator a free reason text: the prefix is one constant that the writer and the reader share. |

## 6. Components

| Id | Component | Red test first | Owner |
|---|---|---|---|
| A1 | The landed claim: the fail-closed check, the `landed_sha` stamp, no hand-off, and no lane menu entry (D1, D2, D4). | `TestApplyNoWork_ALandedClaimOnTheItemSurfaceStampsTheShaAndRoutesNothing`, `TestApplyNoWork_ACommitNotOnMainRoutesTheItem`, `TestApplyNoWork_ACommitOffTheItemSurfaceRoutesTheItem`, `TestApplyNoWork_AGitErrorRoutesTheItem`, `TestApplyNoWork_ALandedClaimIsNotAHandoff`, `TestApplyNoWork_AnAlreadyShippedDropNamesItsSha`, `TestPartitionLaneMenu_AnItemWithALandedShaIsNotOffered` | console (the hand-off path and the field registry) |
| A2 | The protected path of a refusal goes to `protected_files` (D5). | `TestApplyNoWork_AProtectedDropStampsTheConfirmedPath`, `TestApplyNoWork_AnEscalatedProtectedCardStampsThePath`, `TestApplyNoWork_APathThePredicateDeniesIsNotStamped`; then `ConsoleRouted` gives `protected fix surface` for that item | console |
| A3 | The `routed_by` stamp and the reroute table (D6, D7). | `TestRouteConsole_StampsTheOriginOfTheCaller`, `TestReroutes_ALoopRouteWithNoFactGetsALaneProposal`, `TestReroutes_AnOperatorRouteGetsNoProposal`, `TestReroutes_AnOldNoWorkRouteCountsAsLoop`, `TestReroutes_AnOldRouteWithAnotherReasonGetsNoProposal` | console (the field registry and the curator) |
| A4 | The boundary consume of a checked landed claim (D3). | `TestCurate_AnItemWithACheckedLandedShaGetsADeterministicConsume`, `TestCurate_AClaimThatFailsTheCheckAgainStaysOpen` | console (the curator) |

Each component keeps 100% coverage of the functions it changes. Each test goes through the real entry point: `ApplyNoWork`, `Reroutes`, or the route verb.

## 7. Order

1. **A1 and A4 first, together.** A1 stamps the claim, and A4 consumes it at the boundary. They remove the cause of two of the four no-work lanes in wave 174.
2. **A2 next.** Without the protected fact, A3 proposes the 21 protected items to lanes, and triage refuses each again.
3. **A3 after A2,** in the same boundary batch.
4. **Measure 2 waves** (section 8). Then write the Tier B ADR, or stop.

## 8. Verification

| Goal | Measure | Before (wave 174) | Target after Tier A |
|---|---|---|---|
| G1 | No-work lanes for an item already on main | 2 of 4 no-work lanes | 0 |
| G1 | `no-work-handoffs` halts at a wave launch | 1 | 0 |
| G2 | Loop routes with no fact | 29 with only text | 0: each has a fact or a proposal |
| G2 | Lane-routable items | 47 of 268 | 47 plus each accepted proposal |
| G3 | Ship rate (PQ1) and LLM minutes for each shipped cycle | 11 of 20; 112.3 LLM min | no worse |

The health readings of each boundary hold these numbers. The series lives under `docs/research/pipeline-health-readings/` after its first docs PR lands.

Gate for Tier B: two waves with G1 at 0 and PQ1 no worse. Then the Tier B ADR covers two parts (research R4):

- the admission of T0 items to lanes as held proposals. [ADR-0157](../architecture/adr/0157-the-loop-works-its-own-code-a-narrow-fence-and-held-proposals.md) built the hold. The [narrow-fence spec](../architecture/narrow-fence-and-held-proposals.md) names the admission as the step that remains;
- a cap of about 2 such items in each wave.

The refusal of a gate-weakening diff is already on main: `gateclass`, `core.GateClassifierFloorChecks` and the ship check `refuseWeakenedGates`. Tier B uses it and does not build it again.

## 9. Risks

| Risk | Control |
|---|---|
| A false sha retires an open item. | The commit must be on main and must change a declared path of the item. The curator checks the claim again before the consume. The ledger names the sha. |
| A git error retires an item. | The check fails closed: a git error means "not landed", and the item routes as today. |
| A stamp blocks `sync-main`. | `landed_sha`, `protected_files` and `routed_by` are loop-owned stamps written in place. The registry tests of `inboxbatch/fieldowner.go` pin each new field. |
| A lane proposal sends protected work to a lane. | A2 lands first. A lane claim still refuses a protected item (`lifecycle/claim.go`). The operator accepts each proposal. |
| The new field breaks an old reader. | `routed_by` is optional. A reader that does not know it ignores it. |

## 10. Open questions

1. Intake accepted a path that does not exist on main (`inboxmover/mover.go`, cycle 2415). PR #1492 corrected the `files` of the items of cycles 2415 and 2417. Must intake refuse such a path, or only warn as `inbox lint` does today? A refusal blocks a feature item that adds a new file.
2. The 144 operator and intake routes hold most of the console mass. The operator decides if a one-time review of them comes before Tier B.
3. `inbox curate` proposals need the operator. How many proposals in each boundary can the operator judge?

## Delivery status (2026-11-21)

| Component | Status | Change lists | Open item |
|---|---|---|---|
| A1 landed claim | done, 2026-11-17 | [#1494](https://github.com/mickeyyaya/evolve-loop/pull/1494) [`19ef9ec`](https://github.com/mickeyyaya/evolve-loop/commit/19ef9ecb1) (`go/internal/cycleoutcome/nowork_landed_test.go`, `go/internal/inboxmover/lane_menu_partition_test.go`) | none |
| A2 protected path to `protected_files` | done, 2026-11-17 | [#1494](https://github.com/mickeyyaya/evolve-loop/pull/1494) [`19ef9ec`](https://github.com/mickeyyaya/evolve-loop/commit/19ef9ecb1) (`TestApplyNoWork_AProtectedDropStampsTheConfirmedPath`) | none |
| A3 `routed_by` stamp and reroute table | done, 2026-11-17 | [#1494](https://github.com/mickeyyaya/evolve-loop/pull/1494) [`19ef9ec`](https://github.com/mickeyyaya/evolve-loop/commit/19ef9ecb1) (`inboxbatch/fieldowner.go:70`, `go/internal/inboxcurator/reroute_test.go`) | none |
| A4 boundary consume of a checked claim | done, 2026-11-17 | [#1494](https://github.com/mickeyyaya/evolve-loop/pull/1494) [`19ef9ec`](https://github.com/mickeyyaya/evolve-loop/commit/19ef9ecb1) (`TestCurate_AnItemWithACheckedLandedShaGetsADeterministicConsume`) | none |
| Tier B ADR and held-proposal admission | dropped (operator, 2026-11-22) | none | none: the row is gated on two waves of the section 8 measures; no measured need yet. Old status: not started; gated on two waves of the section 8 measures |

Tier A is on main in PR 1494. The operator dropped Tier B on 2026-11-22: no measured need yet. Open question 1 (intake refuses a missing path) is not answered in the code that I read.
