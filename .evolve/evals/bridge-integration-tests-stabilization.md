# Eval: Stabilize Bridge Integration Tests

## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go test -count=1 -tags integration ./internal/bridge/...`

## Regression Evals (full test suite)
- `[code]` `cd go && go test ./...`

## Acceptance Checks
- `[code]` `cd go && go test -count=1 -tags integration -run TestRealTmux_HappyPath ./internal/bridge/...`
- `[code]` `cd go && go test -count=1 -tags integration -run TestExecRunner_WritesPIDFile ./internal/bridge/...`

## Thresholds
- All checks: pass@1 = 1.0
