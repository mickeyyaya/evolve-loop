# Research: how the deliverables of one wave become one change on main

- **Status:** complete, 2026-10-20.
- **Plan:** [wave-integration-2026-10](../plans/wave-integration-2026-10.md). **Decision:** [ADR-0147](../architecture/adr/0147-the-wave-orchestrator-owns-integration-and-decides-when-the-wave-reaches-main.md). **Design:** [wave-orchestrator §15](../architecture/wave-orchestrator.md#15-integration).
- **Related:** [concurrent-cycle-landing-2026-10](concurrent-cycle-landing-2026-10.md) (ADR-0128), [fleet-landing-at-scale-2026-10](../plans/fleet-landing-at-scale-2026-10.md) (ADR-0134), [wave-latency-2026-10](wave-latency-2026-10.md).
- **Method:**
  - the signals and the run artifacts of waves 130 to 135;
  - the git history of `origin/main` from 2026-10-19;
  - a read of the landing code and of the specs of ADR-0128, ADR-0134 and ADR-0139;
  - live web searches on 2026-10-20 (section 3).

## Table of contents

- [1. The request](#1-the-request)
- [2. Local evidence](#2-local-evidence)
- [3. Current practice](#3-current-practice)
- [4. Limits](#4-limits)

## Terms used throughout

- **Deliverable:** the audited change of one cycle: its commits, its audit binding and its declared scope.
- **Integration branch:** a short-lived branch, one for each wave, where the orchestrator composes the deliverables.
- **Batch:** the deliverables that one PR takes from the integration branch to `main`.
- **Prefix check:** a check of the composed tree after each deliverable in order: A, then A+B, then A+B+C.
- **Ejection:** the removal of a deliverable from a batch, back to its lane.

## 1. The request

The operator asked on 2026-10-20: "The wave ochestrator design should include the method to control and decide how to merge different cycles deliverables within the wave as one changes before pushing as PR. If this is not in the design, make sure ochestrator is delegate to optimize the integration / merging sequence from each cycle deliverables and justify how to merge and when to push." The operator added: "directly ship straight to main causing lots of confliction and merging and rebasing and it's not optimized solution for >= 2 cycle/fleets running in a single wave", and "The wave ochestrator should control and control the deliverables and optimize the merging sequence and decision."

## 2. Local evidence

### What the design says today

**F2.1** The wave-orchestrator spec puts landing outside the orchestrator: "The landing stays with ADR-0128 and ADR-0134" (`docs/architecture/wave-orchestrator.md:402`).
*Implication:* no part of the design owns the order of the landings or the decision to push.

**F2.2** ADR-0128 orders landings by ticket (FIFO) and never changes the order (`docs/architecture/fleet-landing-queue.md` §4). Each lane process lands its own candidate on `main`. ADR-0134 adds an order by the conflict graph and a group commit. Both ADRs are Proposed.
*Implication:* the queue design has the parts of an integration engine, but no single owner and no PR.

**F2.3** Neither queue is in use. `.evolve/policy.json` sets no `fleet.landing`, so each lane lands itself (`per-lane`, the default). The order by the conflict graph (S10) and the group commit are not built. The scan cache (S5) runs in shadow.
*Implication:* today the landings race each other.

### What per-lane landing costs

**F2.4** Waves 130 to 135 ran about 55 lanes. The cycles of cycles 1950 to 2003 hold 18 rebase or head-moved recoveries (`GIT_FLEET_REBASE_NEEDED`, `AUDIT_BINDING_HEAD_MOVED`) in 14 cycles, measured on 2026-10-20.
*Implication:* about one lane in four collided at landing. The landing research measured 24 minutes and 1.5 LLM phases for each recovery (research of ADR-0128, F3.2).

**F2.5** `origin/main` moved 100 times from 2026-10-19 to 2026-10-20: 27 lane ships and 73 other commits, such as console PRs and dossier closeouts.
*Implication:* each move can make every lane in flight rebase.

**F2.6** A lane push first retries as a fast-forward when its tip still holds `origin/main`. When an outside merge made the two diverge, the push is rejected with no auto-rebase, and the lane waits for a boundary (`go/internal/phases/ship/landing/push.go`, `repairPush`).
*Implication:* a console PR that merges during a wave strands the next lane.

**F2.7** A draft fix that let the ship follow an outside move (2026-10-20, not merged) met a deeper limit. The plane's `main` holds local commits that origin does not hold: the sync merge, dossier closeouts and inbox stamps. While it holds one, an outside move sends the lane into a re-audit loop. That loop cannot succeed before a boundary (architecture review of that draft, HIGH-2).
*Implication:* a patch to per-lane landing cannot remove the race. The writers of `main` must change.

## 3. Current practice

The searches ran on 2026-10-20. "acc." marks a page with no date of its own.

### One owner of the composed result

**M1** A GitLab merge train runs each request on the target branch plus every request ahead of it. When one fails, it leaves the train, and each request behind it runs again without it.
Source: [GitLab](https://docs.gitlab.com/ci/pipelines/merge_trains/), acc.

**M2** The GitHub merge queue builds a temporary branch of the base plus each PR ahead of it. It has a minimum and a maximum group size and a wait time.
Source: [GitHub](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue), acc.

**M3** Zuul tests each change on the changes ahead of it. Its window starts at 20, grows by 1 on each merge and halves on each failure, as TCP does.
Source: [Zuul](https://zuul-ci.org/docs/zuul/latest/gating.html), acc.

**M4** Uber SubmitQueue treats changes on disjoint build targets as independent, so they can land in any order. It rejects a textual conflict before it verifies.
Source: [Uber](https://www.uber.com/en-US/blog/bypassing-large-diffs-in-submitqueue/), 2023-08-31.

*Implication of M1 to M4:* every surveyed system has one owner of the composed result. A lane that pushes on its own is the state that these systems replaced.

### Batches and their timing

**M5** Aviator has a batch size and a maximum wait. A priority PR runs alone, and a failed batch splits in two.
Source: [Aviator](https://docs.aviator.co/mergequeue/concepts/batching), acc.

**M6** Mergify says that small batches keep latency low and failures cheap to isolate, and large batches save CI. A failed batch splits into parts.
Source: [Mergify](https://docs.mergify.com/merge-queue/batches.md), acc.

**M7** In a study on 276 million Chrome test results, fixed batches of 4 needed up to 72% fewer machines. A batch size that follows the queue length needed up to 91% fewer.
Source: [arXiv 2308.13129](https://arxiv.org/abs/2308.13129), FSE 2023.

**M8** Batch testing saves the most when failures are rare. Flaky failures become more likely as a batch grows. One variant stops the split at size 4 and tests each change alone.
Source: [Beheshtian and others, FSE 2019](https://ece.uwaterloo.ca/~wshang/pubs/Armin_FSE_2019.pdf), read as a summary.

**M9** Trunk keeps a failed PR in the queue while the PRs after it test, so a flaky failure does not eject it at once.
Source: [Trunk](https://docs.trunk.io/merge-queue/batching), read as a summary.

### The shape of the change

**M10** In the GitHub merge queue, batched PRs land as single commits with their authors (an evaluation by Apache Kafka). To revert a merge commit needs `-m 1`, and a later merge of the same branch does not bring the reverted change back.
Sources: [Apache Kafka](https://cwiki.apache.org/confluence/spaces/KAFKA/pages/311627648/INFRA-25932+GitHub+Merge+Queue+Evaluation), acc.; [git howto](https://git-scm.com/docs/howto/revert-a-faulty-merge), acc.
*Implication:* one commit for each cycle keeps the revert of one cycle simple. One squash commit for a wave forces the revert of the whole wave.

**M11** Short-lived branches last one to two days. A branch with more than one author drifts toward a release branch.
Source: [trunk-based development](https://cn.trunkbaseddevelopment.com/youre-doing-it-wrong/), acc.
*Implication:* the integration branch lives for one wave only.

### Culprits and conflicts

**M12** Google finds the change that broke a build by a noisy binary search with a prior on each change. It is more accurate than bisection with retries.
Source: [Google, ICST 2023](https://research.google/pubs/pub52465), 2023.

**M13** In 107,000 simulated agent PRs, 27.67% had textual conflicts.
Source: [AgenticFlict, arXiv 2604.03551](https://arxiv.org/abs/2604.03551), 2026.

**M14** Two patches that pass alone can fail together. On mined real PR pairs this happened in 1 of 834 runs. When the agent was told about the other change, it fixed 82% of the constructed cases.
Source: [arXiv 2609.25396](https://arxiv.org/abs/2609.25396), 2026-09-21.
*Implication:* an ejected deliverable goes back with the diff of the change it met.

**M15** A plan split the work into disjoint scopes and merged producers before consumers. In a simulation, clean integration rose from 1 in 9 scenarios to 9 in 9.
Source: [arXiv 2610.07261](https://arxiv.org/abs/2610.07261), 2026-10-05.

**M16** The best models resolve fewer than 60% of merge conflicts correctly.
Source: [Merge-Bench, arXiv 2605.25890](https://arxiv.org/abs/2605.25890), 2026.
*Implication:* the default action on a conflict is ejection, not an LLM merge.

## 4. Limits

- The local counts of F2.4 and F2.5 cover two days. They show the shape, not a rate to plan on.
- Several sources were read only as vendor pages or summaries: M8, M9 and the defaults of GitHub's queue.
- No public source describes Meta's land queue.
