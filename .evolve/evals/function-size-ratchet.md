---
score_cap:
  - criterion: "An unlisted function past 50 lines fails the ratchet naming its key; a function of exactly 50 lines passes"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestC1726_001_NewFunctionPastFiftyLinesFailsAndExactlyFiftyPasses$' -tags acs ./acs/cycle1726/"
  - criterion: "A listed offender that grows one line past its checked-in allowance fails the ratchet; one at its allowance passes"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestC1726_002_ListedOffenderGrowingPastItsAllowanceFails$' -tags acs ./acs/cycle1726/"
  - criterion: "Allowances only shrink: a shrunk offender must lower its allowance, a healed or deleted offender must leave the list, and every violation is reported at once"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestC1726_003_AllowancesOnlyShrinkAndStaleEntriesFail$' -tags acs ./acs/cycle1726/"
  - criterion: "Walk measures every non-test function declaration (doc comment excluded, any build constraint) under the root, keys methods by receiver, skips vendor/testdata/dot/underscore dirs, and errors on unparsable source"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestC1726_004_WalkMeasuresEveryNonTestFunctionUnderTheRoot$' -tags acs ./acs/cycle1726/"
  - criterion: "LoadOffenders reads the flat JSON key->lines list and rejects a missing file, malformed or non-object JSON, non-integer allowances, and allowances not over 50"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestC1726_005_LoadOffendersReadsTheFlatListAndRejectsBadInput$' -tags acs ./acs/cycle1726/"
  - criterion: "The checked-in go/internal/sizeratchet/offenders.json is tracked and equals an independent go/ast census of every current offender at its exact size"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestC1726_006_CheckedInListIsExactlyTheLiveOffenderCensus$' -tags acs ./acs/cycle1726/"
  - criterion: "The package's own tests, run with CI's tags, pass a copy of the module and fail it once a new 51-line function lands or consumeCommittedItems grows one line"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -run '^TestC1726_007_CIRatchetTestFailsOnARealModuleRegression$' -tags acs ./acs/cycle1726/"
  - criterion: "internal/sizeratchet is enrolled in go/.apicover-enforce, has a tracked apicover_named_test.go, and passes apicover -enforce"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestC1726_008_SizeratchetGraduatesIntoTheApicoverEnforcedSet$' -tags acs ./acs/cycle1726/"
  - criterion: "The repo-wide ratchet is green on the current tree under the CI recipe"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -tags integration ./internal/sizeratchet/"
---

# Eval: The 50-line function limit is one repo-wide ratchet

> Pins design doc §7.9 G2 (inbox item `function-size-ratchet`). Before this
> change, the 50-line limit was enforced only by packages that call
> `test/structure.CheckLimits` on their own directory. That let a bugfix cycle
> (1720) leave `ship/consume.go`'s `consumeCommittedItems` and
> `(*itemConsumer).stage` far over the limit with no test objecting.
> `go/internal/sizeratchet` walks the whole module with go/ast and holds a
> checked-in offender list of the 299 pre-existing violators at the start of
> cycle 1726. A new function past 50 lines fails CI. A listed offender fails
> CI if it grows, and its allowance must drop when it shrinks, so the list
> can only get shorter. The anti-no-op boundary is predicate 007: it runs the
> package's own tests under CI's tags against a module copy with a planted
> regression. A skipped test, a package-local walk, or a Check that ignores
> growth all leave that predicate red. Source incident: cycle 1720 (the
> ship/consume.go growth), cataloged in docs/architecture/logic-first-delivery-design.md §5.10.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| new-offender | unlisted 51-line function fails; 50 passes | 8/10 | `TestC1726_001_NewFunctionPastFiftyLinesFailsAndExactlyFiftyPasses` |
| growth | listed offender +1 line fails | 8/10 | `TestC1726_002_ListedOffenderGrowingPastItsAllowanceFails` |
| shrink-only | slack/stale entries fail; collect-all | 7/10 | `TestC1726_003_AllowancesOnlyShrinkAndStaleEntriesFail` |
| walk-scope | repo-wide non-test walk, receiver keys, skip rules, parse errors | 7/10 | `TestC1726_004_WalkMeasuresEveryNonTestFunctionUnderTheRoot` |
| list-input | LoadOffenders boundary validation | 5/10 | `TestC1726_005_LoadOffendersReadsTheFlatListAndRejectsBadInput` |
| seeded-list | checked-in list == independent live census | 8/10 | `TestC1726_006_CheckedInListIsExactlyTheLiveOffenderCensus` |
| ci-reachability | CI-run test fails on a real planted regression | 9/10 | `TestC1726_007_CIRatchetTestFailsOnARealModuleRegression` |
| apicover | package graduates into the enforced apicover set | 6/10 | `TestC1726_008_SizeratchetGraduatesIntoTheApicoverEnforcedSet` |
| durable-green | ratchet green under the CI recipe | 9/10 | `go test -tags integration ./internal/sizeratchet/` |
