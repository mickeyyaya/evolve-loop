# Plan: finish the partly built features one after another, closest to done first

- **Status:** proposed, 2026-11-22. The operator chose the order on 2026-11-22. The Status becomes `built` when the queue of section 6.2 is empty.
- **Inputs:** the delivery tables of the 83 plans in PR #1671 (state of 2026-11-21). The 32 pull requests of the next boundary. The live inbox of 2026-11-22. The triage of 2026-11-21.
- **Related:** [pipeline-blockers-2026-11](pipeline-blockers-2026-11.md) (a blocker fix can jump the queue).
- **Related:** [shadow-stage-evidence-2026-11](shadow-stage-evidence-2026-11.md) (each flip of section 10.1 needs its evidence).
- **Related:** [wave-orchestrator-2026-10](wave-orchestrator-2026-10.md) (the lanes that take the wave sets).

## Table of contents

- [Decision record: the operator decisions of 2026-11-22](#decision-record-the-operator-decisions-of-2026-11-22)
- [1. Request](#1-request)
- [2. Facts](#2-facts)
- [3. Goals and non-goals](#3-goals-and-non-goals)
- [4. Candidate designs](#4-candidate-designs)
- [5. Decisions: the rule](#5-decisions-the-rule)
- [6. Components: the queue](#6-components-the-queue)
- [7. Order: wave sets and console order](#7-order-wave-sets-and-console-order)
- [8. Verification](#8-verification)
- [9. Risks](#9-risks)
- [10. Open questions: the operator decision list](#10-open-questions-the-operator-decision-list)

## Terms used throughout

- **Component:** one row of a delivery table `## Delivery status (2026-11-21)` in a plan.
- **Closed:** a component that is done, superseded, dropped by the plan, or dropped by a recorded decision (kind d).
- **Boundary:** the stop between two waves, where the console merges the recorded pull requests.
- **Lane item:** an inbox item that a lane can take. It has no route or the route `lane`, and no open dependency.
- **Console item:** an inbox item with the route `console` or `console-manual`.
- **Switch:** a flip of a stage key from `shadow` to `enforce`. The operator makes it on a verified shadow record.
- **Kind a to e:** the five kinds of an open component, defined in section 6.1.

## Decision record: the operator decisions of 2026-11-22

| Id | Question | Decision | Consequence |
|---|---|---|---|
| OD1 | Build the shadow-contract chain next? | Yes. Build `shadow-contract-k2-evidence-seal`, `shadow-contract-k4-protected-contract`, `shadow-contract-k3-stage-verify` and `shadow-contract-k5-stage-flip`. | The chain jumps the order of section 6.2 (rule D9). It blocks every switch of section 10.1: 10 switches over 8 stage keys. Three rows need more: the two `gates.scan_cache_gate` rows and the handoff pair row. Their stage key needs a registry entry first. |
| OD2 | Approve the 14 orphans of section 10.2? | Yes, all 14. | The 13 drops are recorded as dropped. The 14th piece is filed as item `boundary-step-ci-latency-check-calls-a-missing-verb` (filed 2026-11-22). |

The console builds K2 and K4 first, then K3 and K5. Each item gets a design review before a seat builds it.

## 1. Request

The operator asked on 2026-11-22: "could we prioritize the unfinished feature to wrap up feature one after another". The operator chose the order "Closest to done first". The operator also allowed pipeline blocker fixes to jump the queue.

## 2. Facts

| Fact | Source |
|---|---|
| The 83 plans hold 818 components. 491 are done. 45 are closed by the status superseded, dropped, accepted or not applicable. | Delivery tables in PR #1671 |
| 108 components have no item and a recorded drop decision (kind d). | Scratchpad `plans/triage-52.tsv` and `plans/draft-class.tsv` |
| 138 components have an open item (kind a). 10 more have a switch as the last step (kind c). | Live inbox, 2026-11-22 |
| 12 components are built in a recorded pull request and merge at the next boundary (kind b). | The 32 recorded pull requests |
| 14 components have no item and no drop decision (kind e). | This review |
| The live inbox holds 231 items. 62 lane items are in the queue. 35 of them have no open dependency. 4 are held by a running cycle. | `R/.evolve/inbox/` |
| `evolve wave next` has the flag `--only IDS`. The ids are comma separated. An id that no lane can take exits 10. | `evolve wave` usage; `go/cmd/evolve/cmd_loop_args.go:75` |
| `evolve stage report` on 2026-11-22 shows `empty-chain` for all 42 stage keys. The verb reads an empty chain unless `--chain PATH` is given. | `go/cmd/evolve/cmd_stage.go:83` |
| No recorder and no `stage-evidence-seal` entry exist on main. So no switch can reach a verified record today. | Plan `shadow-stage-evidence-2026-11`, row K2 |

*Implication:* the order of the queue is the order of the pieces. A switch needs the evidence recorder (item `shadow-contract-k2-evidence-seal`) before the operator can flip it.

## 3. Goals and non-goals

| Goal | Measure |
|---|---|
| G1 Each plan closes before the next plan starts. | At most 3 plans have a started and unfinished piece at one time. |
| G2 A wave works the head of the queue. | Each wave set of section 7.1 holds lane items of the first plans that still have lane items. |
| G3 The operator sees each decision once. | Section 10 lists each switch and each orphan once. |

Non-goals:

- This plan flips no stage. It files two items only: the item of section 10.2 (filed on 2026-11-22) and `shadow-contract-k3-stage-verify` (OD1).
- This plan does not revive an item that the operator withdrew or deleted.
- This plan does not change a blocker rule. A blocker fix can still jump the queue.

## 4. Candidate designs

Each criterion scores 1 (poor) to 5 (good). The scores are synthesis from the facts of section 2.

| Design | Description | Fewer open plans | Needs no new data | Pipeline value | Total |
|---|---|---|---|---|---|
| A | Closest to done first (the share of closed components after the boundary). | 5 | 5 | 3 | 13 |
| B | Highest weight of the open items first. | 2 | 4 | 5 | 11 |
| C | The plan with the most open items first. | 1 | 5 | 3 | 9 |

Design A wins on the two criteria that the operator named. Design B leaves many plans at 90% for a long time. Design C starts the largest plans, which close last.
Design A uses two tie-breaks. The first is fewer open pieces. The second is the pipeline value: correctness and stability come before the other classes.

## 5. Decisions: the rule

| Id | Decision | Because |
|---|---|---|
| D1 | Finish the features in the order of the queue in section 6.2. | The operator chose "Closest to done first". |
| D2 | A wave works only the lane items of the head features. Start it with `evolve wave next --only <ids>`. | The flag exists, and it scopes the lanes to the ids. |
| D3 | The console builds the console items of the same features in the same order. | One feature closes before the next feature starts. |
| D4 | When the head feature has fewer lane items than lanes, fill the wave from the next feature. | An idle lane wastes the wave. |
| D5 | A feature closes when its delivery table has no open row. Then its plan Status becomes `built`. | The delivery table is the record of the plan. |
| D6 | A pipeline blocker fix can jump the queue. | The operator allowed it. |
| D7 | Never revive an item that the operator withdrew or deleted. | The scratchpad files `plans/draft-class.tsv` and `plans/draft-class2.tsv` list them. |
| D8 | Rebuild the queue at each boundary from the delivery tables and the live inbox. | The order changes when a piece lands. |
| D9 | An enabler that unblocks the switches of several features can jump the order. The first enabler is the shadow-contract chain K2, K4, K3 and K5 (OD1). | One missing recorder blocks 10 switches over 8 stage keys, so the closest-to-done order cannot reach any of them. |

The rule in short form:

1. Take the first plan of the queue that has an open piece.
2. Give its lane items to the next wave, and its console items to the console.
3. When a feature has no more pieces to take, take the next feature.
4. When a delivery table has no open row, set the plan Status to `built`.
5. When an enabler unblocks the switches of several features, build it first (D9).

## 6. Components: the queue

### 6.1 The five kinds

| Kind | Meaning | Next action |
|---|---|---|
| a | An open inbox item exists. | A lane or the console builds the item. |
| b | A recorded pull request builds the piece. | The console merges the pull request at the boundary. |
| c | The last step is a switch. | The operator flips the key on a verified shadow record. |
| d | No item exists, and a recorded decision dropped the piece. | None. Mark the row `dropped` in the plan. |
| e | No item and no drop decision exist. | File the item draft, or drop the piece. The operator decides. |

A switch row has kind c even when an item tracks the flip. A component has one kind only.

The completion share is the number of closed components divided by the total. Closed means done, superseded, dropped by the plan, or kind d. After the boundary, kind b is closed too.

### 6.2 The queue

The queue holds the 52 plans that still have a piece of kind a, c or e after the boundary. Rank 1 is the head. The pieces of each plan are in section 6.3. The columns count the state of 2026-11-21 before OD2; the 14 pieces of kind e are decided (section 10.2).

| Rank | Plan | Closed before | Closed after | a | b | c | d | e |
|---|---|---|---|---|---|---|---|---|
| 1 | [ci-wall-time-2026-10](ci-wall-time-2026-10.md) | 19/20 (95%) | 19/20 (95%) | 1 | 0 | 0 | 1 | 0 |
| 2 | [fleet-landing-at-scale-2026-10](fleet-landing-at-scale-2026-10.md) | 17/18 (94%) | 17/18 (94%) | 0 | 0 | 1 | 7 | 0 |
| 3 | [inbox-stale-claims-2026-10](inbox-stale-claims-2026-10.md) | 11/12 (92%) | 11/12 (92%) | 1 | 0 | 0 | 1 | 0 |
| 4 | [retry-ladder-2026-10](retry-ladder-2026-10.md) | 11/12 (92%) | 11/12 (92%) | 0 | 0 | 1 | 0 | 0 |
| 5 | [dispatch-process-hygiene-2026-10](dispatch-process-hygiene-2026-10.md) | 10/11 (91%) | 10/11 (91%) | 1 | 0 | 0 | 2 | 0 |
| 6 | [gc-closes-the-legacy-gap-2026-11](gc-closes-the-legacy-gap-2026-11.md) | 10/11 (91%) | 10/11 (91%) | 1 | 0 | 0 | 0 | 0 |
| 7 | [opus55-effort-config-2026-10](opus55-effort-config-2026-10.md) | 10/11 (91%) | 10/11 (91%) | 0 | 0 | 0 | 1 | 1 |
| 8 | [token-telemetry-2026-07](token-telemetry-2026-07.md) | 9/10 (90%) | 9/10 (90%) | 1 | 0 | 0 | 1 | 0 |
| 9 | [skills-drift-worktree-generator-2026-10](skills-drift-worktree-generator-2026-10.md) | 8/9 (89%) | 8/9 (89%) | 0 | 0 | 0 | 2 | 1 |
| 10 | [token-telemetry-2026-10](token-telemetry-2026-10.md) | 7/9 (78%) | 8/9 (89%) | 1 | 1 | 0 | 2 | 0 |
| 11 | [llm-isolation-2026-10](llm-isolation-2026-10.md) | 29/34 (85%) | 30/34 (88%) | 4 | 1 | 0 | 1 | 0 |
| 12 | [comment-reduction-2026-09](comment-reduction-2026-09.md) | 7/8 (88%) | 7/8 (88%) | 1 | 0 | 0 | 2 | 0 |
| 13 | [fleet-host-adaptive-window-2026-11](fleet-host-adaptive-window-2026-11.md) | 7/8 (88%) | 7/8 (88%) | 0 | 0 | 1 | 0 | 0 |
| 14 | [logging-and-process-hygiene-2026-10](logging-and-process-hygiene-2026-10.md) | 7/8 (88%) | 7/8 (88%) | 1 | 0 | 0 | 4 | 0 |
| 15 | [convergence-policy-2026-10](convergence-policy-2026-10.md) | 13/15 (87%) | 13/15 (87%) | 0 | 0 | 1 | 4 | 1 |
| 16 | [guidance-layers-2026-10](guidance-layers-2026-10.md) | 13/15 (87%) | 13/15 (87%) | 2 | 0 | 0 | 8 | 0 |
| 17 | [code-index-cli-2026-10](code-index-cli-2026-10.md) | 6/7 (86%) | 6/7 (86%) | 1 | 0 | 0 | 1 | 0 |
| 18 | [latency-gates-2026-10](latency-gates-2026-10.md) | 6/7 (86%) | 6/7 (86%) | 0 | 0 | 0 | 1 | 1 |
| 19 | [wave-latency-2026-10](wave-latency-2026-10.md) | 6/7 (86%) | 6/7 (86%) | 0 | 0 | 1 | 0 | 0 |
| 20 | [phase-handoff-contract-2026-10](phase-handoff-contract-2026-10.md) | 9/12 (75%) | 10/12 (83%) | 1 | 1 | 1 | 0 | 0 |
| 21 | [llm-quality-indexes-2026-10](llm-quality-indexes-2026-10.md) | 9/11 (82%) | 9/11 (82%) | 2 | 0 | 0 | 3 | 0 |
| 22 | [cover-changed-gate-2026-10](cover-changed-gate-2026-10.md) | 9/11 (82%) | 9/11 (82%) | 1 | 0 | 0 | 0 | 1 |
| 23 | [wave-integration-2026-10](wave-integration-2026-10.md) | 17/21 (81%) | 17/21 (81%) | 2 | 0 | 2 | 0 | 0 |
| 24 | [code-index-backends-2026-10](code-index-backends-2026-10.md) | 4/5 (80%) | 4/5 (80%) | 1 | 0 | 0 | 0 | 0 |
| 25 | [console-build-or-route-2026-11](console-build-or-route-2026-11.md) | 4/5 (80%) | 4/5 (80%) | 1 | 0 | 0 | 1 | 0 |
| 26 | [inbox-classifier-room-2026-11](inbox-classifier-room-2026-11.md) | 4/5 (80%) | 4/5 (80%) | 0 | 0 | 0 | 0 | 1 |
| 27 | [workspace-hygiene-2026-07](workspace-hygiene-2026-07.md) | 4/5 (80%) | 4/5 (80%) | 1 | 0 | 0 | 0 | 0 |
| 28 | [admission-cmd-evolve-cell-2026-11](admission-cmd-evolve-cell-2026-11.md) | 3/4 (75%) | 3/4 (75%) | 1 | 0 | 0 | 0 | 0 |
| 29 | [gate-ship-retry-waste-2026-11](gate-ship-retry-waste-2026-11.md) | 6/8 (75%) | 6/8 (75%) | 2 | 0 | 0 | 0 | 0 |
| 30 | [event-notification-protocol-2026-10](event-notification-protocol-2026-10.md) | 11/15 (73%) | 11/15 (73%) | 4 | 0 | 0 | 2 | 0 |
| 31 | [cost-aware-repair-2026-10](cost-aware-repair-2026-10.md) | 8/11 (73%) | 8/11 (73%) | 3 | 0 | 0 | 0 | 0 |
| 32 | [finder-fix-ownership-2026-10](finder-fix-ownership-2026-10.md) | 5/7 (71%) | 5/7 (71%) | 2 | 0 | 0 | 2 | 0 |
| 33 | [gate-contract-2026-11](gate-contract-2026-11.md) | 5/7 (71%) | 5/7 (71%) | 2 | 0 | 0 | 2 | 0 |
| 34 | [ledger-restructure-2026-10](ledger-restructure-2026-10.md) | 8/13 (62%) | 9/13 (69%) | 4 | 1 | 0 | 3 | 0 |
| 35 | [bridge-factory-2026-10](bridge-factory-2026-10.md) | 4/6 (67%) | 4/6 (67%) | 1 | 0 | 0 | 3 | 1 |
| 36 | [agent-messaging-2026-10](agent-messaging-2026-10.md) | 6/9 (67%) | 6/9 (67%) | 3 | 0 | 0 | 1 | 0 |
| 37 | [harness-practices-2026-11](harness-practices-2026-11.md) | 5/9 (56%) | 6/9 (67%) | 3 | 1 | 0 | 0 | 0 |
| 38 | [seat-login-isolation-2026-11](seat-login-isolation-2026-11.md) | 6/9 (67%) | 6/9 (67%) | 3 | 0 | 0 | 1 | 0 |
| 39 | [code-review-phase-2026-10](code-review-phase-2026-10.md) | 9/14 (64%) | 9/14 (64%) | 4 | 0 | 1 | 0 | 0 |
| 40 | [test-quality-audit-2026-11](test-quality-audit-2026-11.md) | 7/11 (64%) | 7/11 (64%) | 3 | 0 | 0 | 0 | 1 |
| 41 | [narrow-fence-2026-10](narrow-fence-2026-10.md) | 5/8 (62%) | 5/8 (62%) | 3 | 0 | 0 | 0 | 0 |
| 42 | [architecture-decisions-2026-11](architecture-decisions-2026-11.md) | 18/30 (60%) | 18/30 (60%) | 9 | 0 | 1 | 2 | 2 |
| 43 | [shadow-stage-evidence-2026-11](shadow-stage-evidence-2026-11.md) | 8/14 (57%) | 8/14 (57%) | 6 | 0 | 0 | 1 | 0 |
| 44 | [incident-knowledge-base-2026-11](incident-knowledge-base-2026-11.md) | 5/13 (38%) | 7/13 (54%) | 6 | 2 | 0 | 0 | 0 |
| 45 | [queue-authority-from-host-facts-2026-11](queue-authority-from-host-facts-2026-11.md) | 5/10 (50%) | 5/10 (50%) | 2 | 0 | 0 | 2 | 3 |
| 46 | [pipeline-blockers-2026-11](pipeline-blockers-2026-11.md) | 8/19 (42%) | 9/19 (47%) | 9 | 1 | 0 | 1 | 1 |
| 47 | [review-routing-2026-11](review-routing-2026-11.md) | 5/12 (42%) | 5/12 (42%) | 7 | 0 | 0 | 5 | 0 |
| 48 | [debug-cli-2026-11](debug-cli-2026-11.md) | 5/13 (38%) | 5/13 (38%) | 8 | 0 | 0 | 1 | 0 |
| 49 | [concurrent-edit-layout-2026-11](concurrent-edit-layout-2026-11.md) | 3/8 (38%) | 3/8 (38%) | 5 | 0 | 0 | 0 | 0 |
| 50 | [split-cmd-evolve-2026-11](split-cmd-evolve-2026-11.md) | 2/6 (33%) | 2/6 (33%) | 4 | 0 | 0 | 0 | 0 |
| 51 | [audit-evaluation-contract-2026-10](audit-evaluation-contract-2026-10.md) | 2/8 (25%) | 2/8 (25%) | 6 | 0 | 0 | 1 | 0 |
| 52 | [cycle-recycle-and-gc-check-2026-11](cycle-recycle-and-gc-check-2026-11.md) | 1/15 (7%) | 1/15 (7%) | 14 | 0 | 0 | 0 | 0 |

The columns a to e count the components of that kind, and column d counts the pieces that are already closed by a drop. The remaining pieces of a plan are a + c + e.

### 6.3 The pieces of each plan

Rows with the same next action are grouped. The column Rank is the rank of section 6.2.

| Rank | Plan | Piece | Kind | Next action |
|---|---|---|---|---|
| 1 | ci-wall-time | C13 macOS CPU sampler and spawn counts | a | The console builds `ci-wall-time-c13-macos-cpu-and-spawn-counts`. |
| 1 | ci-wall-time | C14 fewer git spawns on macOS | d | The operator withdrew or deleted the item `ci-wall-time-c14-fewer-git-spawns-on-macos` earlier (scratchpad `plans/draft-class.tsv`). |
| 2 | fleet-landing-at-scale | S2, S7, S8, S13, S15, S16, S17 | d | Triage of 2026-11-21 dropped the draft `fleet-landing-s-rows-after-adr-0147` (scratchpad `plans/triage-52.tsv`): ADR-0147 and the train replaced the S rows. |
| 2 | fleet-landing-at-scale | S6 scan with keys, enforce | c | The operator flips `gates.scan_cache_gate` after a verified shadow record (section 10.1). |
| 3 | inbox-stale-claims | Follow-up inbox-claims-delta-followups | a | A lane builds `inbox-claims-delta-followups` in wave 187. |
| 3 | inbox-stale-claims | Follow-up inbox-claims-review-followups | d | The operator withdrew or deleted the item `inbox-claims-review-followups-lane` earlier (scratchpad `plans/draft-class.tsv`). |
| 4 | retry-ladder | L8 move to enforce | c | The operator flips `workflow.retry_ladder.stage` after a verified shadow record (section 10.1). |
| 5 | dispatch-process-hygiene | Later M-c wider shared class, L-a tree | a | The console builds `dispatch-hygiene-followups`. |
| 5 | dispatch-process-hygiene | Later Linux cgroup per dispatch, manual | d | Triage of 2026-11-21 dropped the draft `dispatch-hygiene-cgroup-and-legacy` (scratchpad `plans/triage-52.tsv`): cgroups exist on Linux only, and the host is macOS. |
| 5 | dispatch-process-hygiene | Review follow-up dispatch-hygiene-architecture-followups | d | The item was consumed by the bookkeeping of PR 1605 (inbox review 2026-11-20). |
| 6 | gc-closes-the-legacy-gap | X11 wave-end run and gc.wave_end.scope | a | `recycle-x9-loop-runs-host-checks` waits for `recycle-x4-mandatory-recycle-step` and `recycle-x1b-check-registry-run-dirs` and `recycle-x1c-check-registry-cache`. |
| 7 | opus55-effort-config | Effort sweep | d | Triage of 2026-11-21 dropped the draft `opus55-effort-sweep` (scratchpad `plans/triage-52.tsv`): it is a six-wave experiment, and the memory rule keeps medium effort until a measured gain. |
| 7 | opus55-effort-config | Later cli-routing explain variant drift, line-length guard | e | Dropped by the operator on 2026-11-22 (OD2). |
| 8 | token-telemetry-2026-07 | S5 advisor, swarm and subagent tokens | d | Superseded in part by token-telemetry-2026-10; T2 item withdrawn earlier (draft-class.tsv). |
| 8 | token-telemetry-2026-07 | Phase-2 optimization items | a | The console builds `tokenopt-cache-discipline-audit`. A lane builds `tokenopt-session-resume-on-retry` in wave 187. |
| 9 | skills-drift-worktree-generator | Residual the host owns the comparison; Residuals and signal event, run context | d | The operator withdrew or deleted the item `skills-drift-gate-render-verb` earlier (scratchpad `plans/draft-class.tsv`). |
| 9 | skills-drift-worktree-generator | Residual one registry for the generator prefix | e | Dropped by the operator on 2026-11-22 (OD2). |
| 10 | token-telemetry-2026-10 | T2 token-row-declared-purpose | d | The operator withdrew or deleted the item `token-row-declared-purpose` earlier (scratchpad `plans/draft-class.tsv`). |
| 10 | token-telemetry-2026-10 | T5 claude-statusline-rate-limits | d | The operator withdrew or deleted the item `claude-statusline-rate-limits` earlier (scratchpad `plans/draft-class.tsv`). |
| 10 | token-telemetry-2026-10 | T7 tokens-report-from-llm-calls | a | A lane builds `tokens-report-from-llm-calls` in wave 188. |
| 10 | token-telemetry-2026-10 | T8 token-weights-policy-key | b | Merge PR #1623 at the boundary. The boundary consumes `token-weights-policy-key`. |
| 11 | llm-isolation | X4 No nested agency | d | The operator withdrew or deleted the item `isolation-no-nested-agency` earlier (scratchpad `plans/draft-class.tsv`). |
| 11 | llm-isolation | X6b Floor profiles as routing data | b | Merge PR #1646 at the boundary. The boundary consumes `isolation-floor-profiles-allowed-clis-as-routing-data`. |
| 11 | llm-isolation | X9b llmroute and modelquery tables from manifests | a | A lane builds `isolation-x9b-route-and-model-tables-from-manifests` in wave 187. |
| 11 | llm-isolation | X16 Repair-cycle path | a | The console builds `selfheal-repair-cycle-path`. |
| 11 | llm-isolation | X19b Neutral sandbox intents and personas | a | The console builds `cli-neutral-profiles-and-personas`. |
| 11 | llm-isolation | X19c Capability negotiation in the router | a | The console builds `cli-capability-negotiation`. |
| 12 | comment-reduction | 3d production code package by package with | d | Triage of 2026-11-21 dropped the draft `comment-reduction-remaining-lines` (scratchpad `plans/triage-52.tsv`): it is a comment change with no pipeline effect. |
| 12 | comment-reduction | 2c turn kept whys into pinned invariants | d | Triage of 2026-11-21 dropped the draft `comment-reduction-2c-pin-kept-whys` (scratchpad `plans/triage-52.tsv`): it is a comment change with no pipeline effect. |
| 12 | comment-reduction | Loop history archive entry for comment removal | a | The console builds `loop-history-comment-removal-needs-an-archive-entry-lane`. |
| 13 | fleet-host-adaptive-window | Rollout steps 3 to 5 set thresholds | c | The operator flips `fleet.orchestrator.window.adaptive.stage` after a verified shadow record (section 10.1). |
| 14 | logging-and-process-hygiene | H1 review follow-ups | a | The console builds `log-retention-followups`. |
| 14 | logging-and-process-hygiene | K7 drop the duplicate pane capture | d | Plan section 10.6 keeps the copies on purpose. |
| 14 | logging-and-process-hygiene | H3 K2 selection keys and the writer | d | The operator withdrew or deleted the item `logging-hygiene-h3-writers-use-factory` earlier (scratchpad `plans/draft-class.tsv`). |
| 14 | logging-and-process-hygiene | H4 K12, K13, evolve logs and log_access | d | Triage of 2026-11-21 dropped the draft `logging-hygiene-h4-logs-reader` (scratchpad `plans/triage-52.tsv`): it is a large build with no measured failure behind it. |
| 14 | logging-and-process-hygiene | H5 enforce log_access | d | The operator withdrew or deleted the item `logging-hygiene-h5-enforce-log-access` earlier (scratchpad `plans/draft-class.tsv`). |
| 15 | convergence-policy | V6 | d | The operator withdrew or deleted the item `convergence-policy-v6-code-review-loop-decide` earlier (scratchpad `plans/draft-class.tsv`). |
| 15 | convergence-policy | V7 | d | The operator withdrew or deleted the item `convergence-policy-v7-finding-grammar-fields` earlier (scratchpad `plans/draft-class.tsv`). |
| 15 | convergence-policy | V8 | e | Dropped by the operator on 2026-11-22 (OD2). |
| 15 | convergence-policy | V9 | c | The operator flips `workflow.convergence.stage` after a verified shadow record (section 10.1). |
| 15 | convergence-policy | V10 | d | The operator withdrew or deleted the item `convergence-policy-v10-cycle-backward-edges` earlier (scratchpad `plans/draft-class.tsv`). |
| 15 | convergence-policy | V12 | d | The operator withdrew or deleted the item `convergence-policy-v12-ship-recovery-signals` earlier (scratchpad `plans/draft-class.tsv`). |
| 16 | guidance-layers | G1b path rules, action files, skill blocks | d | The operator withdrew or deleted the item `guidance-build-l2-outputs` earlier (scratchpad `plans/draft-class.tsv`). |
| 16 | guidance-layers | G3a phase marker | d | The operator withdrew or deleted the item `guidance-phase-marker` earlier (scratchpad `plans/draft-class.tsv`). |
| 16 | guidance-layers | G3 action and SessionStart hooks | d | The operator withdrew or deleted the item `guidance-action-and-session-hooks` earlier (scratchpad `plans/draft-class.tsv`). |
| 16 | guidance-layers | G5 migration, batch 2 | a | The console builds `guidance-migrate-batch-2-commit-boundary`. |
| 16 | guidance-layers | G5 migration, batch 3 | d | The operator withdrew or deleted the item `guidance-migrate-batch-3-rest` earlier (scratchpad `plans/draft-class.tsv`). |
| 16 | guidance-layers | G6 trigger evals | d | The operator withdrew or deleted the item `guidance-skill-trigger-evals` earlier (scratchpad `plans/draft-class.tsv`). |
| 16 | guidance-layers | G7 with-and-without eval of L0 | d | The operator withdrew or deleted the item `guidance-l0-with-without-eval` earlier (scratchpad `plans/draft-class.tsv`). |
| 16 | guidance-layers | G8 cell index reads the guidance index | d | The operator withdrew or deleted the item `guidance-cell-index-input` earlier (scratchpad `plans/draft-class.tsv`). |
| 16 | guidance-layers | G9 driver instruction loader | a | The console builds `guidance-driver-instruction-loader`. |
| 16 | guidance-layers | G10 AGENTS.md byte budget | d | The operator withdrew or deleted the item `guidance-agents-md-byte-budget` earlier (scratchpad `plans/draft-class.tsv`). |
| 17 | code-index-cli | K4a persona text for the four commands | a | A lane builds `code-index-cli-k4a-persona-commands` in wave 187. |
| 17 | code-index-cli | K4b gate for K4a | d | The operator withdrew or deleted the item `code-index-cli-k4b-gate-persona-change` earlier (scratchpad `plans/draft-class.tsv`). |
| 18 | latency-gates | T5 evolve ci latency check | d | The operator withdrew or deleted the item `latency-gates-t5-ci-trend-gate` earlier (scratchpad `plans/draft-class.tsv`). |
| 18 | latency-gates | T7 boundary runs T5 and T6 | e | Filed on 2026-11-22 as item `boundary-step-ci-latency-check-calls-a-missing-verb` (OD2). |
| 19 | wave-latency | W5 shared verdict store | c | The operator flips `gates.scan_cache_gate` after a verified shadow record (section 10.1). |
| 20 | phase-handoff-contract | H10 independent judge | b | Merge PR #1643 at the boundary. The boundary consumes `handoff-h10-independent-judge`. |
| 20 | phase-handoff-contract | H11 audit chain reads the ledgers | a | The console builds `handoff-h11-audit-chain-ledgers`. |
| 20 | phase-handoff-contract | H12 enforce by pair | c | The operator flips `handoff.pairs.<pair>.stage` after a verified shadow record (section 10.1). |
| 21 | llm-quality-indexes | Q7 escape-and-false-pass-links | d | The operator withdrew or deleted the item `escape-and-false-pass-links` earlier (scratchpad `plans/draft-class.tsv`). |
| 21 | llm-quality-indexes | Q8 judge-calibration-seeded-defects | d | The operator withdrew or deleted the item `judge-calibration-seeded-defects` earlier (scratchpad `plans/draft-class.tsv`). |
| 21 | llm-quality-indexes | Q9 pass-k-replay-harness | d | The operator withdrew or deleted the item `pass-k-replay-harness` earlier (scratchpad `plans/draft-class.tsv`). |
| 21 | llm-quality-indexes | Q10 held-out-acceptance-share | a | A lane builds `held-out-acceptance-share` in wave 187. |
| 21 | llm-quality-indexes | Q11 quality-health-panel | a | `quality-health-panel` is held by a running cycle (2604)); read its seal first. |
| 22 | cover-changed-gate | Commit-gate Go lane | a | A lane builds `commit-gate-go-lane-runs-cover-changed` in wave 188. |
| 22 | cover-changed-gate | Section 8 condition M4 one test run | e | Dropped by the operator on 2026-11-22 (OD2). |
| 23 | wave-integration | I9 move to enforce | c | The operator flips `fleet.integration.stage` after a verified shadow record (section 10.1). |
| 23 | wave-integration | I11 plane main sync and feeds | a | The console builds `wave-integration-i11-plane-main-feeds`. A lane builds `integration-sync-to-origin-uses-the-mainsync-fetch` in wave 188. |
| 23 | wave-integration | I14 integration driver | c | The operator flips `fleet.integration.stage` after a verified shadow record (section 10.1). |
| 23 | wave-integration | Branch protection in two steps | a | The console builds `main-branch-protection-in-two-steps`. |
| 24 | code-index-backends | G4 default route callers to gotypes | a | A lane builds `code-index-backends-g4-default-route-gotypes` in wave 188. |
| 25 | console-build-or-route | K4 boundary reroute of console routes | d | Triage of 2026-11-21 dropped the draft `console-build-or-route-k4-reroute-record` (scratchpad `plans/triage-52.tsv`): it is a task of the operator at the boundary, not a code item. |
| 25 | console-build-or-route | K5 split of the cmd/evolve admission cell | a | The console builds `cmd-evolve-cell-is-too-coarse-for-8-lanes`. |
| 26 | inbox-classifier-room | Tier B ADR and held-proposal admission | e | Dropped by the operator on 2026-11-22 (OD2). |
| 27 | workspace-hygiene | S5 batch-end hook, shadow default, operator surface | a | `recycle-x4-mandatory-recycle-step` waits for `recycle-x5-continuation-reads-the-leaf` and `recycle-x1a-check-registry-worktrees`. `recycle-x9-loop-runs-host-checks` waits for `recycle-x4-mandatory-recycle-step` and `recycle-x1b-check-registry-run-dirs` and `recycle-x1c-check-registry-cache`. |
| 28 | admission-cmd-evolve-cell | K2 self-registering verbs | a | A lane builds `cmd-evolve-self-registering-verbs` in wave 188. |
| 29 | gate-ship-retry-waste | S4 binding check first and composition receipt | a | A lane builds `gate-ship-s4-binding-first-receipt` in wave 189. |
| 29 | gate-ship-retry-waste | S5 explanation-doc fix in place | a | `gate-ship-s5-explanation-fix-in-place` waits for `gate-ship-s4-binding-first-receipt`. |
| 30 | event-notification-protocol | E7 events config, gc categories | a | The console builds `event-notification-protocol-e7-events-config-load`. |
| 30 | event-notification-protocol | E8 subscriptions runner | a | The console builds `event-notification-protocol-e8-subscriptions-runner`. |
| 30 | event-notification-protocol | E9, E13 | d | Triage of 2026-11-21 dropped the draft `event-protocol-e9-e13-e14-refile` (scratchpad `plans/triage-52.tsv`): E9 and E13 have no measured need; E14 joins the E12 item. |
| 30 | event-notification-protocol | E12, E14 | a | A lane builds `event-notification-protocol-e12-poller-migrations` in wave 189. |
| 31 | cost-aware-repair | 5 vacuous-test-guard | a | A lane builds `empty-test-body-guard-in-the-build-floor` in wave 189. |
| 31 | cost-aware-repair | 7 audit-reviews-the-final-diff | a | The console builds `audit-reviews-the-final-diff`. |
| 31 | cost-aware-repair | 11 wire-cost-aware-repair-call-sites | a | The console builds `wire-cost-aware-repair-call-sites`. |
| 32 | finder-fix-ownership | F1 auditor writes a fix spec | a | The console builds `finder-fix-ownership-f1-auditor-writes-fix-spec`. |
| 32 | finder-fix-ownership | F3 R3 verifier | a | The console builds `finder-fix-ownership-f3-r3-verifier`. |
| 32 | finder-fix-ownership | F4 ledger authorship | d | The operator withdrew or deleted the item `finder-fix-ownership-f4-ledger-authorship` earlier (scratchpad `plans/draft-class.tsv`). |
| 32 | finder-fix-ownership | F5 deterministic fast path | d | The operator withdrew or deleted the item `finder-fix-ownership-f5-deterministic-fast-path` earlier (scratchpad `plans/draft-class.tsv`). |
| 33 | gate-contract | G2 hermetic gate | a | A lane builds `gate-contract-g2-hermetic-guard` in wave 189. |
| 33 | gate-contract | G3 parity harness | d | The operator withdrew or deleted the item `gate-contract-g3-three-state-parity` earlier (scratchpad `plans/draft-class.tsv`). |
| 33 | gate-contract | G5 one TDD floor | d | Triage of 2026-11-21 dropped the draft `gate-contract-g5-one-tdd-floor-rest` (scratchpad `plans/triage-52.tsv`): the helper-copy check is on main (#1425, #1467, #1601) and no failure is cited. |
| 33 | gate-contract | G6 key baseline | a | A lane builds `gate-contract-g6-key-baseline` in wave 189. |
| 34 | ledger-restructure | C6 | d | The operator withdrew or deleted the item `ledger-restructure-c6-collapse-phase-skipped` earlier (scratchpad `plans/draft-class.tsv`). |
| 34 | ledger-restructure | C7 | d | The operator withdrew or deleted the item `ledger-restructure-c7-store-artifacts-at-append` earlier (scratchpad `plans/draft-class.tsv`). |
| 34 | ledger-restructure | C8 | a | The console builds `ledger-restructure-c8-ledgerscan-one-reader`. |
| 34 | ledger-restructure | C9 | a | `ledger-restructure-c9-agent-query-verbs` waits for `ledger-restructure-c8-ledgerscan-one-reader`. |
| 34 | ledger-restructure | C10 | a | `ledger-restructure-c10-segment-aware-scan` waits for `ledger-restructure-c8-ledgerscan-one-reader`. |
| 34 | ledger-restructure | C11 | b | Merge PR #1665 at the boundary. The boundary consumes `ledger-restructure-c11-seal-manifest`. |
| 34 | ledger-restructure | C12 | a | The console builds `ledger-restructure-c12-incremental-verify`. |
| 34 | ledger-restructure | C13 | d | The operator withdrew or deleted the item `ledger-restructure-c13-boundary-seal` earlier (scratchpad `plans/draft-class.tsv`). |
| 35 | bridge-factory | F2 deep probe through the driver | e | Dropped by the operator on 2026-11-22 (OD2). |
| 35 | bridge-factory | F3 model lister in the bridge | d | The operator withdrew or deleted the item `bridge-factory-f3-model-lister-in-bridge` earlier (scratchpad `plans/draft-class.tsv`). |
| 35 | bridge-factory | F4 one bridge.Version | d | The operator withdrew or deleted the item `bridge-factory-f4-one-version-call` earlier (scratchpad `plans/draft-class.tsv`). |
| 35 | bridge-factory | F5 effort and picker in the manifest | a | The console builds `bridge-factory-f5-effort-and-picker-in-manifest`. |
| 35 | bridge-factory | F6 internal/modelrefresh package | d | The operator withdrew or deleted the item `bridge-factory-f6-modelrefresh-package` earlier (scratchpad `plans/draft-class.tsv`). |
| 36 | agent-messaging | M5 deliverers | a | A lane builds `agent-messaging-m5-deliverers` after wave 189. |
| 36 | agent-messaging | M6 sandbox rules | a | The console builds `agent-messaging-m6-sandbox-rules`. |
| 36 | agent-messaging | M7 cycle wiring | a | The console builds `agent-messaging-m7-cycle-wiring`. |
| 36 | agent-messaging | M8 live wave check | d | The operator withdrew or deleted the item `agent-messaging-m8-live-wave-check` earlier (scratchpad `plans/draft-class.tsv`). |
| 37 | harness-practices | R2 guard-denial-ends-the-dispatch-chain | a | The console builds `guard-denial-ends-the-dispatch-chain`. |
| 37 | harness-practices | R5 journey-registry-and-trailer | b | Merge PR #1663 at the boundary. The boundary consumes `journey-registry-and-trailer`. |
| 37 | harness-practices | R6 regression-proof-gate | a | The console builds `regression-proof-gate`. |
| 37 | harness-practices | R8 enforce-evidence-for-integration | a | The console builds `enforce-evidence-for-integration`. |
| 38 | seat-login-isolation | L2, L3, L7 | a | The console builds `sandboxed-seats-cannot-save-a-refreshed-login`. |
| 38 | seat-login-isolation | L8 count auth errors by place | d | Triage of 2026-11-21 dropped the draft `seat-login-l8-auth-errors-by-place` (scratchpad `plans/triage-52.tsv`): it is a read-only diagnostic, and the open login item covers the incidents. |
| 39 | code-review-phase | Q3 | a | The console builds `code-review-phase-q3-findings-driven-repair`. |
| 39 | code-review-phase | Q4 | a | The console builds `code-review-phase-q4-delta-re-review`. |
| 39 | code-review-phase | Q5 | a | The console builds `code-review-phase-q5-audit-on-the-index`. |
| 39 | code-review-phase | Q6 | a | A lane builds `code-review-phase-q6-capacity-skip-metrics` after wave 189. |
| 39 | code-review-phase | R8b | c | The operator flips `workflow.findings_repair.code-review.stage` after a verified shadow record (section 10.1). |
| 40 | test-quality-audit | T6 tautology fix batches | a | A lane builds `test-audit-t6c-tautology-cmd` after wave 189. The console builds `test-audit-t6d-tautology-rest-ac-1b-console`. |
| 40 | test-quality-audit | T8 acs/ predicate batches | a | A lane builds `test-audit-t8a-acs-predicates-early` after wave 189. A lane builds `test-audit-t8b-acs-predicates-mid` after wave 189. A lane builds `test-audit-t8c-acs-predicates-late` after wave 189. |
| 40 | test-quality-audit | T10 fix-item admission checks | a | A lane builds `test-audit-t10-fix-item-checks` after wave 189. |
| 40 | test-quality-audit | T9 apicover_named_test.go files | e | Dropped by the operator on 2026-11-22 (OD2). |
| 41 | narrow-fence | H3 Operator verb, list and wave status | a | A lane builds `held-proposal-inbox-show-line` after wave 189. |
| 41 | narrow-fence | H4 Extra adversarial audit dispatch | a | The console builds `held-proposal-adversarial-audit-dispatch`. |
| 41 | narrow-fence | H5 Admission of a held item to | a | The console builds `held-proposal-lane-admission`. |
| 42 | architecture-decisions | AD1-3 cell tool view | a | The console builds `arch-decisions-ad1-3-the-cell-tool-view-gh-config-dir-git`. |
| 42 | architecture-decisions | AD1-5 secret stage | a | The console builds `arch-decisions-ad1-5-a-separate-stage-for-the-secret`. |
| 42 | architecture-decisions | AD1-6 wider secret list | a | The console builds `arch-decisions-ad1-6-a-wider-secret-list-git-credentials`. |
| 42 | architecture-decisions | AD1-7 CredentialSource | a | The console builds `arch-decisions-ad1-7-the-credentialsource-interface-and`. |
| 42 | architecture-decisions | AD1-8 sandbox layer per OS | a | The console builds `arch-decisions-ad1-8-sandbox-layer-per-os`. |
| 42 | architecture-decisions | AD1-10 $HOME allowlist | a | `arch-decisions-ad1-10-queued-next-e-the-home-allowlist` waits for `arch-decisions-ad1-5-a-separate-stage-for-the-secret`. |
| 42 | architecture-decisions | AD2-5 curator enforce | c | The operator flips `inbox.curator.stage` after a verified shadow record (section 10.1). |
| 42 | architecture-decisions | AD2-9 inbox audit | a | The console builds `arch-decisions-ad2-9-graft-d-an-on-demand-llm-curator`. |
| 42 | architecture-decisions | AD3-2 directory routing rule | a | A lane builds `arch-decisions-ad3-2-the-directory-routing-rule-a` after wave 189. |
| 42 | architecture-decisions | AD3-3 agreement rate in the Q8 item | d | The operator withdrew or deleted the item `arch-decisions-ad3-3-judge-agreement-note` earlier (scratchpad `plans/draft-class.tsv`). |
| 42 | architecture-decisions | AD3-4 successor item for the audit verdict | d | The operator withdrew or deleted the item `arch-decisions-ad3-4-audit-verdict-as-an-i8-seat` earlier (scratchpad `plans/draft-class.tsv`). |
| 42 | architecture-decisions | AD4-3 merge #1070 and run the ratchet | e | Dropped by the operator on 2026-11-22 (OD2). |
| 42 | architecture-decisions | AD4-4 ratchet step merges its own PR | a | The console builds `arch-decisions-ad4-4-the-ratchet-step-merges-its-own-pr`. |
| 42 | architecture-decisions | AD4-5 ratchet docs and enforce | e | Dropped by the operator on 2026-11-22 (OD2). |
| 43 | shadow-stage-evidence | K1 registry and baseline | a | The console builds `shadow-contract-k1b-stage-actor-ports`. |
| 43 | shadow-stage-evidence | K2 evidence recorder and seal | a | The console builds `shadow-contract-k2-evidence-seal`. |
| 43 | shadow-stage-evidence | K4 protected contract and ratchet | a | The console builds `shadow-contract-k4-protected-contract`. |
| 43 | shadow-stage-evidence | K4b migration | a | The console builds `shadow-contract-k4-protected-contract`. |
| 43 | shadow-stage-evidence | K3 stage verify and report | new (OD1) | The console files `shadow-contract-k3-stage-verify`, then builds it after K2. It is not counted in section 6.2. |
| 43 | shadow-stage-evidence | K5 flip verb and preflight | a | The console builds `shadow-contract-k5-stage-flip`. |
| 43 | shadow-stage-evidence | F3 main-red stop for step keys | d | The boundary edit list of 2026-11-21 withdrew the item `shadow-contract-f3-main-red-step-keys`: it conflicts with design D3 of `main-red-stop-provable` (#1641). |
| 43 | shadow-stage-evidence | F7 spend limits | a | The console builds `shadow-contract-f7-spend-limit-retune`. |
| 44 | incident-knowledge-base | I1 template and procedure | b | Merge PR #1636 at the boundary. The boundary consumes `incident-kb-i1-template-and-skill`. |
| 44 | incident-knowledge-base | I4 incidentcapture, capture sweep | a | A lane builds `incident-kb-i4-capture-sweep` after wave 189. |
| 44 | incident-knowledge-base | I5 levels, INCIDENT_RECORD_DUE | a | `incident-kb-i5-levels-and-due` waits for `incident-kb-i4-capture-sweep`. |
| 44 | incident-knowledge-base | I11 blocked-state rules | a | `incident-kb-i11-blocked-state-rules` waits for `incident-kb-i5-levels-and-due`. |
| 44 | incident-knowledge-base | I6 three stale detectors | a | `incident-kb-i6-stale-detectors` waits for `incident-kb-i5-levels-and-due`. |
| 44 | incident-knowledge-base | I7 record_doc and refused close | b | Merge PR #1624 at the boundary. The boundary consumes `incident-kb-i7-record-doc`. |
| 44 | incident-knowledge-base | I9 verify, due, boundary steps | a | `incident-kb-i9-verify-and-due` waits for `incident-kb-i4-capture-sweep`. |
| 44 | incident-knowledge-base | I10 gate wiring proofs | a | The console builds `incident-kb-i10-gate-wiring`. |
| 45 | queue-authority-from-host-facts | K2, K3 | e | Dropped by the operator on 2026-11-22 (OD2). |
| 45 | queue-authority-from-host-facts | K4 write failure and symlink refusal | d | Triage of 2026-11-21 dropped the draft `queue-authority-k4-record-write-failure-and-symlink` (scratchpad `plans/triage-52.tsv`): it is hardening with no measured failure. |
| 45 | queue-authority-from-host-facts | T1 route at triage | e | Dropped by the operator on 2026-11-22 (OD2). |
| 45 | queue-authority-from-host-facts | T2 typed reason | d | Triage of 2026-11-21 dropped the draft `queue-authority-t2-typed-console-route-reason` (scratchpad `plans/triage-52.tsv`): it is type hardening against a theoretical injection with no failure cited. |
| 45 | queue-authority-from-host-facts | B1 breaker | a | A lane builds `queue-authority-b1-breaker-charges-prebuild-failures` after wave 189. |
| 45 | queue-authority-from-host-facts | B2 brake | a | `queue-authority-b2-prebuild-brake` waits for `queue-authority-b1-breaker-charges-prebuild-failures`. |
| 46 | pipeline-blockers | B4 ship check reads the lane tree | e | Dropped by the operator on 2026-11-22 (OD2). |
| 46 | pipeline-blockers | B6 phase cell runs the host evolve | b | Merge PR #1668 at the boundary. The boundary consumes `phase-cell-has-no-evolve-on-path`. |
| 46 | pipeline-blockers | B8 class from the writer, one go | d | Triage of 2026-11-21 dropped the draft `pipeline-blockers-b8-class-from-the-writer-check` (scratchpad `plans/triage-52.tsv`): `ci classify` reads the test JSON on main and no failure is cited. |
| 46 | pipeline-blockers | B9 one INFO signal for each launch | a | A lane builds `signal-severity-and-noise-cleanup` after wave 189. |
| 46 | pipeline-blockers | R1 one block count per subject | a | A lane builds `pipeline-blockers-r1-one-block-count` after wave 189. |
| 46 | pipeline-blockers | R2 convergence.Decide returns ActionRedesign | a | A lane builds `pipeline-blockers-r1-one-block-count` after wave 189. |
| 46 | pipeline-blockers | R3 audit repair, code review and convergence | a | `pipeline-blockers-r3-entries-act-on-redesign` waits for `pipeline-blockers-r1-one-block-count`. |
| 46 | pipeline-blockers | R4 failure events feed the count; an | a | `pipeline-blockers-r4-failure-events-feed-the-count` waits for `pipeline-blockers-r1-one-block-count`. |
| 46 | pipeline-blockers | R5 a redesign subject leaves the lane | a | `pipeline-blockers-r4-failure-events-feed-the-count` waits for `pipeline-blockers-r1-one-block-count`. |
| 46 | pipeline-blockers | R6 six-part text of the convergence signals | a | `pipeline-blockers-r3-entries-act-on-redesign` waits for `pipeline-blockers-r1-one-block-count`. |
| 46 | pipeline-blockers | P2 equality helper for signal text | a | `pipeline-blockers-r3-entries-act-on-redesign` waits for `pipeline-blockers-r1-one-block-count`. |
| 46 | pipeline-blockers | P3 debug verbs | a | A lane builds `debug-cli-c6-c13-verbs` after wave 189. |
| 47 | review-routing | S0 console commits carry review trailers | a | `review-routing-s0-console-commits-carry-review-trailers` waits for `review-routing-s1-reviewclass-registry`. |
| 47 | review-routing | S1, S8 | a | A lane builds `review-routing-s1-reviewclass-registry` after wave 189. |
| 47 | review-routing | S2 deterministic checks | d | Triage of 2026-11-21 dropped the draft `review-routing-s2-deterministic-checks` (scratchpad `plans/triage-52.tsv`): no measured need exists before the S1 and S4 shadow evidence. |
| 47 | review-routing | S3, S4, S5, S7 | a | `review-routing-s4-commit-gate-reads-requirements` waits for `review-routing-s1-reviewclass-registry`. |
| 47 | review-routing | S6 diff variants of the security and | d | Triage of 2026-11-21 dropped the draft `review-routing-s6-diff-security-adversarial-seats` (scratchpad `plans/triage-52.tsv`): no measured need exists. |
| 47 | review-routing | S9 stage flip | d | Triage of 2026-11-21 dropped the draft `review-routing-s9-flip-stage-to-enforce` (scratchpad `plans/triage-52.tsv`): it is a rollout step and no shadow evidence exists yet. |
| 47 | review-routing | S10 family routing | d | Triage of 2026-11-21 dropped the draft `review-routing-s10-family-routing-adversarial-security` (scratchpad `plans/triage-52.tsv`): it waits for the S1 and S4 shadow evidence. |
| 47 | review-routing | S11 diff-scoped mutation runner | d | Triage of 2026-11-21 dropped the draft `review-routing-s11-diff-scoped-guard-mutation` (scratchpad `plans/triage-52.tsv`): it waits for the S1 and S4 shadow evidence. |
| 48 | debug-cli | C5 Gate step and end message | d | Triage of 2026-11-21 dropped the draft `debug-cli-c5-ci-gate-step` (scratchpad `plans/triage-52.tsv`): the gate row is on main, and the row prints the next command. |
| 48 | debug-cli | C6, C7, C8, C9, C10, C11, C12, C13 | a | A lane builds `debug-cli-c6-c13-verbs` after wave 189. |
| 49 | concurrent-edit-layout | C3 serial tests declare in their own | a | The console builds `edit-layout-c3-serial-tests-declare-in-place`. |
| 49 | concurrent-edit-layout | C4 verbs self-register | a | A lane builds `cmd-evolve-self-registering-verbs` in wave 188. |
| 49 | concurrent-edit-layout | C5 gate lists as fragments | a | A lane builds `edit-layout-c5-gate-lists-as-fragments` after wave 189. |
| 49 | concurrent-edit-layout | C6, C7 | a | A lane builds `split-cmd-evolve-gccmd` after wave 189. `split-cmd-evolve-wavecmd` waits for `split-cmd-evolve-gccmd`. `split-cmd-evolve-landcmd-rest` waits for `split-cmd-evolve-wavecmd`. `split-cmd-evolve-spine-design` waits for `split-cmd-evolve-landcmd-rest`. |
| 50 | split-cmd-evolve | Step 3 gccmd | a | A lane builds `split-cmd-evolve-gccmd` after wave 189. |
| 50 | split-cmd-evolve | Step 4 wavecmd | a | `split-cmd-evolve-wavecmd` waits for `split-cmd-evolve-gccmd`. |
| 50 | split-cmd-evolve | Step 5 landcmd rest | a | `split-cmd-evolve-landcmd-rest` waits for `split-cmd-evolve-wavecmd`. |
| 50 | split-cmd-evolve | Step 6 spine design | a | `split-cmd-evolve-spine-design` waits for `split-cmd-evolve-landcmd-rest`. |
| 51 | audit-evaluation-contract | E1 | a | The console builds `audit-evaluation-contract-e1-standard`. |
| 51 | audit-evaluation-contract | E2 | a | The console builds `audit-evaluation-contract-e2-audit-plan-phase`. |
| 51 | audit-evaluation-contract | E3 | a | The console builds `audit-evaluation-contract-e3-expectations-delivery`. |
| 51 | audit-evaluation-contract | E4 | a | The console builds `code-review-phase-q5-audit-on-the-index`. |
| 51 | audit-evaluation-contract | E5 | a | The console builds `code-review-phase-q4-delta-re-review`. |
| 51 | audit-evaluation-contract | E6 | a | `audit-evaluation-contract-e6-learning-signals` waits for `code-review-phase-q5-audit-on-the-index`. |
| 51 | audit-evaluation-contract | E7 | d | The operator withdrew or deleted the item `audit-evaluation-contract-e7-persona-duty-and-flip` earlier (scratchpad `plans/draft-class.tsv`). |
| 52 | cycle-recycle-and-gc-check | X1a check registry worktrees | a | `recycle-x1a-check-registry-worktrees` is held by a running cycle (2601)); read its seal first. |
| 52 | cycle-recycle-and-gc-check | X1b check registry run dirs | a | `recycle-x1b-check-registry-run-dirs` waits for `recycle-x1a-check-registry-worktrees`. |
| 52 | cycle-recycle-and-gc-check | X1c check registry cache | a | `recycle-x1c-check-registry-cache` waits for `recycle-x1a-check-registry-worktrees` and `recycle-x2-gocache-counts-executable-dirs`. |
| 52 | cycle-recycle-and-gc-check | X1d check registry temp | a | `recycle-x1d-check-registry-temp` waits for `recycle-x1a-check-registry-worktrees`. |
| 52 | cycle-recycle-and-gc-check | X1e check registry processes | a | `recycle-x1e-check-registry-processes` waits for `recycle-x1a-check-registry-worktrees`. |
| 52 | cycle-recycle-and-gc-check | X2 trim counts executable dirs | a | A lane builds `recycle-x2-gocache-counts-executable-dirs` after wave 189. |
| 52 | cycle-recycle-and-gc-check | X3 leaf writer and evolve archive restore | a | A lane builds `recycle-x3-archive-leaf-writer` after wave 189. |
| 52 | cycle-recycle-and-gc-check | X4 mandatory recycle step | a | `recycle-x4-mandatory-recycle-step` waits for `recycle-x5-continuation-reads-the-leaf` and `recycle-x1a-check-registry-worktrees`. |
| 52 | cycle-recycle-and-gc-check | X5 continuation reads the leaf | a | `recycle-x5-continuation-reads-the-leaf` waits for `recycle-x3-archive-leaf-writer`. |
| 52 | cycle-recycle-and-gc-check | X6 size cap, migration, retention | a | `recycle-x6-salvage-retention-and-migration` waits for `recycle-x3-archive-leaf-writer`. |
| 52 | cycle-recycle-and-gc-check | X7 temp shape registry | a | `recycle-x7-temp-shape-registry` waits for `recycle-x8-test-tmux-socket-reap` and `recycle-x1d-check-registry-temp`. |
| 52 | cycle-recycle-and-gc-check | X8 test tmux socket reap | a | A lane builds `recycle-x8-test-tmux-socket-reap` after wave 189. |
| 52 | cycle-recycle-and-gc-check | X9 loop runs host checks | a | `recycle-x9-loop-runs-host-checks` waits for `recycle-x4-mandatory-recycle-step` and `recycle-x1b-check-registry-run-dirs` and `recycle-x1c-check-registry-cache`. |
| 52 | cycle-recycle-and-gc-check | X10 reports and integrity rows | a | `recycle-x10-reports-and-integrity-rows` waits for `recycle-x1a-check-registry-worktrees`. |

### 6.4 Plans that close at the boundary

These 31 plans have no open piece after the boundary. Set the Status of each plan to `built` in one docs change. A plan with kind b rows closes when its pull request merges.

| Plan | Components | Kind b | Kind d |
|---|---|---|---|
| [agent-request-inbox-add-2026-11](agent-request-inbox-add-2026-11.md) | 5 | 2 | 1 |
| [inbox-prioritization-2026-10](inbox-prioritization-2026-10.md) | 7 | 1 | 0 |
| [model-tier-policy-2026-10](model-tier-policy-2026-10.md) | 9 | 1 | 2 |
| [agent-run-standalone-2026-10](agent-run-standalone-2026-10.md) | 5 | 0 | 0 |
| [attempt-postmortem-2026-10](attempt-postmortem-2026-10.md) | 8 | 0 | 1 |
| [bridge-floor-coverage-2026-11](bridge-floor-coverage-2026-11.md) | 3 | 0 | 0 |
| [build-blocker-exit-2026-10](build-blocker-exit-2026-10.md) | 7 | 0 | 0 |
| [cell-context-index-2026-10](cell-context-index-2026-10.md) | 8 | 0 | 5 |
| [cli-routing-table-2026-10](cli-routing-table-2026-10.md) | 7 | 0 | 3 |
| [concurrent-cycle-landing-2026-10](concurrent-cycle-landing-2026-10.md) | 20 | 0 | 4 |
| [data-file-test-floor-2026-11](data-file-test-floor-2026-11.md) | 8 | 0 | 4 |
| [deep-tier-share-2026-10](deep-tier-share-2026-10.md) | 6 | 0 | 0 |
| [docs-write-skill-2026-10](docs-write-skill-2026-10.md) | 5 | 0 | 0 |
| [evolve-wave-cli-2026-10](evolve-wave-cli-2026-10.md) | 8 | 0 | 0 |
| [fable-simulation-2026-07](fable-simulation-2026-07.md) | 7 | 0 | 0 |
| [harness-removal-test-2026-11](harness-removal-test-2026-11.md) | 4 | 0 | 3 |
| [issue-record-skill-2026-11](issue-record-skill-2026-11.md) | 6 | 0 | 0 |
| [model-capability-practice-2026-10](model-capability-practice-2026-10.md) | 6 | 0 | 2 |
| [model-currency-2026-10](model-currency-2026-10.md) | 6 | 0 | 0 |
| [model-level-benches-2026-10](model-level-benches-2026-10.md) | 6 | 0 | 1 |
| [pipeline-health-indexes-2026-11](pipeline-health-indexes-2026-11.md) | 8 | 0 | 0 |
| [pool-resupply-2026-10](pool-resupply-2026-10.md) | 3 | 0 | 0 |
| [pr-merge-latency-2026-10](pr-merge-latency-2026-10.md) | 8 | 0 | 1 |
| [recovery-retrospective-2026-10](recovery-retrospective-2026-10.md) | 6 | 0 | 0 |
| [release-keeps-site-current-2026-11](release-keeps-site-current-2026-11.md) | 4 | 0 | 0 |
| [skills-drift-host-stamp-2026-10](skills-drift-host-stamp-2026-10.md) | 9 | 0 | 1 |
| [ste100-docs-rewrite-2026-10](ste100-docs-rewrite-2026-10.md) | 14 | 0 | 9 |
| [test-system-efficiency-2026-10](test-system-efficiency-2026-10.md) | 18 | 0 | 1 |
| [train-commit-chain-attestation-2026-11](train-commit-chain-attestation-2026-11.md) | 4 | 0 | 0 |
| [train-red-batch-attribution-2026-11](train-red-batch-attribution-2026-11.md) | 5 | 0 | 0 |
| [wave-orchestrator-2026-10](wave-orchestrator-2026-10.md) | 12 | 0 | 1 |

## 7. Order: wave sets and console order

### 7.1 The wave sets

The waves have 5 lanes each. A set holds lane items only. An item with an open dependency waits for a later wave. Wave 187 holds no item that depends on a pull request of the boundary.

| Wave | Items (queue rank of the plan) |
|---|---|
| 187 | `inbox-claims-delta-followups` (3), `tokenopt-session-resume-on-retry` (8), `isolation-x9b-route-and-model-tables-from-manifests` (11), `code-index-cli-k4a-persona-commands` (17), `held-out-acceptance-share` (21) |
| 188 | `tokens-report-from-llm-calls` (10), `commit-gate-go-lane-runs-cover-changed` (22), `integration-sync-to-origin-uses-the-mainsync-fetch` (23), `code-index-backends-g4-default-route-gotypes` (24), `cmd-evolve-self-registering-verbs` (28) |
| 189 | `gate-ship-s4-binding-first-receipt` (29), `event-notification-protocol-e12-poller-migrations` (30), `empty-test-body-guard-in-the-build-floor` (31), `gate-contract-g2-hermetic-guard` (33), `gate-contract-g6-key-baseline` (33) |

The three commands:

```
evolve wave next --only inbox-claims-delta-followups,tokenopt-session-resume-on-retry,isolation-x9b-route-and-model-tables-from-manifests,code-index-cli-k4a-persona-commands,held-out-acceptance-share
evolve wave next --only tokens-report-from-llm-calls,commit-gate-go-lane-runs-cover-changed,integration-sync-to-origin-uses-the-mainsync-fetch,code-index-backends-g4-default-route-gotypes,cmd-evolve-self-registering-verbs
evolve wave next --only gate-ship-s4-binding-first-receipt,event-notification-protocol-e12-poller-migrations,empty-test-body-guard-in-the-build-floor,gate-contract-g2-hermetic-guard,gate-contract-g6-key-baseline
```

Before the command of wave 188 or 189, run `evolve wave next --only <ids> --dry-run`. An id that no lane can take exits 10.

Items of the head plans that no wave can take now:

- Rank 1 `ci-wall-time` and rank 5 `dispatch-process-hygiene` hold console items only.
- Rank 2 `fleet-landing-at-scale` and rank 4 `retry-ladder` hold a switch only.
- Rank 6 `gc-closes-the-legacy-gap` waits for the recycle items of rank 52, and a running cycle holds `recycle-x1a-check-registry-worktrees`.

### 7.2 The console order

The console takes the console items in the order of the queue. The column Waits on names the open dependencies.

| Rank | Item | Class | Weight | Waits on |
|---|---|---|---|---|
| 1 | `ci-wall-time-c13-macos-cpu-and-spawn-counts` | performance | 0.4 | none |
| 5 | `dispatch-hygiene-followups` | security | 0.3 | none |
| 8 | `tokenopt-cache-discipline-audit` | feature | 0.35 | none |
| 11 | `selfheal-repair-cycle-path` | stability | 0.32 | none |
| 11 | `cli-neutral-profiles-and-personas` | maintainability | 0.5 | none |
| 11 | `cli-capability-negotiation` | stability | 0.4 | none |
| 12 | `loop-history-comment-removal-needs-an-archive-entry-lane` | stability | 0.45 | none |
| 14 | `log-retention-followups` | maintainability | 0.3 | none |
| 16 | `guidance-migrate-batch-2-commit-boundary` | stability | 0.35 | none |
| 16 | `guidance-driver-instruction-loader` | stability | 0.35 | none |
| 20 | `handoff-h11-audit-chain-ledgers` | correctness | 1.0 | none |
| 23 | `wave-integration-i11-plane-main-feeds` | feature | 0.5 | `wave-integration-i5-batch-pr` |
| 23 | `main-branch-protection-in-two-steps` | stability | 0.7 | none |
| 23 | `wave-integration-i0-hardening-before-enforce` | stability | 0.7 | none |
| 25 | `cmd-evolve-cell-is-too-coarse-for-8-lanes` | performance | 0.85 | none |
| 30 | `event-notification-protocol-e7-events-config-load` | correctness | 0.4 | none |
| 30 | `event-notification-protocol-e8-subscriptions-runner` | feature | 0.4 | `event-notification-protocol-e7-events-config-load` |
| 31 | `audit-reviews-the-final-diff` | correctness | 0.75 | none |
| 31 | `wire-cost-aware-repair-call-sites` | performance | 0.78 | none |
| 32 | `finder-fix-ownership-f1-auditor-writes-fix-spec` | correctness | 0.5 | none |
| 32 | `finder-fix-ownership-f3-r3-verifier` | correctness | 0.6 | `finder-fix-ownership-f1-auditor-writes-fix-spec` |
| 34 | `ledger-restructure-c8-ledgerscan-one-reader` | performance | 0.4 | none |
| 34 | `ledger-restructure-c12-incremental-verify` | performance | 0.4 | none |
| 35 | `bridge-factory-f5-effort-and-picker-in-manifest` | hygiene | 0.35 | none |
| 36 | `agent-messaging-m6-sandbox-rules` | security | 0.5 | none |
| 36 | `agent-messaging-m7-cycle-wiring` | feature | 0.4 | `agent-messaging-m5-deliverers`, `agent-messaging-m6-sandbox-rules` |
| 37 | `guard-denial-ends-the-dispatch-chain` | stability | 0.9 | none |
| 37 | `regression-proof-gate` | stability | 0.85 | none |
| 37 | `enforce-evidence-for-integration` | stability | 0.65 | none |
| 38 | `sandboxed-seats-cannot-save-a-refreshed-login` | stability | 0.95 | none |
| 39 | `code-review-phase-q3-findings-driven-repair` | correctness | 0.5 | none |
| 39 | `code-review-phase-q4-delta-re-review` | correctness | 0.5 | `code-review-phase-q3-findings-driven-repair`, `audit-evaluation-contract-e2-audit-plan-phase` |
| 39 | `code-review-phase-q5-audit-on-the-index` | correctness | 0.5 | `code-review-phase-q3-findings-driven-repair`, `audit-evaluation-contract-e1-standard`, `audit-evaluation-contract-e3-expectations-delivery` |
| 40 | `test-audit-t6d-tautology-rest-ac-1b-console` | correctness | 0.75 | none |
| 41 | `held-proposal-adversarial-audit-dispatch` | pipeline | 0.7 | none |
| 41 | `held-proposal-lane-admission` | pipeline | 0.6 | `held-proposal-adversarial-audit-dispatch` |
| 42 | `arch-decisions-ad1-3-the-cell-tool-view-gh-config-dir-git` | security | 0.8 | none |
| 42 | `arch-decisions-ad1-5-a-separate-stage-for-the-secret` | security | 0.8 | `arch-decisions-ad1-3-the-cell-tool-view-gh-config-dir-git` |
| 42 | `arch-decisions-ad1-6-a-wider-secret-list-git-credentials` | security | 0.8 | none |
| 42 | `arch-decisions-ad1-7-the-credentialsource-interface-and` | security | 0.8 | none |
| 42 | `arch-decisions-ad1-8-sandbox-layer-per-os` | security | 0.8 | none |
| 42 | `arch-decisions-ad2-9-graft-d-an-on-demand-llm-curator` | correctness | 0.8 | none |
| 42 | `arch-decisions-ad4-4-the-ratchet-step-merges-its-own-pr` | maintainability | 0.8 | none |
| 43 | `shadow-contract-k1b-stage-actor-ports` | correctness | 0.85 | none |
| 43 | `shadow-contract-k2-evidence-seal` | correctness | 0.85 | none |
| 43 | `shadow-contract-k4-protected-contract` | correctness | 0.85 | none |
| 43 | `shadow-contract-k3-stage-verify` (to be filed, OD1) | correctness | 0.85 | `shadow-contract-k2-evidence-seal` |
| 43 | `shadow-contract-k5-stage-flip` | correctness | 0.85 | `shadow-contract-k3-stage-verify`, `shadow-contract-k4-protected-contract` |
| 43 | `shadow-contract-f7-spend-limit-retune` | correctness | 0.85 | none |
| 44 | `incident-kb-i10-gate-wiring` | debuggability | 0.7 | none |
| 49 | `edit-layout-c3-serial-tests-declare-in-place` | performance | 0.65 | none |
| 51 | `audit-evaluation-contract-e1-standard` | correctness | 0.5 | none |
| 51 | `audit-evaluation-contract-e2-audit-plan-phase` | correctness | 0.5 | `audit-evaluation-contract-e1-standard` |
| 51 | `audit-evaluation-contract-e3-expectations-delivery` | correctness | 0.5 | `audit-evaluation-contract-e2-audit-plan-phase` |

## 8. Verification

| Check | How | Pass |
|---|---|---|
| The counts of section 2 add up. | Add the kinds: 491 + 45 + 108 + 138 + 12 + 10 + 14. | The sum is 818. |
| Each id of a wave set is a live lane item. | `evolve wave next --only <ids> --dry-run`. | The command exits 0. |
| No id of a wave set is a withdrawn item. | Compare the ids with `plans/draft-class.tsv`. | No id is in the file. |
| The queue moves. | After each boundary, rebuild the table of section 6.2. | At least one plan leaves the queue each 2 boundaries. |

The classification used scratch scripts in the scratchpad. A scratch script is not a lasting tool. The queue needs a tested `evolve` verb before the next rebuild (section 10.3).

## 9. Risks

| Risk | Control |
|---|---|
| The delivery tables are 1 day old, so an item can be consumed after the review. | Rebuild the queue at each boundary (D8). |
| A built pull request does not merge at the boundary. | The piece stays kind b. Its plan stays in the queue. |
| A plan closes as `built` while a flip waits. | A flip is a piece only when the plan lists it as a row. Section 10.1 lists the flips of rows that are open. |
| Dropped pieces make a plan look complete. | Column d of section 6.2 shows them, and the pieces keep the source of each drop. |
| A wave takes an item that a boundary pull request makes stale. | Wave 187 holds no item that depends on such a pull request. |

## 10. Open questions: the operator decision list

### 10.1 The switches (kind c)

Each switch needs a verified shadow record. Today `evolve stage verify <key>` reads an empty chain, so it returns `empty-chain` for each key. The evidence recorder (`shadow-contract-k2-evidence-seal`) and the flip verb (`shadow-contract-k5-stage-flip`) come first. The shadow records of the cycles are at `R/.evolve/runs/cycle-N/`.

| Rank | Plan | Piece | Stage key | Evidence path | Item |
|---|---|---|---|---|---|
| 2 | fleet-landing-at-scale | S6 scan with keys, enforce | `gates.scan_cache_gate` | The stage key needs a registry entry first. It has no verifiable evidence until then. | none |
| 4 | retry-ladder | L8 move to enforce | `workflow.retry_ladder.stage` | `R/.evolve/runs/cycle-N/` ladder records | none |
| 13 | fleet-host-adaptive-window | Rollout steps 3 to 5 set thresholds | `fleet.orchestrator.window.adaptive.stage` | `R/.evolve/runs/cycle-N/` records of the shadow waves | none |
| 15 | convergence-policy | V9 | `workflow.convergence.stage` | `R/.evolve/runs/cycle-N/` records of two shadow waves | `convergence-policy-v9-stage-enforce-flip` |
| 19 | wave-latency | W5 shared verdict store | `gates.scan_cache_gate` | The stage key needs a registry entry first. It has no verifiable evidence until then. | none |
| 20 | phase-handoff-contract | H12 enforce by pair | `handoff.pairs.<pair>.stage` | Each pair stage key needs a registry entry first. It has no verifiable evidence until then. | `handoff-h12-enforce-by-pair` |
| 23 | wave-integration | I9 move to enforce | `fleet.integration.stage` | `R/.evolve/waves/integration-N.json` and `R/.evolve/waves/integration-N/deliverable-*.json` | `wave-integration-i0-hardening-before-enforce`, `enforce-evidence-for-integration` |
| 23 | wave-integration | I14 integration driver | `fleet.integration.stage` | `R/.evolve/waves/integration-N.json` and `R/.evolve/waves/integration-N/deliverable-*.json` | none |
| 39 | code-review-phase | R8b | `workflow.findings_repair.code-review.stage` | `R/.evolve/runs/cycle-N/` code-review shadow records | `code-review-phase-r8b-enforce-flip` |
| 42 | architecture-decisions | AD2-5 curator enforce | `inbox.curator.stage` | The curator proposals in `R/.evolve/inbox/` | `arch-decisions-ad2-5-flip-inbox-curator-stage-to-enforce` |

Notes on the switches:

- The policy file already reads `enforce` for `inbox.curator.stage`. The operator confirms the shadow record, then consumes the item.
- `fleet.integration.stage` also needs the items `wave-integration-i0-hardening-before-enforce` and `enforce-evidence-for-integration`.
- `gates.scan_cache_gate` and the handoff pair stages are not in the output of `evolve stage report`. The registry must list them before a flip. Until then these stage keys have no verifiable evidence.
- The chain K2, K4, K3 and K5 comes first (OD1, D9). It is the evidence base of all 10 switches over 8 stage keys. The 3 rows with no registry entry also need that entry first.
- The rows I14 and I9 of `wave-integration` name one switch. The rows S6 and W5 name one switch.

### 10.2 The orphans (kind e)

Each row had no item and no drop decision. The operator approved all 14 rows on 2026-11-22. The 13 drops are recorded. The 14th row is filed as an item, so it is no longer an orphan.

| Rank | Plan | Piece | Decision (2026-11-22) |
|---|---|---|---|
| 7 | opus55-effort-config | Later cli-routing explain variant drift, line-length guard | Approved: dropped. Reason: No failed cycle and no measured need. |
| 9 | skills-drift-worktree-generator | Residual one registry for the generator prefix | Approved: dropped. Reason: `ssotPrefix` is no longer in `go/`; the text is stale. |
| 15 | convergence-policy | V8 | Approved: dropped. Reason: The metrics verb is built; the signal codes are unverified and no failure is cited; V9 evidence shows the need. |
| 18 | latency-gates | T7 boundary runs T5 and T6 | Approved: filed on 2026-11-22 as item `boundary-step-ci-latency-check-calls-a-missing-verb` (class stability, weight 0.5). Step `ci latency check` of `go/cmd/evolve/cmd_boundary.go:237` calls a verb that does not exist. |
| 22 | cover-changed-gate | Section 8 condition M4 one test run | Approved: dropped. Reason: No evidence of the condition and no failure cited. |
| 26 | inbox-classifier-room | Tier B ADR and held-proposal admission | Approved: dropped. Reason: The row is gated on two waves of the section 8 measures; no measured need yet. |
| 35 | bridge-factory | F2 deep probe through the driver | Approved: dropped. Reason: The `DriverFor` removal is cleanup with no failed cycle behind it. |
| 40 | test-quality-audit | T9 apicover_named_test.go files | Approved: dropped. Reason: Coverage hygiene with no failed cycle cited; 165 files stay a background task. |
| 42 | architecture-decisions | AD4-3 merge #1070 and run the ratchet | Approved: dropped. Reason: PR #1070 is not found; the AD4-4 item owns the ratchet merge. |
| 42 | architecture-decisions | AD4-5 ratchet docs and enforce | Approved: dropped. Reason: The docs and the flip follow AD4-4 and its shadow evidence; AD4-4 item covers them. |
| 45 | queue-authority-from-host-facts | K2 drift signal on each exit path | Approved: dropped. Reason: Unverified and no failed cycle cited. |
| 45 | queue-authority-from-host-facts | K3 deny and contract rows | Approved: dropped. Reason: Unverified and no failed cycle cited. |
| 45 | queue-authority-from-host-facts | T1 route at triage | Approved: dropped. Reason: The D7 rule is unverified and no failed cycle is cited. |
| 46 | pipeline-blockers | B4 ship check reads the lane tree | Approved: dropped. Reason: The shell-write proof is on main (go/internal/bridge/protected_write_test.go), #1611 and #1615 fix the ship backstop, and no failure is cited. |

The item `boundary-step-ci-latency-check-calls-a-missing-verb` was filed on 2026-11-22. The 13 dropped pieces are marked `dropped (operator, 2026-11-22)` in the delivery tables of their plans (PR #1671, second commit).

### 10.3 Other questions

1. The queue needs a verb that rebuilds section 6.2 from the delivery tables and the inbox. Does the operator want an item for it?
2. The 31 plans of section 6.4 close in one docs change. Does the operator want that change at the next boundary?
3. These 21 pull requests map to no open plan row: `#1616 #1626 #1627 #1629 #1633 #1638 #1639 #1640 #1641 #1647 #1648 #1650 #1653 #1661 #1662 #1664 #1666 #1667 #1670 #1671 #1672`.

They are blocker fixes, tests and documents. They close no piece of the queue.
