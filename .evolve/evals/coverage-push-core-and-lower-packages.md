# Eval: coverage-push-core-and-lower-packages

## Code Graders (bash commands that must exit 0)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./... -count=1 -short 2>&1 | grep -v "^?" | grep -v "^ok" | grep -c "FAIL" | grep -q "^0$"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./... -count=1 -short -coverprofile=/tmp/cycle281_cov.out 2>&1 | tail -1 && go tool cover -func=/tmp/cycle281_cov.out | grep "^total:" | awk '{split($NF, a, "%"); exit (a[1]+0 >= 93) ? 0 : 1}'`

## Package-level gates

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -count=1 -short -coverprofile=/tmp/core281.out && go tool cover -func=/tmp/core281.out | grep "^total:" | awk '{split($NF, a, "%"); exit (a[1]+0 >= 90) ? 0 : 1}'`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/routingtest/... -count=1 -short -coverprofile=/tmp/rt281.out && go tool cover -func=/tmp/rt281.out | grep "^total:" | awk '{split($NF, a, "%"); exit (a[1]+0 >= 90) ? 0 : 1}'`

## Regression Evals (full test suite)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./... -count=1 -short 2>&1 | grep "^ok" | wc -l | awk '{exit ($1 >= 110) ? 0 : 1}'`

## Acceptance Checks — intent-probing (not surface-line padding)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -count=1 -short -run TestFailureAdvisor -v 2>&1 | grep -q "PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -count=1 -short -run TestCorrectionLadder -v 2>&1 | grep -q "PASS"`

## Negative / Edge Cases

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -count=1 -short -run "TestFailureAdvisor.*Nil\|TestFailureAdvisor.*Empty\|TestCorrectionLadder.*Exhaust" -v 2>&1 | grep -c "=== RUN" | awk '{exit ($1 >= 2) ? 0 : 1}'`

## Thresholds

- All checks: pass@1 = 1.0
