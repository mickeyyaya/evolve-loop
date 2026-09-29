---
score_cap:
  - criterion: "All three named offenders (posteditvalidate.Run, evalqualitycheck.CheckDiversity, verifyeval.Verify) measure <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1765_001_ThreeTargetFunctionsFitTheRatchetLimit ./acs/cycle1765"
  - criterion: "go/internal/sizeratchet/offenders.json is byte-unchanged since 156ee9ab (this lane's merge-base with origin/main) and still lists all three entries at their original allowances — an allowance is a ceiling, so a lane never edits the file"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1765_002_OffendersJSONLeftUnchanged ./acs/cycle1765"
  - criterion: "The module-wide sizeratchet.Check reports zero problems"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1765_003_ModuleWideRatchetCheckPasses ./acs/cycle1765"
  - criterion: "Every baseline *_test.go under the three target packages (posteditvalidate, evalqualitycheck, verifyeval) is unmodified and undeleted — behavior is pinned by the existing suites, not weakened to pass"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1765_004_BaselineTestFilesUnchanged ./acs/cycle1765"
  - criterion: "go test -count=1 passes unmodified for all three target packages"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1765_005_TargetPackageTestsPass ./acs/cycle1765"
  - criterion: "The three target packages are go vet clean and gofmt-formatted after extraction"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1765_006_TargetPackagesVetAndGofmtClean ./acs/cycle1765"
  - criterion: "No comment lines are added anywhere under the three target packages (docs/conventions/code-comments.md: names carry the intent)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1765_007_NoCommentLinesAdded ./acs/cycle1765"
  - criterion: "Every target package has at least one non-test .go file changed (the extraction actually happened) and no baseline comment line is dropped from any of them"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1765_008_NoCommentsLostFromShrunkFunctions ./acs/cycle1765"
  - criterion: "This hygiene lane touches only the three named packages — no scope creep into unrelated packages"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1765_009_OnlyTargetPackagesTouched ./acs/cycle1765"
---

# Eval: sizeratchet-shrink-eval-validators

> Fleet lane `sizeratchet-shrink-eval-validators`, cycle 1765 — the
> continuation of the unshipped cycle-1761 and cycle-1764 attempts at this
> item, whose predicate packages the harness archived under
> `docs/private/research/archived-2026-09-29/superseded-predicate-packages/`.
> The inbox record at
> `.evolve/inbox/2026-09-29T20-12-00Z-sizeratchet-shrink-eval-validators.json`
> is unreadable in this worktree (no such file), so per the harness's Task
> Contract the triage report and this eval file are the acceptance authority.
> The size ratchet (`go/internal/sizeratchet`), measured at base `156ee9ab`
> (this lane's merge-base with origin/main, 2026-09-29), lists three live
> offenders across three non-protected leaf packages:
> `internal/posteditvalidate.Run` (108 lines, `posteditvalidate.go:60`),
> `internal/evalqualitycheck.CheckDiversity` (59 lines, `diversity.go:58`),
> `internal/verifyeval.Verify` (53 lines, `verifyeval.go:86`). Each must shrink
> to <=50 lines by extracting named steps at one level of abstraction, with
> behavior unchanged and no comments added. `offenders.json` is left
> untouched — its entries become slack for a later boundary tighten, per the
> 2026-09-28 ceiling-only rule this repo already enforces (see the
> `sizeratchet-shrink-release-tooling`, `sizeratchet-naminguard-fix-shrink`
> precedent evals in this directory).
>
> All three target functions already carry extensive existing branch
> coverage per the scout report: `posteditvalidate_test.go` covers every
> switch arm of `Run` (skip/no-payload/bypass/no-file-path/file-missing/json
> valid+invalid/bash valid+error/py valid+error/guards-log-append/noop);
> `diversity_test.go` covers every `CheckDiversity` level plus slug-filter,
> skip, and per-file-fingerprint branches; `verifyeval_test.go` +
> `verifyeval_execution_integration_test.go` cover every `Verify` predicate
> combination and runner-error path. No untested branch was found, so no
> characterization test is required before extraction — TestC1765_004
> freezes those baseline test files so Builder cannot loosen an assertion to
> make the shrink easier, and TestC1765_005 requires them to keep passing
> unmodified. TestC1765_001 is the ratchet's own scanner (red at base
> `156ee9ab`, where `Run` is 108 lines); TestC1765_008 requires an actual
> per-package diff (also red at base — "the extraction has not happened")
> while guarding that no baseline comment is lost in the process.
>
> Source incidents: cycle 1761 authored this eval; cycle 1765's first audit
> round (M1) rejected it because every evidence command still ran the
> archived cycle-1761 package and exited 1 with "directory not found", while
> `evolve eval quality-check` passed it (score_cap evidence is never executed
> there). Every evidence command below is now executed by TestC1765_010,
> which also rejects a `-run` that selects no test.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | all 3 targets <=50 lines | 9/10 | `go test -run TestC1765_001...` |
| ceiling-untouched | offenders.json byte-unchanged since `156ee9ab`, values held | 8/10 | `go test -run TestC1765_002...` |
| ratchet-gate | module-wide ratchet Check passes | 9/10 | `go test -run TestC1765_003...` |
| test-files-frozen | baseline tests unmodified across 3 packages | 8/10 | `go test -run TestC1765_004...` |
| behavior-preserved | `go test` passes across 3 packages | 9/10 | `go test -run TestC1765_005...` |
| style-clean | vet/gofmt clean | 6/10 | `go test -run TestC1765_006...` |
| no-comments-added | no new comment lines | 7/10 | `go test -run TestC1765_007...` |
| extraction-happened + no-comments-lost | each package changed, no comment dropped | 8/10 | `go test -run TestC1765_008...` |
| scope-fence | only the 3 target packages touched | 7/10 | `go test -run TestC1765_009...` |
