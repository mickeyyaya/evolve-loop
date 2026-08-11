# Eval: rollback-revert-ship-coverage

## Task
Add tests for the two uncovered branches of `revertAndShipWith` in `go/internal/rollback`: (a) binary present but ship command fails → "local-only", (b) binary present and ship succeeds → "reverted".

## Acceptance Criteria

### AC-1: `revertAndShipWith` returns "local-only" when ship binary exits non-zero [code]
```bash
cd go && go test -run TestRevertAndShipWith_BinaryFailsShip_LocalOnly ./internal/rollback/... -v 2>&1 | grep -q "PASS"
```

### AC-2: `revertAndShipWith` returns "reverted" when ship binary exits zero [code]
```bash
cd go && go test -run TestRevertAndShipWith_BinarySucceeds_Reverted ./internal/rollback/... -v 2>&1 | grep -q "PASS"
```

### AC-3: Coverage of `revertAndShipWith` reaches 100% [code]
```bash
cd go && go test -coverprofile=/tmp/rb343-eval.out -coverpkg=./internal/rollback/... ./internal/rollback/... 2>&1 && go tool cover -func=/tmp/rb343-eval.out | grep "revertAndShipWith" | awk '{if ($3+0 >= 90.0) print "OK"; else print "FAIL: " $0}'
```

### AC-4: Overall rollback coverage lifts from 86.8% to ≥89% [code]
```bash
cd go && go test -cover ./internal/rollback/... 2>&1 | grep -E "coverage: [0-9.]+" | awk -F'[: %]' '{if ($3+0 >= 89.0) print "OK"; else print "FAIL coverage too low: " $0}'
```

### AC-NEG: All existing rollback tests still pass [code]
```bash
cd go && go test ./internal/rollback/... 2>&1 | tail -1 | grep -q "^ok"
```

### AC-EDGE: Negative case — binary present but revert still fails → "failed" (existing, must not regress) [code]
```bash
cd go && go test -run TestRevertAndShipWith_RevertFails_ReturnsFailed ./internal/rollback/... -v 2>&1 | grep -q "PASS"
```
