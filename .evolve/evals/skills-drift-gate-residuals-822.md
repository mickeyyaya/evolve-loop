---
score_cap:
  - criterion: "A drift report with exit 2 at the deadline instant is a FAIL with the drift offenders, and a deadline with no drift report stays a NOT-graded WARN"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^(TestWorktreeSkillsDrift_Exit2AtDeadlineWithDriftIsOffender|TestWorktreeSkillsDrift_DeadlineWithoutADriftReportStaysUngraded|TestWorktreeSkillsDrift_ATimeoutCannotGradeAndSaysSo)$' ./internal/phases/audit"
  - criterion: "Cancelling the phase context stops the worktree run and reports context.Canceled, not a deadline"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^(TestWorktreeSkillsDrift_ContextCancellationStopsRun|TestWorktreeSkillsDrift_CancelledPhaseIsNotADeadline)$' ./internal/phases/audit"
  - criterion: "A missing go binary yields a named could-not-run WARN that wraps exec.ErrNotFound"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestWorktreeSkillsDrift_MissingGoBinaryWarns$' ./internal/phases/audit"
  - criterion: "The cycle 1848 acceptance predicates pass"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs ./acs/cycle1848"
---

# Eval: skills-drift gate residuals after #822

> Pins the three LOW residuals that the PR #822 architecture delta review left in
> `go/internal/phases/audit/skillsdrift.go` (`worktreeSkillsDrift`): an exit-2 drift
> report that lands at the deadline must fail the gate instead of reading as a skip
> WARN; the worktree run must derive its context from the phase context so a
> cancellation stops it; a missing `go` binary stays an informative NOT-graded WARN.
> Source incident: inbox item skills-drift-gate-residuals-822, cycle 1848.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| deadline-drift-fail | exit 2 + drift report at the deadline is a FAIL | 8/10 | `go test -run TestWorktreeSkillsDrift_Exit2AtDeadlineWithDriftIsOffender` |
| phase-cancellation | a cancelled phase context stops the run | 8/10 | `go test -run TestWorktreeSkillsDrift_ContextCancellationStopsRun` |
| missing-go-warn | no go on PATH is a named skip WARN | 6/10 | `go test -run TestWorktreeSkillsDrift_MissingGoBinaryWarns` |
| acs | cycle 1848 predicates | 7/10 | `go test -tags acs ./acs/cycle1848` |
