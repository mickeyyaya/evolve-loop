# Build Explanation — Cycle 1832

## Build Binding
- Cycle: 1832
- Base SHA: 6ac7bf0e6cd15915c0c0aa587efe93c3799facdf

## Summary
`evolve release` no longer reports success when a journal write fails after the journal is opened. `appendStep` and `setJournalField` now return an error that names the journal path, and every stage of `Run` propagates it under that stage's sentinel. A test now pins that release notes are classified before `Ship` commits the rebuilt binary, so the comment that held this order is gone. `initJournal` drops its never-read `fromTag` parameter, and `resolveInitCommit` treats empty `git rev-list` output as an error instead of returning an empty commit through an unreachable branch.

## Rationale
Rollback reads the journal to know what to undo. A silently incomplete journal turned a recoverable release failure into a manual forensic job, and the operator saw exit 0. Surfacing the write error at the stage where it happens makes the failure visible while the operator still knows what ran. The rejected alternative was to log a warning and carry on: the run would still exit 0 with a journal that rollback cannot trust.

## Changed Areas
- `go/internal/releasepipeline/releasepipeline.go` — `appendStep` and `setJournalField` return the `writeJournal` error wrapped with the journal path; `failPostPublish` joins journal errors into its returned error and still rolls back; `Run` returns `complete`'s error; `initJournal` takes `(Options, time.Time)`; `resolveInitCommit` cuts the first line and errors on an empty one.
- `go/internal/releasepipeline/release_run.go` — a new `recordStep` turns a journal write failure into the stage's sentinel error; pre-publish ok/skip records, the ship record and `commit_sha`, the post-publish ok records and `complete` all propagate it. A failed step whose journal write also fails joins both errors. The why comment above the release-class computation is removed.
- `go/internal/releasepipeline/journal_fail_path_test.go` — Build's tests for the paths the TDD tests leave open: a step failure plus a broken journal keeps both errors and the stage sentinel at pre-publish, ship and post-publish (rollback still runs there), and a journal failure after `Ship` pushed is `ErrPostPublishFailed`, keeps `NewCommitSHA` and does not roll back.
- `go/internal/releasepipeline/releasepipeline_test.go` — the `initJournal` callers drop the `fromTag` argument, and `TestSetJournalField` asserts the new error return instead of discarding it.
- `go/internal/releasepipeline/journal_write_test.go` — the `initJournal` caller drops the `fromTag` argument.
- `go/internal/releasepipeline/journal_write_error_test.go` — TDD phase's `TestRun_JournalWriteError` and `TestResolveInitCommit_EmptyGitOutputIsAnError`, unchanged by Build.
- `go/internal/releasepipeline/classify_before_ship_test.go` — TDD phase's `TestRun_ReleaseClassComputedBeforeShipCommits`, unchanged by Build.
- `go/acs/cycle1832/predicates_test.go` — TDD phase's six acceptance predicates, unchanged by Build.
- `.evolve/evals/releasepipeline-journal-errors-and-classify-pin.md` — TDD phase's eval for the task.
- `docs/architecture/packages/internal-releasepipeline.md` — the journal invariant now states that write errors surface with their stage sentinel. The FromTag entry names the empty-output case. The classification-order entry names the test that pins it.

## Design Decisions
The sentinel follows the release state, not the step name. Before `Ship` a journal failure is `ErrPrePublishFailed` (exit 1), and nothing is published. After `Ship` pushed, it is `ErrPostPublishFailed` (exit 3), because the release exists. `ErrShipFailed` (exit 2) would claim that the push failed. A journal-only failure after publishing does not trigger rollback: the release is sound and the rollback input is the broken file. The operator gets the pushed SHA in `Result.NewCommitSHA` and an error that names the journal path. A step that has run is recorded in `StepsCompleted` before its journal write, so `Result` stays truthful when the write fails. `errors.Join(err, nil)` returns a new error value, not `err` itself, but its `Error()` text is the same as `err.Error()`. The stage wraps format the join with `%v`, so when the journal write succeeds the exact wrapped-label texts pinned by `TestRun_PrePublishStepFailure_WrapsLabelAndJournals` do not change.

## Verification
All six `go/acs/cycle1832` predicates pass. So do `TestRun_JournalWriteError`, `TestRun_ReleaseClassComputedBeforeShipCommits`, `TestResolveInitCommit_EmptyGitOutputIsAnError`, the two new Build tests and the whole `releasepipeline` package. `internal/cli/opscmd` and `internal/rollback` pass, and gofmt and go vet are clean. The new Build tests failed before the change: there was no journal path in the error, and `Run` returned nil after a journal failure at ship.

## Compatibility
The exported API is unchanged; only unexported signatures change. A release whose journal stays writable behaves and exits exactly as before. A run that previously exited 0 with a broken journal now exits 1 or 3.

## Limitations
A journal failure after publishing stops the run before the remaining post-publish steps, so marketplace propagation or release verification is left for the operator to confirm by hand. The `initJournal` write failure path is unchanged and still maps to `ErrPrePublishFailed`.
