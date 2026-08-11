# Eval: routingtest-engine-brick-tests

## Code Graders (bash commands that must exit 0)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/routingtest/... -count=1 -cover 2>&1 | tee /tmp/routingtest_cover.txt && grep "coverage:" /tmp/routingtest_cover.txt | awk -F'coverage: ' '{print $2}' | awk -F'%' '{if ($1+0 < 70) exit 1}'`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/routingtest/... -count=1 -run TestEngine -v 2>&1 | grep -E "PASS|FAIL" | grep -v "^---" | head -1 | grep PASS`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/routingtest/... -count=1 -run TestBrick -v 2>&1 | grep -E "--- PASS|--- FAIL" | grep -v "FAIL"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/routingtest/... -count=1 -run TestInvariant -v 2>&1 | grep -E "--- PASS|--- FAIL" | grep -v "FAIL"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/routingtest/... -count=1 -run TestBuildConfig_Defaults 2>&1 | grep -c PASS | grep -v "^0$"`

## Regression Evals (full test suite)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/routingtest/... ./internal/router/... -count=1 2>&1 | grep -E "^(ok|FAIL)" | grep -v "^FAIL"`

## Acceptance Checks

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/routingtest/... -count=1 -cover 2>&1 | grep "coverage:" | awk -F'coverage: ' '{cov=$2+0} END {if (cov < 70) {print "FAIL: coverage " cov "% < 70%"; exit 1} else print "PASS: " cov "%"}'`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go vet ./internal/routingtest/... 2>&1 | grep -c "^" | awk '{if ($1 > 0) exit 1}'`

## Thresholds

- All checks: pass@1 = 1.0
