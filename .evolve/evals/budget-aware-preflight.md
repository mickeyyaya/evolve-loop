# Eval: Implement Token-Budget-Aware Pre-Flight Guard

## Code Graders (bash commands that must exit 0)
- `grep -q "budget" go/internal/phases/runner/runner.go`
- `go test -v ./go/internal/phases/runner/... | grep -q "PASS"`

## Regression Evals (full test suite)
- `go test -v ./go/internal/adapters/bridge/... | grep -q "PASS"`

## Acceptance Checks (verification commands)
- `grep -q "Test.*Budget" go/internal/phases/runner/runner_test.go`

## Thresholds
- All checks: pass@1 = 1.0
