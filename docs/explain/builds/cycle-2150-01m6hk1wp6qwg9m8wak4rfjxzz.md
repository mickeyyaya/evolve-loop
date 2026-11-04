# Build Explanation — Cycle 2150

## Build Binding
- Cycle: 2150
- Base SHA: 23d09f7c638b71e7c9906fcf6b0f76ff387c4570

## Summary
The skills-drift gate now takes its command runner and its time limit as arguments. The skills-drift tests pass fakes through these arguments, so they run in parallel and the serial list no longer names them.

## Rationale
Eleven skills-drift tests swapped the package variables runCmd and worktreeSkillsCheckLimit, so they had to sit in serialTests. Passing the two values as arguments removes the shared state. The package variable worktreeSkillsCheckLimit had no other reader, so it is deleted.

## Changed Areas
- `go/internal/phases/audit/skillsdrift.go` — worktreeSkillsDrift takes a runner and a limit. Two new functions carry the seams: skillsDriftCheck takes the request, the runner and the limit, and newSkillsDriftCheck binds the runner and the limit. skillsDriftCheckDefault passes runCmd and the go vet budget.
- `go/internal/phases/audit/skillsdrift_test.go` — each worktree test passes a fake runner and a limit, and calls t.Parallel.
- `go/internal/phases/audit/skillsdrift_residuals_test.go` — the same change; the helper that shrank the package limit is deleted.
- `go/internal/phases/audit/audit_skillsdrift_worktree_integration_test.go` — the in-process and real-runner timeout tests inject their seams and call t.Parallel.
- `go/internal/phases/audit/parallel_serial_tests_test.go` — the eleven skills-drift entries leave the serial list.

## Design Decisions
Injection through arguments keeps production behavior the same: the default check reads runCmd per call, as before. A closure type was rejected for worktreeSkillsDrift itself, because plain arguments are the smaller change.

## Verification
The four cycle predicates pass. The race, count=3 and shuffle run of the skills-drift tests alone passes, and each test pauses three times. The full-package command of acceptance criterion 2 does not pass: it exits 1 because TestIntegrationTierLog_MatchesTheGoldenBytes fails. That test also fails on the base tree, so it is pre-existing. Acceptance criterion 3 is not met: the package wall time did not go down. The changed functions stay at 100% coverage.

## Compatibility
No exported symbol changes. The gate output is unchanged.

## Limitations
The gate limit binding to the go vet budget now lives in skillsDriftCheckDefault. No test can swap it. The package wall time does not measurably change, because the fake-runner tests were already fast.
