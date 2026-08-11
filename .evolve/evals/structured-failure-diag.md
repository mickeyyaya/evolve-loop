# Eval: structured-failure-diag

<!-- challenge-token: bd89ec70f56cebf4 -->

## Task
When a mandatory phase aborts (error after all relaunch attempts), the orchestrator must write
`<workspace>/<phase>-failure-diag.json` before propagating the error.

## Acceptance Criteria

### AC-1: failure-diag write code present in orchestrator [code]
```bash
grep -q "failure-diag\|failurediag\|failureDiag\|FailureDiag" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go
# Should exit 0 — failure-diag write must be present in orchestrator
```

### AC-2: failure-diag JSON has required fields [code]
```bash
grep -n "error_message\|ErrorMessage\|exit_code\|ExitCode\|attempt_count\|AttemptCount" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go | head -5
# Should output matches — struct fields for failure diag must exist
```

### AC-3: Unit test for failure-diag write on phase abort [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run "TestRunCycle.*FailureDiag\|TestFailureDiag\|TestPhaseAbort.*Diag" -v 2>&1 | grep -E "PASS|RUN|FAIL"
# Should find and pass at least one failure-diag test
```

### AC-4: Negative — failure-diag NOT written on PASS [code]
```bash
# The build must not regress existing tests
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... 2>&1 | tail -5
# Should exit 0 — no regressions
```

### AC-5: Negative — exit_code correctly extracted from ErrArtifactTimeout [code]
```bash
# ErrArtifactTimeout is exit 81 — diag should reflect this
grep -q "81\|ArtifactTimeout\|ErrArtifactTimeout" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go
# Should exit 0 — exit code 81 referenced in context of failure diag
```
