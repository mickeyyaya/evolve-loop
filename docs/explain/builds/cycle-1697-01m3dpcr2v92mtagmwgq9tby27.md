# Build Explanation — Cycle 1697

## Build Binding
- Cycle: 1697
- Base SHA: f341bc89847b7962c9642cbe40968ebca341d2fc

## Summary
This build deletes the two dead-red ACS predicate packages, `go/acs/cycle1257` and `go/acs/cycle1259`. It also
closes finding F5 of the 2026-08-04 batch-integrity review with a Closure paragraph that records measured
evidence, including the truthful `skipped_count` outcome.

## Rationale
Commit `fcdd466e` shipped both predicate files from cycles whose audits FAILED. They grade an abandoned
acssuite-internal selection design whose unit tests never existed, so they are red by construction. At base
`f341bc89`, `go test -tags acs` reported 5 FAIL / 1 PASS in cycle1257 and 2 FAIL / 1 PASS in cycle1259, with no
SKIP. Anyone who runs the whole ACS corpus sees seven permanent reds that no code change can fix. Deleting the
files is the smallest change that fully removes that noise.

## Changed Areas
- `go/acs/cycle1257/predicates_test.go` — deleted. It had five red-by-construction predicates that grade the
  phantom acssuite-internal selection machinery. Its sixth predicate, an apicover check, still passed, but
  grades nothing that current gates do not already cover.
- `go/acs/cycle1259/predicates_test.go` — deleted. It had two red-by-construction predicates for the same
  abandoned design, plus one never-narrows check that still passed. It has no production counterpart to protect.
- `docs/operations/batch-integrity-review-2026-08-04.md` — extends the existing F5 section in place with a
  `**Closure — cycle 1697**` paragraph. The paragraph cites both deleted paths and the measured baseline, and it
  corrects the Gap's "skip-guarded" claim: the files FAILed and did not SKIP. It also records that `skipped_count`
  does not drop, and explains why. Issue, Gap and Solution are untouched, and F6 still follows F5.
- `go/internal/core/phase_bindings_selfcheck.go` — `realGoUnitTest` now reports ok when the package's
  directory no longer exists. Without this, the build handoff floor flagged this task's own deletions as
  "unit tests FAIL" (`stat …/acs/cycle1257: directory not found [setup failed]`). The reason:
  `changedGoTestPackages` maps deleted `.go` paths to their package, and `go list -e` returns an empty `Dir`
  for a missing package, so `buildTagVisiblePackages` cannot filter it out. A deleted package has nothing left
  to unit-test. The check reads the filesystem, not `go test` output text, so a package that still exists keeps
  its real verdict.
- `go/internal/core/phase_bindings_selfcheck_test.go` — adds `TestRealGoUnitTest_DeletedPackageIsNotFailure`.
  A missing package directory must report ok, and a present failing package must still report not-ok, so the
  tolerance cannot widen. The test failed with the `directory not found` output before the fix.
- `.evolve/evals/dead-red-acs-corpus-cleanup.md` — the eval the TDD phase authored for this item. The build
  tracks it unchanged, so its score caps ship with the deliverable.
- `go/acs/cycle1697/predicates_test.go` — the TDD phase's ACS predicates for this cycle (removal, F5 closure
  structure, and change-set scope). The build tracks them unchanged and did not edit them.

## Design Decisions
The files are deleted, not tombstoned. The Solution allowed either option. A tombstone would still be a package
that `go test` resolves, and the only honest measurable (`TestC1697_001`) is that the package no longer resolves
at all. The shipped redesign (`internal/regressiontia`, cycle 1260) already has its own tests, so a tombstone
pointing at it adds nothing. The inbox also asked to "verify the EGPS skipped_count drops". That cannot happen,
because EGPS (`acssuite.goLanePatterns`, `go/internal/acssuite/acssuite.go:379-395`) never runs historical cycle
directories. The closure records this, and invents no drop. Historical mentions in `docs/chronicle/` and in the
dated inventories under `docs/research/testing-review-2026-09-14/` are left unchanged, because they are
point-in-time records.

## Verification
`go test -tags acs -count=1 -v ./acs/cycle1697` passes all three predicates: 001 (both packages give
`directory not found`, and no go/acs source references the phantom machinery), 002 (F5 is extended in place with
the closure citations) and 003 (the change set deletes exactly the two dead files and touches no protected
surface). `go test -count=1 ./internal/core/` passes, including the new selfcheck test, and
`evolve selfcheck build` is GREEN when run from a binary built from this tree. `git ls-files go/acs/cycle1257 go/acs/cycle1259` is empty. No Go source outside the deleted files
referenced either package.

## Compatibility
The only production change is the selfcheck runner: it now passes a package whose directory the diff
deleted, and nothing else changes behavior. The EGPS gate never selected these packages, so no cycle verdict, gate count or
`skipped_count` changes. Only a manual full-corpus `go test -tags acs ./acs/...` changes: it loses seven
permanent FAILs.

## Limitations
The build scans only these two packages. It does not search the rest of the historical `go/acs/cycle*` corpus
for other dead-red packages.
