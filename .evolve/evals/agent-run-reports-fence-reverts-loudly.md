---
score_cap:
  - criterion: "a fence revert prints a WARN line on the stderr of evolve agent run that names the reverted paths and the --writable next step"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^(TestAgentRun_ASeatThatCreatesAFileOutsideItsGrantPrintsOneWarnLineWithThePathAndTheNextStep|TestAgentRun_TheFenceWarnLineNamesEveryRevertedPathOfTheRun)$' ./internal/cli/agentcmd"
  - criterion: "a seat that created a file outside its grant exits with a code that no other outcome of evolve agent run uses"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestAgentRun_ASeatThatCreatesAFileOutsideItsGrantExitsWithACodeNoOtherOutcomeUses$' ./internal/cli/agentcmd"
  - criterion: "the console re-runs the seat with the corrected grant and the run passes without a WARN line"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestAgentRun_TheConsoleRerunsASeatWithTheCorrectedGrantAndTheRunPassesWithoutAWarnLine$' ./internal/cli/agentcmd"
  - criterion: "the usage documents the fence-revert exit code"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestAgentRun_TheUsageDocumentsTheFenceRevertExitCode$' ./internal/cli/agentcmd"
---

# Eval: evolve agent run reports fence-reverted files as a hard warning

> Pins the console contract of `evolve agent run` for a builder seat that writes outside its grant.
> The fence already removes the stray file and the envelope carries an error diagnostic, but the
> command stderr stays silent and the exit code (1) is shared with every other FAIL.
> The run now prints a WARN line with each reverted path and exits with a code of its own, so a
> console script can re-run with a corrected grant. Source: inbox item
> agent-run-reports-fence-reverts-loudly (builder seats F3, F4, F5, F6, K1 reported PASS after a revert).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| warn-line | WARN line names the reverted paths and the next step | 4/10 | `TestAgentRun_ASeatThatCreatesAFileOutsideItsGrantPrintsOneWarnLine...` |
| distinct-exit | exit code used by no other outcome | 4/10 | `TestAgentRun_ASeatThatCreatesAFileOutsideItsGrantExitsWithACode...` |
| rerun | corrected grant passes quietly | 6/10 | `TestAgentRun_TheConsoleRerunsASeatWithTheCorrectedGrant...` |
| usage | usage names the code | 7/10 | `TestAgentRun_TheUsageDocumentsTheFenceRevertExitCode` |
