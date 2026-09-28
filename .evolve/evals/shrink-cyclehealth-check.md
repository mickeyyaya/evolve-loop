---
score_cap:
  - criterion: "internal/cyclehealth.Check is <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_001_ThreeTargetFunctionsFitTheRatchetLimit ./acs/cycle1749"
  - criterion: "go/internal/sizeratchet/offenders.json is byte-unchanged since b5d63a40 and still lists internal/cyclehealth.Check at 51 — an allowance is a ceiling, so a lane never edits the file"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_002_OffendersJSONLeftUnchanged ./acs/cycle1749"
  - criterion: "The module-wide sizeratchet.Check reports zero problems"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_003_ModuleWideRatchetCheckPasses ./acs/cycle1749"
  - criterion: "Every baseline *_test.go under go/internal/cyclehealth is unmodified and undeleted"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_004_BaselineTestFilesUnchanged ./acs/cycle1749"
  - criterion: "go test -count=1 ./internal/cyclehealth passes"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_005_TargetPackageTestsPass ./acs/cycle1749"
  - criterion: "The cyclehealth suite passes with the baseline Check substituted back and kills all 7 one-line behavior mutants of it (clock, signal list, zero cycle, OverallFatal, swallowed, unwrapped and report-dropping write failures)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_009_CheckCharacterizationKillsBaselineMutants ./acs/cycle1749"
  - criterion: "No file under a protected surface (go/cmd/evolve, go/internal/core, go/internal/bridge, go/internal/phases/ship, ...) changed since b5d63a40"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1749_013_NoProtectedSurfaceTouched ./acs/cycle1749"
---

# Eval: shrink-cyclehealth-check

> Pins `internal/cyclehealth.Check` inside the 50-line `sizeratchet.MaxLines`
> ceiling, with its behavior pinned by characterization tests. Cycle 1741
> (sizeratchet-shrink-cyclecost-cyclehealth) already shrank `Check` to 34
> lines and added `check_characterization_test.go`, and left
> `offenders.json` byte-unchanged per the wave-25 ceiling rule. So every
> criterion here is pre-existing GREEN at base `b5d63a40`. The remaining
> `"internal/cyclehealth.Check": 51` entry is slack. The boundary tighten
> (`docs/architecture/logic-first-delivery-design.md:294-296`, R1/R3) removes
> it, not a lane. The scout and triage asked to "drop the entry". The goal's
> later rule ("a lane never edits that file") overrides them, and predicate
> 002 enforces that.
>
> Source: cycle 1749 triage top_n `shrink-cyclehealth-check`. There is no inbox
> record, and the Task Contract names this eval as the authority. The mutation
> probe substitutes mutants of the baseline function text into the current
> package through `go test -overlay`, so it holds however the function is
> later refactored. The empty-workspace mutant is excluded because it writes
> `cycle-health.json` into the package directory (the process cwd).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | Check <=50 lines | 8/10 | `go test -run TestC1749_001...` |
| ceiling-untouched | offenders.json byte-unchanged | 8/10 | `go test -run TestC1749_002...` |
| ratchet-gate | module-wide ratchet Check passes | 9/10 | `go test -run TestC1749_003...` |
| test-files-frozen | baseline cyclehealth tests unmodified | 7/10 | `go test -run TestC1749_004...` |
| behavior-preserved | `go test ./internal/cyclehealth` passes | 8/10 | `go test -run TestC1749_005...` |
| characterization | suite kills 7 baseline Check mutants | 7/10 | `go test -run TestC1749_009...` |
| scope | no protected surface touched | 6/10 | `go test -run TestC1749_013...` |
