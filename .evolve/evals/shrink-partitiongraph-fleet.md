---
score_cap:
  - criterion: "internal/fleet.PartitionGraph is <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_001_TwoTargetFunctionsFitTheRatchetLimit ./acs/cycle1752"
  - criterion: "The shrink extracts logic into a helper: PartitionGraph has fewer non-blank lines than its baseline 49, so deleting its 4 blank lines alone does not count"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_002_ShrinkExtractsCodeRatherThanStrippingBlankLines ./acs/cycle1752"
  - criterion: "go/internal/sizeratchet/offenders.json is byte-unchanged since aedac036 and still lists internal/fleet.PartitionGraph at 53. An allowance is a ceiling, so a lane never edits the file"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_003_OffendersJSONLeftUnchanged ./acs/cycle1752"
  - criterion: "The module-wide sizeratchet.Check reports zero problems, so no extracted helper lands past 50 lines unlisted"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_004_ModuleWideRatchetCheckPasses ./acs/cycle1752"
  - criterion: "Every baseline test function under go/internal/fleet survives byte-identical; characterization tests are added beside them"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_005_BaselineTestFunctionsPreserved ./acs/cycle1752"
  - criterion: "go test -count=1 ./internal/fleet passes"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_006_TargetPackageTestsPass ./acs/cycle1752"
  - criterion: "go vet and gofmt are clean on go/internal/fleet"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_007_TargetPackagesVetAndGofmtClean ./acs/cycle1752"
  - criterion: "No comment line is added under go/internal/fleet: `commentaudit comments -base aedac036` exits 0 on production and test files alike"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_008_NoCommentLinesAdded ./acs/cycle1752"
  - criterion: "A non-test source under go/internal/fleet changed, and its comment set equals the baseline's, trailing comments included. The two trailing comments on owner and gzBucket stay with their code"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_009_ShrunkSourcesChangedAndKeepEveryComment ./acs/cycle1752"
  - criterion: "The fleet suite passes with the baseline PartitionGraph substituted back, and it kills 7 behavior mutants that the baseline suite lets survive: n<1 clamp, wrapped failure cause, nil result on failure, pull-in after a global-zone todo in bucket 0, the global-zone bucket claim, deferral of a two-bucket conflict, ownership credited to the chosen bucket"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_010_PartitionGraphCharacterizationKillsBaselineMutants ./acs/cycle1752"
  - criterion: "PartitionGraph gives the same buckets, deferred list and error text as its baseline over an 18-scenario table (n in {-1,0,1,2,3}, global-zone first/last/mixed, two-bucket conflicts, empty file lists, a missing file first and last)"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_013_PartitionGraphMatchesBaselineOverAScenarioTable ./acs/cycle1752"
  - criterion: "No file on the guards.IsProtectedSurface manifest changed since aedac036"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_014_NoProtectedSurfaceTouched ./acs/cycle1752"
  - criterion: "The cycle explanation document (docs/explain/builds/cycle-1752-01m3nkcsxd128fns9f5xs8mx93.md) states no order between the characterization tests and the shrink: no sentence relates tests to the shrink with before/after/first/then, since the build transcript shows the extraction first and the tree cannot prove any order"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_015_ExplanationDocumentClaimsNoTestFirstOrder ./acs/cycle1752"
  - criterion: "The explanation document's ## Summary keeps the true claim that the characterization tests kill the baseline mutants and grounds it in predicates 010 and 011"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_016_ExplanationSummaryGroundsTheMutantKillClaimInPredicates010And011 ./acs/cycle1752"
---

# Eval: shrink-partitiongraph-fleet

> Pins the behavior-preserving extraction of `internal/fleet.PartitionGraph`
> to <=50 lines. At base `aedac036` it is 53 lines, 3 over the ratchet. The
> triage action names the owning-bucket resolution block as the one to move
> into a helper such as `resolveOwningBuckets`. Its 4 blank lines make a
> blank-strip shrink possible, so predicate 002 requires fewer non-blank
> lines than the baseline's 49. `offenders.json` stays byte-unchanged under
> the goal's wave-25 ceiling rule. A baseline mutation probe found 7
> one-line mutants that the existing `PartitionGraph` tests do not kill,
> because nothing asserted deferral, error wrapping, the n<1 clamp or the
> ordering of a global-zone todo placed first. Before this contract was
> frozen, all 7 were killed in an isolated scratch clone by throwaway tests
> that pass on the baseline. A trial extraction there turned all 14
> cycle-1752 predicates GREEN. Negative probes confirmed that each guard
> predicate goes RED when its target is broken. The probe substitutes
> mutants of the *baseline* text into the current package through
> `go test -overlay`. So `PartitionGraph` must keep its name and signature,
> and the helpers the baseline calls (`splitGlobalZone`,
> `TransitivePackageSet`, `leastLoaded`, `only`) must stay. Source: cycle 1752
> triage top_n `shrink-partitiongraph-fleet`. There is no inbox record; the
> Task Contract names this eval as the authority.

Audit round 1 of cycle 1752 (H1) rejected the explanation document for
saying the characterization tests were added "before the shrink"; the
build transcript shows the extraction was written first. Predicates 015
and 016 pin the correction: no test-versus-shrink order claim anywhere in
the document, and a Summary that grounds the mutant kill in predicates
010 and 011.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | PartitionGraph <=50 lines | 8/10 | `go test -run TestC1752_001...` |
| real-extraction | fewer non-blank lines than baseline 49 | 7/10 | `go test -run TestC1752_002...` |
| ceiling-untouched | offenders.json byte-unchanged | 8/10 | `go test -run TestC1752_003...` |
| ratchet-gate | module-wide ratchet Check passes | 9/10 | `go test -run TestC1752_004...` |
| tests-frozen | baseline fleet test funcs byte-identical | 7/10 | `go test -run TestC1752_005...` |
| behavior-preserved | `go test ./internal/fleet` passes | 8/10 | `go test -run TestC1752_006...` |
| vet-clean | vet + gofmt clean | 6/10 | `go test -run TestC1752_007...` |
| no-comments-added | commentaudit lists zero added lines | 8/10 | `go test -run TestC1752_008...` |
| comments-kept | source changed, comment set unchanged | 7/10 | `go test -run TestC1752_009...` |
| characterization | suite kills 7 baseline PartitionGraph mutants | 8/10 | `go test -run TestC1752_010...` |
| differential | matches baseline over 18 scenarios | 9/10 | `go test -run TestC1752_013...` |
| scope | no protected surface touched | 6/10 | `go test -run TestC1752_014...` |
| explanation-no-false-order | explanation doc relates no test to the shrink in time | 7/10 | `go test -run TestC1752_015...` |
| explanation-grounded-claim | Summary cites predicates 010/011 for the mutant kill | 6/10 | `go test -run TestC1752_016...` |
