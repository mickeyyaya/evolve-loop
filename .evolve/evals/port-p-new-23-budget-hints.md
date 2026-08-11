# Eval: port-p-new-23-budget-hints
## Code Graders (bash commands that must exit 0)
- `[code]` `go test -v github.com/mickeyyaya/evolve-loop/go/internal/phases/runner -run TestRun_InjectsAdvisoryBudgetHintWhenProfileHasTurnBudgetHint`
## Regression Evals (full test suite)
- `[code]` `go test ./internal/...`
## Acceptance Checks
- `[code]` `go test -v github.com/mickeyyaya/evolve-loop/go/internal/phases/runner -run TestRun_InjectsAdvisoryBudgetHintWhenProfileHasTurnBudgetHint`
## Thresholds
- All checks: pass@1 = 1.0
