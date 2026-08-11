# Eval: fix-inserted-phase-worktree-dispatch

## Code Graders (bash commands that must exit 0)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -count=1 -short -run TestInsertedPhaseWritableInheritsWorktree -v 2>&1 | grep -q "PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -count=1 -short -run TestAbortCleanupPreservesWorktreeDiff -v 2>&1 | grep -q "PASS"`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -count=1 -short 2>&1 | tail -3 | grep -q "ok"`

## Regression Evals (full test suite)

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./... -count=1 -short 2>&1 | grep -v "^?" | grep -v "^ok" | grep -c "FAIL" | grep -q "^0$"`

## Acceptance Checks

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && grep -q 'writes_source.*true\|WritesSource.*true\|writeCapable\|worktree.*inherit\|activeWorktree.*inserted' internal/core/phase_advisor.go || grep -q 'TestInsertedPhaseWritableInheritsWorktree' internal/core/orchestrator_inserted_worktree_test.go`
- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go build ./... 2>&1 | wc -l | grep -q "^0$"`

## Negative / Edge Cases

- `[code]` `cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -count=1 -short -run TestInsertedReadOnlyPhaseDoesNotGetWorktree -v 2>&1 | grep -q "PASS"`

## Thresholds

- All checks: pass@1 = 1.0
