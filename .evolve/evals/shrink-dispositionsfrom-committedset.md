---
score_cap:
  - criterion: "internal/committedset.DispositionsFrom is <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_001_TwoTargetFunctionsFitTheRatchetLimit ./acs/cycle1752"
  - criterion: "The shrink extracts logic out of DispositionsFrom: it has fewer non-blank lines than its baseline 57"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_002_ShrinkExtractsCodeRatherThanStrippingBlankLines ./acs/cycle1752"
  - criterion: "go/internal/sizeratchet/offenders.json is byte-unchanged since aedac036 and still lists internal/committedset.DispositionsFrom at 57. An allowance is a ceiling, so a lane never edits the file"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_003_OffendersJSONLeftUnchanged ./acs/cycle1752"
  - criterion: "The module-wide sizeratchet.Check reports zero problems, so no extracted helper lands past 50 lines unlisted"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_004_ModuleWideRatchetCheckPasses ./acs/cycle1752"
  - criterion: "Every baseline test function under go/internal/committedset survives byte-identical; characterization tests are added beside them"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_005_BaselineTestFunctionsPreserved ./acs/cycle1752"
  - criterion: "go test -count=1 ./internal/committedset passes"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_006_TargetPackageTestsPass ./acs/cycle1752"
  - criterion: "go vet and gofmt are clean on go/internal/committedset"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_007_TargetPackagesVetAndGofmtClean ./acs/cycle1752"
  - criterion: "No comment line is added under go/internal/committedset: `commentaudit comments -base aedac036` exits 0 on production and test files alike"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_008_NoCommentLinesAdded ./acs/cycle1752"
  - criterion: "A non-test source under go/internal/committedset changed, and its comment set equals the baseline's. The F30 doc comment on DispositionsFrom is neither cut nor rewritten"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_009_ShrunkSourcesChangedAndKeepEveryComment ./acs/cycle1752"
  - criterion: "The committedset suite passes with the baseline DispositionsFrom substituted back, and it kills 7 behavior mutants that the baseline suite lets survive: a mistyped decision answers for nothing, id trim, evidence trim, blank escalation reason, reason-over-fail_count precedence, negative fail_count, sha trim"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_011_DispositionsFromCharacterizationKillsBaselineMutants ./acs/cycle1752"
  - criterion: "DispositionsFrom returns the same answers as its baseline over 11,034 generated decision bodies: every ordered pair of 105 single-bucket entries, plus 9 hand-written malformed, mistyped and ignored-bucket bodies"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1752_012_DispositionsFromMatchesBaselineOverAGeneratedCorpus ./acs/cycle1752"
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

# Eval: shrink-dispositionsfrom-committedset

> Pins the behavior-preserving extraction of
> `internal/committedset.DispositionsFrom` to <=50 lines. At base `aedac036`
> it is 57 lines, 7 over the ratchet, with no blank lines. The triage action
> is to extract its four per-bucket append loops into named helpers.
> Hoisting the anonymous decision struct into a named package type is an
> equally valid shrink; the contract pins behavior, not shape.
> `offenders.json` stays byte-unchanged under the goal's wave-25 ceiling rule.
> The scout's "malformed-JSON edge test" is already pinned at base by
> `TestDispositionsFrom_ReadsTheDecisionBody` (`{not json` gives nil). The
> stronger unpinned case is a *well-formed* decision with one mistyped field:
> `json.Unmarshal` fills the good buckets and still returns an error. That is
> mutant 1. A baseline mutation probe found 7 one-line mutants that the
> existing `Dispositions*` tests do not kill: no test used padded ids,
> reasons or shas, or combined a reason with a fail_count. Before this
> contract was frozen, all 7 were killed in an isolated scratch clone by
> throwaway tests that pass on the baseline. A trial extraction there turned
> all 14 cycle-1752 predicates GREEN. `DispositionsFrom` must keep its name
> and signature. Source: cycle 1752 triage top_n
> `shrink-dispositionsfrom-committedset`. There is no inbox record; the Task
> Contract names this eval as the authority.

Audit round 1 of cycle 1752 (H1) rejected the explanation document for
saying the characterization tests were added "before the shrink"; the
build transcript shows the extraction was written first. Predicates 015
and 016 pin the correction: no test-versus-shrink order claim anywhere in
the document, and a Summary that grounds the mutant kill in predicates
010 and 011.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | DispositionsFrom <=50 lines | 8/10 | `go test -run TestC1752_001...` |
| real-extraction | fewer non-blank lines than baseline 57 | 7/10 | `go test -run TestC1752_002...` |
| ceiling-untouched | offenders.json byte-unchanged | 8/10 | `go test -run TestC1752_003...` |
| ratchet-gate | module-wide ratchet Check passes | 9/10 | `go test -run TestC1752_004...` |
| tests-frozen | baseline committedset test funcs byte-identical | 7/10 | `go test -run TestC1752_005...` |
| behavior-preserved | `go test ./internal/committedset` passes | 8/10 | `go test -run TestC1752_006...` |
| vet-clean | vet + gofmt clean | 6/10 | `go test -run TestC1752_007...` |
| no-comments-added | commentaudit lists zero added lines | 8/10 | `go test -run TestC1752_008...` |
| comments-kept | source changed, comment set unchanged | 7/10 | `go test -run TestC1752_009...` |
| characterization | suite kills 7 baseline DispositionsFrom mutants | 8/10 | `go test -run TestC1752_011...` |
| differential | matches baseline over 11,034 bodies | 9/10 | `go test -run TestC1752_012...` |
| scope | no protected surface touched | 6/10 | `go test -run TestC1752_014...` |
| explanation-no-false-order | explanation doc relates no test to the shrink in time | 7/10 | `go test -run TestC1752_015...` |
| explanation-grounded-claim | Summary cites predicates 010/011 for the mutant kill | 6/10 | `go test -run TestC1752_016...` |
