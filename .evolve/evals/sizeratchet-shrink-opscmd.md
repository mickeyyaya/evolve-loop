---
score_cap:
  - criterion: "Each of the 8 named internal/cli/opscmd functions is <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1730_001_EightOpscmdFunctionsFitTheRatchetLimit ./acs/cycle1730/..."
  - criterion: "None of the 8 opscmd keys remain in go/internal/sizeratchet/offenders.json (a fixed function's allowance is deleted, never just lowered)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1730_002_OffendersJSONHasNoOpscmdEntriesLeft ./acs/cycle1730/..."
  - criterion: "The repo-wide sizeratchet.Check gate (every function in the module against offenders.json) reports zero problems — defeats delete-without-shrink (an unlisted function past the limit); since 2026-09-28 an allowance is a ceiling, so shrink-without-delete is slack here and the _002 key-absence check catches it"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1730_003_ModuleWideRatchetCheckPasses ./acs/cycle1730/..."
  - criterion: "Every *_test.go under go/internal/cli/opscmd that exists at the cycle baseline (ad816b70) is unmodified and undeleted — existing tests pass unmodified; added characterization tests are allowed (criterion 2)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1730_004_OpscmdTestFilesUnchangedFromBaseline ./acs/cycle1730/..."
  - criterion: "go test -count=1 ./internal/cli/opscmd/... passes end to end (existing behavior preserved through every extraction step)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1730_005_OpscmdPackageTestsPass ./acs/cycle1730/..."
  - criterion: "go build ./... and go vet ./... both exit 0 from go/ (extraction introduced no compile or vet regression anywhere in the module)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1730_006_ModuleBuildsAndVetsClean ./acs/cycle1730/..."
  - criterion: "No comment lines are added under go/internal/cli/opscmd — `commentaudit comments -base ad816b70 internal/cli/opscmd` exits 0 (inbox criterion 3; audit round 1 H1: 54 added doc-comment lines)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1730_007_NoCommentLinesAddedToOpscmd ./acs/cycle1730/..."
  - criterion: "The doc comment of each of the 8 target functions is identical to its baseline text — exit-code tables are not moved into unexported helpers or left as fragments (audit round 1 L1)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1730_008_TargetFunctionDocsMatchBaseline ./acs/cycle1730/..."
  - criterion: "runDoctorLive/runDoctorBoot result reporting is pinned by TestDoctorCharacterization* tests that pass on the real code and fail on each of 5 message / JSON-field / pane-tail mutants (inbox criterion 2 characterization clause; audit round 1 M2)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1730_009_DoctorCharacterizationTestsKillReportMutants ./acs/cycle1730/..."
  - criterion: "No baseline comment line is deleted from the changed internal/cli/opscmd sources — the extraction moves comments with their code, it never strips a why (docs/conventions/code-comments.md:43; audit round 2 M1: console_lease.go interspersed-parse rationale)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1730_010_NoBaselineCommentDeletedFromOpscmd ./acs/cycle1730/..."
---

# Eval: sizeratchet-shrink-opscmd

> Pins the eight-function extraction refactor of `go/internal/cli/opscmd`
> (`RunChangelogGen`, `RunConsoleLease`, `RunMarketplacePoll`,
> `RunReleasePipeline`, `RunReleasePreflight`, `RunRollback`, `runDoctorBoot`,
> `runDoctorLive`) down to the `sizeratchet.MaxLines` (50) bar, with their
> `go/internal/sizeratchet/offenders.json` allowances removed, behavior
> unchanged, and no comments added. Source: inbox item
> `2026-09-28T05-10-00Z-sizeratchet-shrink-opscmd.json` (read from
> `.evolve/inbox/processing/cycle-1730/`; its three acceptance criteria are
> materialized 1:1 in `test-report.md`), fleet lane `sizeratchet-shrink-opscmd`,
> cycle 1730.
>
> Source incident: cycle 1730 audit round 1 FAILed a build that met the line
> limit because it added 54 doc-comment lines (criterion 3, which the first
> TDD round left manual although `commentaudit comments` grades it), trimmed
> three exported docs to fragments, and skipped the characterization-test
> clause of criterion 2 for the doctor live/boot reporting. Predicates 007-009
> close those gaps. Audit round 2 FAILed the repaired build because the
> comment-strip also deleted the baseline 5-line *why* comment above
> RunConsoleLease's interspersed parse loop, a deletion the add-only
> `commentaudit comments` gate cannot see; predicate 010 closes that gap. The
> "one level of abstraction" readability judgment has no mechanical grader and
> stays a manual auditor checklist.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | All 8 functions <=50 lines | 8/10 | `go test -run TestC1730_001...` |
| offenders-cleanup | All 8 offenders.json keys removed | 8/10 | `go test -run TestC1730_002...` |
| ratchet-gate | Module-wide ratchet Check reports zero problems | 9/10 | `go test -run TestC1730_003...` |
| test-files-frozen | Baseline opscmd `*_test.go` files unmodified/undeleted | 7/10 | `go test -run TestC1730_004...` |
| behavior-preserved | `go test ./internal/cli/opscmd/...` passes | 8/10 | `go test -run TestC1730_005...` |
| build-vet-clean | `go build ./...` and `go vet ./...` exit 0 | 6/10 | `go test -run TestC1730_006...` |
| no-comments-added | `commentaudit comments` lists zero added lines in opscmd | 8/10 | `go test -run TestC1730_007...` |
| target-docs-intact | 8 target doc comments equal baseline | 5/10 | `go test -run TestC1730_008...` |
| characterization | Doctor report tests pass and kill 5 mutants | 7/10 | `go test -run TestC1730_009...` |
| no-comments-deleted | No baseline comment line lost from the changed opscmd sources | 6/10 | `go test -run TestC1730_010...` |
