---
score_cap:
  - criterion: "required.yml declares a daily schedule trigger and the decoder reads it"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestRequiredWorkflow_RunsTheFullGraphOnceADayOnTheDefaultBranch$' ./internal/ciparity/"
  - criterion: "the schedule concurrency group carries the run id and never cancels in progress"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestCIConcurrency_ScheduleGroupsByRunIDAndNeverCancels|TestRequiredWorkflow_ConcurrencyCancelsOnlySupersededPullRequestRuns)$' ./internal/ciparity/"
  - criterion: "the real changes script selects every suite for a scheduled run whatever the diff"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags integration -run '^TestRequiredRouting_ScheduleSelectsEverySuiteWhateverTheDiff$' ./internal/ciparity/"
  - criterion: "the merge-window parser reads the live routing table with the schedule arm"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestParse_TheLiveRoutingAcceptsAScheduledRunAsAFullRun$' ./internal/mergewindow/"
---

# Eval: one full required run of main each day, which no push cancels

> Pins the daily `schedule` run of `required.yml`. During a merge train GitHub keeps one pending run per group, so main rarely gets a finished full run (plan ci-wall-time-2026-10, D24 and D27). The `schedule` event routes like `workflow_dispatch` and has its own group `<workflow>-schedule-<run id>`. Source incident: cycle 2247, inbox item `ci-main-daily-full-run`, the cancelled runs of 2026-10-26.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| trigger | daily cron on required.yml | 6/10 | `TestRequiredWorkflow_RunsTheFullGraphOnceADayOnTheDefaultBranch` |
| group | schedule group has the run id, no cancel | 7/10 | `TestCIConcurrency_ScheduleGroupsByRunIDAndNeverCancels` |
| routing | every suite selected for schedule | 7/10 | `TestRequiredRouting_ScheduleSelectsEverySuiteWhateverTheDiff` |
| merge window | parser accepts the schedule arm | 6/10 | `TestParse_TheLiveRoutingAcceptsAScheduledRunAsAFullRun` |
