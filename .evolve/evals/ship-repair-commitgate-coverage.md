# Eval: ship repair and commit-gate coverage
## Code Graders (bash commands that must exit 0)
- `[code]` `cd go && go test ./internal/phases/ship -run 'Test.*CommitGate|Test.*Repair|Test.*Manual|Test.*Reviewed' -count=1`
## Regression Evals (full test suite)
- `[code]` `cd go && go test ./internal/phases/ship -cover -count=1`
## Acceptance Checks
- `[code]` `cd go && go test ./internal/phases/ship -coverprofile=/tmp/ship-cycle410.out -count=1 && go tool cover -func=/tmp/ship-cycle410.out | awk '/total:/ { sub(/%/, "", $3); exit !($3 >= 55.0) }'`
## Negative Cases
- `[code]` `cd go && go test ./internal/phases/ship -run 'Test.*Missing.*CommitGate|Test.*Stale.*CommitGate|Test.*Malformed.*CommitGate' -count=1`
## Edge Cases
- `[code]` `cd go && go test ./internal/phases/ship -run 'Test.*DryRun|Test.*Bypass|Test.*Empty' -count=1`
## Thresholds
- All checks: pass@1 = 1.0
