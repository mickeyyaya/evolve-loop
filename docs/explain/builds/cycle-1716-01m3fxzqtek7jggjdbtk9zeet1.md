# Build Explanation — Cycle 1716

## Build Binding
- Cycle: 1716
- Base SHA: 447c0744ed0ede94cca4d5d36c01bd2d2a08e597

## Summary
Three value-only `go/pkg/acsassert` readers — `FileContainsAny`, `CountOccurrencesAny`,
`LineContainsAll` — return only a bool/int, so a not-exist read comes back
indistinguishable from "file present, content absent" and the calling ACS
predicate's own failure message says nothing about a moved or renamed file.
This build adds `(value, error)` siblings — `FileContainsAnyChecked`,
`CountOccurrencesAnyChecked`, `LineContainsAllChecked` — that name the path and
carry the existing `movedHint` on a not-exist read (mirroring the
`CountInGoFunc` convention already in the package), and rewires the three
original functions into thin lenient wrappers over their Checked siblings.

## Rationale
`CountInGoFunc` already solved this exact problem for its own reader with a
`(value, error)` return plus `movedHint`; the fix reuses that convention rather
than inventing a second error-shape, and reuses the existing `movedHint`
helper rather than duplicating relocation-hint text. Keeping the original
bool/int functions as thin wrappers (`hit, _ := FileContainsAnyChecked(...)`)
preserves the call signature the 46 existing predicate files depend on, so
callers that don't need the distinction are unaffected and no predicate file
needs to change.

## Changed Areas
- `go/pkg/acsassert/assertions.go` — adds `FileContainsAnyChecked`,
  `CountOccurrencesAnyChecked`, `LineContainsAllChecked`; rewrites
  `FileContainsAny`, `CountOccurrencesAny`, `LineContainsAll` to delegate to
  the new Checked forms and discard the error.
- `go/pkg/acsassert/assertions_test.go` — (TDD-phase, pre-existing in this
  diff) unit tests pinning the Checked forms' moved-hint-on-missing and
  no-hint-on-existing-but-empty behavior.
- `go/acs/cycle1716/predicates_test.go` — (TDD-phase, pre-existing in this
  diff) ACS predicate that runs the real `go/pkg/acsassert` unit suite as its
  production-caller reachability proof.

## Design Decisions
The Checked functions are the single source of truth for the read + hint
logic; the lenient originals are pure delegation with no independent branch,
so there is exactly one place a moved-file bug could hide in this diff.

## Verification
`go test -run 'TestFileContainsAnyChecked_MissingFileHintsAtRelocation|TestCountOccurrencesAnyChecked_MissingFileHintsAtRelocation|TestLineContainsAllChecked_MissingFileHintsAtRelocation|TestFileContainsAnyChecked_ExistingFileNoHintOnMiss|TestCountOccurrencesAnyChecked_ExistingFileNoHintOnZero|TestLineContainsAllChecked_ExistingFileNoHintOnMiss' -count=1 ./pkg/acsassert/...`
passes, the full `go test -count=1 ./pkg/acsassert/...` suite is green, and
`evolve acs suite --cycle 1716` reports `green=168 red=0` including the new
`TestC1716_001_AcsassertCheckedReadersNameMovedFile` predicate.

## Compatibility
`FileContainsAny`, `CountOccurrencesAny`, and `LineContainsAll` keep their
existing signatures and bool/int-only return values; none of the 46 calling
predicate files needed to change.

## Limitations
Only the three named readers gained Checked siblings; no other acsassert
helper's error-reporting behavior changed.
