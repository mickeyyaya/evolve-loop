# internal/rawgitratchet

## Purpose

`internal/rawgitratchet` keeps new raw git fixtures out of the module's tests. A test outside `internal/gittest` that builds its own git repository fails the ratchet unless `baseline.json` lists its file. A listed file's count of `init` sites is a ceiling. The boundary step `ratchet-tighten` lowers the list to the counts of `origin/main`.

## Design

- **One home for fixtures.** `internal/gittest` repositories keep git's detached maintenance child from racing `t.TempDir()` removal. A raw `git init` fixture does not, so it flakes.
- **Bound files are the git-tracked ones.** The scan binds the module's tracked test files, including index-staged ones, so an untracked file never turns the ratchet red ([ADR-0084](../adr/0084-gate-integrity-invariants.md) I1). When git cannot answer (the module is not in a work tree), every on-disk test file is bound instead. That is the strict direction, never bind-none.
- **Site shapes.** A site is an `"init"` string literal passed to a call or listed in a composite literal. The file must be in a directory that holds a bound test file with a `"git"` string literal. A helper in a sibling file that spawns git counts as much as an inline `exec.Command`.
- **A ceiling, not an exact match (2026-10-21).** `Check` fails a file that is not listed and a file that grew past its entry. A file that shrank, or that has no site left, passes. A lane that removes sites thus does not edit `baseline.json`, and two such lanes rebase identically.
  - Before, `Check` demanded equality, so every lane that removed a site edited the one shared file. A rebase of one lane over the other was then never identical. This is the follow-up of `sizeratchet-allowances-not-a-shared-registry` ([internal-sizeratchet.md](internal-sizeratchet.md)).
  - `TightenModule` is the one writer. It calls `sizeratchet.TightenEncoded` with a floor of zero sites: an entry drops to the current count, and an entry with no site left goes away. The boundary step `ratchet-tighten` runs it beside the size tighten and commits both lists in the same pull request.
  - The cost: until the next boundary tighten, a lane can add a site back into the slack of its own file. The size ratchet closes the same gap with a check against the base list in the build floor. The raw-git list has no such check yet.
- **The package's own tests assemble the git binary and the `init` subcommand at run time.** They then hold no raw-init shape for the ratchet to list.

## Invariants

- **An empty binding set is an error**, never a silent pass. Pinned by `TestBoundTestFiles_NoTestFilesIsAnErrorNotAnEmptyPass`.
- **An unparsable bound file fails the scan** instead of dropping out of it. Pinned by `TestSites_UnparsableFileFails`.
- **A listed file missing from disk is skipped.** Pinned by `TestSites_CountsEachShapeOutsideTheOwner`.
- **Directories named `vendor` or `testdata`, or starting with `.` or `_`, are never bound.** Pinned by `TestBoundTestFiles_BindsTrackedOnly`.
- **The list is a ceiling.** Unlisted and grown files each fail by name, and shrunk and migrated files pass. Pinned by `TestCheck_TheBaselineIsACeiling`.
- **Two lanes that shrink sites in different packages rebase identically**, and one boundary tighten then lowers both entries. Pinned by `TestTightenModule_TwoLanesThatShrinkSitesInDifferentPackagesRebaseIdentically` and `TestTightenModule_EachStageFailureIsReturned`.
