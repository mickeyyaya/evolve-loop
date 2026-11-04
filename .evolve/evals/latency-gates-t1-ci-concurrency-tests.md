---
score_cap:
  - criterion: "a pull_request run gets the pr-number group and cancel-in-progress true, evaluated from required.yml"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestCIConcurrency_PullRequestGroupsByNumberAndCancels$' ./internal/ciparity"
  - criterion: "a push gets the group of its ref and cancel-in-progress false"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestCIConcurrency_PushGroupsByRefAndNeverCancels$' ./internal/ciparity"
  - criterion: "a workflow_dispatch run gets a group for its run id"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestCIConcurrency_DispatchGroupsByRunID$' ./internal/ciparity"
  - criterion: "required.yml and go.yml are both parsed and evaluated for every event"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestCIConcurrency_RequiredAndGoWorkflowsAreEvaluatedForEveryEvent$' ./internal/ciparity"
  - criterion: "an inbox-only diff takes the light route"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -tags integration -run '^TestRequiredRouting_InboxOnlyChangesSkipTheGoMatrix$' ./internal/ciparity"
---

# Eval: CI concurrency is evaluated for each event

> Pins the concurrency group and cancel-in-progress of required.yml and go.yml for pull_request, push and workflow_dispatch. Source: inbox item latency-gates-t1-ci-concurrency-tests, plan docs/plans/latency-gates-2026-10.md section 3 (ADR-0144), cycle 2159.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| pr-group | PR number group, cancel true | 6/10 | `go test -run TestCIConcurrency_PullRequestGroupsByNumberAndCancels` |
| push-group | ref group, cancel false | 6/10 | `go test -run TestCIConcurrency_PushGroupsByRefAndNeverCancels` |
| dispatch-group | run id group | 6/10 | `go test -run TestCIConcurrency_DispatchGroupsByRunID` |
| both-files | required.yml and go.yml evaluated | 7/10 | `go test -run TestCIConcurrency_RequiredAndGoWorkflowsAreEvaluatedForEveryEvent` |
| light-route | inbox-only diff routes light | 6/10 | `go test -tags integration -run TestRequiredRouting_InboxOnlyChangesSkipTheGoMatrix` |
