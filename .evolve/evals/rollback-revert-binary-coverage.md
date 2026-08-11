# Eval: rollback-revert-binary-coverage

## Objective
Raise `revertAndShipWith` coverage from 54.5% to ≥80% by adding seam-injected tests for
the two untested paths:
1. Revert succeeds, evolve binary exists but exits non-zero → returns `"local-only"`
2. Revert succeeds, evolve binary exists and exits zero → returns `"reverted"`

Both use the existing `gitexec.Git{Exec: fake.Run}` seam plus a temporary shell-script
binary to exercise `exec.Command(binPath, ...)`.

## Criteria

### C1: `TestRevertAndShipWith_RevertOK_BinaryFails_LocalOnly` passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/rollback/... -run TestRevertAndShipWith_RevertOK_BinaryFails_LocalOnly -count=1 -v
```
Expected: `--- PASS: TestRevertAndShipWith_RevertOK_BinaryFails_LocalOnly`

### C2: `TestRevertAndShipWith_RevertOK_BinarySucceeds_Reverted` passes [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/rollback/... -run TestRevertAndShipWith_RevertOK_BinarySucceeds_Reverted -count=1 -v
```
Expected: `--- PASS: TestRevertAndShipWith_RevertOK_BinarySucceeds_Reverted`

### C3: `revertAndShipWith` coverage rises to ≥75% [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/rollback/... -count=1 -coverprofile=/tmp/c-rollback.out && go tool cover -func=/tmp/c-rollback.out | grep revertAndShipWith
```
Expected: the `revertAndShipWith` line shows ≥75%.

### C4: Full rollback suite passes (no regression) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/rollback/... -count=1 -short
```
Expected: `ok` with no `FAIL`.

### C5 (negative): The fake binary argument contract is enforced [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/rollback/... -run TestRevertAndShipWith -count=1 -v 2>&1 | grep -E "PASS|FAIL"
```
Expected: ALL `TestRevertAndShipWith*` tests show `PASS` (verifies no test is broken by the new fake-binary approach).
