---
score_cap:
  - criterion: "Each of the 6 named functions (internal/releasepipeline defaultReleaseVerify, newReleaseRun, releaseRun.prePublish; internal/setup Apply, Detect, Recommend) is <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1742_001_SixSetupReleasepipelineFunctionsFitTheRatchetLimit ./acs/cycle1742/..."
  - criterion: "go/internal/sizeratchet/offenders.json is byte-unchanged since the baseline (d29ffcb1) and still lists the 6 keys at 60/62/68/63/104/65 — an allowance is a ceiling, so this lane never edits the shared file"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1742_002_OffendersJSONLeftUnchanged ./acs/cycle1742/..."
  - criterion: "The module-wide sizeratchet.Check reports zero problems — no extracted helper lands past 50 lines unlisted and no listed function grows past its allowance"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1742_003_ModuleWideRatchetCheckPasses ./acs/cycle1742/..."
  - criterion: "Every *_test.go under go/internal/setup and go/internal/releasepipeline that exists at d29ffcb1 is unmodified and undeleted; added characterization tests are allowed"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1742_004_BaselineTestFilesUnchanged ./acs/cycle1742/..."
  - criterion: "go test -count=1 ./internal/setup passes (behavior preserved through every extraction step)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1742_005_SetupPackageTestsPass ./acs/cycle1742/..."
  - criterion: "go test -count=1 ./internal/releasepipeline passes (behavior preserved through every extraction step)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1742_006_ReleasepipelinePackageTestsPass ./acs/cycle1742/..."
  - criterion: "go vet exits 0 on ./internal/setup and on ./internal/releasepipeline"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1742_007_BothPackagesVetClean ./acs/cycle1742/..."
  - criterion: "No comment lines are added under go/internal/setup or go/internal/releasepipeline — `commentaudit comments -base d29ffcb1` over both dirs exits 0 (production and test files alike)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1742_008_NoCommentLinesAdded ./acs/cycle1742/..."
  - criterion: "The doc comment of each of the 6 target functions is identical to its baseline text"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1742_009_TargetFunctionDocsMatchBaseline ./acs/cycle1742/..."
  - criterion: "Both packages' non-test sources changed and no baseline comment line was deleted from them — the extraction moves comments with their code, never strips a why (docs/conventions/code-comments.md:43)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1742_010_NoBaselineCommentDeleted ./acs/cycle1742/..."
  - criterion: "The setup test suite kills all 24 behavior mutants of Apply, Recommend and Detect that the baseline suite let survive"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1742_011_SetupTestsKillBehaviorMutants ./acs/cycle1742/..."
  - criterion: "The releasepipeline test suite kills all 27 behavior mutants of defaultReleaseVerify, newReleaseRun and releaseRun.prePublish that the baseline suite let survive"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1742_012_ReleasepipelineTestsKillBehaviorMutants ./acs/cycle1742/..."
---

# Eval: sizeratchet-shrink-setup-releasepipeline

> Pins the extraction refactor of six oversized functions to the
> `sizeratchet.MaxLines` (50) bar: `defaultReleaseVerify` (60 lines),
> `newReleaseRun` (62) and `releaseRun.prePublish` (68) in
> `go/internal/releasepipeline`, and `Apply` (63), `Detect` (104) and
> `Recommend` (65) in `go/internal/setup`. Behavior must stay the same, no
> comment may be added or stripped, and `go/internal/sizeratchet/offenders.json`
> must stay byte-unchanged. Since fix/sizeratchet-is-a-ceiling (2026-09-28) an
> allowance is a ceiling. Two shrink lanes' keys sit on adjacent lines, so
> editing the file would conflict at the fleet rebase. Source: inbox item
> `2026-09-28T05-15-00Z-sizeratchet-shrink-setup-releasepipeline.json`, read
> from `.evolve/inbox/processing/cycle-1742/`. Its three acceptance criteria
> are materialized 1:1 in `test-report.md`. Fleet lane of cycle 1742.
>
> Source incidents: the gc lane (cycle 1732), the opscmd lane (cycle 1730) and
> the dashboard-deliverable lane (cycle 1738). Opscmd took four audit rounds
> because characterization and comment discipline were left implicit. A
> baseline mutation probe (114 candidates) found 51 behavior mutants that the
> existing suites do not kill: 24 in setup and 27 in releasepipeline.
> `defaultReleaseVerify` had only a happy-path test. The disk-vs-blob check,
> the `--version` stamp check and the tag-at-release-commit rule were all
> unpinned. Each of the 51 was killed by a throwaway test that passes on the
> baseline. The real predicates 011/012 went GREEN with those tests present
> before the contract was frozen.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | All 6 functions <=50 lines | 8/10 | `go test -run TestC1742_001...` |
| offenders-untouched | offenders.json byte-unchanged, 6 allowances intact | 8/10 | `go test -run TestC1742_002...` |
| ratchet-gate | Module-wide ratchet Check reports zero problems | 9/10 | `go test -run TestC1742_003...` |
| test-files-frozen | Baseline `*_test.go` in both packages unmodified/undeleted | 7/10 | `go test -run TestC1742_004...` |
| behavior-preserved (setup) | `go test ./internal/setup` passes | 8/10 | `go test -run TestC1742_005...` |
| behavior-preserved (releasepipeline) | `go test ./internal/releasepipeline` passes | 8/10 | `go test -run TestC1742_006...` |
| vet-clean | `go vet` on both packages exits 0 | 6/10 | `go test -run TestC1742_007...` |
| no-comments-added | `commentaudit comments` lists zero added lines | 8/10 | `go test -run TestC1742_008...` |
| target-docs-intact | 6 target doc comments equal baseline | 5/10 | `go test -run TestC1742_009...` |
| no-comments-deleted | No baseline comment line lost from changed sources | 6/10 | `go test -run TestC1742_010...` |
| characterization (setup) | setup suite kills all 24 behavior mutants | 7/10 | `go test -run TestC1742_011...` |
| characterization (releasepipeline) | releasepipeline suite kills all 27 behavior mutants | 7/10 | `go test -run TestC1742_012...` |
