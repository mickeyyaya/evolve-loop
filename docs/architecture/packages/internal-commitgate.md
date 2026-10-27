# internal/commitgate

> Operator view: [`/evo:commit`](../../../skills/commit/SKILL.md) and the commit-gate row of [runtime-reference.md](../../operations/runtime-reference.md). The comment-removal waiver's rule and the history archive's: [code-comments.md](../../conventions/code-comments.md) (Enforcement). The attestation's reader: `internal/phases/ship/commitgate.go` ([internal-phases-ship.md](internal-phases-ship.md)). Decisions: [ADR-0111](../adr/0111-code-carries-no-comments.md), [ADR-0115](../adr/0115-commit-gate-keeps-history-and-records-waivers.md).

## Purpose

`internal/commitgate` is the pre-commit quality gate that the `/commit` skill runs as `evolve commit-gate run`. It does these steps:

1. It refuses what never commits (a compiled executable, an added comment, a removed history comment no archive page records).
2. It detects the languages of the changed files.
3. It checks that a simplifier and one reviewer were declared (unless the change is a proven comment removal).
4. It runs lint and targeted tests per language.
5. On a full pass, it writes `.commit-gate/attestation.json` bound to the tree-state SHA.

`evolve ship --class manual` is the reader that enforces that attestation. It is the Go port of the deleted `commit-gate-runner.sh`.

## Design

- **Exit codes** (`TestExitCodeContract`):
  - 0: pass, with the attestation written.
  - 1: a lint or test failure, an unmet reviewer precondition, a refusal, or nothing to gate. A refusal is an executable, an added comment, an unrecorded history removal, or a changed Go file that does not parse.
  - 2: a git or SHA fatal:
    - not a repo;
    - `git diff HEAD` exiting above 1;
    - no sha256 tool;
    - a binary-guard error;
    - an unreadable HEAD;
    - an attestation write failure.
  - 3: a required tool is missing and was not installed.
  - 10: bad CLI arguments (the cmd layer returns it).
