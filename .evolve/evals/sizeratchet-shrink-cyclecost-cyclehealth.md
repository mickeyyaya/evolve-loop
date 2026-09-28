---
score_cap:
  - criterion: "Each of the 4 named functions (internal/cyclecost SummarizeCycle, parseEventsLog; internal/cyclehealth Check, ClassifyOutcome) is <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1741_001_FourCyclecostCyclehealthFunctionsFitTheRatchetLimit ./acs/cycle1741"
  - criterion: "go/internal/sizeratchet/offenders.json is byte-unchanged since the baseline (d29ffcb1) and still lists the 4 keys at 57/54/51/70 — an allowance is a ceiling, so this lane never edits the shared file"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1741_002_OffendersJSONLeftUnchanged ./acs/cycle1741"
  - criterion: "The module-wide sizeratchet.Check reports zero problems — no extracted helper lands past 50 lines unlisted and no listed function grows past its allowance"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1741_003_ModuleWideRatchetCheckPasses ./acs/cycle1741"
  - criterion: "Every *_test.go under go/internal/cyclecost and go/internal/cyclehealth that exists at d29ffcb1 is unmodified and undeleted; added characterization tests are allowed"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1741_004_BaselineTestFilesUnchanged ./acs/cycle1741"
  - criterion: "go test -count=1 ./internal/cyclecost passes (behavior preserved through every extraction step)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1741_005_CyclecostPackageTestsPass ./acs/cycle1741"
  - criterion: "go test -count=1 ./internal/cyclehealth passes (behavior preserved through every extraction step)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1741_006_CyclehealthPackageTestsPass ./acs/cycle1741"
  - criterion: "go vet exits 0 on ./internal/cyclecost and on ./internal/cyclehealth"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1741_007_BothPackagesVetClean ./acs/cycle1741"
  - criterion: "No comment lines are added under go/internal/cyclecost or go/internal/cyclehealth — `commentaudit comments -base d29ffcb1` over both dirs exits 0 (production and test files alike)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1741_008_NoCommentLinesAdded ./acs/cycle1741"
  - criterion: "The doc comment of each of the 4 target functions is identical to its baseline text"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1741_009_TargetFunctionDocsMatchBaseline ./acs/cycle1741"
  - criterion: "Both packages' non-test sources changed and no baseline comment line was deleted from them — the extraction moves comments with their code, never strips a why (docs/conventions/code-comments.md:43)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1741_010_NoBaselineCommentDeleted ./acs/cycle1741"
  - criterion: "The cyclecost test suite kills all 6 behavior mutants of SummarizeCycle and parseEventsLog that the baseline suite let survive"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1741_011_CyclecostTestsKillBehaviorMutants ./acs/cycle1741"
  - criterion: "The cyclehealth test suite kills all 12 behavior mutants of Check and ClassifyOutcome that the baseline suite let survive"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1741_012_CyclehealthTestsKillBehaviorMutants ./acs/cycle1741"
  - criterion: "ClassifyOutcome keeps its precedence ladder and exact detail strings: SHIPPED > SALVAGED > DEFERRED > explained abort > first FAIL verdict > initialization failure > FAILED_UNEXPLAINED, across 10 ordering fixtures"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1741_013_ClassifyOutcomePrecedenceUnchanged ./acs/cycle1741"
---

# Eval: sizeratchet-shrink-cyclecost-cyclehealth

> Pins the extraction refactor of four oversized functions to the
> `sizeratchet.MaxLines` (50) bar: `SummarizeCycle` (57 lines) and
> `parseEventsLog` (54) in `go/internal/cyclecost`, and `Check` (51) and
> `ClassifyOutcome` (70) in `go/internal/cyclehealth`. Behavior must stay the
> same, no comment may be added or stripped, and
> `go/internal/sizeratchet/offenders.json` must stay byte-unchanged. Since
> fix/sizeratchet-is-a-ceiling (2026-09-28) an allowance is a ceiling, and two
> shrink lanes' keys sit on adjacent lines, so editing the file would conflict
> at the fleet rebase. Source: inbox item
> `2026-09-28T05-16-00Z-sizeratchet-shrink-cyclecost-cyclehealth.json`, read
> from `.evolve/inbox/processing/cycle-1741/`. Its three acceptance criteria
> are materialized 1:1 in `test-report.md`. Fleet lane of cycle 1741.
>
> Source incidents: the gc (cycle 1732), skillcheck (cycle 1736) and
> dashboard/deliverable (cycle 1738) shrink lanes, whose contracts made
> characterization and comment discipline explicit after the opscmd lane
> (cycle 1730) took four audit rounds without them. A baseline mutation probe
> of 78 candidates found 18 that the existing suites do not kill: 6 in
> cyclecost and 12 in cyclehealth. Each of the 18 was killed by an
> overlay-only throwaway test that passes on the baseline, before this
> contract was frozen. Predicates 011 and 012 require characterization tests
> that kill them. `ClassifyOutcome`'s first-match precedence cannot be pinned
> by a one-line mutant that survives helper extraction, so predicate 013 pins
> it directly. It catches a hoisted quota defer, a hoisted explained abort,
> and merged deferred/explained loops (each shown by overlay).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | All 4 functions <=50 lines | 8/10 | `go test -run TestC1741_001...` |
| ceiling-untouched | offenders.json byte-unchanged, 4 allowances intact | 8/10 | `go test -run TestC1741_002...` |
| ratchet-gate | Module-wide ratchet Check reports zero problems | 9/10 | `go test -run TestC1741_003...` |
| test-files-frozen | Baseline cyclecost/cyclehealth `*_test.go` unmodified/undeleted | 7/10 | `go test -run TestC1741_004...` |
| behavior-preserved | `go test ./internal/cyclecost` passes | 8/10 | `go test -run TestC1741_005...` |
| behavior-preserved | `go test ./internal/cyclehealth` passes | 8/10 | `go test -run TestC1741_006...` |
| vet-clean | `go vet` clean on both packages | 6/10 | `go test -run TestC1741_007...` |
| no-comments-added | `commentaudit comments` lists zero added lines | 8/10 | `go test -run TestC1741_008...` |
| target-docs-intact | 4 target doc comments equal baseline | 5/10 | `go test -run TestC1741_009...` |
| no-comments-deleted | No baseline comment lost from changed sources | 6/10 | `go test -run TestC1741_010...` |
| characterization | cyclecost suite kills all 6 behavior mutants | 7/10 | `go test -run TestC1741_011...` |
| characterization | cyclehealth suite kills all 12 behavior mutants | 7/10 | `go test -run TestC1741_012...` |
| precedence | ClassifyOutcome ladder and details unchanged | 8/10 | `go test -run TestC1741_013...` |
