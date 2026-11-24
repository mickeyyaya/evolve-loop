# internal/repostate

## Purpose

`internal/repostate` answers one question for repo-wide scanners: which files under a directory does git actually track?

## Design

- Several guard suites and the ship-time persona lint walk on-disk config directories. The runtime also writes untracked stubs into them.
- A scanner that binds every on-disk file reds on state that never reaches a CI checkout.
- The per-name `.gitignore` ratchet re-arms on every new stub. So this package binds the tracked set instead.
- `TrackedSet` returns the basenames of files git tracks directly under the directory.
- Nested entries are excluded, so a nested tracked file cannot basename-alias a same-named top-level stub.
- It includes index-staged files, the stricter direction, so the plane and CI do not skew.
- `TrackedTree` returns every tracked path at any depth and is the binding set for a scanner that walks a whole tree.

## Invariants

- Tracked and staged files appear. Untracked and on-disk-only files do not. Nested entries do not appear in `TrackedSet`. Pinned by `TestTrackedSet_BindsTrackedExcludesUntrackedAndNested`, `TestTrackedSet_StagedUncommittedCounts` and `TestTrackedSet_BindsTrackedFilesThroughRealGit`.
- `TrackedTree` lists tracked and staged files at every depth and never an untracked one. Pinned by `TestTrackedTree_ListsOnlyTrackedFilesUnderTheDir` and `TestTrackedTree_BindsNestedTrackedFiles`.
- An empty directory is a success with an empty set. Pinned by `TestTrackedSet_EmptyDirIsSuccessWithEmptySet`. That is why the caller must treat an unexpectedly empty set as a failure.
- The exit error of git says only `exit status N`. The reason an operator needs (`not a git repository` and so on) is on stderr, so the error carries stderr. Pinned by `TestTrackedFiles_NonRepoErrorCarriesGitStderr` and `TestTrackedTree_NotARepositoryCarriesGitsReason`. A git failure with no stderr is wrapped without it (`TestTrackedTree_SilentGitFailureIsWrappedWithoutStderr`).
- The reference call site of the fallback rule is `internal/phasecoherence/unpaired_test.go`.
- On any git failure the error carries git's stderr. A caller must fall back to binding every on-disk file (the strict direction).
- A caller must treat an unexpectedly empty set as a failure. Going dark unbinds the gate.

## Findings

- None open.

## The working-tree inventory

- **Purpose.** `PresentFiles(ctx, git)` lists the files that are present in the working tree: tracked files and untracked files that git does not ignore. A caller that reads file contents uses it, for example `evolve yaml check`. The paths are sorted and relative to the git directory.
- **Deleted files are excluded.** `git ls-files --cached` still lists a tracked file that was deleted and not staged. `PresentFiles` runs `git ls-files -z --deleted` and removes those paths. A caller thus never reads a file that is not on disk.
- **Precedents.** `go/internal/phases/ship/scanshadow_tree.go:36` and `go/internal/core/ship_recovery.go:257` list `--cached --others` and do not remove the deleted files. They are not changed. `PresentFiles` is the one home of the inventory with the subtraction.
- **Pinned by.** `TestPresentFiles_ListsTrackedAndUntrackedAndSkipsDeletedAndIgnored`, `TestPresentFiles_AGitFailureOutsideARepositoryIsAnError` and `TestPresentFiles_AFailingDeletedListingIsAnErrorAfterTheFirstListing`.
