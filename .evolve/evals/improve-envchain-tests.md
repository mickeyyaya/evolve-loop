# Eval: improve-envchain-tests
## Code Graders (bash commands that must exit 0)
- `[code]` `go test -v ./internal/envchain/...`
## Regression Evals (full test suite)
- `[code]` `go test ./...`
## Acceptance Checks
- `[code]` `grep -q "builder" go/internal/envchain/envchain_test.go`
- `[code]` `grep -q "auditor" go/internal/envchain/envchain_test.go`
## Thresholds
- All checks: pass@1 = 1.0
