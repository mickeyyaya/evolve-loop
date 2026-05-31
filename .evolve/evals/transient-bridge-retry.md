# Eval: transient-bridge-retry

## Metadata
- slug: transient-bridge-retry
- cycle: 173
- task: Extend phase retry to transient non-ArtifactTimeout bridge errors (exit 80/85/86) + non-canonical verdicts (GAP 1 + GAP 5)

## Acceptance Criteria

### AC-1: isTransientBridgeError helper exists and classifies exit codes correctly [code]
Exit 80 (REPL boot timeout), 85 (unknown prompt), 86 (respond-loop guard) must return true.
Exit 2, 3, 10, 99, 127 must return false. ErrArtifactTimeout (exit 81) must NOT classify as transient.
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestIsTransientBridgeError -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: `--- PASS: TestIsTransientBridgeError` (no "no tests to run" line)

### AC-2: Exit code 80 triggers retry instead of hard-abort [code]
A phase returning exit 80 on attempt 1 must be retried; if attempt 2 succeeds, RunCycle continues.
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestOrchestrator_RetryOnTransientExit -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: `--- PASS: TestOrchestrator_RetryOnTransientExit` (no "no tests to run" line)

### AC-3: Non-canonical verdict triggers retry within phaseMaxAttempts (GAP 5) [code]
When runner returns err=nil but verdict not in {PASS,FAIL,WARN,SKIPPED} on attempt 1, orchestrator retries.
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestOrchestrator_NonCanonicalVerdictRetry -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: `--- PASS: TestOrchestrator_NonCanonicalVerdictRetry` (no "no tests to run" line)

### AC-4: Non-transient errors hard-abort without retry (negative case) [code]
Exit 2 (safety-gate) and 127 (missing binary) must NOT be retried; scout must run only 1 time.
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestOrchestrator_NonTransientError_NoRetry -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: `--- PASS: TestOrchestrator_NonTransientError_NoRetry` (no "no tests to run" line)

### AC-5: Exhausted retries (all transient) falls through to abort with failure-diag [code]
If both attempts return transient errors, the cycle aborts and failure-diag.json is written.
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestTransientRetry_Exhausted_WritesFailureDiag -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: `--- PASS: TestTransientRetry_Exhausted_WritesFailureDiag` (no "no tests to run" line)

### AC-6: FAIL verdict is never retried as transient [code]
A phase that returns err=nil + VerdictFAIL must not be retried; FAIL routes through normal path.
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestFAILVerdict_NotRetried -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: `--- PASS: TestFAILVerdict_NotRetried` (no "no tests to run" line)

### AC-7: All existing core tests still pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -timeout 90s -count=1 2>&1 | grep -E "^ok |^FAIL "
```
Expected: lines starting with `ok` only (no FAIL lines)

### AC-8: self-healing-gaps.md marks GAP 1 and GAP 5 as DONE [code]
```bash
grep -c "DONE" /Users/danleemh/ai/claude/evolve-loop/docs/architecture/self-healing-gaps.md
```
Expected: at least 3 (GAP 9 was already DONE; GAP 1 and GAP 5 added this cycle)

### AC-9: ErrTransientBridgeFailure sentinel exists in core/errors.go [code]
```bash
grep -c "ErrTransientBridgeFailure" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/errors.go
```
Expected: at least 1

## Negative Cases

### NC-1: ErrArtifactTimeout (exit 81) self-heal still works after the change [code]
The existing ArtifactTimeout retry path must still work; not broken by the new transient path.
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestOrchestrator_PhaseArtifactTimeout_RetriesAndRecovers -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL"
```
Expected: `--- PASS: TestOrchestrator_PhaseArtifactTimeout_RetriesAndRecovers`

### NC-2: Transient exit code appears accurately in phase_retry ledger entry [code]
The ledger entry for a transient retry must record the actual exit code (not hardcoded 81).
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -run TestTransientRetry_LedgerEntry -count=1 -timeout 30s -v 2>&1 | grep -E "--- PASS|--- FAIL|\[no tests to run\]"
```
Expected: `--- PASS: TestTransientRetry_LedgerEntry` (no "no tests to run" line)
