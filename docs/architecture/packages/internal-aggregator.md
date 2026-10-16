# internal/aggregator

## Purpose

`internal/aggregator` merges the artifacts of a fan-out phase into one canonical phase artifact, following the per-phase merge rules.

## Design

- The merge is deterministic and makes no model call, so a malicious worker cannot coerce it into accepting forged output.
- Merge rules by phase:
  - scout, research and discover: concatenate with `## Worker:` headers;
  - audit: ALL-PASS verdict; any FAIL fails the aggregate;
  - learn, retrospective and retro: union of `## Lesson:` sections, deduplicated;
  - plan-review: multi-lens score and verdict aggregation;
  - audit-consensus and cross-cli-vote: MAJORITY-PASS with FAIL-VETO.

- The exit codes match the shell script `aggregator.sh` that this package ports. The shell callers read them.
  - `ExitOK` (0): the merge succeeded.
  - `ExitVerdictBad` (1): an audit FAIL, a plan-review ABORT or a cross-CLI FAIL.
  - `ExitUsageErr` (2): a usage error, a missing input file or an unknown phase.

## Invariants

- A FAIL from any worker in an audit fan-out fails the aggregate. Pinned by `TestAggregate_VerdictAnyFail`; all PASS merges to PASS in `TestAggregate_VerdictAllPass`.
- A mixed or missing verdict is a WARN, not a PASS. Pinned by `TestAggregate_VerdictMixedToWarn` and `TestAggregate_VerdictMissingFieldIsWarn`.
- A plan-review average of 5 or more with one weak lens is REVISE. An average under 5 is ABORT, and so is any ABORT lens. Pinned by `TestAggregate_PlanReviewWeakLensReturnsRevise`, `TestAggregate_PlanReviewAvgUnder5IsAbort` and `TestAggregate_PlanReviewAnyAbort`.
- A cross-CLI vote is a majority PASS with a FAIL veto. One FAIL vetoes the vote. Fewer PASS votes than the quorum is a WARN. Pinned by `TestAggregate_CrossCLIVetoOnAnyFail` and `TestAggregate_CrossCLIBelowQuorumIsWarn`.
- Every usage failure returns `ExitUsageErr` and leaves no partial output. Pinned by the `TestAggregate_Rejects*`, `TestAggregate_UnknownPhase` and `TestAmplifiedAggregate_ReadFileFailureDoesNotLeavePartialOutput` tests.
- The merge makes no model call. No test pins this property, so it is unpinned.

## Findings

- None open.
