---
score_cap:
  - criterion: "ScanConfigRoot, Aggregate, DirectImporters and BuildTags each measure at most 50 lines under sizeratchet.Walk, keeping their names and packages"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1771_001_TargetFunctionsFitTheRatchetLimit ./acs/cycle1771"
  - criterion: "go/internal/sizeratchet/offenders.json is left byte-unchanged since 6aa43b70 (ScanConfigRoot=60, Aggregate=53, DirectImporters=79, BuildTags=56 stay as slack)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1771_002_OffendersJSONLeftUnchanged ./acs/cycle1771"
  - criterion: "the module-wide size ratchet (sizeratchet.Check) reports zero violations, extracted helpers included"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1771_003_ModuleWideRatchetCheckPasses ./acs/cycle1771"
  - criterion: "every baseline top-level declaration in the tokenusage, llmcalls, changedpkgs and addedtests _test.go files survives byte-for-byte"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1771_004_BaselineTestDeclarationsUnchanged ./acs/cycle1771"
  - criterion: "the existing tokenusage, llmcalls, changedpkgs and addedtests test suites pass"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 ./internal/tokenusage ./internal/llmcalls ./internal/changedpkgs ./internal/addedtests"
  - criterion: "no comment lines are added to any of the four target packages (names carry intent)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1771_006_NoCommentLinesAdded ./acs/cycle1771"
---

# Eval: Shrink four usage-scanner functions under the 50-line limit

> Pins the behavior-preserving decomposition of `tokenusage.ScanConfigRoot`
> (60 lines), `llmcalls.Aggregate` (53 lines), `changedpkgs.DirectImporters`
> (79 lines) and `addedtests.BuildTags` (56 lines), the four live offenders in
> `go/internal/sizeratchet/offenders.json` for these packages (measured on
> main 6aa43b70, 2026-09-30), to the repo-wide 50-line function-size ratchet.
> The allowance is a ceiling, so the lane extracts named helper steps and
> leaves `offenders.json` alone. All four packages already carry named
> `*_test.go` coverage of the target function (per scout's confirmation), so
> no new characterization tests were required before extraction — the pin
> here is that the baseline test declarations and the packages' own suites
> stay green through the refactor (inbox record
> `2026-09-29T20-17-00Z-sizeratchet-shrink-usage-scanners.json`).

## Criteria

1. **[code]** All four functions ≤50 lines per `sizeratchet.Walk`; `offenders.json` unchanged; module-wide `sizeratchet.Check` green.
2. **[code]** Behavior unchanged: existing suites pass and baseline test declarations stay intact.
3. **[code]** No comment lines added under any of the four packages (`commentaudit comments`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| size-fit | All 4 functions ≤50 lines | 8/10 | `go test -tags acs -run TestC1771_001_...` |
| ratchet-file-untouched | offenders.json byte-unchanged | 6/10 | `go test -tags acs -run TestC1771_002_...` |
| module-green | sizeratchet.Check zero violations | 5/10 | `go test -tags acs -run TestC1771_003_...` |
| test-decls-frozen | Baseline test declarations intact | 7/10 | `go test -tags acs -run TestC1771_004_...` |
| suites-green | Existing package tests pass | 9/10 | `go test ./internal/tokenusage ./internal/llmcalls ./internal/changedpkgs ./internal/addedtests` |
| no-comments-added | Extraction adds zero comment lines | 5/10 | `go test -tags acs -run TestC1771_006_...` |
