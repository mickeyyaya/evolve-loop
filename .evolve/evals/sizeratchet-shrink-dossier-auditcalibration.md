---
score_cap:
  - criterion: "Each of the 4 named functions (internal/auditcalibration loadPair, render; internal/dossier Build, SweepOrphans) is <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1743_001_FourAuditcalibrationDossierFunctionsFitTheRatchetLimit ./acs/cycle1743/..."
  - criterion: "go/internal/sizeratchet/offenders.json is byte-unchanged since the baseline (baba8085) and still lists the 4 keys at 56/67/59/53. An allowance is a ceiling, so this lane never edits the shared file"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1743_002_OffendersJSONLeftUnchanged ./acs/cycle1743/..."
  - criterion: "The module-wide sizeratchet.Check reports zero problems: no extracted helper lands past 50 lines unlisted, and no listed function grows past its allowance"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1743_003_ModuleWideRatchetCheckPasses ./acs/cycle1743/..."
  - criterion: "Every *_test.go under go/internal/auditcalibration and go/internal/dossier that exists at baba8085 is unmodified and undeleted; added characterization tests are allowed"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1743_004_BaselineTestFilesUnchanged ./acs/cycle1743/..."
  - criterion: "go test -count=1 ./internal/auditcalibration passes (behavior preserved through every extraction step)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1743_005_AuditcalibrationPackageTestsPass ./acs/cycle1743/..."
  - criterion: "go test -count=1 ./internal/dossier passes (behavior preserved through every extraction step)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1743_006_DossierPackageTestsPass ./acs/cycle1743/..."
  - criterion: "go vet exits 0 on ./internal/auditcalibration and on ./internal/dossier"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1743_007_BothPackagesVetClean ./acs/cycle1743/..."
  - criterion: "No comment lines are added under go/internal/auditcalibration or go/internal/dossier: `commentaudit comments -base baba8085` over both dirs exits 0 (production and test files alike)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1743_008_NoCommentLinesAdded ./acs/cycle1743/..."
  - criterion: "The doc comment of each of the 4 target functions is identical to its baseline text (loadPair and render have none and gain none)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1743_009_TargetFunctionDocsMatchBaseline ./acs/cycle1743/..."
  - criterion: "Both packages' non-test sources changed and no baseline comment line was deleted from them. The extraction moves comments with their code and never strips a why (docs/conventions/code-comments.md:43)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1743_010_NoBaselineCommentDeleted ./acs/cycle1743/..."
  - criterion: "The auditcalibration test suite kills all 35 behavior mutants of loadPair and render that the baseline suite let survive"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1743_011_AuditcalibrationTestsKillBehaviorMutants ./acs/cycle1743/..."
  - criterion: "The dossier test suite kills all 19 behavior mutants of Build and SweepOrphans that the baseline suite let survive"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1743_012_DossierTestsKillBehaviorMutants ./acs/cycle1743/..."
---

# Eval: sizeratchet-shrink-dossier-auditcalibration

> Pins the extraction refactor of four oversized functions to the
> `sizeratchet.MaxLines` (50) bar:
>
> - `loadPair` (56 lines) and `render` (67) in `go/internal/auditcalibration`
> - `Build` (59) and `SweepOrphans` (53) in `go/internal/dossier`
>
> Behavior must stay the same, no comment may be added or stripped, and
> `go/internal/sizeratchet/offenders.json` must stay byte-unchanged. Since
> fix/sizeratchet-is-a-ceiling (2026-09-28) an allowance is a ceiling. Two
> shrink lanes' keys sit on adjacent lines, so editing the file would conflict
> at the fleet rebase.
>
> Source: inbox item
> `2026-09-28T05-18-00Z-sizeratchet-shrink-dossier-auditcalibration.json`, read
> from `.evolve/inbox/processing/cycle-1743/`. Its three acceptance criteria are
> materialized 1:1 in `test-report.md`. Fleet lane of cycle 1743.
>
> Source incidents:
> - The opscmd lane (cycle 1730) took four audit rounds because
>   characterization and comment discipline were left implicit.
> - The gc (1732) and setup-releasepipeline (1742) lanes fixed that by
>   encoding both from the start.
>
> A baseline mutation probe of 89 candidates found 54 behavior mutants that
> the existing suites do not kill: 35 in auditcalibration and 19 in dossier.
> auditcalibration's suite has only three tests, and none of them pins
> `render`'s layout or `loadPair`'s exclusion labels.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | All 4 functions <=50 lines | 8/10 | `go test -run TestC1743_001...` |
| offenders-untouched | offenders.json byte-unchanged, 4 allowances intact | 8/10 | `go test -run TestC1743_002...` |
| ratchet-gate | Module-wide ratchet Check reports zero problems | 9/10 | `go test -run TestC1743_003...` |
| test-files-frozen | Baseline `*_test.go` in both packages unmodified/undeleted | 7/10 | `go test -run TestC1743_004...` |
| behavior-preserved | `go test ./internal/auditcalibration` passes | 8/10 | `go test -run TestC1743_005...` |
| behavior-preserved | `go test ./internal/dossier` passes | 8/10 | `go test -run TestC1743_006...` |
| vet-clean | `go vet` on both packages exits 0 | 6/10 | `go test -run TestC1743_007...` |
| no-comments-added | `commentaudit comments` lists zero added lines | 8/10 | `go test -run TestC1743_008...` |
| target-docs-intact | 4 target doc comments equal baseline | 5/10 | `go test -run TestC1743_009...` |
| no-comments-deleted | No baseline comment line lost from the changed sources | 6/10 | `go test -run TestC1743_010...` |
| characterization | auditcalibration suite kills all 35 behavior mutants | 7/10 | `go test -run TestC1743_011...` |
| characterization | dossier suite kills all 19 behavior mutants | 7/10 | `go test -run TestC1743_012...` |
