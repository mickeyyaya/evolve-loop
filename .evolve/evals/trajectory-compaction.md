# Eval: Introduce Trajectory Compaction & Compactor Protocol

## Code Graders (bash commands that must exit 0)
- `go test -v ./go/internal/compactor/... | grep -q "PASS"`
- `grep -q "compactor" go/internal/phases/runner/runner.go`

## Regression Evals (full test suite)
- `go test -v ./go/internal/phases/runner/... | grep -q "PASS"`
- `go test -v ./go/internal/adapters/bridge/... | grep -q "PASS"`

## Acceptance Checks (verification commands)
- `[ -f go/internal/compactor/compactor.go ]`

## Thresholds
- All checks: pass@1 = 1.0
