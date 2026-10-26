# Research: a deterministic review of the inbox (2026-10)

> **Request (operator, 2026-10-27):** "Let's build this directly now, as it's critical to the pipeline." The inbox has no review: duplicates and wrong routes enter at add time unchecked, and items whose work merged stay open.
>
> **Design set:** the spec is [inbox-curator.md](../architecture/inbox-curator.md). The plan is the [P3 revision](../plans/inbox-prioritization-2026-10.md#p3-revision-2026-10-27-a-deterministic-intake-and-an-event-driven-curator). The decision record is [ADR-0121](../architecture/adr/0121-inbox-priority-is-a-computed-rank.md).

## Questions

1. Which deterministic measure finds a duplicate inbox item, and at which threshold?
2. Which deterministic fact shows that the work of an item merged?
3. What does current practice do with a stale or duplicate issue: close it, or propose it?

## Method

- A read-only copy of the live inbox of 2026-10-27 (258 items at the root, `runtime/.evolve/inbox`) went to a scratch directory. A throwaway Go test computed the measure on every pair. The test was not committed.
- The named tests of the 4 known stale items were looked up on `origin/main` (`b7d54a934`) with `grep -rl "func <name>" go/`.
- Live web searches on 2026-10-27 covered duplicate detection in issue trackers, the closing keywords of a merged change, and backlog groomers.

## Findings

### The live inbox

**F1.1** The 258 items hold 2,710 pairs that share a fix surface. Their similarity is below 0.1 for 2,407 pairs, between 0.1 and 0.2 for 234, and between 0.2 and 0.3 for 55. It is between 0.3 and 0.5 for 13 pairs, and 1 pair is at 0.52. *Implication:* most pairs on one surface share few words. A threshold of 0.5 refuses only near copies.

**F1.2** The pair at 0.52 is `code-index-l3-on-demand-entry` and `context-index-knowledge-layer`, on `go/cmd/evolve` and `go/internal/inputindex`. The pairs between 0.3 and 0.5 are mostly siblings of one campaign, for example the `context-index-*` and `guidance-*` items. *Implication:* the band between 0.3 and 0.5 is the place for a warning, not a refusal.

**F1.3** The 4 known stale items name 9 tests in their acceptance. 8 exist on `origin/main`, for example `TestAdmit_ATodoInABusyCellWaits` in `go/internal/admission/admission_test.go`. `TestLease_ALanePastItsDeadlineStopsAndReleasesItsItem` of `rolling-pool-backfills-lanes` does not exist. Each stale item also has a criterion with no test, such as "the utilization is at least 0.85 over 3 runs". *Implication:* the share of named tests that exist finds the stale items. A missing test or an unmeasured criterion leaves the decision with the operator.

**F1.4** The 258 items carry no `origin` field. 133 name a console route, and 3 name `route: lane`. *Implication:* the origin must be stamped from now on. A console route with no fact behind it is a candidate for a lane.

### Current practice

**F2.1** Issue trackers compare two reports with a token Jaccard of the title and the description. One tool computes it over every pair. Its docs say that it is fast but misses reports with different words ([beads find-duplicates](https://beads.gascity.com/cli-reference/find-duplicates.md)). A study of static-analysis reports used a Jaccard of 0.95 with MinHash ([arXiv 2511.10323](https://arxiv.org/pdf/2511.10323)). A patent learns the threshold from pairs that people labelled as duplicates ([US 12455808](https://image-ppubs.uspto.gov/dirsearch-public/print/downloadPdf/12455808), read on 2026-10-27). *Implication:* a token Jaccard is an accepted deterministic measure, and the threshold is data, set from measurements.

**F2.2** A merged change closes the issues that its text names with a closing keyword, such as "Closes #10" ([GitHub closing keywords](https://help.github.com/articles/closing-issues-using-keywords)). *Implication:* the text of a landed commit is evidence that it finishes an item. The evidence is weak alone, because the text can name an item that the change only touches.

**F2.3** Backlog groomers flag open issues whose linked change merged. One groomer applies its rules in code before any model call, and it flags issues but never closes them ([withone backlog groomer](https://www.withone.ai/use-cases/backlog-groomer)). Another template closes stale issues itself ([cotera Jira groomer](https://cotera.co/solutions/ops/jira-backlog-grooming-agent)). *Implication:* "propose, and close only on a verified fact" is the conservative choice.

## Conclusions

| Question | Answer | Findings |
|---|---|---|
| 1 | A shared fix surface plus a token Jaccard of the title and the acceptance. Refuse at 0.5, warn at 0.3. | F1.1, F1.2, F2.1 |
| 2 | The landed change touched the fix surface, and the named tests exist. The fact is complete only when every criterion names a test and every named test exists. | F1.3, F2.2 |
| 3 | Publish a proposal. Apply only a proposal whose fact Go can verify, and only at the wave boundary in the `enforce` stage. | F2.3 |

## Limits

- The measure reads the title and the acceptance only, so two items with different words for one change score low (F2.1).
- A named test that existed before the item was filed gives a false stale signal. The sweep therefore never applies a proposal.
