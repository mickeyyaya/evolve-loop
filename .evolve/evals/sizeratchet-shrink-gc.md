---
score_cap:
  - criterion: "Each of the 5 named internal/gc functions (Apply, ApplyWorktrees, Discover, Plan, PlanWorktrees) is <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1732_001_FiveGcFunctionsFitTheRatchetLimit ./acs/cycle1732/..."
  - criterion: "None of the 5 internal/gc keys remain in go/internal/sizeratchet/offenders.json (a fixed function's allowance is deleted, never just lowered)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1732_002_OffendersJSONHasNoGcEntriesLeft ./acs/cycle1732/..."
  - criterion: "The repo-wide sizeratchet.Check gate reports zero problems — defeats delete-without-shrink (an unlisted function past the limit); since 2026-09-28 an allowance is a ceiling, so shrink-without-delete is slack here and the _002 key-absence check catches it"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1732_003_ModuleWideRatchetCheckPasses ./acs/cycle1732/..."
  - criterion: "Every *_test.go under go/internal/gc that exists at the cycle baseline (c4ddce65) is unmodified and undeleted; added characterization tests are allowed"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1732_004_GcTestFilesUnchangedFromBaseline ./acs/cycle1732/..."
  - criterion: "go test -count=1 ./internal/gc passes (behavior preserved through every extraction step)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1732_005_GcPackageTestsPass ./acs/cycle1732/..."
  - criterion: "go vet ./internal/gc exits 0 (the extraction introduced no compile or vet regression in the package or its tests)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1732_006_GcPackageVetsClean ./acs/cycle1732/..."
  - criterion: "No comment lines are added under go/internal/gc — `commentaudit comments -base c4ddce65 internal/gc` exits 0 (production and test files alike)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1732_007_NoCommentLinesAddedToGc ./acs/cycle1732/..."
  - criterion: "The doc comment of each of the 5 target functions is identical to its baseline text"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1732_008_TargetFunctionDocsMatchBaseline ./acs/cycle1732/..."
  - criterion: "The gc test suite kills all 16 behavior mutants of the 5 target functions (characterization tests pin the behavior the pre-existing suite left unpinned)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1732_009_GcTestsKillBehaviorMutants ./acs/cycle1732/..."
  - criterion: "No baseline comment line is deleted from the changed internal/gc sources — the extraction moves comments with their code, it never strips a why (docs/conventions/code-comments.md:43)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1732_010_NoBaselineCommentDeletedFromGc ./acs/cycle1732/..."
---

# Eval: sizeratchet-shrink-gc

> Pins the extraction refactor of the five oversized `go/internal/gc`
> functions — `Plan` (69 lines), `Apply` (55), `Discover` (57),
> `PlanWorktrees` (99), `ApplyWorktrees` (74) — down to the
> `sizeratchet.MaxLines` (50) bar, with their
> `go/internal/sizeratchet/offenders.json` allowances removed, behavior
> unchanged, and no comments added or stripped. Source: inbox item
> `2026-09-28T05-11-00Z-sizeratchet-shrink-gc.json` (read from
> `.evolve/inbox/processing/cycle-1732/`; its three acceptance criteria are
> materialized 1:1 in `test-report.md`), fleet lane `sizeratchet-shrink-gc`,
> cycle 1732.
>
> Source incident: the sibling lane `sizeratchet-shrink-opscmd` (cycle 1730)
> took four audit rounds because its first contract left criterion 3 manual
> (54 doc-comment lines added), skipped the characterization clause of
> criterion 2, and could not see a stripped baseline *why* comment. This
> contract encodes all three from the start (predicates 007-010). A
> mutation probe at the baseline found 16 behavior mutants across all five
> functions that the existing gc suite does not kill. Predicate 009 requires
> characterization tests that kill them. Each one was shown killable by a
> throwaway test before the contract was frozen.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | All 5 functions <=50 lines | 8/10 | `go test -run TestC1732_001...` |
| offenders-cleanup | All 5 offenders.json keys removed | 8/10 | `go test -run TestC1732_002...` |
| ratchet-gate | Module-wide ratchet Check reports zero problems | 9/10 | `go test -run TestC1732_003...` |
| test-files-frozen | Baseline gc `*_test.go` files unmodified/undeleted | 7/10 | `go test -run TestC1732_004...` |
| behavior-preserved | `go test ./internal/gc` passes | 8/10 | `go test -run TestC1732_005...` |
| vet-clean | `go vet ./internal/gc` exits 0 | 6/10 | `go test -run TestC1732_006...` |
| no-comments-added | `commentaudit comments` lists zero added lines in gc | 8/10 | `go test -run TestC1732_007...` |
| target-docs-intact | 5 target doc comments equal baseline | 5/10 | `go test -run TestC1732_008...` |
| characterization | gc suite kills all 16 behavior mutants | 7/10 | `go test -run TestC1732_009...` |
| no-comments-deleted | No baseline comment line lost from the changed gc sources | 6/10 | `go test -run TestC1732_010...` |
