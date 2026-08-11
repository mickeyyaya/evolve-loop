# Eval: c197-debugger-coverage-boost

## Goal
Boost `internal/phases/debugger` coverage from 43.3% to ≥75% by adding unit tests for
the currently-uncovered paths: `nextPhaseFor`, `hooks` trivial methods, and `Phase.Run`
with a mock bridge. These are pure or easily-mockable paths.

## Acceptance Criteria

### AC-1: Coverage improves to ≥75% [code]
```
cd go && go test -cover ./internal/phases/debugger/... 2>&1 | tail -3
```
Must report `coverage: 7[5-9]\.[0-9]+%` or higher (≥75.0%). Must exit 0.

### AC-2: New tests cover nextPhaseFor function [code]
```
grep -n "nextPhaseFor\|TestNextPhaseFor\|reship.*ship\|RESHIP.*ship" /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/debugger/debugger_test.go
```
Must produce at least one match — evidence that `nextPhaseFor` is exercised in tests.

### AC-3: New tests cover hooks methods [code]
```
grep -n "PhaseName\|AgentPromptName\|ArtifactFilename\|DefaultModel\|ComposePrompt\|hooks{}" /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/debugger/debugger_test.go
```
Must produce at least one match — evidence that at least one hooks method is tested.

### AC-4: All new tests pass [code]
```
cd go && go test -v ./internal/phases/debugger/... 2>&1 | grep -E "PASS|FAIL|---" | tail -20
```
Must show only PASS results for debugger tests; no FAIL lines.

## Negative Cases

### NEG-1: Missing artifact is always BLOCK (never RESHIP on parse failure) [code]
This negative case is already tested in `TestClassifyNeverReshipOnParseFailure`.
Verify it still passes after the new test additions:
```
cd go && go test -run TestClassifyNeverReshipOnParseFailure ./internal/phases/debugger/... 2>&1
```
Must exit 0 and print PASS.

### NEG-2: nextPhaseFor with non-RESHIP action returns empty string [code]
```
grep -n "nextPhaseFor.*BLOCK\|nextPhaseFor.*empty\|nextPhaseFor.*RERUN" /Users/danleemh/ai/claude/evolve-loop/go/internal/phases/debugger/debugger_test.go
```
Must produce at least one match showing the non-RESHIP path is tested.
