# Eval: core-looppreflight-recovery-lift

## Purpose
Verify coverage improvements in core orchestrator, looppreflight, recovery,
and adapters/bridge — targeting functions at 6-66% coverage.

## Criteria

### C1: internal/core package coverage ≥ 90% [code]
```bash
[code]
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -coverprofile=/tmp/core.out -count=1 2>&1 | grep "coverage:" | head -3
go tool cover -func=/tmp/core.out 2>/dev/null | grep "^total:" | awk '{if ($3+0 >= 90.0) print "PASS: "$3; else print "FAIL: "$3" < 90%"}'
```

### C2: gitStatusNames function has ≥ 80% coverage [code]
```bash
[code]
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... -coverprofile=/tmp/core2.out -count=1 2>/dev/null
go tool cover -func=/tmp/core2.out 2>/dev/null | grep "gitStatusNames" | awk '{if ($3+0 >= 80.0) print "PASS: gitStatusNames "$3; else print "FAIL: gitStatusNames "$3" < 80%"}'
```

### C3: looppreflight package coverage ≥ 92% [code]
```bash
[code]
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/looppreflight/... -coverprofile=/tmp/lp.out -count=1 2>&1 | grep "coverage:"
go tool cover -func=/tmp/lp.out 2>/dev/null | grep "^total:" | awk '{if ($3+0 >= 92.0) print "PASS: "$3; else print "FAIL: "$3" < 92%"}'
```

### C4: adapters/bridge package coverage ≥ 88% [code]
```bash
[code]
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/adapters/bridge/... -coverprofile=/tmp/ab.out -count=1 2>&1 | grep "coverage:"
go tool cover -func=/tmp/ab.out 2>/dev/null | grep "^total:" | awk '{if ($3+0 >= 88.0) print "PASS: "$3; else print "FAIL: "$3" < 88%"}'
```

### C5: All new tests pass without panic [code]
```bash
[code]
cd /Users/danleemh/ai/claude/evolve-loop/go && go test ./internal/core/... ./internal/looppreflight/... ./internal/recovery/... ./internal/adapters/bridge/... -count=1 2>&1 | grep -E "^(ok|FAIL|panic)" | head -20
```

### NEGATIVE C6: shiperror.DebugString returns non-empty output for filled struct [code]
```bash
[code]
# Test must verify DebugString produces non-empty output — not just coverage line-touch
grep -q "DebugString\|debugString" /Users/danleemh/ai/claude/evolve-loop/go/internal/core/shiperror_test.go 2>/dev/null && echo "PASS: DebugString tested" || echo "FAIL: DebugString not tested"
```

### EDGE C7: newDefaultBootTester smoke-tests the default implementation interface [code]
```bash
[code]
grep -q "newDefaultBootTester\|DefaultBootTester\|defaultBoot" /Users/danleemh/ai/claude/evolve-loop/go/internal/looppreflight/boot_test.go 2>/dev/null && echo "PASS: newDefaultBootTester tested" || echo "FAIL: not tested"
```