- **`Run` order:**
  1. the change set (listed under `Run`'s context, `TestRun_ListsTheChangeUnderItsOwnContext`);
  2. what never commits (`refuseWhatNeverCommits`). This is the binary guard, then the chain of added-comment, unrecorded-history and rewritten-archive refusals. Each lists the change itself;
  3. language detection;
  4. the review waiver or the reviewer precondition (`reviewDecision`, which returns the waiver to record);
  5. the language lanes in sorted order;
  6. the attestation (whose writer logs the PASS line).

  Every failure maps onto an exit code. `Run` never panics on a missing tool, a lint failure or a git error.
- **The change set** is the `--files` override (whitespace-separated) or `git diff --name-only HEAD`, with blank lines dropped. An empty set is ExitFail: nothing to gate.
- **The binary guard** runs `binaryguard.Scan` at its default threshold over the change set before any other work. So a large compiled executable (a stray `go build` output) is rejected repo-wide, named, with no attestation and no further subprocess (`TestRun_ACompiledExecutableIsRefusedBeforeAnyOtherWork`). A scan error, such as a path it cannot stat, is ExitGitFatal (`TestRun_AnUnscannablePathIsAFault`).
- **The tracked plugin binary** `go/evolve` (ADR-0094) gets its own refusal. A bare `go build ./cmd/evolve` in `go/` overwrites it, and advice to add it to `.gitignore` is wrong for a tracked file. So the refusal names the restore command `git checkout HEAD -- go/evolve`. It also names the dev build target `make -C go build`, which writes `go/bin/evolve`. Pinned by `TestRefuseBinaries_AnOverwrittenPluginBinaryNamesItsRestoreCommand`.
- **Language detection** maps `go` → go, `py` → python, `ts`/`tsx` → ts, `js`/`jsx`/`mjs`/`cjs` → js, `rs` → rust, sorted and unique (`TestDetectLangs_ExtensionMapping`).
- **The reviewer precondition** needs a simplify capability and a review capability.
  - The simplify capability is `code-simplifier`, `code-review-simplify` or `refactor`.
  - The review capability is `code-reviewer`, `code-review`, `code-review-simplify`, or the `<lang>-reviewer` / `<lang>-review` of a changed language. There, ts and js map to `typescript`.
  - One review capability is enough, and `code-review-simplify` satisfies both.
  - Each declared reviewer is normalized by dropping everything through its last `:` (so `ecc:go-reviewer` counts as `go-reviewer`) and all ASCII whitespace. Only the check normalizes: `reviewers_run` records the raw spellings with empties dropped.
  - Pinned by `TestReviewersSatisfied`, `TestSplitReviewers_RawVerbatim`, `TestRun_GeneralReviewerAlone_ExitPass` and `TestRun_EccPrefixedReviewer_ExitPass`.
- **A review run** is a token `run:<run label>` in `--reviewers` (`resolveReviewRuns`, `review_runs.go`). The design is [agent-run.md](../agent-run.md) §7.
  - The gate computes the tree-state SHA once and reads the launch record of each run (`launchrecord.Read`).
  - It refuses the commit (ExitFail, `DENY: review run <label> does not attest this commit`) when the record is absent or unreadable, or holds another run id.
  - It also refuses a record with no driver, a writer record, a verdict other than `PASS` or `WARN`, no report hash, or another tree (`Record.BoundTo`).
  - A tree-state error is ExitGitFatal. A gate with no `run:` token reads no record and no extra tree.
  - Each accepted token becomes the profile name of the run, so the reviewer precondition reads it like a named reviewer.
  - Pinned by `TestRun_BridgeRunReviewsSatisfyTheGateAndTheAttestationNamesEachRun`, `TestRun_ABridgeRunThatCannotAttestIsDenied` and `TestRun_NamedReviewersAloneReadNoTreeBeforeTheLanes`.
- **A docs-only change** is a change whose every path ends in `.md` (`isDocsOnly`).
  - The gate lists the change itself with `--no-renames` (`listedChange`), not the `--files` override. So a code file renamed to Markdown still lists its old path and needs the code review (`TestRun_ACodeFileRenamedToMarkdownStillNeedsTheCodeReview`). It needs the capability "docs review" in place of simplify and review (`requiredReviewSatisfied`). The skill [docs-review](../../../skills/docs-review/SKILL.md) satisfies it. The code reviewers do not satisfy it, and `docs-review` does not satisfy the code capabilities of a change with any other path. A denied docs-only change logs `missing required review capability: docs review`.
  - Pinned by `TestRequiredReview_ADocsOnlyChangeNeedsTheDocsReviewCapability`, `TestRun_ADocsOnlyChangeCommitsWithTheDocsReviewAloneAndRecordsIt` and `TestRun_ADocsOnlyChangeWithOnlyCodeReviewersIsDenied`.
  - A skill file (`skills/*/SKILL.md`) is Markdown, so it is docs-only. It instructs agents, so the docs review checks its claims too (lens 2).
  - The research: [docs-review-skill-2026-10](../../research/docs-review-skill-2026-10.md).
- **The review waiver** lists the change itself with `git diff --name-only --no-renames HEAD`, independent of `--files`. The waiver needs these conditions:
  - Every path must be a regular file and must not sit under `testdata/`.
  - Every path must be either a Go file that `commentaudit.Equivalent` proves comment-only against HEAD or Markdown under `docs/`.
  - At least one Go file must be proven.

  A denied waiver logs the path and the reason. A granted waiver is recorded in the attestation as `review_waiver: "comment-only"` (`commentOnlyWaiver`). Ship turns it into a `Review-waived: comment-only` trailer. So `git log` tells a waived commit from a `--bypass-commit-gate` one (`TestRun_AWaivedCommitRecordsItsWaiverAndAReviewedOneDoesNot`).
- **Lanes skip files deleted from the working tree**, since a deleted file carries nothing to lint. The Go, Python and Rust lanes skip them through `existingFilesWithExt`. The Node lane skips them through `fileExists` over `nodeExts` (`TestLaneNode_ADeletedFileNeverReachesEslint`, `TestLaneNode_OnlyDeletedFilesRunNoLint`).
- **Go lane:**
  - First, `gofmt -s -l` per changed file (`checkGofmt`). A `gofmt` that fails to run (a parse error or a missing binary) fails the lane with its output. It then records no `go:gofmt` (`TestRun_AFailingGofmtFailsTheGoLane`).
  - Then, per module (the nearest `go.mod`): `go vet`, `golangci-lint run --allow-serial-runners` when it is on PATH, and `go test` over the changed packages.
  - golangci-lint takes a host-wide lock (`golangci-lint.lock` in the temp directory). Without a runner option, it exits with `parallel golangci-lint is running` whenever another run holds it. So the gate went red whenever the operator, another console lane or a test linted at the same moment.
  - `--allow-serial-runners` waits for the lock instead. It keeps runs serialized so they never write the shared analysis cache at once. `--allow-parallel-runners` drops that protection. golangci-lint 2.x supports it (2.13.2 on the development host). Pinned by `TestRun_GolangciLintWaitsForAnotherRunInsteadOfFailing`.
  - The wait needs a bound. Otherwise a suspended or very long run can hold the gate forever. golangci-lint's own `--timeout` does not cover the lock wait, and the gate's context (`guardcmd`) has no deadline.
  - So the lint alone runs under `context.WithTimeout` for `Options.LintBudget`, `defaultLintBudget` (10 minutes) when unset. The budget is long enough for a cold lint of the largest packages plus a wait behind one other run. It is short enough that a stuck lock fails the commit within one coffee break.
  - On the deadline the lane fails with `golangci-lint waited <budget> for another run's lock or its own lint, and was stopped`. Pinned by `TestRun_GolangciLintRunsUnderABoundedContext`, `TestRun_AGolangciLintThatOutwaitsItsBudgetFailsTheLaneLoudly` and `TestRun_TheLintBudgetDefaultsWhenUnset`.
  - The `-s` matches CI's `gofmt -d -s`; plain gofmt passes code CI rejects (`TestRun_GofmtUnformatted_ExitFail`).
  - Packages under `./acs/` (build-tagged predicate packages, gated separately) are excluded, but `./acs` itself is not.
  - A file with no `go.mod` above it lies outside every Go module (the retention rule archives superseded predicate packages under `docs/private/research/archived-*/`). Such a file is gofmt-checked, logged as `outside every Go module`, and never vetted or tested.
  - The `go.mod` and `Cargo.toml` lookup stops at the repository root. So a manifest on the host above it never claims the repository's files (`TestLaneGo_AGoModAboveTheRepoRootDoesNotClaimTheRepositorysFiles`).
  - When every changed Go file is outside a module, only `go:gofmt` is recorded (`TestLaneGo_AnArchivedGoFileOutsideEveryModuleIsFormattedButNotBuilt`, `TestLaneGo_OnlyArchivedGoFilesAreFormattedAndNothingIsBuilt`).
  - The checks are recorded as `go:gofmt`, `go:vet`, `go:golangci-lint` when it ran, and `go:test` (`TestRun_GolangciLintRecordedWhenPresent`).
- **Python lane:** `ruff check` over the changed `.py` files, then `pytest -q` over the changed test files (`test_*.py` or `*_test.py`; `TestIsPyTest`).
- **Node lane:** `eslint`, or `npx eslint` when only npx is present, over the changed ts/tsx/js/jsx/mjs/cjs files. Neither present is ExitToolMissing.
- **Rust lane:** per crate (the nearest `Cargo.toml`), `cargo fmt --check`, `cargo clippy -- -D warnings`, `cargo test`. A `.rs` file with no `Cargo.toml` above it contributes no crate and is skipped, as the Go lane skips a file outside every module (`TestLaneRust_NoCargoTomlAbove_NoCrateFound`).
- **Tool presence.** `ensureTool` honours `ForceMissing` (tools treated as absent regardless of PATH) and `TestInstall` (`ok` or `fail` stands in for the install result). Only a test sets these fields; `evolve commit-gate run` reads no environment variable for them (X18e). `NoInstall` makes every missing tool a hard ExitToolMissing. The two seams come from `Options` fields, never from `Env`, so tests stay hermetic; the unexported `lookPath` is the presence probe tests replace. `Env` is the lint and test subprocesses' environment (nil inherits the parent).
- **The attestation** is `attestation.json` under `AttestDir` (`CG_ATTEST_DIR`; default `<RepoRoot>/.commit-gate`), written atomically. `AttestDirName` is the one home of `.commit-gate`. The cmd layer (`internal/cli/guardcmd/commitgate.go`) reads `CG_ATTEST_DIR` into `Options.AttestDir`. Its fields, in order:
  - `tree_state_sha`: the sha256 of `git diff HEAD` from `internal/treestate.SHA`. That is the function the ship reader uses, so writer and reader agree by construction.
  - `ts`: the UTC write time, informational only.
  - `checks_passed`: in execution order.
  - `reviewers_run`.
  - `review_waiver`: present only when the review waiver applied.
  - `review_runs`: present only when a `run:` token applied. Each entry has the run id, the profile, the driver, the family and the model.
  - `tool`: `shasum` when present, else `sha256sum` (neither is ExitGitFatal).

  The reader (`internal/phases/ship`) decodes into this same `Attestation` type, so the schema has one home. The reader consumes `tree_state_sha`, `reviewers_run` and `review_waiver`. It turns each reviewer into a `Reviewed-by` trailer and the waiver into a `Review-waived` trailer. Ship's tests write their attestations with this package's `Marshal`. `TestAttestation_MarshalWritesValidJSONForAnyReviewerName` round-trips every field, so a writer key that drifts from its struct tag fails a test.
- **The byte layout** is the deleted bash heredoc's. It has a two-space indent, one field per line, inline arrays with no spacing (`["a","b"]`, `[]` when empty) and a trailing newline. `Marshal` encodes every value with `encoding/json`, so any reviewer name writes valid JSON (`TestAttestation_MarshalWritesValidJSONForAnyReviewerName`). The layout is unchanged for an attestation with no waiver (`TestRun_AttestationStableBytes`, `TestRun_EmptyArraysMarshalInline`, the golden). A waiver adds one line before `tool` (`TestAttestation_AWaiverIsWrittenOnlyWhenOneApplied`).
- **`Runner` is exactly `sysexec.RunFunc`**, so production passes `sysexec.DefaultRunner` and tests inject a fake without touching PATH (`TestRunner_AliasIsRunFunc`).

## Invariants

- **A commit that adds a comment is refused.**
  - `refuseAddedComments` runs `commentaudit.AddedAcrossDiff` over the change set (HEAD against the working tree). It is the same rule the build floor and `commentaudit comments` apply. It runs before the review waiver. Otherwise a comment-only *addition* can ride the waiver, since it is AST-equivalent.
  - The change is listed with `--no-renames -z` as well (as the waiver lists it), so:
    - a rename's source reaches the rule;
    - a `git mv` of a commented file moves its comments rather than adding them;
    - a non-ASCII path arrives as git stores it instead of quoted. A quoted `"\303\251.go"` will fail the `.go` check and pass its comments.
  - HEAD is read through `commentaudit.ReadAtBase`. So a path new at HEAD is absent, while an unreadable repository is a fault (`ExitGitFatal`), never a new file.
  - A comment moved between files in the change is not added.
  - The refusal runs with the binary guard in `refuseWhatNeverCommits`.
  - Pinned by `TestRun_ACommitThatAddsACommentIsRefused`, `TestRun_WhatAToolReadsIsNotAnAddedComment`, `TestRefuseAddedComments_ARenamedCommentedFileAddsNoComment` (a real `git mv`), `TestRefuseAddedComments_ANonASCIIPathIsRead` (a real `é.go`), `TestRefuseAddedComments_AnUnreadableHeadIsAFaultNotANewFile` and `TestRefuseAddedComments_AnUnlistableChangeIsAFault`.

- **A commit that removes history-carrying comments records them.**
  - `refuseUnrecordedHistory` (`history_recorded.go`) lists the change as the added-comment refusal does. Then it runs `commentaudit.RemovedHistoryAcrossDiff`. That is the rule `commentaudit history` uses, so a group moved whole elsewhere in the change is not removed. Then it runs `commentaudit.UnrecordedHistory`.
  - Each removed group must appear, as its fenced text, in text the change adds to its package's page under `docs/history/code-comments/`.
  - Each unrecorded group is refused by `file:line` with the command that records it (`recordHistoryCommand`, its one spelling). No attestation is written.
  - A git fault is ExitGitFatal. A changed Go file that does not parse is ExitFail (`historyCheckFault`, through `commentaudit.IsUnparsable`). So the gate never names the parser's error type.
  - The archive pages are `docs/**.md`, so a comment removal that records its history is still waived.
  - Pinned by `TestRun_RemovingHistoryWithoutRecordingItIsRefused`, `TestRun_RemovingHistoryTheChangeRecordsIsWaived`, `TestRun_HistoryMovedWholeToAnotherFileNeedsNoRecord`, `TestRefuseUnrecordedHistory_TheCommandTheRefusalNamesSatisfiesIt` (a real repo: refused, `commentaudit.Main history` run, then passed) and `TestRefuseUnrecordedHistory_AFaultIsNotARefusal`.

- **The history archive only grows, even under the waiver.** A waived commit has no reviewer.
  - So `refuseRewrittenHistory` refuses a change whose archive page no longer starts with its text at HEAD, a deleted page included (`commentaudit.RewrittenHistoryPages`). It names the page.
  - The archive pages are those under `docs/history/code-comments/`, the generated `README.md` index aside.
  - An unreadable HEAD or an unlistable change is ExitGitFatal.
  - Pinned by `TestRun_ARewrittenHistoryPageIsRefusedEvenWhenTheChangeIsWaived` and `TestRefuseRewrittenHistory_AnUnreadableArchiveIsAFault`.

- **The attestation binds to sha256(`git diff HEAD`).** `TestGolden_GoPipelineWritesByteExactAttestation` runs the real pipeline over a real repo, computes the SHA independently and checks both the binding and the ts-normalized byte layout. `TestRun_AttestationStableBytes` and `TestRun_EmptyArraysMarshalInline` pin the layout from the writer side.
- **The gate never mutates the host.** Without a test seam, a missing auto-installable tool is reported as ExitToolMissing with the manual install hint; `Run` never shells out an installer (`TestEnsureTool_ForceMissingSeam`, `TestEnsureTool_NotAutoInstallable`).
- **Only a proven comment removal waives review, and nothing can hide a path from the proof.** A `--files` selection and a git rename cannot narrow the listing. A symlink or a deletion is refused, since a symlink can point a docs- or Go-looking path at content the proof never saw. A `testdata/` fixture is refused because tests read it. Pinned by `TestRun_AProvenCommentOnlyChangeNeedsNoReviewers`, `TestRun_AnyChangeBeyondCommentRemovalStillNeedsReviewers`, `TestRun_ADeniedWaiverSaysWhy` and `TestRun_ARealGitRenameCannotHideItsSourceFromTheWaiver`.
- **`go:vet` and `go:test` are recorded even when every changed package was under `./acs/` and nothing ran**, which keeps `checks_passed` identical to the bash runner's (`TestRun_AcsPackagesExcluded`).

## Findings

- Fixed 2026-10-06 (the real-tmux isolation landing): the Go lane ran `golangci-lint run` with no runner option. So a commit gate went red whenever another golangci-lint on the host held its lock (`parallel golangci-lint is running`). `TestGolden_GoPipelineWritesByteExactAttestation` went red the same way in every floor that ran beside another. A live check with a run holding the lock: plain `run` exited 3; `run --allow-serial-runners` waited about 7 s and passed. See the [incident](../../incidents/2026-10-06-realtmux-tests-share-a-stale-tmux-server.md).
- Fixed 2026-10-05 (the unsatisfiability-lint landing): the Go lane refused any commit that archived Go sources under `docs/`. It failed a file with no `go.mod` above it. So a console landing was not able to follow the retention rule. A cycle commit (which does not run the lane) already followed that rule for cycles 1761 and 1764. The package-key collection moved into `goPackagesByModule`, which also brought `laneGo` under the 50-line limit and retired its size-ratchet allowance.
- Fixed 2026-10-01 (inbox `commitgate-attestation-and-lane-defects`):
  - the writer did not escape values, so a reviewer name with `"`, `\` or a control character wrote invalid JSON;
  - the Node lane passed deleted files to eslint;
  - a failing `gofmt` counted as formatted;
  - the change was listed under `context.Background()`;
  - no test drove the binary guard through `Run`.
- The history refusal matches a removed group by its fenced text on its package's page, not by `file:line`. So a record made against an older base (the line numbers moved) still counts. A block comment that itself holds a fenced copy of another group's text can satisfy that group; no such comment exists.
