---
score_cap:
  - criterion: "Check, CheckArtifactNames and CheckProvenance in go/internal/phasecoherence each measure at most 50 lines under sizeratchet.Walk"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1768_001_ThreeTargetFunctionsFitTheRatchetLimit ./acs/cycle1768"
  - criterion: "go/internal/sizeratchet/offenders.json is left byte-unchanged for the three phasecoherence keys since e404b914"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1768_002_OffendersJSONLeftUnchanged ./acs/cycle1768"
  - criterion: "the module-wide size ratchet (sizeratchet.Check) reports zero violations after the shrink"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1768_003_ModuleWideRatchetCheckPasses ./acs/cycle1768"
  - criterion: "no baseline _test.go file in go/internal/phasecoherence is modified or deleted since e404b914"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1768_004_BaselineTestFilesUnchanged ./acs/cycle1768"
  - criterion: "the existing go/internal/phasecoherence test suite passes unmodified (behavior unchanged)"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 ./internal/phasecoherence/..."
  - criterion: "no comment lines are added to go/internal/phasecoherence by the extraction (names carry intent)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1768_007_NoCommentLinesAdded ./acs/cycle1768"
---

# Eval: Shrink the 3 functions in go/internal/phasecoherence over the 50-line limit

> Pins the behavior-preserving decomposition of `phasecoherence.Check` (80
> lines), `phasecoherence.CheckArtifactNames` (97 lines) and
> `phasecoherence.CheckProvenance` (95 lines) — all three live offenders in
> `go/internal/sizeratchet/offenders.json` measured on main 6cc69ccc
> (2026-09-29) — to the repo-wide 50-line function-size ratchet
> (`go/internal/sizeratchet`). Source incident: the sizeratchet allowance is a
> ceiling (since 2026-09-28), so this lane must extract named steps until each
> function fits the limit, leave the ratchet's `offenders.json` entries
> untouched (a boundary-tighten's job, not this lane's), and preserve exact
> observable behavior under the package's existing 14 test files — no
> characterization test is required unless a branch turns out unpinned during
> extraction (cycle 1768, inbox record
> `2026-09-29T20-13-00Z-sizeratchet-shrink-phasecoherence.json`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| size-fit | All 3 functions ≤50 lines | 8/10 | `go test -tags acs -run TestC1768_001_...` |
| ratchet-file-untouched | offenders.json byte-unchanged for the 3 keys | 6/10 | `go test -tags acs -run TestC1768_002_...` |
| module-green | sizeratchet.Check reports zero violations | 5/10 | `go test -tags acs -run TestC1768_003_...` |
| test-files-frozen | No baseline `_test.go` edited or deleted | 7/10 | `go test -tags acs -run TestC1768_004_...` |
| behavior-unchanged | Package's own test suite passes unmodified | 9/10 | `go test ./internal/phasecoherence/...` |
| no-comments-added | Extraction adds zero comment lines | 5/10 | `go test -tags acs -run TestC1768_007_...` |
