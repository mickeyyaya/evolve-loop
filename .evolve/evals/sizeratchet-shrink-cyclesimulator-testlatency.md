---
score_cap:
  - criterion: "Each of the 4 named functions (cmd/testlatency Parse, Report.Markdown; internal/cyclesimulator Run, appendSimLedger) is <=50 lines (sizeratchet.MaxLines), measured by the ratchet's own AST-based scanner"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_001_FourCyclesimulatorTestlatencyFunctionsFitTheRatchetLimit ./acs/cycle1745/..."
  - criterion: "go/internal/sizeratchet/offenders.json is byte-unchanged since the baseline (cc16fb89) and still lists the 4 keys at 69/60/152/71 — an allowance is a ceiling, so this lane never edits the shared file"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_002_OffendersJSONLeftUnchanged ./acs/cycle1745/..."
  - criterion: "The module-wide sizeratchet.Check reports zero problems — no extracted helper lands past 50 lines unlisted and no listed function grows past its allowance"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_003_ModuleWideRatchetCheckPasses ./acs/cycle1745/..."
  - criterion: "Every *_test.go and testdata fixture under go/internal/cyclesimulator and go/cmd/testlatency that exists at cc16fb89 is unmodified and undeleted; added characterization tests are allowed"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_004_BaselineTestFilesUnchanged ./acs/cycle1745/..."
  - criterion: "go test -count=1 ./internal/cyclesimulator passes (behavior preserved through every extraction step)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_005_CyclesimulatorPackageTestsPass ./acs/cycle1745/..."
  - criterion: "go test -count=1 ./cmd/testlatency passes (behavior preserved through every extraction step)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_006_TestlatencyPackageTestsPass ./acs/cycle1745/..."
  - criterion: "go vet exits 0 and gofmt -l lists nothing on both packages — a line count shrunk by hand-joining statements is not an extraction"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_007_BothPackagesVetAndGofmtClean ./acs/cycle1745/..."
  - criterion: "No comment lines are added under go/internal/cyclesimulator or go/cmd/testlatency — `commentaudit comments -base cc16fb89` over both dirs exits 0 (production and test files alike)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_008_NoCommentLinesAdded ./acs/cycle1745/..."
  - criterion: "The doc comment of each of the 4 target functions is identical to its baseline text"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_009_TargetFunctionDocsMatchBaseline ./acs/cycle1745/..."
  - criterion: "Both packages' non-test sources changed and no baseline comment line was deleted from them — the in-body comments of Run, appendSimLedger, Parse and Markdown move with their code (docs/conventions/code-comments.md:3)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_010_NoBaselineCommentDeleted ./acs/cycle1745/..."
  - criterion: "The cyclesimulator test suite kills all 50 behavior mutants of Run and appendSimLedger that the baseline suite let survive"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_011_CyclesimulatorTestsKillBehaviorMutants ./acs/cycle1745/..."
  - criterion: "The testlatency test suite kills all 30 behavior mutants of Parse and Report.Markdown that the baseline suite let survive"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_012_TestlatencyTestsKillBehaviorMutants ./acs/cycle1745/..."
  - criterion: "cyclesimulator.Run and appendSimLedger produce byte-identical stderr, exit codes, seam calls, artifacts, ledger lines and ledger.tip to the baseline cc16fb89 source across 24 probe scenarios (validation, every refusal, every write failure, default seams, run_id, git repo)"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_013_CyclesimulatorBehaviorMatchesBaseline ./acs/cycle1745/..."
  - criterion: "testlatency Parse and Report.Markdown produce byte-identical reports, errors and Markdown to the baseline cc16fb89 source across 5 probe streams x 3 option sets"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_014_TestlatencyBehaviorMatchesBaseline ./acs/cycle1745/..."
  - criterion: "Every `name` (before → after) size claim in the cycle-1745 build explanation equals the sizeratchet measurement: before = the cc16fb89 length, after = sizeratchet.Walk on the shipped sources; all 4 targets are claimed"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_015_ExplanationSizeClaimsMatchTheRatchetScanner ./acs/cycle1745/..."
  - criterion: "The build explanation's Limitations still discloses the map-random equal-wall Packages order, gives the behavior-change / out-of-scope reason for leaving it, and does not credit the differential probe with pinning it (the probe dump is identical under ascending and descending tie-breaks)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_016_ExplanationDoesNotCreditTheProbeWithPinningEqualWallOrder ./acs/cycle1745/..."
  - criterion: "The rawgitratchet ship gate (part of the repo-contract scanner pack) reports zero problems module-wide — no test outside internal/gittest builds a raw git repo unless baseline.json lists it"
    max_if_missing: 9
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_017_NoRawGitFixtureOutsideGittest ./acs/cycle1745/..."
  - criterion: "go/internal/rawgitratchet/baseline.json is byte-unchanged since cc16fb89 — the lane moves its fixture to gittest instead of granting itself a slot"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_018_RawGitBaselineLeftUnchanged ./acs/cycle1745/..."
  - criterion: "Under a recording git shim, the cyclesimulator tests commit in at least one repo, and every repo they commit in had maintenance.auto=false set first (the gittest.Fixture guarantee)"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1745_019_CyclesimulatorTestReposDisableMaintenanceBeforeCommitting ./acs/cycle1745/..."
