---
score_cap:
  - criterion: "A git rev-list that exits non-zero with partial stdout is an error for the git-evidence detector, not evidence"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestGitEvidence_RevListExitOneWithPartialStdoutIsAnErrorNotEvidence$' ./internal/bridge"
  - criterion: "A non-zero git log or rev-parse exit never verifies a commit or sets the HEAD baseline"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestGitEvidence_LogExitOneWithTheEvidenceMessageIsNotEvidence|TestGitEvidence_RevParseExitOneNeverBecomesTheBaseline)$' ./internal/bridge"
  - criterion: "The bridge fake git and its contract suite assert that a non-zero exit gives an error for every subcommand"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestExitingGit_ReportsTheScriptedExitCodeAndTheStdout|TestGitEvidence_GitCmdContract_NonZeroExitIsAnErrorForEverySubcommand|TestGitEvidence_GitCmdContract_ZeroExitKeepsTheTrimmedStdout)$' ./internal/bridge"
---

# Eval: git-evidence detector honours the exit code of git

> Pins the contract that `gitEvidenceDetector.gitCmd` returns an error for any non-zero git exit. The bridge `CmdRunner` reports a non-zero exit as `(code, nil)`, so a detector that keeps only the error read a failed `rev-list` with partial stdout as a list of commits. Source incident: inbox item bridge-git-evidence-drops-the-exit-code, cycle 2490.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| failed-rev-list | rev-list exit 1 with partial stdout is not evidence | 4/10 | `go test -run TestGitEvidence_RevListExitOneWithPartialStdoutIsAnErrorNotEvidence ./internal/bridge` |
| failed-log-or-rev-parse | log and rev-parse non-zero exits are not data | 5/10 | `go test -run TestGitEvidence_LogExitOne ./internal/bridge` |
| fake-contract | the fake git and the gitCmd contract assert error on non-zero exit | 6/10 | `go test -run TestGitEvidence_GitCmdContract_... ./internal/bridge` |
