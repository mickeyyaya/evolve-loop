---
score_cap:
  - criterion: "Each of the 4 named internal/skillcheck functions (ManifestProblems, Run, collectSkillFacts, commandDiffs) is <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1736_001_FourSkillcheckFunctionsFitTheRatchetLimit ./acs/cycle1736/..."
  - criterion: "None of the 4 internal/skillcheck keys remain in go/internal/sizeratchet/offenders.json (a fixed function's allowance is deleted, never just lowered)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1736_002_OffendersJSONHasNoSkillcheckEntriesLeft ./acs/cycle1736/..."
  - criterion: "The repo-wide sizeratchet.Check gate reports zero problems — defeats delete-without-shrink (an unlisted function past the limit); since 2026-09-28 an allowance is a ceiling, so shrink-without-delete is slack here and the _002 key-absence check catches it"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1736_003_ModuleWideRatchetCheckPasses ./acs/cycle1736/..."
  - criterion: "Every *_test.go under go/internal/skillcheck that exists at the cycle baseline (b22dea3b) is unmodified and undeleted; added characterization tests are allowed"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1736_004_SkillcheckTestFilesUnchangedFromBaseline ./acs/cycle1736/..."
  - criterion: "go test -count=1 ./internal/skillcheck passes (behavior preserved through every extraction step)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1736_005_SkillcheckPackageTestsPass ./acs/cycle1736/..."
  - criterion: "go vet ./internal/skillcheck exits 0 (the extraction introduced no compile or vet regression in the package or its tests)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1736_006_SkillcheckPackageVetsClean ./acs/cycle1736/..."
  - criterion: "No comment lines are added under go/internal/skillcheck — `commentaudit comments -base b22dea3b internal/skillcheck` exits 0 (production and test files alike)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1736_007_NoCommentLinesAddedToSkillcheck ./acs/cycle1736/..."
  - criterion: "The doc comment of each of the 4 target functions is identical to its baseline text"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1736_008_TargetFunctionDocsMatchBaseline ./acs/cycle1736/..."
  - criterion: "The skillcheck test suite kills all 20 behavior mutants of the 4 target functions (characterization tests pin the behavior the pre-existing suite left unpinned)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1736_009_SkillcheckTestsKillBehaviorMutants ./acs/cycle1736/..."
  - criterion: "No baseline comment line is deleted from the changed internal/skillcheck sources — the extraction moves comments with their code, it never strips a why (docs/conventions/code-comments.md:43)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1736_010_NoBaselineCommentDeletedFromSkillcheck ./acs/cycle1736/..."
---

# Eval: sizeratchet-shrink-skillcheck

> Pins the extraction refactor of the four oversized `go/internal/skillcheck`
> functions — `Run` (113 lines), `commandDiffs` (71), `ManifestProblems` (69),
> `collectSkillFacts` (51) — down to the `sizeratchet.MaxLines` (50) bar, with
> their `go/internal/sizeratchet/offenders.json` allowances removed, behavior
> unchanged, and no comments added or stripped. Source: inbox item
> `2026-09-28T05-13-00Z-sizeratchet-shrink-skillcheck.json` (read from
> `.evolve/inbox/processing/cycle-1736/`; its three acceptance criteria are
> materialized 1:1 in `test-report.md`), fleet lane
> `sizeratchet-shrink-skillcheck`, cycle 1736.
>
> Source incident: the sibling lane `sizeratchet-shrink-opscmd` (cycle 1730)
> took four audit rounds because its first contract left the comment and
> characterization clauses implicit; `sizeratchet-shrink-gc` (cycle 1732)
> encoded them from the start and shipped. This contract reuses that shape.
> A mutation probe at baseline `b22dea3b` ran 23 candidate behavior mutants
> across the four functions. The existing skillcheck suite kills 3 of them
> and lets 20 survive: ManifestProblems 6, Run 7, collectSkillFacts 2,
> commandDiffs 5. Predicate 009 requires characterization tests that kill
> all 20. A throwaway test file, applied with `go test -overlay`, killed each
> one before the contract was frozen, so none is an equivalent mutant.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | All 4 functions <=50 lines | 8/10 | `go test -run TestC1736_001...` |
| offenders-cleanup | All 4 offenders.json keys removed | 8/10 | `go test -run TestC1736_002...` |
| ratchet-gate | Module-wide ratchet Check reports zero problems | 9/10 | `go test -run TestC1736_003...` |
| test-files-frozen | Baseline skillcheck `*_test.go` files unmodified/undeleted | 7/10 | `go test -run TestC1736_004...` |
| behavior-preserved | `go test ./internal/skillcheck` passes | 8/10 | `go test -run TestC1736_005...` |
| vet-clean | `go vet ./internal/skillcheck` exits 0 | 6/10 | `go test -run TestC1736_006...` |
| no-comments-added | `commentaudit comments` lists zero added lines in skillcheck | 8/10 | `go test -run TestC1736_007...` |
| target-docs-intact | 4 target doc comments equal baseline | 5/10 | `go test -run TestC1736_008...` |
| characterization | skillcheck suite kills all 20 behavior mutants | 7/10 | `go test -run TestC1736_009...` |
| no-comments-deleted | No baseline comment line lost from the changed skillcheck sources | 6/10 | `go test -run TestC1736_010...` |