---

# Eval: sizeratchet-shrink-cyclesimulator-testlatency

> Pins the extraction refactor of four oversized functions to the
> `sizeratchet.MaxLines` (50) bar:
>
> - `Parse` (69 lines) and `Report.Markdown` (60) in `go/cmd/testlatency`
> - `Run` (152) and `appendSimLedger` (71) in `go/internal/cyclesimulator`
>
> Behavior must stay the same, and no comment may be added or stripped.
> `go/internal/sizeratchet/offenders.json` must stay byte-unchanged. Since
> fix/sizeratchet-is-a-ceiling (2026-09-28), an allowance is a ceiling, and
> two shrink lanes' keys sit on adjacent lines, so editing the file would
> conflict at the fleet rebase.
>
> Source: inbox item
> `2026-09-28T05-19-00Z-sizeratchet-shrink-cyclesimulator-testlatency.json`,
> read from `.evolve/inbox/processing/cycle-1745/`. `test-report.md` maps its
> three acceptance criteria 1:1. This is a fleet lane of cycle 1745.
>
> **Source incidents:** the sibling shrink lanes of cycles 1730, 1732, 1738,
> 1742 and 1744. In cycle 1730 (opscmd), the audit took four rounds because
> characterization and comment discipline were left implicit.
>
> **Mutation probe:** a baseline probe of 92 candidates found 80 behavior
> mutants that the existing suites do not kill, 50 in cyclesimulator and 30 in
> testlatency. The gaps:
> - No test pinned `Run`'s stderr log lines, the simulator-report body, the
>   ledger roles, or a gate refusal in the middle of the phase loop.
> - The default seams (PluginRoot fallback, pid token, advance/ship argv) were
>   unpinned. So were `appendSimLedger`'s UTC timestamp, model and duration
>   fields, git_head and tree-state SHA, and its ledger.tip format, rename and
>   temp cleanup.
> - `Parse` had no pin for start events, fail summaries, tie-breaks, the
>   Incomplete sort, or the 16 MiB buffer.
> - `Markdown` had no golden test.
>
> Throwaway tests that pass on baseline killed every one of the 80. With them
> in place, predicates 011 and 012 went GREEN. They were deleted before the
> contract froze.
>
> **Differential predicates:** 013 and 014 overlay the baseline `cc16fb89`
> sources and compare their probe dumps byte for byte against the worktree's.
> The dumps are normalized only for the temp root, pid, recent time and
> self-consistent hashes. The comparison caught 92/92 candidate mutants
> through the probe alone.
>
> **Explanation predicates (audit round 1 FAIL, M1/M2):** the code passed,
> but the build explanation said `Report.Markdown` shrank to 9 lines (the
> scanner measures 8) and that the differential probe pins the equal-wall
> order (the probe sorts by `Pkg`). Both claims were written by hand.
> 015 compares each size claim with `sizeratchet.Walk`. 016 runs the probe
> under both tie-break orders to show it cannot see the order, then rejects
> any Limitations sentence that says it pins it.
>
> **Repo-contract predicates (audit round 3 FAIL, H1):** rounds 1 and 2
> passed, but ship refused with `REPO_CONTRACT_GATE`. The new
> `internal/cyclesimulator/characterization_test.go` ran `git init` through a
> hand-rolled `exec.Command` fixture instead of `gittest.Fixture(t)`, so
> `rawgitratchet.TestRatchet_NoNewRawGitFixtures` went red. No predicate ran
> that gate. 017 runs the ratchet in-process over the module. 018 blocks the
> fix of listing the file in `baseline.json`. 019 records every git call the
> cyclesimulator tests make through a PATH shim, and requires each repo they
> commit in to set maintenance.auto=false first. That also catches a fixture
> that dodges the static scanner.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| line-limit | All 4 functions <=50 lines | 8/10 | `go test -run TestC1745_001...` |
| offenders-untouched | offenders.json byte-unchanged, 4 allowances intact | 8/10 | `go test -run TestC1745_002...` |
| ratchet-gate | Module-wide ratchet Check reports zero problems | 9/10 | `go test -run TestC1745_003...` |
| test-files-frozen | Baseline `*_test.go` and testdata in both packages unmodified/undeleted | 7/10 | `go test -run TestC1745_004...` |
| behavior-preserved (cyclesimulator) | `go test ./internal/cyclesimulator` passes | 8/10 | `go test -run TestC1745_005...` |
| behavior-preserved (testlatency) | `go test ./cmd/testlatency` passes | 8/10 | `go test -run TestC1745_006...` |
| vet-gofmt-clean | `go vet` exits 0 and `gofmt -l` is empty on both packages | 6/10 | `go test -run TestC1745_007...` |
| no-comments-added | `commentaudit comments` lists zero added lines | 8/10 | `go test -run TestC1745_008...` |
| target-docs-intact | 4 target doc comments equal baseline | 5/10 | `go test -run TestC1745_009...` |
| no-comments-deleted | No baseline comment line lost from changed sources | 6/10 | `go test -run TestC1745_010...` |
| characterization (cyclesimulator) | cyclesimulator suite kills all 50 behavior mutants | 7/10 | `go test -run TestC1745_011...` |
| characterization (testlatency) | testlatency suite kills all 30 behavior mutants | 7/10 | `go test -run TestC1745_012...` |
| baseline-differential (cyclesimulator) | 24-scenario probe dump equals the baseline's | 9/10 | `go test -run TestC1745_013...` |
| baseline-differential (testlatency) | 5-stream x 3-option probe dump equals the baseline's | 9/10 | `go test -run TestC1745_014...` |
| explanation-sizes | Doc size claims equal the ratchet scanner's measurements | 6/10 | `go test -run TestC1745_015...` |
| explanation-limitations | Doc discloses the equal-wall quirk with a true reason, not probe-pinning | 6/10 | `go test -run TestC1745_016...` |
| rawgit-ratchet | rawgitratchet Check is clean module-wide (ship repo-contract gate) | 9/10 | `go test -run TestC1745_017...` |
| rawgit-baseline-untouched | `rawgitratchet/baseline.json` byte-unchanged, no slot granted | 8/10 | `go test -run TestC1745_018...` |
| quiet-fixture | cyclesimulator test repos disable maintenance.auto before their first commit | 8/10 | `go test -run TestC1745_019...` |
