---
score_cap:
  - criterion: "All five named offenders (releasepreflight.resolve, releasepreflight.checkRecentAudit, releaseconsistency.Run, releasetargets.ParseConfig, versionbump.Run) measure <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1760_001_FiveTargetFunctionsFitTheRatchetLimit ./acs/cycle1760"
  - criterion: "go/internal/sizeratchet/offenders.json is byte-unchanged since b401e73d and still lists all five entries at their original allowances — an allowance is a ceiling, so a lane never edits the file"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1760_002_OffendersJSONLeftUnchanged ./acs/cycle1760"
  - criterion: "The module-wide sizeratchet.Check reports zero problems"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1760_003_ModuleWideRatchetCheckPasses ./acs/cycle1760"
  - criterion: "Every baseline *_test.go under the four target packages (releasepreflight, releaseconsistency, releasetargets, versionbump) is unmodified and undeleted — behavior is pinned by the existing suites, not weakened to pass"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1760_004_BaselineTestFilesUnchanged ./acs/cycle1760"
  - criterion: "go test -count=1 passes unmodified for all four target packages"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1760_005_TargetPackageTestsPass ./acs/cycle1760"
  - criterion: "The four target packages are go vet clean and gofmt-formatted after extraction"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1760_006_TargetPackagesVetAndGofmtClean ./acs/cycle1760"
  - criterion: "No comment lines are added anywhere under the four target packages (docs/conventions/code-comments.md: names carry the intent)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1760_007_NoCommentLinesAdded ./acs/cycle1760"
  - criterion: "Every target package has at least one non-test .go file changed (the extraction actually happened) and no baseline comment line is dropped from any of them"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1760_008_NoCommentsLostFromShrunkFunctions ./acs/cycle1760"
  - criterion: "No protected orchestrator/ship-gate surface (go/cmd/evolve, go/internal/core, go/internal/bridge, etc.) is touched by this hygiene lane"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1760_009_NoProtectedSurfaceTouched ./acs/cycle1760"
---

# Eval: sizeratchet-shrink-release-tooling

> Wave-40-boundary hygiene lane (fleet scope `sizeratchet-shrink-release-tooling`).
> The size ratchet (`go/internal/sizeratchet`), measured on main `b401e73d`
> (2026-09-29), lists five live offenders across four non-protected leaf
> packages: `internal/releasepreflight.resolve` (54 lines,
> `preflight_run.go`), `internal/releasepreflight.checkRecentAudit` (114
> lines, `releasepreflight.go`), `internal/releaseconsistency.Run` (97
> lines), `internal/releasetargets.ParseConfig` (65 lines), and
> `internal/versionbump.Run` (52 lines). Each must shrink to <=50 lines by
> extracting named steps at one level of abstraction, with behavior
> unchanged and no comments added. `offenders.json` is left untouched — its
> entries become slack for a later boundary tighten, per the 2026-09-28
> ceiling-only rule this repo already enforces (see cycle-1749/1750/1755
> precedent evals in this directory).
>
> All five target functions already carry extensive existing coverage
> (`releasepreflight_test.go` + `extra_coverage_test.go` +
> `recent_audit_scope_test.go` + `run_stage_order_test.go` for
> `releasepreflight`; `releaseconsistency_test.go`;
> `releasetargets_test.go`; `versionbump_test.go`), so the inbox item's
> "characterization test first" clause is satisfied by the existing suites
> — TestC1760_004 freezes those files so Builder cannot loosen an assertion
> to make the shrink easier, and TestC1760_005 requires them to keep passing
> unmodified. TestC1760_001 is the ratchet's own scanner (the RED signal
> proving the shrink is still outstanding); TestC1760_008 requires an actual
> per-package diff (also RED at TDD time — "the extraction has not
> happened") while guarding that no baseline comment is lost in the process.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | all 5 targets <=50 lines | 9/10 | `go test -run TestC1760_001...` |
| ceiling-untouched | offenders.json byte-unchanged, values held | 8/10 | `go test -run TestC1760_002...` |
| ratchet-gate | module-wide ratchet Check passes | 9/10 | `go test -run TestC1760_003...` |
| test-files-frozen | baseline tests unmodified across 4 packages | 8/10 | `go test -run TestC1760_004...` |
| behavior-preserved | `go test` passes across 4 packages | 9/10 | `go test -run TestC1760_005...` |
| style-clean | vet/gofmt clean | 6/10 | `go test -run TestC1760_006...` |
| no-comments-added | no new comment lines | 7/10 | `go test -run TestC1760_007...` |
| extraction-happened + no-comments-lost | each package changed, no comment dropped | 8/10 | `go test -run TestC1760_008...` |
| scope-fence | no protected surface touched | 7/10 | `go test -run TestC1760_009...` |
