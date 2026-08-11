# Eval: ship-test-parallel-isolation

## Code Graders (bash commands that must exit 0)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test -count=1 ./internal/phases/ship/... 2>&1 | grep "^ok" | awk '{gsub(/[()s]/,""); n=split($NF, a, "."); t=a[1]+0; if (t > 15) {print "FAIL: " $0 " exceeded 15s ceiling"; exit 1}}'`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test -count=1 ./internal/phases/ship/... 2>&1 | grep -E "^(ok|FAIL)" | grep -v "^FAIL"`
- `[code]` `grep -rn "t.Parallel()" /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/ship/closure_idempotency_test.go && echo "t.Parallel present"`

## Regression Evals (full test suite)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test -count=1 -race ./internal/phases/ship/... 2>&1 | grep -E "^(ok|FAIL)" | grep -v "^FAIL"`

## Acceptance Checks

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test -count=1 ./internal/phases/ship/... 2>&1 | grep "^ok" | awk '{gsub(/[()]/,""); t=0; for(i=NF;i>0;i--) if($i ~ /s$/) {t=$i+0; break}; if (t > 15) {print "SLOW: " $0; exit 1}}' && echo "ship tests under 15s ceiling"`

## Thresholds

- All checks: pass@1 = 1.0
