---
schema: incident-record/1
id: 2026-11-26-push-only-dry-run-pushed-to-main
title: "A push-only dry run pushed 23 commits to main"
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

# A push-only dry run pushed 23 commits to main

## Summary

<!-- GENERATED:status BEGIN -->
- **Status:** open
<!-- GENERATED:status END -->

## Trigger and level

- **Trigger:** on 2026-11-26 the console ran `evolve ship --push-only --dry-run` in the plane to preview the push of the bookkeeping commits.
- **Impact:** the dry run pushed 23 commits to `origin/main`. The content was the push that the console meant to run next, so no wrong content reached `main`.
- **Level:** record. A dry run that writes to `main` breaks the contract of every preview step.

## Timeline

| Time (2026-11-26, CST) | Event | Evidence |
|---|---|---|
| 14:21 | `evolve rebuild --repin` built the binary that holds the bookkeeping provenance class. | the rebuild output |
| 14:21 | `evolve ship --push-only --dry-run` printed `pushed 23 attested commit(s) to origin/main`. | the log line below |
| 14:21 | `origin/main` moved to `150bc8ffa`, and main CI started on it. | `git ls-remote origin refs/heads/main`, `gh run list` |

## Key logs

Source: the output of `evolve ship --push-only --dry-run` in the plane.

```
[ship] PUSH-ONLY: pushed 23 attested commit(s) to origin/main
[ship] DRY-RUN: journal preview written to .evolve/release-journal/dry-run-20261126T062109Z.json
```

## Root cause and contributing factors

- `Run` sends a push-only request to `runPushOnly` before any dry-run handling. `runPushOnly` never read `opts.DryRun`.
- `landing.Push` treated `DryRun` as "skip the repair". It still sent the push. The package note stated this rule, and a test pinned it.
- The normal ship path stops before the push in a dry run, so the rule caused no harm there. Push-only was the first path that reached `Push` in a dry run.
- Two earlier dry runs on the same day looked safe only because provenance refused every commit before the push.

## Analysis

- A dry run is a promise to the operator: it reads, and it never writes. The landing layer owns every push, so the promise must live there.
- The golden matrix of the push sites recorded the push of the dry-run row as correct. A characterization test keeps a defect when nobody reads the row.

## Approaches taken

| Approach | Result | Kept |
|---|---|---|
| Check `DryRun` only in `runPushOnly` | It stops the push of this path, but a later caller of `landing.Push` can repeat the defect | Partly: kept for the log and the bindings |
| `landing.Push` sends no push in a dry run | One home for the rule; every caller gets it | Yes |
| Leave the landing rule and document it | The documented rule caused this incident | No |

## Actions

<!-- GENERATED:actions BEGIN -->
_No actions._
<!-- GENERATED:actions END -->

- `landing.Push` returns before any git call when `DryRun` is set.
- `runPushOnly` checks provenance, then in a dry run logs the count of commits that the push holds, and returns. It releases no landed binding.
- Regression tests: `TestPush_ADryRunSendsNoPush`, `TestPushOnly_ADryRunLeavesOriginUnchanged` and `TestPushOnly_ADryRunStillRefusesAnUnprovenancedCommit`. The golden row `pushonly/dry-run` now holds no push and no error.

## Effectiveness

<!-- GENERATED:effectiveness BEGIN -->
- **Metric:** `-` (unit: `-`, since: `-`, filters: none)
- **Baseline:** 0 over 0 -(s) | **Target:** 0 | **Checks:** 0
- **Results:** none
<!-- GENERATED:effectiveness END -->

- `TestPushOnly_ADryRunLeavesOriginUnchanged` failed before the fix: the dry run moved `origin/main` in a test repository.

## Lessons

- A preview flag must stop every write at the layer that owns the write, not only at the caller.
- Read each row of a golden file when the behavior is a contract. A golden test can pin a defect.
