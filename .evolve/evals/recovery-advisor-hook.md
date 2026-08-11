# Eval: recovery-advisor-hook

## Objective
Wire the FailureAdvisor to the orchestrator composition root in SHADOW mode so that recovery decisions are evaluated (and logged) on every phase failure — closing the gap noted in ADR-0044 memory where "advisor NOT wired at composition root (post-soak step)."

## Acceptance Criteria

### Criterion 1: FailureAdvisor is called in orchestrator on phase failure [code]
```bash
grep -n "FailureAdvis\|Recover(\|recovery\.Recover\|RecoverInput" \
  /Users/danleemh/ai/claude/evolve-loop/go/internal/core/orchestrator.go | head -20
```
Expected: At least one call site where `recovery.Recover(...)` or equivalent is invoked within the phase-failure handling path (not just referenced in comments).

### Criterion 2: SHADOW mode does not mutate cycle state [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/recovery/... -v -run TestShadow 2>&1 | tail -10
```
Expected: PASS — shadow mode calls advisor, logs decision, but does NOT change the cycle stop_reason or retry count.

### Criterion 3: ENFORCE mode triggers retry on ErrTransientBridgeFailure [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/recovery/... -v -run TestEnforce_TransientBridge 2>&1 | tail -10
```
Expected: PASS — when dial=ENFORCE and error matches ErrTransientBridgeFailure, Decision.Action == RETRY.

### Criterion 4: Recovery decisions are persisted to run log [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/... -v -run TestOrchestrator_RecoveryHookLogged 2>&1 | tail -10
```
Expected: PASS — a phase failure during an orchestrator unit test produces a recovery-decision log entry (advisory or action) in the workspace.

### Criterion 5 (negative): No recovery action fires in SHADOW mode on non-transient errors [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/recovery/... -v -run TestShadow_NoActionOnFatal 2>&1 | tail -5
```
Expected: PASS — fatal/unclassified errors in shadow mode produce a Decision with Action=="observe", not "retry" or "rollback".

### Criterion 6: Full go test suite still green [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/core/... ./internal/recovery/... 2>&1 | grep -E "ok|FAIL"
```
Expected: All `ok` lines, no FAIL.
