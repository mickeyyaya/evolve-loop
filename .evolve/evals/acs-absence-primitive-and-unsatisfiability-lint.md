---
score_cap:
  - criterion: "an absence assertion is expressible directly through acsassert.FileNotContains"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestFileNotContains_' ./pkg/acsassert"
  - criterion: "the unsatisfiability lint flags the live cycle-1488 TestC1488_003 bytes once unrepaired and every self-reporting positive primitive (FileContains, FileMatchesRegex, FileExists, JSONFieldEquals) used as a bare failure condition"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestLintUnsatisfiablePredicates_(FlagsTheLiveCycle1488PredicateOnceUnrepaired|FlagsOnlyUnconditionalSelfReportingShapes)$' ./internal/evalqualitycheck"
  - criterion: "satisfiable shapes are never flagged, including a primitive called with a TB that swallows its own report, and a missing path or empty directory is an error, never a clean report"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestLintUnsatisfiablePredicates_(FlagsOnlyUnconditionalSelfReportingShapes|RefusesAMissingPathAndAnEmptyDirectory)$' ./internal/evalqualitycheck"
  - criterion: "evolve eval quality-check runs the lint: findings raise PASS to WARN, a tautology HALT stays HALT, and the unsatisfiable-lint receipt prints on every run"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run 'TestRunEval_QualityCheckPredicatesUnsatisfiableLint$' ./internal/cli/guardcmd && go test -count=1 -run 'TestDispatch_EvalQualityCheckPrintsTheUnsatisfiableLintReceipt$' ./cmd/evolve"
  - criterion: "the production TDD-phase review (evalgate.NewReviewer) names an unsatisfiable predicate before build dispatch and logs a receipt on a clean package"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run 'TestUnsatisfiableShapeGate_|TestNewReviewer_UnsatisfiablePredicateSurfacesButNeverBlocksAtEnforce$|TestPredicateLintGates_|TestPredicateGates_' ./internal/evalgate"
  - criterion: "the corpus sweep lints every .go file under go/acs, including the nested regression packages, fails on an inverted-idiom or go-run-exit-code finding, and only logs an absence-message finding"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run '(TestLintUnsatisfiablePredicates_GoAcsCorpusHasNoProofKindFinding|TestCorpusProofFindings_ReturnsProofKindsAndOnlyLogsAnAbsenceMessage)$' ./internal/evalqualitycheck"
  - criterion: "an ExitCode() on a go run result compared to a code go run cannot return is flagged (cycle 1788's TestC1788_008), and a built binary's exit code, a shadowed name, or 2 after go flags is not"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestLintUnsatisfiablePredicates_(FlagsCycle1788ExitCodeThroughGoRun|GoRunExitCodeRuleSparesReachableCodesAndBuiltBinaries)$' ./internal/evalqualitycheck"
  - criterion: "touched packages vet and race clean"
    max_if_missing: 6
    evidence: "cd go && go vet ./internal/evalqualitycheck ./internal/evalgate ./internal/cli/guardcmd && go test -race -count=1 ./internal/evalqualitycheck ./internal/evalgate ./internal/cli/guardcmd"
---

# Eval: ACS unsatisfiability lint in the eval-quality pre-flight

> Pins acs-absence-primitive-and-unsatisfiability-lint and unsatisfiable-lint-flags-exit-codes-through-go-run. acsassert.FileNotContains already exists. The open half is a lint that flags a predicate that cannot pass on any tree: a self-reporting positive primitive used as a bare failure condition, a negated positive primitive whose failure text demands absence, or an exit code that `go run` can never return. Source incident: cycle 1488 TestC1488_003, which burned cycles 1488, 1492 and 1495. Cycles 1788 and 1793 built the lint and each failed on a predicate no lane could satisfy (an exit code through `go run`, then output only the protected guardcmd formats). The console landed the lint, its CLI wiring and its host gate, and moved the acceptance from the two cycles' predicate packages (archived under docs/private/research/archived-2026-10-05/superseded-predicate-packages/) into the package tests named below.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| absence-primitive | FileNotContains holds in every state | 5/10 | `go test -run TestFileNotContains_ ./pkg/acsassert` |
| live-bytes + family | 1488 bytes and all self-reporting primitives flagged | 7/10 | `go test -run 'TestLintUnsatisfiablePredicates_(FlagsTheLiveCycle1488PredicateOnceUnrepaired\|FlagsOnlyUnconditionalSelfReportingShapes)$' ./internal/evalqualitycheck` |
| no-false-positive | swallowing-TB and absence-primitive shapes clean; bad paths loud | 7/10 | `go test -run 'TestLintUnsatisfiablePredicates_(FlagsOnlyUnconditionalSelfReportingShapes\|RefusesAMissingPathAndAnEmptyDirectory)$' ./internal/evalqualitycheck` |
| cli-preflight | WARN on finding, HALT preserved, receipt always | 8/10 | `go test -run TestRunEval_QualityCheckPredicatesUnsatisfiableLint ./internal/cli/guardcmd` and `go test -run TestDispatch_EvalQualityCheckPrintsTheUnsatisfiableLintReceipt ./cmd/evolve` |
| host-wired | TDD-phase review names the finding | 8/10 | `go test -run 'TestUnsatisfiableShapeGate_\|TestNewReviewer_UnsatisfiablePredicateSurfacesButNeverBlocksAtEnforce$' ./internal/evalgate` |
| corpus-sweep | whole go/acs tree linted, zero proof-kind findings asserted, heuristic findings logged | 6/10 | `go test -v -run TestLintUnsatisfiablePredicates_GoAcsCorpusHasNoProofKindFinding ./internal/evalqualitycheck` |
| go-run exit code | TestC1788_008 flagged; 0, 1 and built binaries spared | 6/10 | `go test -run 'TestLintUnsatisfiablePredicates_(FlagsCycle1788ExitCodeThroughGoRun\|GoRunExitCodeRuleSparesReachableCodesAndBuiltBinaries)$' ./internal/evalqualitycheck` |
| hygiene | vet and -race | 6/10 | `go vet` and `go test -race` on the three packages |
