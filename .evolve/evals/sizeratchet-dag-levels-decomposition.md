---
score_cap:
  - criterion: "internal/dag.Levels is <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner, and keeps its name and signature"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_001_ThreeTargetFunctionsFitTheRatchetLimit ./acs/cycle1750"
  - criterion: "go/internal/sizeratchet/offenders.json is byte-unchanged since d4467628 and still lists internal/dag.Levels at 57 — an allowance is a ceiling, so a lane never edits the file"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_002_OffendersJSONLeftUnchanged ./acs/cycle1750"
  - criterion: "The module-wide sizeratchet.Check reports zero problems — no extracted helper lands past 50 lines unlisted"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_003_ModuleWideRatchetCheckPasses ./acs/cycle1750"
  - criterion: "Every baseline *_test.go under go/internal/dag (dag_test.go, apicover_named_test.go) is unmodified and undeleted; characterization tests go in new _test.go files"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_004_BaselineTestFilesUnchanged ./acs/cycle1750"
  - criterion: "go test -count=1 ./internal/dag passes"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_005_TargetPackageTestsPass ./acs/cycle1750"
  - criterion: "go vet and gofmt are clean on go/internal/dag"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_006_TargetPackagesVetAndGofmtClean ./acs/cycle1750"
  - criterion: "No comment line is added under go/internal/dag — commentaudit comments -base d4467628 exits 0 (production and test files alike)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_007_NoCommentLinesAdded ./acs/cycle1750"
  - criterion: "A non-test source under go/internal/dag changed and no baseline comment line was deleted from it — the isolated-nodes comment moves with its code"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_008_DagSourceChangedWithoutLosingAComment ./acs/cycle1750"
  - criterion: "The dag suite passes with the baseline Levels substituted back and kills all 9 one-line behavior mutants of it the baseline suite let survive (self-dependency and dangling-ref errors folded into the cycle error, swapped error operands, duplicate nodes counted twice, a partial cycle unreported, partial levels returned on error, a non-nil empty leveling)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_009_LevelsCharacterizationKillsBaselineMutants ./acs/cycle1750"
  - criterion: "Levels returns the same levels and the same error text as its baseline, and leaves its inputs unmodified, over every relation on 4 nodes (4096 graphs plus reordered, duplicate-node, empty-list and duplicate-edge variants) and every single-key invalid-reference input"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_010_LevelsMatchesBaselineOverAnExhaustiveCorpus ./acs/cycle1750"
  - criterion: "No file under a protected surface changed since d4467628"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_012_NoProtectedSurfaceTouched ./acs/cycle1750"
---

# Eval: sizeratchet-dag-levels-decomposition

> Pins the behavior-preserving extraction of `internal/dag.Levels` (57 lines
> at base `d4467628`) to <=50 lines, with characterization tests written first.
> `offenders.json` stays byte-unchanged. The goal's wave-25 rule, "a lane never
> edits that file", overrides the scout's criterion 4 ("offenders.json no longer
> lists the entry"), as it did in cycles 1741 and 1749.
>
> The existing suite only checks `err != nil` on the error paths, which leaves
> most of `Levels`' contract unpinned. Before this contract was frozen, a
> baseline mutation probe found 9 one-line mutants the suite does not kill.
> Each one changes a message, swallows a distinct error class into the cycle
> error, or changes nil/empty or partial-result behavior. All 9 were killed by
> a throwaway characterization test that passes on the baseline, and by the
> predicate-010 equivalence harness.
>
> A throwaway extraction that moved the validation loop into a helper measured
> 37 lines for `Levels`. It passed the harness 40 times in a row, which shows
> the harness is deterministic despite Go's randomized map iteration. An
> input-sorting variant of that extraction was caught. The Builder may extract
> however it likes; only `Levels` must keep its name and signature.
>
> Source: cycle 1750 triage top_n `sizeratchet-dag-levels-decomposition`. There
> is no inbox record, so the Task Contract names this eval as the authority.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | Levels <=50 lines | 8/10 | `go test -run TestC1750_001...` |
| ceiling-untouched | offenders.json byte-unchanged | 8/10 | `go test -run TestC1750_002...` |
| ratchet-gate | module-wide ratchet Check passes | 9/10 | `go test -run TestC1750_003...` |
| test-files-frozen | baseline dag tests unmodified | 7/10 | `go test -run TestC1750_004...` |
| behavior-preserved | `go test ./internal/dag` passes | 8/10 | `go test -run TestC1750_005...` |
| vet-clean | vet + gofmt clean | 6/10 | `go test -run TestC1750_006...` |
| no-comments-added | commentaudit lists zero added lines | 8/10 | `go test -run TestC1750_007...` |
| no-comments-deleted | source changed, no baseline comment lost | 7/10 | `go test -run TestC1750_008...` |
| characterization | suite kills 9 baseline Levels mutants | 8/10 | `go test -run TestC1750_009...` |
| equivalence | Levels == baseline over exhaustive corpus | 9/10 | `go test -run TestC1750_010...` |
| scope | no protected surface touched | 6/10 | `go test -run TestC1750_012...` |
