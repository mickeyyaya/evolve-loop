# internal/rollback

> The command is `evolve rollback <journal.json> [--reason "..."] [--dry-run]` in `internal/cli/opscmd`, which maps the outcome to exit codes; the release pipeline (`internal/releasepipeline`) calls `Run` when a publish fails. This page keeps the package-level detail.

## Purpose

`rollback` reverts a failed release in three independently auditable steps: delete the GitHub release, delete the remote tag, and revert the release commit and push it through `evolve ship --class manual`. It reads the publish journal the release pipeline wrote, runs the steps, and appends one NDJSON outcome line per attempt to `.evolve/release-rollbacks.jsonl` as the audit trail.

## Design

- **Journal.** `ReadJournal` loads the per-publish record and treats an empty `version`, `tag`, `commit_sha` or `branch` as malformed. A missing file is `ErrJournalNotFound`; any other read failure, or bad JSON, is `ErrJournalMalformed`. An empty `Options.JournalPath` is `ErrJournalNotFound` without a lookup.
- **Injectable steps.** `Steps` holds the three step functions. `Run` fills each nil field with its production default (the same three `DefaultSteps` wires), and a dry run replaces all three with announcers that log the intended command and return `dry-run-ok`.
- **Step statuses.** Step 1 (`GhDeleteRelease`): `deleted`, `not-present` (`gh release view` reports not found), `failed` (`gh release view` or `gh release delete` failed) or `skipped` (no `gh` on PATH). Step 2 (`DeleteRemoteTag`): `deleted`, `not-present` (the tag is absent from `git ls-remote --tags origin`) or `failed` (`git ls-remote` failed or the push of `:refs/tags/<tag>` failed); the local tag is deleted best-effort when the remote tag is absent or was deleted. Step 3 (`RevertAndShip`): `reverted`, `local-only` (the revert commit exists but no evolve binary was found or `evolve ship` failed) or `failed` (`git revert --no-edit` failed, so nothing was committed and ship is not reached).
- **Overall outcome.** A run succeeds only when step 3 is `reverted` and neither step 1 nor step 2 is `failed`; `not-present` and `skipped` do not block. Gating on step 3 alone would report success while a release or tag deletion failed, leaving a dangling release or tag. Anything else returns `ErrPartial` naming the three statuses. A dry run always succeeds.
- **Exit codes** (`opscmd.rollbackExitCode`): 0 complete, 1 partial, 2 journal not found or malformed; an argument error exits 10 before `Run`.
- **Ledger.** `LedgerEntry` is the snake_case NDJSON schema downstream tooling parses; the same line comes back as `Result.LedgerEntryJSON`. `LedgerPath` defaults to `<RepoRoot>/.evolve/release-rollbacks.jsonl`. The append is best-effort: a failure logs a `WARN:` line and never changes the outcome. A dry run logs the entry instead of writing it. `appendLedger` creates the parent directory and returns the file's `Close` error on the success path, since a durable append must not swallow a failed flush.
- **Ship binary.** `resolveEvolveBinForRollback` takes `EVOLVE_GO_BIN` when it names an executable file, then `<repoRoot>/go/bin/evolve`, then `evolve` on PATH, the same order as `releasepipeline.resolveEvolveBin`. There is no bash fallback. Ship runs with `EVOLVE_SHIP_AUTO_CONFIRM=1` and the message `revert: <reason> [rollback of v<version>]`.
- **Seams.** `deleteRemoteTagWith` and `revertAndShipWith` take a `gitexec.Git`, so the git half is unit-tested with `fixtures.FakeExec`; the `default*` wrappers bind `gitexec.Default(repoRoot)`. The evolve-binary ship stays a direct exec. `Options.Now` stamps the ledger timestamp. The tests that drive the `default*` wrappers against real `git` or fake scripts mostly live in `*_integration_test.go` behind `//go:build integration` (run with `go test -tags integration -race -count=1 ./internal/rollback/`).

## Invariants

- Any `failed` step, not only a failed revert, blocks overall success, and `local-only` blocks too. Pinned by the `TestRun_PartialRollback_*` tests; `TestRun_SkippedStatuses_StillSuccess` pins that `skipped` and `not-present` do not.
- A failed tag push never falls through to deleting the local tag, which would mask the dangling remote tag. Pinned by `TestDeleteRemoteTagWith_PushFails_ReturnsFailed`.
- A failed revert never reaches ship. Pinned by `TestRevertAndShipWith_RevertFails_ReturnsFailed`.
- A dry run writes no ledger. Pinned by `TestRun_DryRun`.
- A ledger write failure is non-fatal. Pinned by `TestRun_AppendLedgerFailWarns`.
- The journal and ledger JSON keys are snake_case. Pinned by `TestJournal_JSONUnmarshalContract` and `TestLedgerEntry_JSONMarshalKeys`.

## Findings

- `appendLedger` writes the line and its newline in two `Write` calls. `O_APPEND` makes each call atomic but not the pair, so concurrent callers can interleave into merged lines (20 goroutines produced 12 to 16 lines). Rollback is single-threaded, so this is a known gap rather than a defect in use; one `Write` of the line with its newline would close it. `TestAppendLedger_ConcurrentWrites_GapDoc` only logs.
- Remote tag lookup and release view failures fail closed: `git ls-remote` non-zero exit or error reports `failed` (not `not-present`), `gh release view` reports `not-present` only on not-found / 404 and reports `failed` on other errors, and the default `gh` step executes in `RepoRoot`.
- `TestRun_NilSteps_FallbacksAssigned` runs with hermetic fake `gh` and `git` binaries installed on `PATH` and asserts step wiring and directory isolation without executing un-stubbed commands against the live repository.
- `TestDefaultGhDeleteRelease_FakeGhFails_*` strictly assert `failed` for unrecognised `gh` errors and `not-present` for release-not-found messages.
