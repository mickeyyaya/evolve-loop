# Eval: adversarial-fault-injection-suite

## Code Graders (bash commands that must exit 0)

- `[code]` `test -f /Users/danleemh/ai/claude/evolve-loop/go/internal/bridge/adversarial_faults_test.go`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -count=1 -short -run TestAdversarial -v 2>&1 | grep -q "PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -count=1 -short -run TestAdversarial -v 2>&1 | grep -c "=== RUN" | awk '{exit ($1 >= 18) ? 0 : 1}'`

## Regression Evals (full test suite)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./... -count=1 -short 2>&1 | grep -v "^?" | grep -v "^ok" | grep -c "FAIL" | grep -q "^0$"`

## Acceptance Checks — fault family coverage

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -count=1 -short -run TestAdversarialFault -v 2>&1 | grep -Ei "stall|crash|update.menu|weak.busy|empty.pane|malformed" | sort -u | wc -l | awk '{exit ($1 >= 6) ? 0 : 1}'`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -count=1 -short -run TestAdversarialFault -v 2>&1 | grep -Ei "claude|codex|agy" | sort -u | wc -l | awk '{exit ($1 >= 2) ? 0 : 1}'`

## Negative / Edge Cases

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -count=1 -short -run TestAdversarialFaultMatrix_RequiredFamiliesCovered -v 2>&1 | grep -q "PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/bridge/... -count=1 -short -run TestAdversarialFaultMatrix_RequiredFaultTypesPresent -v 2>&1 | grep -q "PASS"`

## Thresholds

- All checks: pass@1 = 1.0
