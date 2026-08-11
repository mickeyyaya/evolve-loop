# Eval: commitgate lane coverage
## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go test ./internal/commitgate -run 'Test.*Lane|TestDetectLangs|TestGolden' -count=1`
## Regression Evals (full test suite)
- `[code]` `cd go && go test ./internal/commitgate -cover -count=1`
## Acceptance Checks
- `[code]` `cd go && go test ./internal/commitgate -coverprofile=/tmp/commitgate-cycle410.out -count=1 && go tool cover -func=/tmp/commitgate-cycle410.out | awk '/total:/ { sub(/%/, "", $3); exit !($3 >= 75.0) }'`
## Negative Cases
- `[code]` `cd go && go test ./internal/commitgate -run TestReviewersSatisfied -count=1`
## Edge Cases
- `[code]` `cd go && go test ./internal/commitgate -run TestDetectLangs_Empty -count=1`
## Thresholds
- All checks: pass@1 = 1.0
