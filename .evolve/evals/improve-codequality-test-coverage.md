# Eval: Improve Code Quality Test Coverage

## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go test -count=1 -cover ./internal/codequality/...`

## Regression Evals (full test suite)
- `[code]` `cd go && go test ./...`

## Acceptance Checks
- `[code]` `cd go && go test -count=1 -run TestUnformattedGoFiles_MissingGofmtBinary ./internal/codequality/...`

## Thresholds
- All checks: pass@1 = 1.0
