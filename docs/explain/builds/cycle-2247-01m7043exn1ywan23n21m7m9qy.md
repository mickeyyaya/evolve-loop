# Build Explanation — Cycle 2247

## Build Binding
- Cycle: 2247
- Base SHA: ae84e4ad2a78cc6767ce1fd13943401f10752624

## Summary
The required workflow now runs once a day on a `schedule` trigger. A scheduled run selects every suite and uses its own concurrency group, so no push cancels it.

## Rationale
During a merge train, GitHub keeps one pending run per group. Main then rarely gets a finished full run. A daily run with a group that holds the run id gives main one complete run each day.

## Changed Areas
- `.github/workflows/required.yml` — adds the daily cron, the `schedule` group arm, and the `schedule` case arm that routes like a manual run.
- `go/internal/mergewindow/mergewindow.go` — the parser accepts the `workflow_dispatch|schedule` arm, so the merge-window reader keeps working.
- `docs/plans/ci-wall-time-2026-10.md` — records decision D28 and component C18.
- `go/internal/ciparity/schedule_workflow_test.go` — pins the cron and the unchanged result graph.
- `go/internal/ciparity/workflows_test.go` — the decoder reads cron entries.
- `go/internal/ciparity/concurrency_event_test.go` — pins the schedule group and the cancel rule.
- `go/internal/ciparity/required_workflow_test.go` — requires the trigger and the group expression.
- `go/internal/ciparity/routing_workflow_integration_test.go` — runs the real routing script for `schedule`.
- `go/internal/mergewindow/mergewindow_test.go` — checks the live table with the schedule arm.

## Design Decisions
The `schedule` event reuses the manual-run route. This adds no new routing branch. The group uses the run id, as the manual run does.

## Verification
`go test -count=1 -tags integration ./internal/ciparity/ ./internal/mergewindow/` passes. The six ACS predicates of cycle 2247 pass.

## Compatibility
The push, pull request and manual groups are unchanged. Cancel in progress stays on for pull requests only.

## Limitations
The tests read the cron and the group expression. They cannot run the GitHub scheduler.
