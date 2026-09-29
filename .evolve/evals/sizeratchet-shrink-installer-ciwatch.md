---
score_cap:
  - criterion: "installer.Validate and ciwatch.Watch each measure at most 50 lines under sizeratchet.Walk, keeping their names and packages"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1769_001_ValidateAndWatchFitTheRatchetLimit ./acs/cycle1769"
  - criterion: "go/internal/sizeratchet/offenders.json is left byte-unchanged since fe0f8f20 (Validate=58, Watch=64 stay as slack)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1769_002_OffendersJSONLeftUnchanged ./acs/cycle1769"
  - criterion: "the module-wide size ratchet (sizeratchet.Check) reports zero violations, extracted helpers included"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1769_003_ModuleWideRatchetCheckPasses ./acs/cycle1769"
  - criterion: "every baseline top-level declaration in the installer and ciwatch _test.go files survives byte-for-byte"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1769_004_BaselineTestDeclarationsUnchanged ./acs/cycle1769"
  - criterion: "the existing installer and ciwatch test suites pass"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 ./internal/installer ./internal/ciwatch"
  - criterion: "Watch keeps its fetch-error short-circuit, 900s/30s defaults, deadline boundary, input-guard order and real-clock fallbacks"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1769_00[6-9]' ./acs/cycle1769"
  - criterion: "Validate prints the identical OK/FAIL transcript in the identical order and returns identical counters"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1769_010_ValidateTranscriptIsUnchanged ./acs/cycle1769"
  - criterion: "no comment lines are added to go/internal/installer or go/internal/ciwatch (names carry intent)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1769_011_NoCommentLinesAdded ./acs/cycle1769"
---

# Eval: Shrink installer.Validate and ciwatch.Watch under the 50-line limit

> Pins the behavior-preserving decomposition of `installer.Validate` (58
> lines) and `ciwatch.Watch` (64 lines), the two live offenders in
> `go/internal/sizeratchet/offenders.json` for these packages (measured on
> main 6cc69ccc, 2026-09-29), to the repo-wide 50-line function-size ratchet.
> The allowance is a ceiling, so the lane extracts named steps and leaves
> `offenders.json` alone. Source incident: cycle 1769 TDD mutation probe —
> five of six behavior mutants of the two functions (timeout default, ignored
> fetch error, nil-Sleep fallback, guard order, skill/doc step order) passed
> the packages' own existing tests, so the characterization predicates
> (TestC1769_006..010) are the pins that make "behavior unchanged" checkable
> (inbox record `2026-09-29T20-15-00Z-sizeratchet-shrink-installer-ciwatch.json`).

## Criteria

1. **[code]** Both functions ≤50 lines per `sizeratchet.Walk`; `offenders.json` unchanged; module-wide `sizeratchet.Check` green.
2. **[code]** Behavior unchanged: existing suites pass, baseline test declarations intact, and the Watch/Validate characterization predicates stay green (each is red on a named mutant).
3. **[code]** No comment lines added under either package (`commentaudit comments`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| size-fit | Validate and Watch ≤50 lines | 8/10 | `go test -tags acs -run TestC1769_001_...` |
| ratchet-file-untouched | offenders.json byte-unchanged | 6/10 | `go test -tags acs -run TestC1769_002_...` |
| module-green | sizeratchet.Check zero violations | 5/10 | `go test -tags acs -run TestC1769_003_...` |
| test-decls-frozen | Baseline test declarations intact | 7/10 | `go test -tags acs -run TestC1769_004_...` |
| suites-green | Existing package tests pass | 9/10 | `go test ./internal/installer ./internal/ciwatch` |
| watch-characterized | Error path, defaults, boundary, guards, real clock | 9/10 | `go test -tags acs -run 'TestC1769_00[6-9]'` |
| validate-characterized | Golden OK/FAIL transcript + counters | 9/10 | `go test -tags acs -run TestC1769_010_...` |
| no-comments-added | Extraction adds zero comment lines | 5/10 | `go test -tags acs -run TestC1769_011_...` |
