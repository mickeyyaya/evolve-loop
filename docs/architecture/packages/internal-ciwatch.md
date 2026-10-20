# internal/ciwatch

## Purpose

`internal/ciwatch` closes the feedback edge from GitHub Actions back into the loop. `Watch` polls the CI run of a pushed SHA until it completes or times out. It records the verdict as a workspace artifact that the dossier ingests (`dossier.CIWatchVerdictFile`). On a red conclusion it files a critical fix-forward inbox item that names the failing job or test with a bounded log excerpt. A green run files nothing.

## Design

- **Why it exists.** The `go.yml` matrix runs on ubuntu and macOS, but every local gate runs on macOS only. A failure that happens on ubuntu only is invisible to the local gates, so the loop can ship on top of a red main. The incident record and the history archive hold the dates.
- **No env flags.** The knobs (enabled, timeout, poll interval) come from the `ci_watch` block of `policy.json` (`policy.CIWatchConfig`). A zero `Timeout` or `Poll` falls back to the same compiled defaults (900 seconds and 30 seconds).
- **Seams.** `Fetcher` returns the run status for a SHA. Tests inject a fake, and production uses `NewGHFetcher`. `Now` and `Sleep` are test seams, and nil means the real clock. `execCapture` is the subprocess seam of the fetcher, so no test invokes the live `gh` CLI.
- **`NewGHFetcher`.** It observes the newest run of `ciparity.RequiredWorkflow` for the pushed SHA through the `gh` CLI and the existing `gh` auth. A run that is not visible yet reads as status `queued`, so the watch keeps polling until the timeout.
- **Run states.** `StatusCompleted` is the one terminal status. Every other status means the run is queued or in progress. `ConclusionSuccess` is the one green conclusion. Anything else on a completed run files the escalation item.
- **Bounded excerpt.** `maxLogExcerpt` bounds the log excerpt in the inbox item. The item stays a pointer plus evidence, not a log archive, so a huge CI log cannot bloat the inbox.
- **Best-effort failure detail.** `failedLogSummary` extracts the first failing test name and a bounded excerpt from the failed-job logs. A `gh` error gives empty values, and the escalation item still files, pointing at the run URL.
- **Atomic verdict.** `writeVerdict` writes a temp file and renames it, so a concurrent dossier build never reads a torn file.
- **Escalation shape.** `escalationItem` mirrors the hand-filed `.evolve/inbox` items that triage already reads.
- **Options.** `SHA` and `Fetch` are required. `Cycle` is recorded in the item (0 means unknown). A non-empty `WorkspaceDir` receives the verdict artifact. `ErrWatchTimeout` reports a run that did not complete in time.

## Failure attribution of `ci classify`

`ClassifyTarget` labels the failures of one red run. The failure list of each run comes from `withFailures` in `classify_source.go`. This section tells how that list is made.

- **The problem.** The CI step renders the `go test -json` stream as plain text, and `go test -json` interleaves the output of all packages. Some passing tests also print fixture text such as `--- FAIL: TestC1946_001_CoreStatementCoverageIsFull` and `FAIL\tp`. On 2026-10-20 the text parser listed such fixtures as failures on runs 39381369373, 39385027012, 39386265940 and 39389719309, and it missed the real ones. The plain text cannot tell a printed line from a result line, so no text rule is correct.
- **The test JSON wins.** Each test job uploads its `go test -json` stream as `test-integration.json` in an artifact whose name starts with `coverage-`. `testJSONFailures` lists the run artifacts, downloads each such artifact and reads its `fail` events. A failure row is a failed top-level test of a failed package, or a failed package with no failed test. A test that fails inside a passing package is not a row. Printed text is an `output` event, so it is never a row.
- **Stream limits.** The parser also reads `test-e2e.json` from the same artifact. It counts only packages of the module, because a nested fixture module can write its own events into the stream (for example `gateprobe/internal/probe`). A failed download gets 3 tries.
- **Covered jobs.** The log of each job shows the line `Artifact <name> has been successfully uploaded!`. That line maps each job to its artifact. A job is covered when its own artifact holds test JSON. Only a covered job takes rows from the JSON. Each other job, for example `acs-durable` or a job whose artifact has no JSON, keeps the rows of the text parser for its own lines.
- **Jobs of a JSON row.** Each JSON row goes to the job that uploaded its artifact. If no job log shows the upload, the row keeps the artifact name.
- **Non-test failures.** A failed job with no test row gets one step row. The step is the first `make` error target of the job, for example `make apicover-check`. Else it is the step name of the log. Else the row names only the job.
- **Fallback.** The text parser `parseFailingTests` still runs when the run has no test JSON. It also runs for a rerun (`Attempt` 2 or more), because the artifact can belong to an earlier run of the jobs.
- **Fixtures.** `testdata/cirun/<run>/` holds the trimmed `--log-failed` text and the trimmed test JSON of the four runs. `TestWithFailures_RealCIRunsYieldOnlyTheGoCommandsFailures` pins the rows of each run.

## Invariants

- A green run files nothing. Pinned by `ciwatch_test.go`.
- A red run files one critical item that names the failing test. Pinned by `ciwatch_test.go`.
- Tests never call the live `gh` CLI. The `execCapture` seam enforces this.
- `ci classify` makes no failure row from text that a test prints. Pinned by `classify_testjson_test.go`.

## Findings

- None open.
