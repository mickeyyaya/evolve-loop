---
score_cap:
  - criterion: "The go-run-exit-code lint flags an exit code copied into a variable (ExitError or ProcessState) and the exit code SubprocessOutput returns for a go run argv, and spares reachable codes and built binaries"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestLintUnsatisfiablePredicates_GoRunExitCodeRuleFollowsExitCodeVariables$' ./internal/evalqualitycheck/"
  - criterion: "The finding on a copied go run exit code names the unreachable code and the go build remedy"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestLintUnsatisfiablePredicates_GoRunExitCodeVariableReasonNamesTheCodeAndRemedy$' ./internal/evalqualitycheck/"
  - criterion: "evolve eval quality-check -predicates reports both variable shapes as unsatisfiable[go-run-exit-code] and leaves reachable codes unflagged"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1846_00[123]_' ./acs/cycle1846"
  - criterion: "The go/acs corpus still has no proof-kind unsatisfiable finding"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestLintUnsatisfiablePredicates_GoAcsCorpusHasNoProofKindFinding$' ./internal/evalqualitycheck/"
---

# Eval: The go-run exit-code lint follows exit codes in variables

> Pins the extension of evalqualitycheck's go-run-exit-code rule (cycle 1846). `go run` reports any
> non-zero exit of the program it runs as 1, so a predicate that compares that exit code to 3 is red
> on every tree. Before this cycle, the rule saw only a direct `ExitCode()` call. It missed
> `code := ee.ExitCode(); if code != 3` and `_, _, code, _ := acsassert.SubprocessOutput("go", "run", ...); code != 3`.
> Source incident: the cycle-1788 unsatisfiable predicate class (lint landed in cycles 1488/1492/1495);
> inbox item unsatisfiable-lint-follows-exit-codes-in-variables (2026-10-05).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| variable-shapes-flagged / reachable-spared | Both variable shapes are flagged; codes 0, 1, 2-with-flags and built-binary codes stay unflagged | 8/10 | `go test -run TestLintUnsatisfiablePredicates_GoRunExitCodeRuleFollowsExitCodeVariables` |
| reason-names-remedy | The reason names the code and the go build remedy | 6/10 | `go test -run TestLintUnsatisfiablePredicates_GoRunExitCodeVariableReasonNamesTheCodeAndRemedy` |
| cli-caller | The quality-check CLI path reports the finding | 7/10 | `go test -tags acs -run TestC1846_00[123]_ ./acs/cycle1846` |
| corpus-clean | The go/acs corpus has no proof-kind finding | 7/10 | `go test -run TestLintUnsatisfiablePredicates_GoAcsCorpusHasNoProofKindFinding` |
