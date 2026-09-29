---
score_cap:
  - criterion: "cmd/evolve.runACSSuite is <=50 lines (sizeratchet.MaxLines) under its original name, measured by the ratchet's own AST scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1755_001_RunACSSuiteWithinSizeRatchetLimit ./acs/cycle1755"
  - criterion: "go/internal/sizeratchet/offenders.json no longer lists cmd/evolve.runACSSuite (removed, since an allowance cannot be lowered to 50 or fewer)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1755_005_OffendersJSONDropsTheThreeShrunkEntries ./acs/cycle1755"
  - criterion: "offenders.json gains no key and raises no allowance relative to base ad310db6; the extracted helper is never listed"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1755_006_OffendersJSONNeverGainsOrRaisesAnEntry ./acs/cycle1755"
  - criterion: "The module-wide sizeratchet.Check passes, so no extracted helper exceeds 50 lines"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1755_004_ModuleSizeRatchetCheckPasses ./acs/cycle1755"
  - criterion: "evolve acs suite keeps its CLI contract through the production entry runACS: exit 10 on a bad flag or cycle<=0, 1 on a suite or verdict-write error, 2 on a RED predicate with one RED line each, 0 on PASS; the summary line, --json=false, and the '.' root auto-resolution to active_worktree and plane root are all unchanged"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1755_007_RunACSSuiteCLIContractPreserved ./acs/cycle1755"
  - criterion: "No comments added under go/cmd/evolve since base (craft rule; commentaudit comments exits 0)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1755_009_NoCommentsAddedUnderCmdEvolve ./acs/cycle1755"
  - criterion: "cmd_acs.go and cmd_acs_suite_test.go are gofmt-clean and go vet ./cmd/evolve/ passes"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1755_010_TouchedFilesGofmtAndVetClean ./acs/cycle1755"
  - criterion: "No protected control-plane surface (guards.IsProtectedSurface) changed since base"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1755_011_NoProtectedSurfaceTouched ./acs/cycle1755"
---

# Eval: sizeratchet-runacssuite-shrink

> This eval pins the size-ratchet shrink of `runACSSuite`
> (`go/cmd/evolve/cmd_acs.go`), the `evolve acs suite` entry point. At base
> `ad310db6` it measures 51 lines against a cap of 50. The scout cited three
> existing test files, but none of them called `runACSSuite`: they cover only
> `resolveACSSuiteRoot` and `suiteProjectRoot`. The function was unpinned,
> so cycle 1755 TDD added 9 characterization tests
> (`go/cmd/evolve/cmd_acs_suite_test.go`). They go through the production
> dispatch `runACS("suite", ...)`, and 10 of 10 mutants of `cmd_acs.go` fail
> them. The shrink must be one behavior-preserving extraction. Scout and triage
> ask for the `offenders.json` entry to be dropped. This is a sequential cycle
> with no lanes, which follows the cycle 1751/1753/1754 precedent. Source: cycle
> 1755 triage top_n. No inbox record exists, so the Task Contract names this
> eval as the authority.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | runACSSuite <=50 lines | 8/10 | `go test -run TestC1755_001...` |
| ceiling-dropped | entry removed from offenders.json | 6/10 | `go test -run TestC1755_005...` |
| no-gaming | no key added / allowance raised | 8/10 | `go test -run TestC1755_006...` |
| ratchet-gate | module-wide ratchet Check passes | 9/10 | `go test -run TestC1755_004...` |
| behavior-preserved | 16 runACSSuite/root-resolution tests pass | 9/10 | `go test -run TestC1755_007...` |
| craft | no comments added | 5/10 | `go test -run TestC1755_009...` |
| hygiene | gofmt + vet clean | 6/10 | `go test -run TestC1755_010...` |
| boundary | no protected surface touched | 8/10 | `go test -run TestC1755_011...` |
