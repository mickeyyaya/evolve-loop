# internal/shipmanifest

> Consumers: [internal/phases/ship](internal-phases-ship.md) today; the audit phase next (F43 part 2).

## Purpose

`shipmanifest` is the single source of "which paths Ship commits". A non-release ship never runs a blanket `git add -A`: it stages the paths the cycle's build and TDD reports declare, plus the changed paths those declarations cover. This package holds every pure step of that selection, so a second reader can compute the same tree without importing the ship phase:

- `Declared(workspace, reportFiles)` is the union of the repo-relative paths named in the given phase reports, plus the prior attempt's declarations when the workspace carries a continuation manifest (ADR-0076 slice C).
- `ChangedPaths(porcelain)` parses `git status --porcelain` into the sorted set of paths it names, both sides of a rename included.
- `Stageable(porcelain, manifest, isFile)` is the one composition every caller stages from: `ChangedPaths`, then `pathspec`, minus the paths a `git add` pathspec must not carry (a staged deletion, the source of a staged rename; `gonePaths`, unexported), because naming one fails the whole add with rc=128. It returns a fresh slice. Ship and the audit both call it, so neither can compose the steps differently.
- `RegularFileIn(root)` is the `isFile` rule both callers pass: the path under `root` exists and is a regular file.
- `pathspec` (unexported; reached only through `Stageable`) is the explicit pathspec: every declared entry that is a file on disk, plus every changed path a declared entry covers exactly or by directory prefix, falling back to the changed set when there is no manifest or it covers nothing.
- `OutOfManifest(changed, manifest)` is the manifest gate's question: the changed paths no declaration covers.
- `UnquoteGitPath(token)` decodes one C-quoted path token from git output; every ship reader of git path output decodes through it.

The audit phase will consume this package so that the tree it audits is the tree Ship will commit (F43 part 2). `internal/phases/audit` must not import `internal/phases/ship`, which is why the selection moved here instead of being exported from ship.

## Design

- **Pure, no git.** The package reads phase-report files and the continuation manifest from disk and runs no git command. Callers pass porcelain text in and run `git add` themselves; ship keeps the git execution (`stageExplicitPaths`, `dropIgnoredPaths`, `reconcileManifest`) and its options. The ignored-path drop needs git (`check-ignore`), so it is not here; its home is one git executor Ship and the audit's staged snapshot share (F43 part 2, component 2).
- **The report list is the caller's.** `Declared` takes the report file names as a parameter. Ship passes `manifestReportFiles` (the build and TDD artifact names resolved through `phasecontract`), so the package does not depend on the phase registry.
- **One coverage predicate.** An entry covers a path exactly or as a directory prefix, component-wise (`docs/adr` covers `docs/adr/x.md`, not `docs/adr-other.md`). `pathspec` and `OutOfManifest` share it.
- **One repo-relative guard.** Extraction drops absolute and repo-escaping tokens, and `pathspec` drops them again from a manifest that came from an older serialized source, so an absolute pathspec never reaches git argv.
- **Quote-aware porcelain.** Rename entries are split only on the structural ` -> ` delimiter, never on one inside a quoted filename, and every endpoint is decoded. Decoding happens only for a token wrapped in quotes on both ends; any other token is returned verbatim.

## Invariants

- **Report prose reduces to repo-relative paths.** Slashed paths and bare root files with a known extension are extracted; `./` prefixes normalize; absolute, `../` and ellipsis-glued tokens are dropped; dot-directories survive. Pinned by `TestExtractReportPaths`, `TestExtractReportPaths_BareRootFilenames`, `TestExtractReportPaths_RelativeDotPrefix`, `TestExtractReportPaths_NeverAbsoluteOrParent`, `TestExtractReportPaths_DotDirsPreserved` and `TestIsRepoRelative`.
- **A continuation unions the prior attempt; without one the manifest is unchanged.** Pinned by `TestDeclaredManifest_UnionsPriorAttemptViaContinuation` and `TestDeclaredManifest_NoContinuationIsByteIdentical`.
- **Only the named reports count, and no readable report is an empty manifest.** Pinned by `TestDeclared_ReadsOnlyTheNamedReports`.
- **Coverage is component-wise.** Pinned by `TestOutOfManifest_FlagsUndeclaredPaths` and `TestOutOfManifest_InManifestDiffHasNoFalsePositive`.
- **No non-relative manifest entry reaches the pathspec.** Pinned by `TestStagePathspec_RejectsNonRelativeManifestEntries`.
- **Quoted porcelain decodes to the on-disk path; ASCII porcelain is unchanged.** Pinned by `TestPorcelainChangedPaths_QuotePathUnescapesNonASCII`, `TestPorcelainChangedPaths_QuotePathAsciiUnchanged` and `TestUnquoteGitPath_DecodesOnlyWrappedTokens`.
- **A ` -> ` inside a quoted name is content, not a delimiter; malformed input degrades verbatim.** Pinned by `TestPorcelainChangedPaths_QuotedRenameArrowKeepsBothEndpoints`, `TestStagedGonePaths_QuotedRenameArrowSourceDecodes`, `TestPorcelainChangedPaths_RenameArrowMalformedIsSafe` and `TestPorcelainChangedPaths_OrdinaryRenameArrowUnchanged`.
- **`Stageable` is the pathspec Ship stages.** A changed declared path is named though no file backs it; a declared directory takes only the changes it covers (the cycle-645 leak); a manifest that covers nothing falls back to the changed set; a staged deletion and a staged rename's source are never named; a both-deleted conflict (`DD`) stays named as its resolution. Pinned by `TestStageable_IsThePathspecShipStages` and `TestRegularFileIn_CountsOnlyRegularFilesUnderRoot`.
- **Every export is named by a test** (`.apicover-enforce`).

## Findings

- **2026-09-28, the extraction.** The selection lived in `internal/phases/ship/manifest.go`, so only Ship could answer "which paths will be committed". The audit had no way to bind the same tree without importing the ship phase. The pure functions moved here unchanged, and their tests moved with them under their original names. The behaviour-level staging tests (`TestShipDirect_CycleClass_StagesDeclaredPathsNotAddAll`, `TestStageExplicitPaths_QuotePathDisabledOnGitReads`, the `dropIgnoredPaths` tests) stay in ship.
- **2026-09-28, the review.** The architecture review found the gone-path drop still composed inside ship (a caller building on `Pathspec` alone would name a staged rename's source and fail rc=128, F43's bug class again) and four mutants of the contract surviving the moved suites. `Stageable` now owns the composition and is the only exported door (`pathspec` is unexported), and its table kills three of them; the fourth (`changedSet[d] ||`) was equivalent, because a changed declared path is also staged by the coverage loop, which matches exact paths, so the dead set was deleted.
- **History carried with the code.** The quote-path decoding (cycle 1108), the rename-arrow tokenizer (cycle 1469), the absolute-pathspec guard (cycle 1098), the staged-rename gone set, and the bare-root-file extraction (2026-07-14) were all fixed in ship before the move. Their contracts are the tests listed above.
