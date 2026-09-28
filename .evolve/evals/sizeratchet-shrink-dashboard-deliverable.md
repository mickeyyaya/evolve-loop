---
score_cap:
  - criterion: "Each of the 6 named functions (internal/dashboard callForPhaseWindow, collector.collect, readPlan; internal/deliverable Reviewer.Review, SalvageSummaryLine, verdictCandidates) is <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1738_001_SixDashboardDeliverableFunctionsFitTheRatchetLimit ./acs/cycle1738/..."
  - criterion: "go/internal/sizeratchet/offenders.json is byte-unchanged since the baseline (c9093d5c) and still lists the 6 keys at 67/54/59/68/58/64 — an allowance is a ceiling, so this lane never edits the shared file"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1738_002_OffendersJSONLeftUnchanged ./acs/cycle1738/..."
  - criterion: "The module-wide sizeratchet.Check reports zero problems — no extracted helper lands past 50 lines unlisted and no listed function grows past its allowance"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1738_003_ModuleWideRatchetCheckPasses ./acs/cycle1738/..."
  - criterion: "Every *_test.go under go/internal/dashboard and go/internal/deliverable that exists at c9093d5c is unmodified and undeleted; added characterization tests are allowed"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1738_004_BaselineTestFilesUnchanged ./acs/cycle1738/..."
  - criterion: "go test -count=1 ./internal/dashboard passes (behavior preserved through every extraction step)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1738_005_DashboardPackageTestsPass ./acs/cycle1738/..."
  - criterion: "go test -count=1 ./internal/deliverable passes (behavior preserved through every extraction step)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1738_006_DeliverablePackageTestsPass ./acs/cycle1738/..."
  - criterion: "go vet exits 0 on ./internal/dashboard and on ./internal/deliverable"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1738_007_BothPackagesVetClean ./acs/cycle1738/..."
  - criterion: "No comment lines are added under go/internal/dashboard or go/internal/deliverable — `commentaudit comments -base c9093d5c` over both dirs exits 0 (production and test files alike)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1738_008_NoCommentLinesAdded ./acs/cycle1738/..."
  - criterion: "The doc comment of each of the 6 target functions is identical to its baseline text (callForPhaseWindow stays undocumented)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1738_009_TargetFunctionDocsMatchBaseline ./acs/cycle1738/..."
  - criterion: "Both packages' non-test sources changed and no baseline comment line was deleted from them — the extraction moves comments with their code, never strips a why (docs/conventions/code-comments.md:43)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1738_010_NoBaselineCommentDeleted ./acs/cycle1738/..."
  - criterion: "The dashboard test suite kills all 11 behavior mutants of callForPhaseWindow, collector.collect and readPlan that the baseline suite let survive"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1738_011_DashboardTestsKillBehaviorMutants ./acs/cycle1738/..."
  - criterion: "The deliverable test suite kills all 18 behavior mutants of Reviewer.Review, SalvageSummaryLine and verdictCandidates that the baseline suite let survive"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1738_012_DeliverableTestsKillBehaviorMutants ./acs/cycle1738/..."
---

# Eval: sizeratchet-shrink-dashboard-deliverable

> Pins the extraction refactor of six oversized functions to the
> `sizeratchet.MaxLines` (50) bar: `callForPhaseWindow` (67 lines),
> `collector.collect` (54) and `readPlan` (59) in `go/internal/dashboard`, and
> `Reviewer.Review` (68), `SalvageSummaryLine` (58) and `verdictCandidates`
> (64) in `go/internal/deliverable`. Behavior must stay the same, no comment
> may be added or stripped, and `go/internal/sizeratchet/offenders.json` must
> stay byte-unchanged. Since fix/sizeratchet-is-a-ceiling (2026-09-28) an
> allowance is a ceiling. Two shrink lanes' keys sit on adjacent lines, so
> editing the file would conflict at the fleet rebase. Source: inbox item
> `2026-09-28T05-14-00Z-sizeratchet-shrink-dashboard-deliverable.json`, read
> from `.evolve/inbox/processing/cycle-1738/`. Its three acceptance criteria
> are materialized 1:1 in `test-report.md`. Fleet lane of cycle 1738.
>
> Source incidents: the gc lane (cycle 1732) and the opscmd lane (cycle 1730).
> Opscmd took four audit rounds because characterization and comment
> discipline were left implicit. A baseline mutation probe (78 mutants) found
> 29 that the existing suites do not kill: 11 in dashboard and 18 in
> deliverable. `SalvageSummaryLine` and `verdictCandidates` have no direct
> behavioral pin at all. Each of the 29 was shown killable by an overlay-only
> throwaway test before this contract was frozen. Predicates 011 and 012
> require characterization tests that kill them.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | All 6 functions <=50 lines | 8/10 | `go test -run TestC1738_001...` |
| ceiling-untouched | offenders.json byte-unchanged, 6 allowances intact | 8/10 | `go test -run TestC1738_002...` |
| ratchet-gate | Module-wide ratchet Check reports zero problems | 9/10 | `go test -run TestC1738_003...` |
| test-files-frozen | Baseline dashboard/deliverable `*_test.go` unmodified/undeleted | 7/10 | `go test -run TestC1738_004...` |
| behavior-preserved | `go test ./internal/dashboard` passes | 8/10 | `go test -run TestC1738_005...` |
| behavior-preserved | `go test ./internal/deliverable` passes | 8/10 | `go test -run TestC1738_006...` |
| vet-clean | `go vet` clean on both packages | 6/10 | `go test -run TestC1738_007...` |
| no-comments-added | `commentaudit comments` lists zero added lines | 8/10 | `go test -run TestC1738_008...` |
| target-docs-intact | 6 target doc comments equal baseline | 5/10 | `go test -run TestC1738_009...` |
| no-comments-deleted | No baseline comment lost from changed sources | 6/10 | `go test -run TestC1738_010...` |
| characterization | dashboard suite kills all 11 behavior mutants | 7/10 | `go test -run TestC1738_011...` |
| characterization | deliverable suite kills all 18 behavior mutants | 7/10 | `go test -run TestC1738_012...` |
