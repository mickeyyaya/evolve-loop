---
score_cap:
  - criterion: "Each of the 4 named functions (internal/aggregator Aggregate, writeCrossCLIVote; internal/apicover Run, exportedSymbols) is <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1744_001_FourAggregatorApicoverFunctionsFitTheRatchetLimit ./acs/cycle1744/..."
  - criterion: "go/internal/sizeratchet/offenders.json is byte-unchanged since the baseline (baba8085) and still lists the 4 keys at 73/64/68/67 — an allowance is a ceiling, so this lane never edits the shared file"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1744_002_OffendersJSONLeftUnchanged ./acs/cycle1744/..."
  - criterion: "The module-wide sizeratchet.Check reports zero problems — no extracted helper lands past 50 lines unlisted and no listed function grows past its allowance"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1744_003_ModuleWideRatchetCheckPasses ./acs/cycle1744/..."
  - criterion: "Every *_test.go and testdata fixture under go/internal/aggregator and go/internal/apicover that exists at baba8085 is unmodified and undeleted; added characterization tests are allowed"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1744_004_BaselineTestFilesUnchanged ./acs/cycle1744/..."
  - criterion: "go test -count=1 ./internal/aggregator passes (behavior preserved through every extraction step)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1744_005_AggregatorPackageTestsPass ./acs/cycle1744/..."
  - criterion: "go test -count=1 ./internal/apicover passes (behavior preserved through every extraction step)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1744_006_ApicoverPackageTestsPass ./acs/cycle1744/..."
  - criterion: "go vet exits 0 and gofmt -l lists nothing on both packages — a line count shrunk by hand-joining statements is not an extraction"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1744_007_BothPackagesVetAndGofmtClean ./acs/cycle1744/..."
  - criterion: "No comment lines are added under go/internal/aggregator or go/internal/apicover — `commentaudit comments -base baba8085` over both dirs exits 0 (production and test files alike)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1744_008_NoCommentLinesAdded ./acs/cycle1744/..."
  - criterion: "The doc comment of each of the 4 target functions is identical to its baseline text"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1744_009_TargetFunctionDocsMatchBaseline ./acs/cycle1744/..."
  - criterion: "Both packages' non-test sources changed and no baseline comment line was deleted from them — the in-body comments of Run and exportedSymbols move with their code (docs/conventions/code-comments.md:43)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1744_010_NoBaselineCommentDeleted ./acs/cycle1744/..."
  - criterion: "The aggregator test suite kills all 34 behavior mutants of Aggregate and writeCrossCLIVote that the baseline suite let survive"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1744_011_AggregatorTestsKillBehaviorMutants ./acs/cycle1744/..."
  - criterion: "The apicover test suite kills all 16 behavior mutants of Run and exportedSymbols that the baseline suite let survive"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1744_012_ApicoverTestsKillBehaviorMutants ./acs/cycle1744/..."
---

# Eval: sizeratchet-shrink-aggregator-apicover

> Pins the extraction refactor of four oversized functions to the
> `sizeratchet.MaxLines` (50) bar: `Aggregate` (73 lines) and
> `writeCrossCLIVote` (64) in `go/internal/aggregator`, and `Run` (68) and
> `exportedSymbols` (67) in `go/internal/apicover`. Behavior must stay the
> same, no comment may be added or stripped, and
> `go/internal/sizeratchet/offenders.json` must stay byte-unchanged. Since
> fix/sizeratchet-is-a-ceiling (2026-09-28) an allowance is a ceiling. Two
> shrink lanes' keys sit on adjacent lines, so editing the file would conflict
> at the fleet rebase. Source: inbox item
> `2026-09-28T05-17-00Z-sizeratchet-shrink-aggregator-apicover.json`, read from
> `.evolve/inbox/processing/cycle-1744/`. Its three acceptance criteria are
> materialized 1:1 in `test-report.md`. Fleet lane of cycle 1744.
>
> Source incidents: the sibling shrink lanes (cycles 1730, 1732, 1738, 1742).
> In cycle 1730 (opscmd) the audit took four rounds because characterization
> and comment discipline were left implicit. A baseline mutation probe (85
> candidates) found 50 behavior mutants that the existing suites do not kill:
> 34 in aggregator and 16 in apicover. The cross-CLI consensus report had no
> golden test: its body, quorum boundary, veto flags and per-CLI labels were
> all unpinned. `Aggregate`'s stderr lines, UTC timestamp and temp-file cleanup
> were unpinned too, and so were `exportedSymbols`' line, doc, package and
> grouped-decl handling. Each of the 50 was killed by a throwaway test that
> passes on the baseline. The real predicates 011/012 went GREEN with those
> tests present before the contract was frozen.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | All 4 functions <=50 lines | 8/10 | `go test -run TestC1744_001...` |
| offenders-untouched | offenders.json byte-unchanged, 4 allowances intact | 8/10 | `go test -run TestC1744_002...` |
| ratchet-gate | Module-wide ratchet Check reports zero problems | 9/10 | `go test -run TestC1744_003...` |
| test-files-frozen | Baseline `*_test.go` and testdata in both packages unmodified/undeleted | 7/10 | `go test -run TestC1744_004...` |
| behavior-preserved (aggregator) | `go test ./internal/aggregator` passes | 8/10 | `go test -run TestC1744_005...` |
| behavior-preserved (apicover) | `go test ./internal/apicover` passes | 8/10 | `go test -run TestC1744_006...` |
| vet-gofmt-clean | `go vet` exits 0 and `gofmt -l` is empty on both packages | 6/10 | `go test -run TestC1744_007...` |
| no-comments-added | `commentaudit comments` lists zero added lines | 8/10 | `go test -run TestC1744_008...` |
| target-docs-intact | 4 target doc comments equal baseline | 5/10 | `go test -run TestC1744_009...` |
| no-comments-deleted | No baseline comment line lost from changed sources | 6/10 | `go test -run TestC1744_010...` |
| characterization (aggregator) | aggregator suite kills all 34 behavior mutants | 7/10 | `go test -run TestC1744_011...` |
| characterization (apicover) | apicover suite kills all 16 behavior mutants | 7/10 | `go test -run TestC1744_012...` |
