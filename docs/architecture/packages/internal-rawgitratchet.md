# internal/rawgitratchet

## Purpose

`internal/rawgitratchet` keeps new raw git fixtures out of the module's tests. A test outside `internal/gittest` that builds its own git repository fails the ratchet unless `baseline.json` lists its file. A listed file's count of `init` sites can only shrink until its entry disappears.

## Design

- **One home for fixtures.** `internal/gittest` repositories keep git's detached maintenance child from racing `t.TempDir()` removal. A raw `git init` fixture does not, so it flakes.
- **Bound files are the git-tracked ones.** The scan binds the module's tracked test files, including index-staged ones, so an untracked file never turns the ratchet red ([ADR-0084](../adr/0084-gate-integrity-invariants.md) I1). When git cannot answer (the module is not in a work tree), every on-disk test file is bound instead. That is the strict direction, never bind-none.
- **Site shapes.** A site is an `"init"` string literal passed to a call or listed in a composite literal. The file must be in a directory that holds a bound test file with a `"git"` string literal. A helper in a sibling file that spawns git counts as much as an inline `exec.Command`.
- **Exact match.** `Check` demands equality. A migration that removes a site must lower or remove its entry, so that a migration frees no slot for a new raw fixture.
- **The package's own tests assemble the git binary and the `init` subcommand at run time.** They then hold no raw-init shape for the ratchet to list.

## Invariants

- **An empty binding set is an error**, never a silent pass. Pinned by `TestBoundTestFiles_NoTestFilesIsAnErrorNotAnEmptyPass`.
- **An unparsable bound file fails the scan** instead of dropping out of it. Pinned by `TestSites_UnparsableFileFails`.
- **A listed file missing from disk is skipped.** Pinned by `TestSites_CountsEachShapeOutsideTheOwner`.
- **Directories named `vendor` or `testdata`, or starting with `.` or `_`, are never bound.** Pinned by `TestBoundTestFiles_BindsTrackedOnly`.
- **The ratchet only turns one way.** Unlisted, grown, shrunk and migrated files each fail by name. Pinned by `TestCheck_ListOnlyShrinks`.
