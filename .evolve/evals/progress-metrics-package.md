# Eval: progress-metrics-package

## Task
Add a progressmetrics package that computes rolling-window cycle-outcome metrics (PASS rate, task completion rate, repetition detection) from ledger.jsonl, plus a CLI command.

## Criteria

### C1: Package tests pass [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/progressmetrics/... -v -timeout 30s 2>&1 | \
  grep -q "PASS" && echo "PASS" || echo "FAIL"
```

### C2: CLI command exits 0 and emits valid JSON [code]
```bash
/Users/danleemh/ai/claude/evolve-loop/go/evolve progress \
  --evolve-dir /Users/danleemh/ai/claude/evolve-loop/.evolve \
  --cycles 10 2>&1 | python3 -c "import json,sys; d=json.load(sys.stdin); print('PASS' if 'pass_rate' in d else 'FAIL - missing pass_rate')"
```

### C3: PASS rate computed correctly from fixture [code]
```bash
# Unit test verifying: 7 PASS + 3 FAIL in ledger → pass_rate=0.7
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/progressmetrics/... -run TestPassRateFromLedger -v -timeout 30s 2>&1 | \
  grep -q "PASS" && echo "PASS" || echo "FAIL"
```

### C4: Repetition detection flags stuck pattern [code]
```bash
# Unit test: 5 consecutive FAIL cycles → repetition_detected=true
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/progressmetrics/... -run TestRepetitionDetection -v -timeout 30s 2>&1 | \
  grep -q "PASS" && echo "PASS" || echo "FAIL"
```

### C5: Negative case — healthy pipeline (all PASS) → repetition_detected=false [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/progressmetrics/... -run TestNoRepetitionWhenAllPass -v -timeout 30s 2>&1 | \
  grep -q "PASS" && echo "PASS" || echo "FAIL"
```

### C6: Edge case — empty ledger emits empty-safe metrics (no panic) [code]
```bash
cd /Users/danleemh/ai/claude/evolve-loop/go && \
  go test ./internal/progressmetrics/... -run TestEmptyLedger -v -timeout 30s 2>&1 | \
  grep -q "PASS" && echo "PASS" || echo "FAIL"
```
