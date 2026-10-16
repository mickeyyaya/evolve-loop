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

## Invariants

- A green run files nothing. Pinned by `ciwatch_test.go`.
- A red run files one critical item that names the failing test. Pinned by `ciwatch_test.go`.
- Tests never call the live `gh` CLI. The `execCapture` seam enforces this.

## Findings

- None open.
