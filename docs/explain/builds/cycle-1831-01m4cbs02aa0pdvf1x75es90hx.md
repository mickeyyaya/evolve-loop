# Build Explanation — Cycle 1831

## Build Binding
- Cycle: 1831
- Base SHA: 6ac7bf0e6cd15915c0c0aa587efe93c3799facdf

## Summary
`TestC1723_003_CommentsExemptsOnlyANewFilesPackageDoc` now asserts the package-doc rule that train #753 put into `commentaudit comments` (`sparesPackageDoc`, `go/internal/commentaudit/stats.go`). The rule spares a rewrite of an existing package doc while it stays within three lines or within its previous length. It still lists a doc that is added to a file that had none at base. The predicate had expected the pre-#753 rule, which lists every rewrite, so `go test -tags acs ./acs/cycle1723` failed on main.

## Rationale
The rule change in #753 was deliberate. The predicate was the stale side, so the fix is in the predicate and `go/internal/commentaudit` is not changed. The alternative was to archive the cycle1723 package, which the inbox item allowed. I rejected it because the package's other seven predicates are still valid and still guard the `comments` subcommand. Updating only the literal example (q/q.go's one-line rewrite becomes spared) would not be enough: no fixture would exercise the old-length allowance or the upper bound, so a mutant of either half of the rule would survive.

## Changed Areas
- `go/acs/cycle1723/predicates_test.go` — the `existing` scenario of `TestC1723_003` now covers all four halves of the #753 rule. In p/a.go, a doc is added where none was, so it is listed. In q/q.go, a one-line rewrite to one line is spared. In r/r.go, a one-line doc is rewritten to four lines (past three lines and past its old length), so all four lines are listed. In s/s.go, a five-line rewrite to five lines is spared under the old-length allowance. The spared files are checked by an explicit "not listed" assertion.
- `go/acs/cycle1831/predicates_test.go` — TDD phase's three acceptance predicates, including the four-mutant kill test. Build did not change them.
- `.evolve/evals/acs-cycle1723-package-doc-predicate-stale-after-753.md` — TDD phase's eval for the task. Build did not change it.

## Design Decisions
The test keeps its name: `.evolve/evals/commentaudit-reports-every-added-comment.md` pins `^TestC1723_003_CommentsExemptsOnlyANewFilesPackageDoc$` as evidence, and a rename would break that pin. The s/s.go fixture lines (`alpha`/`beta`/`gamma` at base, `uno`/`dos`/`tres` after) do not share text with the r/r.go lines. The reason is that `comments` nets out a base comment line that reappears anywhere in the diff, so a shared line such as `// one,` would hide a listed line from the assertion.

## Verification
`go test -tags acs -count=1 -v ./acs/cycle1831` passes. TestC1831_001 confirms that all eight cycle1723 predicates run and pass. TestC1831_002 confirms that the unmutated control passes and that each of the four `sparesPackageDoc` mutants fails `TestC1723_003`. TestC1831_003 confirms that the commentaudit sources are untouched. `go test -tags acs -count=1 ./acs/cycle1723` passes 8/8. `go vet -tags acs` and `gofmt -l` are clean.

## Compatibility
No production code changed. The predicate's function name, and the eval evidence that references it, stay the same.

## Limitations
The predicate pins the boundary rule through fixtures, not through an exhaustive table of line counts. For example, a rewrite of exactly three lines over a one-line doc is not a fixture.
