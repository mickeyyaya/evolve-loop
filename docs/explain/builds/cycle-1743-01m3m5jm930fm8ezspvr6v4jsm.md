# Build Explanation — Cycle 1743

## Build Binding
- Cycle: 1743
- Base SHA: baba8085b240b624af2f7f2b2791119a1a272dc3

## Summary
Four oversized functions now fit the 50-line `sizeratchet.MaxLines` limit. In `internal/auditcalibration` they are `loadPair` (56 → 28 lines) and `render` (67 → 10). In `internal/dossier` they are `Build` (59 → 40) and `SweepOrphans` (53 → 32). Each change is an extract-method refactor that keeps behavior. New characterization tests pin all 54 behavior mutants that the baseline suites let survive (35 in auditcalibration, 19 in dossier). `offenders.json` is not edited: an allowance is a ceiling, so the four entries stay as slack for a later boundary tighten.

## Rationale
Extract-method is the smallest change that gets each function under the limit. The moved lines keep their statements, variable names, format strings and comments, so every mutation anchor of the cycle-1743 predicates still occurs verbatim in an expression that compiles. Leaving `offenders.json` unchanged avoids a rebase conflict with the sibling shrink lane, whose keys sit on adjacent lines.

## Changed Areas
- `go/internal/auditcalibration/auditcalibration.go` — `loadPair` hands the shadow read, parse and validation to `loadShadow`, which returns `*auditchain.ShadowRecord`, and the dossier read, parse and validation to `loadDossier`. The shipped-verdict fallback moves to `shippedVerdict(shadow, d)`, which is called from the `pair` literal; it is a function rather than an inline local so that `d` stays a used parameter when a mutant drops the fallback. `render` becomes six section writers (`writeRules`, `writeMatrix`, `writeDefectClasses`, `writeForceOverrides`, `writeValidPairs`, `writeExclusions`) that take `*bytes.Buffer`. The single tally loop at the top of `render` is split: the matrix tally now runs in `writeMatrix` and the class tally in `writeDefectClasses`. The counts and output bytes are unchanged. `writeRules` takes the two counts in place of `len(pairs)` and `len(exclusions)`.
- `go/internal/dossier/build.go` — `Build` hands the three input checks to `validateBuildInputs`, which returns the same error strings, and the FAIL defect, carryover and failure-record synthesis to `recordAuditFailure(d, cycle, opts.WorkspacePath)`. The "Validate requires a FAIL" comment stays in `Build` above the `verdict == VerdictFail` guard.
- `go/internal/dossier/sweep.go` — `SweepOrphans` hands the dirty-path grouping loop to `groupOrphanPairs`, moved verbatim. The function-local `type pair struct{ json, md string }` is hoisted to package level as `orphanPairPaths`, because a helper's return type cannot be function-local.
- `go/internal/auditcalibration/anchored_behavior_test.go` — new characterization tests: `loadPair`'s exclusion reason and detail for each unusable artifact (unreadable or foreign-cycle shadow, unreadable or missing dossier, foreign-cycle or invalid dossier, malformed fail reason), its verdict trimming, gate, override join and shipped-verdict fallback, and a golden test of `render`'s full output that covers every heading, row order, empty-cell suppression and cell escaping.
- `go/internal/dossier/anchored_behavior_test.go` — new characterization tests: `Build`'s exact input errors, RunID, SkippedPhases and VerdictsNotAdopted pass-through, no defect for a WARN cycle and the exact FAIL defect and carryover; `SweepOrphans`' wrapped enumerate error, ascending sweep order, skipped overflow cycle numbers and the exact error log line naming the pair's directory.
- `go/acs/cycle1743/predicates_test.go` — the TDD phase's acceptance predicates for this lane (not edited by Build).
- `.evolve/evals/sizeratchet-shrink-dossier-auditcalibration.md` — the TDD phase's eval contract for this lane (not edited by Build).

## Design Decisions
Each helper stays in its origin file next to its caller. None is exported and none has a doc comment, because the comment contract forbids adding comment lines. The new tests live in new files because the baseline `_test.go` files are frozen, and they are named `anchored_behavior_test.go` as in the cycle-1738 and cycle-1742 lanes. `render`'s sections stay explicit calls rather than a table of writers, since each section has its own row shape.

## Verification
- All 12 cycle-1743 ACS predicates pass (`go test -tags acs -count=1 -v ./acs/cycle1743`): the size limit, `offenders.json` unchanged, a green module-wide ratchet, frozen baseline tests, both package suites, vet, no comments added or deleted, target docs identical to baseline, and 35/35 auditcalibration and 19/19 dossier mutants killed.
- `gofmt -l` prints nothing for both packages and `go vet` exits 0.

## Compatibility
No exported signature, report byte, log line or error string changes.

## Limitations
The four `offenders.json` allowances remain until a boundary tighten removes them. The new tests pin the 54 mutants from the TDD probe; behavior outside that probe is covered only by the baseline suites.
