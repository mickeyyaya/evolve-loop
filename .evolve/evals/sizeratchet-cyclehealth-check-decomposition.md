---
score_cap:
  - criterion: "internal/cyclehealth.Check is <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_001_ThreeTargetFunctionsFitTheRatchetLimit ./acs/cycle1750"
  - criterion: "go/internal/sizeratchet/offenders.json is byte-unchanged since d4467628 and still lists internal/cyclehealth.Check at 51 — an allowance is a ceiling, so a lane never edits the file"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_002_OffendersJSONLeftUnchanged ./acs/cycle1750"
  - criterion: "The module-wide sizeratchet.Check reports zero problems"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_003_ModuleWideRatchetCheckPasses ./acs/cycle1750"
  - criterion: "Every baseline *_test.go under go/internal/cyclehealth (check_characterization_test.go, cyclehealth_cycle188_test.go, ...) is unmodified and undeleted"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_004_BaselineTestFilesUnchanged ./acs/cycle1750"
  - criterion: "go test -count=1 ./internal/cyclehealth passes"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_005_TargetPackageTestsPass ./acs/cycle1750"
  - criterion: "No file under go/internal/cyclehealth changed since d4467628 — Check already fits the ratchet (33 lines), so its failure paths stay exactly as cycle 1749 pinned them"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1750_011_AlreadyShrunkPackagesLeftUntouched ./acs/cycle1750"
---

# Eval: sizeratchet-cyclehealth-check-decomposition

> The scout selected `internal/cyclehealth.Check` as a 51-line offender. That
> number is its `offenders.json` allowance, not its measured size. At base
> `d4467628`, `Check` measures 33 lines. Cycle 1741 shrank it, and cycle 1749
> (`shrink-cyclehealth-check`) pinned it with 7 killed mutants. Every
> acceptance criterion of this task already holds at base.
>
> This eval therefore pins the no-op: the package stays untouched (predicate
> 011), its baseline tests stay frozen and green, and `offenders.json` keeps
> the 51 allowance. The goal's wave-25 rule, "a lane never edits that file",
> overrides the scout's criterion 4 ("offenders.json no longer lists the
> entry"). The entry is slack for the boundary tighten to remove.
>
> Source: cycle 1750 triage top_n `sizeratchet-cyclehealth-check-decomposition`.
> There is no inbox record, so the Task Contract names this eval as the
> authority. The stale premise comes from the scout reading the allowance as
> the size.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | Check <=50 lines | 8/10 | `go test -run TestC1750_001...` |
| ceiling-untouched | offenders.json byte-unchanged | 8/10 | `go test -run TestC1750_002...` |
| ratchet-gate | module-wide ratchet Check passes | 9/10 | `go test -run TestC1750_003...` |
| test-files-frozen | baseline cyclehealth tests unmodified | 7/10 | `go test -run TestC1750_004...` |
| behavior-preserved | `go test ./internal/cyclehealth` passes | 8/10 | `go test -run TestC1750_005...` |
| no-op-held | cyclehealth untouched since base | 8/10 | `go test -run TestC1750_011...` |
