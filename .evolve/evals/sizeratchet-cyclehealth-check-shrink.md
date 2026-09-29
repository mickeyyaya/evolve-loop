---
score_cap:
  - criterion: "internal/cyclehealth.Check is <=50 lines (sizeratchet.MaxLines) under its original name, measured by the ratchet's own AST scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1755_002_CyclehealthCheckWithinSizeRatchetLimit ./acs/cycle1755"
  - criterion: "go/internal/sizeratchet/offenders.json no longer lists internal/cyclehealth.Check; the 51 is slack, and the entry is removed rather than lowered"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1755_005_OffendersJSONDropsTheThreeShrunkEntries ./acs/cycle1755"
  - criterion: "offenders.json gains no key and raises no allowance relative to base ad310db6"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1755_006_OffendersJSONNeverGainsOrRaisesAnEntry ./acs/cycle1755"
  - criterion: "The module-wide sizeratchet.Check passes"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1755_004_ModuleSizeRatchetCheckPasses ./acs/cycle1755"
  - criterion: "The existing characterization tests check_characterization_test.go (TestCheck_*, TestClassifyOutcome_*) pass unmodified"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^(TestCheck_|TestClassifyOutcome_)' ./internal/cyclehealth/"
  - criterion: "No file under go/internal/cyclehealth changed since base ad310db6: Check already fits the ratchet (33 lines), so the shrink is a no-op on the package"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1755_008_AlreadyShrunkPackagesLeftUntouched ./acs/cycle1755"
  - criterion: "No protected control-plane surface (guards.IsProtectedSurface) changed since base"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1755_011_NoProtectedSurfaceTouched ./acs/cycle1755"
---

# Eval: sizeratchet-cyclehealth-check-shrink

> The scout selected `internal/cyclehealth.Check` as a 51-line offender. The 51 is its
> `offenders.json` allowance, not its measured size. At base `ad310db6`,
> `Check` measures 33 lines, because it was already shrunk by cycle
> 1741, pinned by cycle 1749. Cycle 1750 made the same misreading
> (`sizeratchet-*-decomposition`). The only work left is to drop the slack
> entry, which scout and triage ask for ("update offenders.json"). The package
> itself stays byte-identical, so its failure and no-op paths remain as they
> were pinned. Source: cycle 1755 triage top_n. No inbox record exists, so the
> Task Contract names this eval as the authority. The stale premise comes from
> the scout reading the allowance as the size, as in cycle 1750.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | Check <=50 lines | 8/10 | `go test -run TestC1755_002...` |
| ceiling-dropped | entry removed from offenders.json | 6/10 | `go test -run TestC1755_005...` |
| no-gaming | no key added / allowance raised | 8/10 | `go test -run TestC1755_006...` |
| ratchet-gate | module-wide ratchet Check passes | 9/10 | `go test -run TestC1755_004...` |
| behavior-preserved | characterization tests pass | 8/10 | `go test -run '^(TestCheck_|TestClassifyOutcome_)' ./internal/cyclehealth/` |
| no-op-held | package untouched since base | 8/10 | `go test -run TestC1755_008...` |
| boundary | no protected surface touched | 8/10 | `go test -run TestC1755_011...` |
