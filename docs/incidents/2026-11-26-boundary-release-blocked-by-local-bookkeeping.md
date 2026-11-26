---
schema: incident-record/1
id: 2026-11-26-boundary-release-blocked-by-local-bookkeeping
title: "Local bookkeeping commits blocked the boundary release"
level: record
status: open
selfheal: ""
fingerprint: ""
signals: []
prs: []
items: []
actions: []
effectiveness:
  metric: {code: "", filters: [], unit: "", since: ""}
  baseline: {value: 0, units: 0}
  target: 0
  checks: 0
  results: []
verified_at: ""
---

# Local bookkeeping commits blocked the boundary release

## Summary

<!-- GENERATED:status BEGIN -->
- **Status:** open
<!-- GENERATED:status END -->

## Trigger and level

- **Trigger:** the console started the release of v23.3.0 at the boundary of wave 189 on 2026-11-26.
- **Impact:** the plane's `main` was 21 commits ahead of `origin/main`. The release skill needs `HEAD` to equal `origin/main`, so the release did not start.
- **Level:** record. Every boundary release has this block while `fleet.integration.stage` is `shadow`.

## Timeline

| Time (2026-11-26, CST) | Event | Evidence |
|---|---|---|
| 11:20 | `evolve sync-main` merged `origin/main` into the plane. `HEAD` stayed 21 commits ahead. | the sync-main output |
| 11:25 | `evolve ship --push-only --dry-run` refused all 21 commits. | the log line below |
| 11:40 | The operator chose to fix the gap before the release. | the console session |

## Key logs

The ahead set held 20 `dossier: cycle-N closeout` commits (cycles 2630 to 2649) and one `chore(inbox): the loop's quarantine moves from the plane` commit. Source: `evolve ship --push-only --dry-run` in the plane.

```
ship --push-only: REFUSED — 21 ahead commit(s) lack ship provenance (not in ship-journal.jsonl, not a sync-main reconcile merge)
```

## Root cause and contributing factors

- `dossier` `publishPair` and `inboxstamps` `moveMessage` commit on the plane. They do not ship, so the ship journal never holds them.
- Decision D12 of the wave integration plan sends these commits to a bookkeeping batch only at enforce. At shadow they stay as today. So they reach `origin` only when the next cycle ship carries them.
- Push-only knew two classes of provenance: journaled, and a sync-main merge. A bookkeeping commit is neither.

## Analysis

- At the end of a wave, the last closeouts always wait on the plane. No ship follows them before the boundary.
- The release gate is correct: it must judge the CI run of the tree it publishes. The gap is the missing route for the commits, not the gate.
- A path prefix was the first rule for the inbox commits. The architecture review proved that a commit with a stamp subject then injected a new task. The rule now applies the writer's own contract.

## Approaches taken

| Approach | Result | Kept |
|---|---|---|
| The dossier publisher writes ship-journal entries | It couples dossier to ship and makes provenance a writable local file | No |
| Push-only accepts a bookkeeping commit that its content proves | Each owner exports the shape of its commit; ship composes the two | Yes |
| A path prefix for the inbox commits | A stamp subject carried a new task to `origin` in a probe | No |
| Release from `origin/main` without the closeouts | It needs a release outside the plane checkout | No |

## Actions

<!-- GENERATED:actions BEGIN -->
_No actions._
<!-- GENERATED:actions END -->

- `dossier.IsCloseoutCommit` and `inboxstamps.IsStampCommit` own the two shapes. `IsStampCommit` applies the contract of the stamp writer, not a path prefix.
- `isBookkeepingCommit` in `pushonly.go` requires one parent, a registered subject, and changes inside the contract of that subject.
- Decision D12a of the plan records the rule.
- Regression tests: `TestIsCloseoutCommit`, `TestIsStampCommit_FollowsTheWriterContract`, `TestPushOnly_PushesJournaledAndBookkeepingCommits`, `TestPushOnly_RefusesBookkeepingSubjectThatTouchesCode`, `TestPushOnly_RefusesUnknownSubjectOnBookkeepingPath`, `TestPushOnly_RefusesMergeCarryingDossierSubject`, `TestPushOnly_RefusesInjectedTaskUnderStampSubjects`, `TestIsBookkeepingCommit_AMergeIsNeverBookkeeping` and `TestIsBookkeepingCommit_AFailedGitReadIsNotProvenance`.
- The writer-contract table refuses these shapes:
  - an injected task, an author-owned key edit, and an add without a deletion;
  - a code deletion under the move subject, and a move that rewrites a curated key;
  - one deletion that feeds two additions, and a stamp of a tracked `processed/` item.

## Effectiveness

<!-- GENERATED:effectiveness BEGIN -->
- **Metric:** `-` (unit: `-`, since: `-`, filters: none)
- **Baseline:** 0 over 0 -(s) | **Target:** 0 | **Checks:** 0
- **Results:** none
<!-- GENERATED:effectiveness END -->

- The check accepted all 21 real ahead commits of the plane in a probe on 2026-11-26.

## Lessons

- A shadow stage can leave a gap at a later step, here the release. When a stage stays in shadow, check each boundary step that assumes the enforce behavior.
- A checker of a commit shape must be the writer's contract. A looser copy is a route for other content.
