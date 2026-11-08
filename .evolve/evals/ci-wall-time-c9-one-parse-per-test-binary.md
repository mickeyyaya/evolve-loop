---
score_cap:
  - criterion: "each bound test file parses once per repocontract test binary, with both whole-tree scanners reading the shared scan"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestStreamParse_EachBoundFileParsesOnceAcrossBothScanners|TestStreamParse_TheTwoScannerTestsUseTheSharedScanAndParseNothingThemselves|TestNoTestWritesIntoTheRealTree|TestPackages_HoldEveryTestThatReadsTheWholeTree)$' ./internal/repocontract"
  - criterion: "the repocontract scan holds one package of ASTs at a time and peaks below 400 MB"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestRepoContract_PeakHeapUnder400MB|TestStreamParse_ReleasesEachUnitBeforeTheNextOne)$' ./internal/repocontract"
  - criterion: "the streamed scan gives the same findings as the old whole-tree scan on the real tree"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestRepoContract_GoldenParity|TestRepoContract_GoldenParityDetectsADivergence|TestStreamParse_ScanFlagsAWriteAndNamesTheReadingTest)$' ./internal/repocontract"
  - criterion: "the default repocontract binary holds no second whole-tree parse: the golden comparison lives behind the acs tag"
    max_if_missing: 7
    evidence: "cd go && ! go test -list '.*' ./internal/repocontract | grep -q '^TestRepoContract_GoldenParity' && go test -tags acs -list '.*' ./internal/repocontract | grep -q '^TestRepoContract_GoldenParity$'"
---

# Eval: CI wall time C9, one parse per test binary

> Pins component C9 of docs/plans/ci-wall-time-2026-10.md (decision D18). Before the change, `TestNoTestWritesIntoTheRealTree` and `TestPackages_HoldEveryTestThatReadsTheWholeTree` each parsed the 4,441 bound test files and held every AST at once (1,222 MB peak RSS in research F3.28). Source incident: cycle 2228. Audit round 1 rejected the first build (H1): the golden test sat in the default binary, re-parsed the tree outside the counter and raised the package wall time from 2.64-2.87 s to 4.48-4.71 s. The golden comparison now runs only with `-tags acs`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| parse-once | the parse counter reads 1 for each bound file | 6/10 | `TestStreamParse_EachBoundFileParsesOnceAcrossBothScanners` |
| bounded-heap | peak heap below 400 MB, one package live at a time | 7/10 | `TestRepoContract_PeakHeapUnder400MB` |
| golden-parity | old and new scanner agree on the real tree | 7/10 | `TestRepoContract_GoldenParity` |
| no-second-parse | the golden test is absent from the default binary and present under `-tags acs` | 7/10 | `go test -list` with and without `-tags acs` |
