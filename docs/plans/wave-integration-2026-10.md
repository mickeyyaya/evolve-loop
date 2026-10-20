# Plan: the wave orchestrator owns the integration of its deliverables

- **Status:** proposed, 2026-10-20. The operator decides the open questions of §7.
- **Research:** [wave-integration-2026-10](../research/wave-integration-2026-10.md). **Decision:** [ADR-0147](../architecture/adr/0147-the-wave-orchestrator-owns-integration-and-decides-when-the-wave-reaches-main.md). **Design:** [wave-orchestrator §15](../architecture/wave-orchestrator.md#15-integration).
- **Amends:** [ADR-0139](../architecture/adr/0139-a-wave-orchestrator-supplies-admits-and-backfills-every-lane.md) (the orchestrator now owns landing), [ADR-0128](../architecture/adr/0128-landing-queue-tiered-reverification.md) and [ADR-0134](../architecture/adr/0134-fleet-landings-scale-through-content-keyed-verification-and-group-commit.md) (their composition, proof, tiers and conflict-graph order become parts of the integration engine).

## Table of contents

- [1. The problem](#1-the-problem)
- [2. A worked example](#2-a-worked-example)
- [3. Candidate designs](#3-candidate-designs)
- [4. The winner and what it takes from the others](#4-the-winner-and-what-it-takes-from-the-others)
- [5. Decisions](#5-decisions)
- [6. Components and order](#6-components-and-order)
- [7. Open questions for the operator](#7-open-questions-for-the-operator)
- [8. Verification](#8-verification)
- [9. Limits](#9-limits)

## 1. The problem

Each lane of a wave lands its own deliverable on `main`. The landings race, and about one lane in four needs a rebase recovery of about 24 minutes (research F2.4). `main` moves often, from lanes and from console PRs, and each move can make every lane in flight rebase (F2.5). A console PR that merges during a wave strands the next lane (F2.6). A patch to the per-lane landing cannot remove the race, because the plane's `main` holds local commits that origin does not hold (F2.7).

No part of the design owns the order of the landings or the decision to push (F2.1). The landing queue of ADR-0128 and ADR-0134 has the parts of an integration engine. But each lane still lands itself, and the queue is not in use (F2.2, F2.3).

## 2. A worked example

Wave 133 had four lanes in flight. Under the design of this plan:

| Time | Event | Integration action |
|---|---|---|
| 11:40 | lane L2 (`retry-ladder-l2-decider`) delivers | compose L2 onto the integration tip; its proof is T1, because no peer touches its packages; the prefix check passes |
| 12:24 | lane H7 (`handoff-h7-cli`) delivers | compose H7; T1; prefix check passes; the batch holds 2 deliverables |
| 12:30 | lane G (`go-test-env-projection-pinned-lane`) fails its audit | no delivery; nothing to compose |
| 12:32 | the batch reaches `max_age_minutes` | flush: push `integration/w133-b1`, open one PR with 2 commits |
| 12:50 | CI of the PR is green, and `main` did not move | fast-forward `main` to the tested tip; `main` moves one time for both lanes |

Under today's per-lane landing, the same wave moved `main` once for each lane, and each move can make the other lanes rebase.

## 3. Candidate designs

| Design | Summary |
|---|---|
| A. Finish the per-lane queue | Build ADR-0128 and ADR-0134 as designed. Each lane composes its candidate and lands it on `main` at its turn. |
| B. A native merge queue | Each deliverable becomes its own PR. The GitHub merge queue orders and tests the PRs. The orchestrator only sets the settings. |
| C. An orchestrator train | The orchestrator orders the deliverables and composes each one onto an integration branch as one commit. It checks each prefix, and pushes a batch as one PR when it decides. |
| D. Parallel trains by partition | As C, but the conflict graph splits the wave into one to three independent batches, each with its own PR and CI. |
| E. Avoidance only | Admission by cell keeps conflicting work apart, and each lane still lands itself. |

Each criterion scores 1 (poor) to 5 (good). "Removes the landing race" counts twice, because it is the purpose of the change.

| Criterion | A | B | C | D | E |
|---|---|---|---|---|---|
| Removes the landing race (×2) | 3 (6) | 4 (8) | 5 (10) | 5 (10) | 2 (4) |
| One owner decides order and timing | 2 | 2 | 5 | 5 | 1 |
| CI runs per wave | 2 | 2 | 5 | 3 | 2 |
| Culprit found cheaply | 4 | 3 | 5 | 4 | 2 |
| Change size | 3 | 4 | 2 | 1 | 5 |
| Fits the audit trail and the ADRs | 4 | 2 | 5 | 4 | 4 |
| **Total (of 35)** | 21 | 21 | **32** | 27 | 18 |

**C wins.**

- A keeps a lane as the writer of `main`, so it keeps the race with the console PRs and the local plane commits (F2.6, F2.7).
- B gives each deliverable its own PR and CI run, and its ordering control is coarse (research M2).
- D costs up to three CI runs for each wave, and its benefit appears only when batch failures are frequent.
- E lowers the conflict rate but leaves each lane to land itself.

Every surveyed merge system has one owner of the composed result (research M1 to M4). C gives that owner to the orchestrator, which already owns the supply, the admission and the pool.

## 4. The winner and what it takes from the others

| From | What C takes | Why |
|---|---|---|
| A | The composition of the audited tree, the overlap proof and the tiers T1 to T4 of ADR-0128. Also the content-keyed verification of ADR-0134. The tiers T1 to T4 are the tiers of a compose, not the test layers L1 to L4. | They decide what each compose must check again, at the lowest cost. |
| B | The ejection rule: a failed deliverable leaves the batch, and the deliverables behind it compose again without it | The rule of GitLab, GitHub and Zuul (M1 to M3). |
| D | Partitions as a later stage: when the batch failure rate passes a bound, the orchestrator splits a batch by the conflict graph | It pays only when failures are frequent (M6, M7). |
| E | Admission by cell stays in front of integration | It keeps conflicts out of the wave in the first place (M15). |

## 5. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | The orchestrator owns integration. A fleet lane delivers its audited change and does not push `main`. | F2.1, F2.6, M1 to M4 | Each lane lands itself: F2.4 to F2.7. |
| D2 | A deliverable is a record. It holds the cycle commits, the audit binding, the declared scope and the deps. It also holds the inbox rank, the risk class, the `base_sha` and the patch-id of each commit. The landed identity is the cycle trailer plus the patch-id, not a SHA. | ADR-0128 §11, review HIGH-1 | A bare branch: the order needs the facts. A SHA identity: each compose makes a new SHA. |
| D3 | The order is a deterministic score. The first key is the deps: a producer before its consumer. Then the conflict graph, the inbox rank and the count of items it unblocks. Then low risk, low verification cost and the time of delivery. | M4, M15 | An LLM decides the order: the inputs are facts. FIFO: it ignores deps and rank. |
| D4 | Each deliverable composes onto the integration tip as one commit, with trailers for the cycle and its audit binding. | M10 | One squash commit for each wave: it forces a whole-wave revert. |
| D5 | Each compose runs the overlap proof and the gates of its tier, scoped and cached (ADR-0128, ADR-0134). The prefix check after each compose names the culprit before CI runs. | M3, ADR-0128 | One check of the whole batch: the culprit is unknown. |
| D6 | A textual conflict, a failed gate or a tier-T4 proof ejects the deliverable to its lane, with the diff of the change it met. The retry ladder of ADR-0142 takes the lane. No LLM resolves the conflict by default. | M14, M16 | An LLM merges: fewer than 60% are correct (M16). |
| D7 | A batch flushes as one PR on the first of these triggers. The wave ends. The batch reaches its cap. Its oldest deliverable reaches `max_age_minutes`. A deliverable unblocks queued work. | M5 to M7 | A flush only at wave end: a long wave holds every deliverable. A rank trigger: it needs a threshold that no measure supports yet. |
| D8 | The cap adapts like Zuul's window. It starts at 4 and stays between 1 and 4. It halves on a red batch CI that the re-run confirms, and grows by 1 on a green batch. | M3, M7 | A fixed cap: it does not react to failures. A halve on the first red: a flake shrinks the cap. |
| D9 | A risky deliverable flushes alone: one that touches the protected surface or the global zone (the "iffy" class of ADR-0128). | M5 | Risky changes in a batch: one failure blocks the rest. |
| D10 | On a red CI, the orchestrator runs the PR once more to screen a flake. On a second red, it splits the batch in halves until each part has at most 2 deliverables, then tests each one alone. | M6, M8, M9 | Eject the whole batch: good work waits. |
| D11 | When the batch CI is green and `origin/main` still equals the batch base, the orchestrator fast-forwards `main` to the tested tip. GitHub then marks the PR merged. If `main` moved, it composes again (D13) and tests again. | D1, M10, review HIGH-2 | A rebase merge: it rewrites each SHA, and after a move of `main` it lands a tree that no CI tested. A squash merge: one commit for each wave. |
| D12 | The loop's own commits (dossier closeouts, inbox stamps) go to a bookkeeping batch of their own. It holds no code, skips L2, and flushes alone at each code flush and at wave end. So the plane's `main` holds no local-only commit. At shadow, they stay as today. | F2.7, review M-5 | Keep them on the plane's `main`: the race of F2.7 stays. Ride the code batch: a red code batch holds them. |
| D13 | A console PR that merges into `main` makes the orchestrator compose the open batch again on the new `main`. It cancels a CI run of the old tip. The patch-ids show which deliverables `main` already holds. | F2.5, review HIGH-2 | Each lane rebases: the cost of F2.4. |
| D14 | `fleet.integration.stage` starts at `shadow`. At shadow, lanes land as today. The orchestrator runs a textual compose only, and records its order, its composes and its flush decisions. It runs no test gate, so the host load stays as today. It is a second stage key, against ADR-0139 decision 16, because integration can only start after the pool is at `enforce`. | the stage pattern of ADR-0139, review HIGH-6 | Enforce at once: the engine is unproven. Real gates at shadow: they double the test load of a host that saturates at width 4. |
| D15 | The merge window of the PR-latency plan (P2) no longer waits for a fix to the per-lane ship. It waits for `fleet.integration.stage: enforce`, because then no lane writes `main`. | D1, F2.7 | Fix the per-lane ship first: F2.7 shows that the fix cannot be complete. |

### The test strategy for the integrated batch

Each lane passed its own tests on its own base. The integration branch holds a tree that no lane tested: the deliverables together. Four layers test that tree, from cheap and early to complete and late: L1 prefix, L2 batch, L3 PR and L4 after merge. The spec of record holds the table of layers and the rules ([wave-orchestrator §15](../architecture/wave-orchestrator.md#15-integration)).

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D16 | Four test layers: prefix, batch, PR and after merge, with the content of the table above. | M3, M10, ADR-0128 §9 | One full suite on the PR only: the culprit is unknown, and each red costs a CI run. |
| D17 | Each deliverable's acceptance predicates run on the combined tree in L2. | M14 | Only each lane's own run: it cannot see interference. |
| D18 | A red after the merge first goes through `evolve ci classify` and one re-run. A confirmed red bisects by cycle commit, and the orchestrator reverts the culprit's commit alone. A confirmed L4 red after a green L3 is a system failure of ADR-0072: it halts the integration and files a P0 item. | D4, M10, M12, ADR-0072 | Revert the batch: good work leaves `main` with the bad. Revert on the first red: a flake reverts good work. |
| D19 | The orchestrator holds an integration record, `.evolve/waves/integration-<wave>.json`: the branch, the tip, the open PR, the members and the loop lease epoch. Each branch write uses `--force-with-lease` on the recorded tip. On a resume, the new lease holder adopts the branch or discards it, and closes a PR of an older epoch. | loop lease fencing, review HIGH-4 | No record: a crash leaves a branch and a PR with no owner. |
| D20 | The orchestrator releases the cells of a deliverable at the merge, not at the delivery. An ejected deliverable goes back to its lane, so its cells stay held until it lands or the lane gives up. | review HIGH-6 | Release at delivery: a new lane in the same cell conflicts with an ejected deliverable. |
| D21 | At enforce, the window of §10 takes its conflict feed from `SHIP_INTEGRATION_EJECTED` with `cause=conflict`. The ship and zero-ship counters count deliverables that merged, from `SHIP_INTEGRATION_MERGED`. | review HIGH-5, HIGH-6 | Keep the lane feeds: at enforce, a lane no longer lands, so they go silent. |
| D22 | The CI waker is one blocking `gh run watch` child for each batch PR, in `internal/integration`. The `max_age_minutes` flush uses one `time.AfterFunc` for the oldest deliverable. `internal/orchestrator` keeps its guard. | the no-poll rule, review HIGH-7 | A poll of the GitHub API: the no-poll rule. |
| D23 | Each batch has its own branch, `integration/w<wave>-b<n>`, cut from `origin/main` when its first deliverable composes. The branch is deleted after the merge. | M11, review M-2 | One branch for each wave: after a merge it needs a reset onto the new `main`. |

### Consequences

- Good: `main` moves once for each batch, not once for each lane, and no lane rebases at landing.
- Good: one owner decides the order and the timing, with a reason that the record shows.
- Good: a console PR can merge during a wave without a stranded lane.
- Bad: a deliverable waits in the batch until a flush, up to `max_age_minutes`. Its cells stay held until the merge (D20).
- Bad: the orchestrator becomes the single writer of `main` for the loop. A defect in it stops all landings. The stage `shadow` and the per-lane fallback bound that risk.
- Bad: a red batch costs extra CI runs for the split.

### What changes our mind

- At shadow, the recorded batches show a red rate above one in three. Then design D (partitions) starts before enforce.
- The flush by `max_age_minutes` holds deliverables longer than the per-lane landing did, over 30 batches. Then `max_age_minutes` goes down.

## 6. Components and order

| # | Component | Home | Route |
|---|---|---|---|
| I0 | The deliverable record of D2. The kind `ship.delivered` and the codes `SHIP_INTEGRATION_*` in the Signal Center registry. At `enforce`, a fleet lane's ship stops before `main`. | `go/internal/phases/ship`, `go/internal/core` | console |
| I1 | The order score of D3, as a pure function | `go/internal/integration` (new) | lane |
| I2 | The compose engine: one commit for each deliverable, the overlap proof and the tier, reusing the ADR-0128 code (`planlanding.go`, `landprefixes.go`) | `go/internal/integration`, `go/internal/phases/ship` | console |
| I3 | The prefix check, the ejection and the ejection brief to the lane | `go/internal/integration`, `go/internal/orchestrator` | console |
| I3b | The batch test L2 of D16 and D17. It finds the affected set and runs each deliverable's predicates on the combined tree. It uses the prefix order for a red and records each verdict. | `go/internal/integration`, `go/internal/phases/ship` | console |
| I4 | The flush policy of D7 to D9, with the adaptive cap, as a pure function | `go/internal/integration` | lane |
| I5 | The batch PR: push and open it, and wake on CI through the `gh run watch` child of D22. Fast-forward `main` to the tested tip (D11). Re-run and split on red. | `go/internal/integration`, `go/cmd/evolve` | console |
| I6 | The bookkeeping batch of D12 | `go/internal/core`, `go/internal/inboxmover` | console |
| I7 | The compose again after a console merge into `main` | `go/internal/integration` | lane |
| I8 | `evolve wave integration status` and the signals | `go/cmd/evolve`, `go/internal/signalcenter` | lane |
| I10 | The integration record of D19, with the lease epoch, the adoption on resume and `--force-with-lease` | `go/internal/integration`, `go/internal/core` | console |
| I11 | The fast-forward of the plane's `main` after each merge, and the feeds of D21 | `go/internal/core`, `go/internal/orchestrator` | console |
| I9 | The move to `enforce` | `.evolve/policy.json` | operator |

The ids name the components; the order is below. I1 and I4 are pure and come first. I0 to I3 make shadow possible. I5, I6, I10 and I11 are needed only for enforce.

## 7. Open questions for the operator

| Id | Question | Proposed answer |
|---|---|---|
| Q1 | The cap and `max_age_minutes` of a batch | cap 4 (adaptive, D8); 45 minutes |
| Q2 | The merge method of a batch | a fast-forward of `main` to the tested tip, one commit for each cycle (D11). The console PRs keep their squash merge. |
| Q3 | The base of a new lane | `main`. A base on the integration tip lets later lanes build on earlier wave work, but an ejection then strands each lane on that base. That option waits for shadow data, and then needs the cascade rule: an ejection also ejects each deliverable whose `base_sha` holds the ejected patch-id. |
| Q4 | May the orchestrator fast-forward `main` during a wave at enforce? This amends the standing rule that code PRs merge only at a boundary. | yes, at enforce only: no lane writes `main` then, and I11 moves the plane's `main` after each merge. Console PRs keep the boundary rule. |

## 8. Verification

- **Order (I1):** a table test over seeded deliverable sets. A consumer never composes before its producer. Two deliverables with no edge keep their order by rank. The same input always gives the same order.
- **Compose and prefix (I2, I3):**
  - three fixture deliverables A, B and C, where C breaks a test. The prefix check passes A and A+B and fails A+B+C. It ejects C with the diff of A+B;
  - a textual conflict ejects with no gate run.
- **Flush (I4):**
  - each trigger of D7 flushes once;
  - a risky deliverable flushes alone;
  - the cap halves on red and grows on green, inside 1 to 4.
- **Batch test (I3b):**
  - two fixture deliverables that pass alone and fail together. L2 runs both predicate sets on the combined tree and fails. It ejects the later one by the prefix order;
  - the affected set of an edit in a package includes each package that imports it;
  - a test that fails and then passes once is recorded as a flake, and nothing is ejected;
  - a package with a green verdict on the same key is skipped and recorded as skipped.
- **Identity (D2, D11):** after a compose and a fast-forward, each cycle is found on `main` by its trailer and its patch-id. The ledger entry, the dossier closeout and the consume check use that identity.
- **Tested tree (D11, D13):** with a fixture `main` that moves after a green CI, the orchestrator does not fast-forward. It composes again and tests again.
- **Resume (D19):** a second lease holder adopts the recorded branch, a write with a stale tip fails, and a PR of the old epoch closes.
- **Batch PR (I5):** with a fake CI, a red run re-runs once. Then it splits until each part has at most 2 deliverables, and the culprit is ejected.
- **After merge (D18):** with a fixture `main`, a red that passes on its re-run reverts nothing. A confirmed red bisects to the culprit and reverts only its commit.
- **Shadow (D14):** at shadow, the landings are byte-identical to today, and no test gate runs. The record holds the order, the composes and the flush decisions of each wave.
- **Shadow exit:** 3 waves with at least 2 deliverables each. The recorded order has no dependency inversion. The textual compose predicts the real `SHIP_GIT_FLEET_REBASE_*` events of those waves with a precision and a recall of at least 0.8.
- **Enforce exit:** over 10 batches, the batch red rate is below one in three. The median time from delivery to merge is at most the per-lane baseline.

## 9. Limits

- A deliverable waits in its batch until a flush.
- The prefix check catches what the scoped gates test. A defect outside them can pass, and the CI of the batch PR is the backstop.
- Two deliverables that pass alone can fail together (research M14). The prefix check catches it only when a test covers the interaction.
- The orchestrator is the single writer of `main` for the loop.
